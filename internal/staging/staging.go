// Package staging orchestrates the addition of working directory files into
// the gogit index (staging area). It handles file resolution, blob creation,
// content-change detection, and index persistence.
package staging

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
)

// resolveFilePaths determines which files to stage based on the provided args.
// If args is ["."], all trackable files in the repository are collected recursively.
// Otherwise, the explicit argument list is returned as-is.
func resolveFilePaths(repoPath string, args []string) ([]string, error) {
	// Check if all files should be added
	if len(args) == 1 && args[0] == "." {
		filePaths, err := collectAllRepoFiles(repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to collect repository files: %w", err)
		}
		return filePaths, nil
	}
	// Individual file arguments
	return slices.Clone(args), nil
}

// collectAllRepoFiles recursively walks repository collecting non-ignored files.
// Returns relative paths from repository root suitable for staging.
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

		// Collect regular files only
		if dirEntry.Type().IsRegular() {
			relPath, err := filepath.Rel(repoPath, path)
			if err != nil {
				return fmt.Errorf("failed to compute relative path for %s: %w", path, err)
			}
			filePaths = append(filePaths, relPath)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk repository: %w", err)
	}

	return filePaths, nil
}

// addFile stages a single file by creating a blob and updating the index.
// Returns the normalized relative path of the staged file, or an empty string
// if the file was unchanged and skipped.
func addFile(repoPath, filePath string, idx *index.Index, store *objects.ObjectStore) (string, error) {
	// Get absolute path
	absolutePath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Verify file exists
	fileInfo, err := os.Stat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file %s: %w", absolutePath, err)
	}
	if fileInfo.IsDir() {
		return "", fmt.Errorf("cannot add directory (not yet implemented)")
	}

	// Compute relative path from repository root
	absoluteRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute repo path: %w", err)
	}

	relativeFilePath, err := filepath.Rel(absoluteRepoPath, absolutePath)
	if err != nil {
		return "", fmt.Errorf("file is not inside the repository: %w", err)
	}

	// Normalize to forward slashes so index and tree entries are OS-independent.
	relativeFilePath = filepath.ToSlash(relativeFilePath)

	// Create blob from file
	blob, err := objects.NewBlobFromFile(absolutePath)
	if err != nil {
		return "", fmt.Errorf("failed to create blob from file %s: %w", absolutePath, err)
	}

	// Determine file mode
	fileMode := index.DetectFileMode(fileInfo)

	// Skip unchanged files: if an index entry already exists with the same
	// content hash and mode, the file has not been modified.
	if existing := idx.GetEntry(relativeFilePath); existing != nil &&
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
		relativeFilePath,
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

	return relativeFilePath, nil
}

// OrchestrateAddExecution coordinates the full staging workflow: loads the
// current index, resolves target file paths from args, creates blob objects
// for modified files, updates the index, and persists it to disk.
// Returns the list of relative paths that were actually staged.
func OrchestrateAddExecution(repoPath string, args []string) ([]string, error) {
	// 1. Load existing index
	indexManager := index.NewManager(repoPath)
	idx, err := indexManager.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load index: %w", err)
	}

	// 2. Resolve file paths
	filePaths, err := resolveFilePaths(repoPath, args)
	if err != nil {
		return nil, err
	}
	// Sort paths for deterministic processing
	slices.Sort(filePaths)

	// 3. Process each file
	addedFiles := make([]string, 0, len(filePaths))
	store := objects.NewObjectStore(repoPath)
	for _, filePath := range filePaths {
		added, err := addFile(repoPath, filePath, idx, store)
		if err != nil {
			return nil, fmt.Errorf("failed to add file %s: %w", filePath, err)
		}
		if added != "" {
			addedFiles = append(addedFiles, added)
		}
	}

	// 4. Save updated index
	if err := indexManager.Save(idx); err != nil {
		return nil, fmt.Errorf("failed to save index: %w", err)
	}

	return addedFiles, nil
}
