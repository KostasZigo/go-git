package index

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/KostasZigo/gogit/internal/constants"
)

// Manager handles index file operations.
type Manager struct {
	repoPath string
}

// NewManager creates index manager for repository.
func NewManager(repoPath string) *Manager {
	return &Manager{
		repoPath: repoPath,
	}
}

// indexPath returns filesystem path to index file.
func (manager *Manager) indexPath() string {
	return filepath.Join(manager.repoPath, constants.Gogit, constants.Index)
}

// Save writes index to disk.
func (manager *Manager) Save(index *Index) error {
	indexPath := manager.indexPath()

	// Create temporary file to write the index
	// If everything is successful this file will replace the existing index file
	tempFile, err := os.CreateTemp(filepath.Dir(indexPath), ".index-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary index file: %w", err)
	}
	tempPath := tempFile.Name()

	// Track success for cleanup
	succeeded := false
	defer func() {
		if !succeeded {
			if err := tempFile.Close(); err != nil {
				slog.Info("failed to close tempfile", "error", err)
			}
			if err := os.Remove(tempPath); err != nil {
				slog.Info("failed to remove temp path", "path", tempPath, "error", err.Error())
			}
		}
	}()

	// Write index to temp file
	if err := WriteIndex(tempFile, index); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}

	// Force OS to flush data to physical disk before rename
	// Critical for durability guarantees on power loss
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}

	// Close temp file descriptor before rename
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Replace index file with temporary file
	if err := os.Rename(tempPath, indexPath); err != nil {
		return fmt.Errorf("failed to rename temporary file to index: %w", err)
	}

	succeeded = true
	return nil
}

// Load reads index from disk, returns empty index if file doesn't exist.
func (manager *Manager) Load() (*Index, error) {
	indexPath := manager.indexPath()

	// Open index file
	indexFile, err := os.Open(indexPath)
	if os.IsNotExist(err) {
		// index does not exist yet - create initially empty index
		return NewIndex(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open index file: %w", err)
	}
	defer func() {
		if err := indexFile.Close(); err != nil {
			slog.Info("failed to close index file", "error", err)
		}
	}()

	// Read index file
	index, err := ReadIndex(indexFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read index file: %w", err)
	}

	return index, nil
}

// Exists checks if index file exists.
func (manager *Manager) Exists() bool {
	_, err := os.Stat(manager.indexPath())
	return err == nil
}
