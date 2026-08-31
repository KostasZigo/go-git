// Package staging reconciles selected working-tree paths with the gogit index.
// It stages regular-file additions and modifications, records tracked
// deletions, resolves file/directory transitions, stores blobs, and persists
// the updated index only after the complete operation succeeds.
package staging

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/filesystem"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
)

// resolveFilePaths returns a sorted, unique set of canonical repository paths
// selected by args. Explicit arguments select only those paths. A sole "."
// combines every regular file found in the repository with every indexed path:
// working-tree paths expose additions and modifications, while indexed paths
// keep deletions and directory-to-file transitions visible after files vanish.
func resolveFilePaths(repoPath string, args []string, idx *index.Index) ([]string, error) {
	filePaths := make([]string, 0, len(args)+idx.CountEntries())
	if len(args) == 1 && args[0] == "." {
		collectedPaths, err := collectAllRepoFiles(repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to collect repository files: %w", err)
		}
		filePaths = append(filePaths, collectedPaths...)
		for _, entry := range idx.GetEntryList() {
			filePaths = append(filePaths, entry.Path())
		}
	} else {
		filePaths = make([]string, 0, len(args))
		for _, arg := range args {
			logicalPath, err := repositoryRelativePath(repoPath, arg)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve path [%s]: %w", arg, err)
			}
			filePaths = append(filePaths, logicalPath)
		}
	}

	slices.Sort(filePaths)
	return slices.Compact(filePaths), nil
}

// repositoryRelativePath converts a working-directory path into a canonical
// repository-relative path.
func repositoryRelativePath(repoPath, candidatePath string) (string, error) {
	absoluteRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve repository root: %w", err)
	}
	absoluteCandidatePath, err := filepath.Abs(candidatePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	relativePath, err := filepath.Rel(absoluteRepoPath, absoluteCandidatePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve repository-relative path: %w", err)
	}
	logicalPath := filepath.ToSlash(relativePath)
	if err := index.ValidatePath(logicalPath); err != nil {
		return "", fmt.Errorf("invalid repository path [%s]: %w", logicalPath, err)
	}
	return logicalPath, nil
}

// collectAllRepoFiles walks the complete repository and returns regular files
// as canonical repository-relative paths. It excludes repository metadata and
// hidden directories and rejects unsupported leaf objects instead of silently
// omitting or following them.
func collectAllRepoFiles(repoPath string) ([]string, error) {
	var filePaths []string
	goGitDir := filepath.Join(repoPath, constants.Gogit)

	err := filepath.WalkDir(repoPath, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to access path %s: %w", path, err)
		}

		// Skip .gogit directory entirely
		if dirEntry.IsDir() && path == goGitDir {
			return filepath.SkipDir
		}

		// Skip hidden directories (starting with .)
		if dirEntry.IsDir() && filepath.Base(path)[0] == '.' && path != repoPath {
			return filepath.SkipDir
		}

		if dirEntry.IsDir() {
			return nil
		}

		if !dirEntry.Type().IsRegular() {
			fileInfo, err := dirEntry.Info()
			if err != nil {
				return fmt.Errorf("failed to inspect path %s: %w", path, err)
			}
			return validateSupportedFileSystemObject(path, fileInfo)
		}

		relPath, err := repositoryRelativePath(repoPath, path)
		if err != nil {
			return fmt.Errorf("failed to normalize repository path %s: %w", path, err)
		}
		filePaths = append(filePaths, relPath)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk repository: %w", err)
	}

	return filePaths, nil
}

// validateSupportedFileSystemObject permits regular files and directories used
// during traversal. Symlinks and other special objects are not stageable.
func validateSupportedFileSystemObject(filePath string, fileInfo fs.FileInfo) error {
	if fileInfo.IsDir() || fileInfo.Mode().IsRegular() {
		return nil
	}
	return fmt.Errorf("unsupported filesystem object [%s] with mode [%s]", filePath, fileInfo.Mode())
}

// inspectPath examines a canonical repository path without following
// symlinks. Missing paths include ENOTDIR, which occurs on Unix when an indexed
// descendant is now blocked by a regular-file ancestor. Other errors are
// returned so orchestration can abort before persisting its in-memory changes.
func inspectPath(repoPath, filePath string) (fs.FileInfo, bool, error) {
	localPath, err := filepath.Localize(filePath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to localize path [%s], %w", filePath, err)
	}
	absPath := filepath.Join(repoPath, localPath)

	fileInfo, err := os.Lstat(absPath)
	if err != nil {
		if filesystem.IsPathMissing(err) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("failed to inspect file [%s]: %w", filePath, err)
	}
	if err := validateSupportedFileSystemObject(filePath, fileInfo); err != nil {
		return nil, false, err
	}

	return fileInfo, false, nil
}

// removeIndexPathConflicts enforces the index invariant that a path cannot
// coexist with a tracked ancestor or descendant. A descendant file replaces a
// tracked ancestor file when that ancestor became a directory; a parent file
// replaces all tracked descendants when their directory became a file. The
// returned paths are sorted for deterministic deletion reporting.
func removeIndexPathConflicts(idx *index.Index, filePath string) []string {
	conflictingPaths := make([]string, 0)
	for ancestorPath := path.Dir(filePath); ancestorPath != "."; ancestorPath = path.Dir(ancestorPath) {
		if idx.GetEntry(ancestorPath) != nil {
			conflictingPaths = append(conflictingPaths, ancestorPath)
		}
	}

	descendantPrefix := filePath + "/"
	for _, entry := range idx.GetEntryList() {
		if strings.HasPrefix(entry.Path(), descendantPrefix) {
			conflictingPaths = append(conflictingPaths, entry.Path())
		}
	}

	slices.Sort(conflictingPaths)
	conflictingPaths = slices.Compact(conflictingPaths)
	idx.RemoveEntries(conflictingPaths...)
	return conflictingPaths
}

