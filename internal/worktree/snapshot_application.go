package worktree

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/hasher"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
)

// plannedFile represents everything needed to materialize a file from
// target snapshot to disk.
type plannedFile struct {
	path          string
	hash          string
	mode          objects.FileMode
	content       []byte
	permissions   os.FileMode
	writeRequired bool
}

// applicationPlan contains the file paths from the existing snapshot
// that need to be removed
// as well as the list of planned files to be materialized in the disk.
// It's a utlity struct that allows comparisons for the resolution
// of what file system elements require to be updated.
type applicationPlan struct {
	pathsToRemove      []string
	targetPlannedFiles []plannedFile
}

// buildApplicationPlan validates the original and target snapshots and
// preloads every target blob before filesystem mutation begins.
//
// It compares each target entry with the index and disk to retain matching files, mark
// missing or changed files for writing, and schedule obsolete or structurally
// conflicting tracked paths for removal. Target files are planned in logical
// path order so later materialization and index rebuilding are deterministic.
//
// Callers must inspect worktree collisions before calling ApplySnapshot and
// must not proceed when collisions are present.
func buildApplicationPlan(repoPath string, store *objects.ObjectStore, idx *index.Index, original, target objects.TreeSnapshot) (*applicationPlan, error) {
	if err := validateWorktreeSnapshot(original); err != nil {
		return nil, fmt.Errorf("invalid original snapshot: %w", err)
	}
	if err := validateWorktreeSnapshot(target); err != nil {
		return nil, fmt.Errorf("invalid target snapshot: %w", err)
	}

	removalSet := make(map[string]struct{}, len(original))

	// if an origin path is not present in target mark it for removal
	for originalPath := range original {
		if _, exists := target[originalPath]; !exists {
			removalSet[originalPath] = struct{}{}
		}
	}

	targetSnapshotPaths := make([]string, 0, len(target))
	for targetPath := range target {
		targetSnapshotPaths = append(targetSnapshotPaths, targetPath)
	}
	// Sorting ensured deterministic result
	slices.Sort(targetSnapshotPaths)

	targetPlannedFiles := make([]plannedFile, 0, len(target))
	for _, targetPath := range targetSnapshotPaths {
		targetEntry := target[targetPath]
		plannedTargetFile, removePath, err := resolvePlanForTargetPath(repoPath, targetPath, targetEntry, store, idx.GetEntry(targetPath))
		if err != nil {
			return nil, err
		}
		if removePath {
			removalSet[targetPath] = struct{}{}
		}
		targetPlannedFiles = append(targetPlannedFiles, plannedTargetFile)
	}

	pathsToRemove := make([]string, 0, len(removalSet))
	for removalPath := range removalSet {
		pathsToRemove = append(pathsToRemove, removalPath)
	}
	slices.Sort(pathsToRemove)

	return &applicationPlan{pathsToRemove: pathsToRemove, targetPlannedFiles: targetPlannedFiles}, nil
}

// resolvePlanForTargetPath preloads the blob and filesystem permissions for
// one target snapshot entry, then compares that entry with the index and disk.
// It returns the complete target file plan and reports whether an existing
// directory at the target path must be removed before materialization.
func resolvePlanForTargetPath(repoPath, targetPath string, targetEntry objects.SnapshotEntry, store *objects.ObjectStore, idxEntry *index.Entry) (plannedFile, bool, error) {
	blob, err := store.ReadBlob(targetEntry.Hash)
	if err != nil {
		return plannedFile{}, false, fmt.Errorf("failed to read blob object for target snapshot path [%s]: %w", targetPath, err)
	}

	permissions, err := targetEntry.Mode.ToOsFileMOde()
	if err != nil {
		return plannedFile{}, false, fmt.Errorf("failed to convert mode [%s]: %w", targetEntry.Mode, err)
	}

	writeRequired, removeTargetPath, err := inspectTargetMaterialization(repoPath, targetPath, targetEntry, idxEntry)
	if err != nil {
		return plannedFile{}, false, err
	}

	return plannedFile{
		path:          targetPath,
		hash:          targetEntry.Hash,
		mode:          targetEntry.Mode,
		content:       bytes.Clone(blob.Content()),
		permissions:   permissions,
		writeRequired: writeRequired,
	}, removeTargetPath, nil
}

// inspectTargetMaterialization determines whether targetPath must be written
// and whether its current filesystem object must first be removed.
//
// Rules:
//   - missing path: write the target
//   - directory: remove the directory, then write the target
//   - unsupported filesystem object: return error
//   - matching file: retain it
//   - content or mode mismatch: don't remove object, write the target to update it.
func inspectTargetMaterialization(repoPath, targetPath string, targetEntry objects.SnapshotEntry, indexEntry *index.Entry) (writeRequired bool, removeTargetPath bool, err error) {
	osTargetPath, err := filepath.Localize(targetPath)
	if err != nil {
		return false, false, fmt.Errorf("failed to convert target path [%s] to local os format: %w", targetPath, err)
	}

	// find target path in disk
	osPath := filepath.Join(repoPath, osTargetPath)
	fileInfo, err := os.Lstat(osPath)
	if err != nil {
		// if we are on unix os system, in stat_unix.go if a parent file of the path exists as file the whole path is not reported as ErrNotExist but as ENOTDIR, element not a directory
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return true, false, nil
		}
		return false, false, fmt.Errorf("failed to inspect target path [%s]: %w", targetPath, err)
	}

	if fileInfo.IsDir() {
		return true, true, nil
	}
	if !fileInfo.Mode().IsRegular() {
		return false, false, fmt.Errorf("unsupported filesystem object at target path [%s]: %s", targetPath, fileInfo.Mode())
	}

	// check target entry against index
	targetIndexMode, err := index.FromObjectFileMode(targetEntry.Mode)
	if err != nil {
		return false, false, err
	}

	modeMatches := targetIndexMode == index.DetectFileMode(fileInfo)
	if doesTargetEntryMatchIndexEntry(indexEntry, fileInfo, targetEntry, targetIndexMode) {
		return !modeMatches, false, nil
	}

	// check target entry against disk
	diskContent, err := os.ReadFile(osPath)
	if err != nil {
		return false, false, fmt.Errorf("failed to read file at target path [%s]: %w", targetPath, err)
	}

	hash, err := hasher.ComputeHash(diskContent, hasher.Blob)
	if err != nil {
		return false, false, fmt.Errorf("failed to compute hash for target path [%s]: %w", targetPath, err)
	}
	contentMatches := hash == targetEntry.Hash

	return !contentMatches || !modeMatches, false, nil
}

