package worktree

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
)

// rollbackSnapshotApplication removes paths belonging to a partially applied
// target snapshot and restores the files and modes represented by currentIndex.
// currentIndex must be the pre-application index, and target must be the same
// snapshot passed to ApplySnapshot. The persisted index is not modified.
// Cleanup and restoration are both attempted, even when cleanup fails. Returned
// errors join every recovery failure under ErrRollback so callers can identify
// an incomplete rollback with errors.Is.
func rollbackSnapshotApplication(repoPath string, store *objects.ObjectStore, currentIndex *index.Index, target objects.TreeSnapshot) error {
	rollbackErrors := make([]error, 0, 2)

	if err := removeTargetSnapshotPaths(repoPath, target); err != nil {
		rollbackErrors = append(rollbackErrors, fmt.Errorf("failed to remove target snapshot paths: %w", err))
	}

	if err := restoreIndexedFiles(repoPath, store, currentIndex); err != nil {
		rollbackErrors = append(rollbackErrors, fmt.Errorf("failed to restore indexed files: %w", err))
	}

	rollbackErr := errors.Join(rollbackErrors...)
	if rollbackErr == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrRollback, rollbackErr)
}

// removeTargetSnapshotPaths removes every target leaf path deepest-first and
// prunes only parent directories that become empty.
func removeTargetSnapshotPaths(repoPath string, target objects.TreeSnapshot) error {
	pathsToRemove := slices.Collect(maps.Keys(target))
	sortLogicalPathsDepthFirst(pathsToRemove)
	return RemovePaths(repoPath, pathsToRemove)
}

// restoreIndexedFiles attempts to recreate every file represented by
// currentIndex and joins all restoration failures.
func restoreIndexedFiles(repoPath string, store *objects.ObjectStore, currentIndex *index.Index) error {
	restorationErrors := make([]error, 0)

	indexEntries := currentIndex.GetEntryList()
	for _, entry := range indexEntries {
		if err := restoreIndexedFile(repoPath, store, entry); err != nil {
			restorationErrors = append(restorationErrors, err)
		}
	}
	return errors.Join(restorationErrors...)
}

// restoreIndexedFile recreates one indexed file from its stored blob and
// reapplies the index mode's filesystem permissions.
func restoreIndexedFile(repoPath string, store *objects.ObjectStore, entry *index.Entry) error {
	localizedEntryPath, err := filepath.Localize(entry.Path())
	if err != nil {
		return fmt.Errorf("failed to localize file path [%s]: %w", entry.Path(), err)
	}
	absPath := filepath.Join(repoPath, localizedEntryPath)

	blob, err := store.ReadBlob(entry.Hash())
	if err != nil {
		return fmt.Errorf("failed to read blob for file [%s]: %w", entry.Path(), err)
	}

	permissions, err := entry.Mode().ToOsFileMOde()
	if err != nil {
		return fmt.Errorf("failed to convert index file mode to os permissions for file [%s]: %w", entry.Path(), err)
	}

	if err := writeFileAndParentDirs(absPath, blob.Content(), permissions); err != nil {
		return fmt.Errorf("failed to restore file [%s]: %w", entry.Path(), err)
	}
	return nil
}