// addFile stages one regular file by storing its blob and replacing its index
// entry. The caller must first reconcile structural path conflicts. It returns
// the canonical path when content or mode changed, or an empty string when the
// existing index entry already matches.
func addFile(repoPath, filePath string, idx *index.Index, store *objects.ObjectStore) (string, error) {
	if err := index.ValidatePath(filePath); err != nil {
		return "", fmt.Errorf("invalid path [%s]: %w", filePath, err)
	}
	localPath, err := filepath.Localize(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to localize path [%s]: %w", filePath, err)
	}
	absolutePath := filepath.Join(repoPath, localPath)

	// Verify file exists
	fileInfo, err := os.Lstat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file %s: %w", absolutePath, err)
	}
	if fileInfo.IsDir() {
		return "", fmt.Errorf("cannot add directory directly; specify files or use '.' for repository-wide staging")
	}
	if err := validateSupportedFileSystemObject(filePath, fileInfo); err != nil {
		return "", err
	}

	// Create blob from file
	blob, err := objects.NewBlobFromFile(absolutePath)
	if err != nil {
		return "", fmt.Errorf("failed to create blob from file %s: %w", absolutePath, err)
	}

	// Determine file mode
	fileMode := index.DetectFileMode(fileInfo)

	// Skip unchanged files: if an index entry already exists with the same
	// content hash and mode, the file has not been modified.
	if existing := idx.GetEntry(filePath); existing != nil &&
		existing.Hash() == blob.Hash() &&
		existing.Mode() == fileMode {
		return "", nil
	}

	// Store Blob in objects/
	if err := store.Store(blob); err != nil {
		return "", fmt.Errorf("failed to store file blob: %w", err)
	}

	// create index entry
	entry, err := index.NewEntry(
		fileMode,
		blob.Hash(),
		filePath,
		fileInfo.Size(),
		fileInfo.ModTime().Truncate(time.Second),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create index entry for %s: %w", absolutePath, err)
	}

	// Add to index
	if err := idx.AddEntry(entry); err != nil {
		return "", fmt.Errorf("failed to add [%s] entry to index: %w", absolutePath, err)
	}

	return filePath, nil
}

// reconcilePaths mutates idx to match the sorted candidate paths and returns
// deterministic additions/modifications and deletions. Missing tracked paths
// stage deletions, directories replace exact tracked file entries, and regular
// files remove conflicting indexed ancestors or descendants before staging.
//
// A directory-to-file transition removes all indexed descendants at once, but
// those descendants remain in filePaths because candidates were resolved from
// the pre-reconciliation index. structurallyRemovedPaths records them so they
// are not subsequently treated as independently missing paths.
func reconcilePaths(repoPath string, filePaths []string, idx *index.Index, store *objects.ObjectStore) ([]string, []string, error) {
	addedFiles := make([]string, 0, len(filePaths))
	deletedFiles := make([]string, 0, idx.CountEntries())
	structurallyRemovedPaths := make(map[string]struct{})
	for _, filePath := range filePaths {
		if _, removed := structurallyRemovedPaths[filePath]; removed {
			continue
		}

		fileInfo, deleted, err := inspectPath(repoPath, filePath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to inspect path [%s]: %w", filePath, err)
		}
		switch {
		case deleted:
			if idx.GetEntry(filePath) != nil {
				idx.RemoveEntries(filePath)
				deletedFiles = append(deletedFiles, filePath)
				continue
			}
		case fileInfo.IsDir():
			if idx.GetEntry(filePath) != nil {
				idx.RemoveEntries(filePath)
				deletedFiles = append(deletedFiles, filePath)
				continue
			}
		default:
			conflictingPaths := removeIndexPathConflicts(idx, filePath)
			deletedFiles = append(deletedFiles, conflictingPaths...)
			for _, conflictingPath := range conflictingPaths {
				structurallyRemovedPaths[conflictingPath] = struct{}{}
			}
		}

		added, err := addFile(repoPath, filePath, idx, store)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to add file %s: %w", filePath, err)
		}
		if added != "" {
			addedFiles = append(addedFiles, added)
		}
	}
	slices.Sort(deletedFiles)
	return addedFiles, deletedFiles, nil
}

// OrchestrateAddExecution coordinates staging as one persistence operation. It
// loads the index once, resolves explicit or repository-wide candidate paths,
// reconciles them in memory, and saves the index only after every path succeeds.
// Inspection or staging errors therefore leave the persisted index unchanged,
// although blobs stored before a later failure may remain unreferenced.
func OrchestrateAddExecution(repoPath string, args []string) ([]string, []string, error) {
	indexManager := index.NewManager(repoPath)
	idx, err := indexManager.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load index: %w", err)
	}

	filePaths, err := resolveFilePaths(repoPath, args, idx)
	if err != nil {
		return nil, nil, err
	}

	store := objects.NewObjectStore(repoPath)
	addedFiles, deletedFiles, err := reconcilePaths(repoPath, filePaths, idx, store)
	if err != nil {
		return nil, nil, err
	}

	// Persist only after the complete candidate set has been reconciled.
	if err := indexManager.Save(idx); err != nil {
		return nil, nil, fmt.Errorf("failed to save index: %w", err)
	}

	return addedFiles, deletedFiles, nil
}
