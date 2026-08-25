package worktree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/KostasZigo/gogit/internal/hasher"
	"github.com/KostasZigo/gogit/internal/index"
)

// InspectWorktreeChanges compares each tracked index entry with its
// corresponding filesystem path and returns every working-tree change.
func (service *Service) InspectWorktreeChanges() ([]Change, error) {
	idx, err := service.indexManager.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load index: %w", err)
	}

	changes := make([]Change, 0, len(idx.GetEntryList()))
	for _, idxEntry := range idx.GetEntryList() {
		entryChanges, err := inspectIndexEntry(service.repoPath, idxEntry)
		if err != nil {
			return nil, err
		}

		changes = append(changes, entryChanges...)
	}

	sortChanges(changes)
	return changes, nil
}

// inspectIndexEntry compares one index entry with the filesystem path it tracks.
func inspectIndexEntry(repoPath string, idxEntry *index.Entry) ([]Change, error) {
	if !supportsWorktreeInspection(idxEntry.Mode()) {
		return nil, fmt.Errorf("unsupported index file mode %o for [%s]", idxEntry.Mode(), idxEntry.Path())
	}

	osFilePath, err := filepath.Localize(idxEntry.Path())
	if err != nil {
		return nil, fmt.Errorf("failed to convert path [%s] to os specific path: %w", idxEntry.Path(), err)
	}

	filePath := filepath.Join(repoPath, osFilePath)
	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		// Check if file was deleted from working tree
		if errors.Is(err, fs.ErrNotExist) {
			return []Change{{Path: idxEntry.Path(), Kind: ChangeDeleted}}, nil
		}
		return nil, fmt.Errorf("failed to inspect file path [%s]: %w", filePath, err)
	}

	// Check if file changed to Iregular Mode
	if !fileInfo.Mode().IsRegular() {
		return []Change{{Path: idxEntry.Path(), Kind: ChangeTypeModified}}, nil
	}

	changes := make([]Change, 0, 2)

	// check if mode has been changed
	if index.DetectFileMode(fileInfo) != idxEntry.Mode() {
		changes = append(changes, Change{Path: idxEntry.Path(), Kind: ChangeModeModified})
	}

	// check if hash/content has been changed, if modification time and filesize is the same
	// then it assumed unchanged, otherwise check the actual hashes
	if fileInfo.Size() == idxEntry.FileSize() &&
		fileInfo.ModTime().Truncate(time.Second).Equal(
			idxEntry.LastModified().Truncate(time.Second),
		) {
		return changes, nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tracked path [%s]: %w", filePath, err)
	}

	hash, err := hasher.ComputeHash(content, hasher.Blob)
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash for tracked path [%s]: %w", filePath, err)
	}
	if hash != idxEntry.Hash() {
		changes = append(changes, Change{Path: idxEntry.Path(), Kind: ChangeContentModified})
	}

	return changes, nil
}

// supportsWorktreeInspection reports whether mode is supported
func supportsWorktreeInspection(mode index.FileMode) bool {
	return mode == index.ModeRegularFile || mode == index.ModeExecutable
}