// doesTargetEntryMatchIndexEntry reports whether index and filesystem
// metadata are sufficient to trust that disk content matches the target blob.
func doesTargetEntryMatchIndexEntry(indexEntry *index.Entry, fileInfo os.FileInfo, targetEntry objects.SnapshotEntry, targetIndexMode index.FileMode) bool {
	return indexEntry != nil &&
		indexEntry.Hash() == targetEntry.Hash &&
		indexEntry.Mode() == targetIndexMode &&
		fileInfo.Size() == indexEntry.FileSize() &&
		fileInfo.ModTime().Truncate(time.Second).Equal(indexEntry.LastModified().Truncate(time.Second))
}

// removeWorkTreePaths removes planned filesystem objects deepest-first so
// explicitly listed descendants are removed before their parent directories.
// It sorts a clone to preserve the deterministic order stored in the plan.
func (plan *applicationPlan) removeWorkTreePaths(repoPath string) error {
	removalPaths := slices.Clone(plan.pathsToRemove)
	sortLogicalPathsDepthFirst(removalPaths)
	return RemovePaths(repoPath, removalPaths)
}

// materializePlannedFiles writes each target file that requires materialization
// and builds a replacement index from the final metadata of every target file,
// including retained files. It returns the index without persisting it.
func (plan *applicationPlan) materializePlannedFiles(repoPath string) (*index.Index, error) {
	idx := index.NewIndex()

	for _, file := range plan.targetPlannedFiles {
		localPath, err := filepath.Localize(file.path)
		if err != nil {
			return nil, fmt.Errorf("failed to convert path [%s] to local os format: %w", file.path, err)
		}
		absPath := filepath.Join(repoPath, localPath)

		if file.writeRequired {
			if err := writeFileAndParentDirs(absPath, file.content, file.permissions); err != nil {
				return nil, err
			}
		}

		if err := createAndAddIndexEntry(absPath, file, idx); err != nil {
			return nil, err
		}
	}
	return idx, nil
}

// writeFileAndParentDirs creates the required parent directories, writes the
// target content, and explicitly applies the target permissions to both new
// and existing files.
func writeFileAndParentDirs(abspath string, fileContent []byte, filePermissions os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(abspath), constants.DirPerms); err != nil {
		return fmt.Errorf("failed to create directory [%s]: %w", filepath.Dir(abspath), err)
	}
	if err := os.WriteFile(abspath, fileContent, filePermissions); err != nil {
		return fmt.Errorf("failed to write file [%s] with permissions [%o] in the disk: %w", abspath, filePermissions, err)
	}
	if err := os.Chmod(abspath, filePermissions); err != nil {
		return fmt.Errorf("failed to change permissions for file [%s]: %w", abspath, err)
	}
	return nil
}

// createAndAddIndexEntry reads the materialized file metadata, creates the
// corresponding target index entry, and adds it to idx.
func createAndAddIndexEntry(abspath string, file plannedFile, idx *index.Index) error {
	fileInfo, err := os.Stat(abspath)
	if err != nil {
		return fmt.Errorf("failed to stat file at absolute path [%s]: %w", abspath, err)
	}

	indexMode, err := index.FromObjectFileMode(file.mode)
	if err != nil {
		return fmt.Errorf("failed to convert object file mode: %w", err)
	}

	entry, err := index.NewEntry(indexMode, file.hash, file.path, fileInfo.Size(), fileInfo.ModTime().Truncate(time.Second))
	if err != nil {
		return fmt.Errorf("failed to create index entry for [%s]: %w", file.path, err)
	}
	if err := idx.AddEntry(entry); err != nil {
		return fmt.Errorf("failed to add [%s] entry to index: %w", file.path, err)
	}
	return nil
}

// ApplySnapshot updates the worktree and index from original to target.
// It validates and plans the complete transition before mutation, removes
// obsolete paths, materializes every target file, and persists the rebuilt
// index only after all filesystem operations succeed.
//
// Planning failures leave both the worktree and persisted index unchanged.
// Failures after mutation begins may leave a partially updated worktree, but
// the persisted index is not replaced unless materialization completes.
func ApplySnapshot(repoPath string, store *objects.ObjectStore, currentIndex *index.Index, original, target objects.TreeSnapshot) error {
	plan, err := buildApplicationPlan(repoPath, store, currentIndex, original, target)
	if err != nil {
		return fmt.Errorf("failed to build application plan: %w", err)
	}

	if err := plan.removeWorkTreePaths(repoPath); err != nil {
		return fmt.Errorf("failed to remove obsolete worktree paths: %w", err)
	}

	updatedIndex, err := plan.materializePlannedFiles(repoPath)
	if err != nil {
		return fmt.Errorf("failed to materialize target snapshot: %w", err)
	}

	indexManager := index.NewManager(repoPath)
	if err := indexManager.Save(updatedIndex); err != nil {
		return fmt.Errorf("failed to save updated index: %w", err)
	}

	return nil
}
