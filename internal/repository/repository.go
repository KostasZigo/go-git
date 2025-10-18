package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

const (
	dirPerms  = 0755
	filePerms = 0644
)

func InitRepository(path string) error {
	// Resolves and adds OS specific separator
	gogitDir := filepath.Join(path, ".gogit")

	if err := checkRepositoryDoesNotExist(gogitDir); err != nil {
		return err
	}

	// Track if initialization of gogit directories and files was successful
	// Default value: false
	var initSuccess bool

	// Defer a func to clean up any directories/files in the case that
	// repository initialization failed (not all directories/files were created successfully).
	// If all resources got created successfully initSuccess is true, and the clean-up
	//  is not executed
	defer func() {
		if !initSuccess {
			cleanupRepository(gogitDir)
		}
	}()

	directories := []string{
		gogitDir,
		filepath.Join(gogitDir, "objects"),
		filepath.Join(gogitDir, "refs"),
		filepath.Join(gogitDir, "refs", "heads"),
		filepath.Join(gogitDir, "refs", "tags"),
	}

	// Create all gogit directories
	for _, directory := range directories {
		if err := os.MkdirAll(directory, dirPerms); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", directory, err)
		}
	}

	// Create HEAD file pointing to main branch
	headFile := filepath.Join(gogitDir, "HEAD")
	headContent := "ref: refs/heads/main\n"

	if err := os.WriteFile(headFile, []byte(headContent), filePerms); err != nil {
		return fmt.Errorf("failed to create HEAD file: %w", err)
	}

	initSuccess = true
	return nil
}

func checkRepositoryDoesNotExist(path string) error {
	_, err := os.Stat(path)

	// If path doesn't exist there is no error
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to check repository path: %w", err)
	}

	return fmt.Errorf("repository already exists at %s", path)
}

// Removes the entire .gogit directory if it exists
func cleanupRepository(gogitDir string) {
	if _, err := os.Stat(gogitDir); err == nil {
		slog.Debug("Cleaning up partial repository initialization",
			"path", gogitDir)

		if err := os.RemoveAll(gogitDir); err != nil {
			slog.Warn("Failed to cleanup repository directory",
				"path", gogitDir,
				"error", err)
		} else {
			slog.Debug("Successfully cleaned up repository directory",
				"path", gogitDir)
		}
	}
}
