package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/KostasZigo/gogit/internal/constants"
)

// InitRepository creates .gogit directory structure with objects/, refs/, and HEAD file.
// Returns error if repository already exists or directory creation fails.
func InitRepository(path string) error {
	gogitDir := filepath.Join(path, constants.Gogit)
	if err := checkRepositoryDoesNotExist(gogitDir); err != nil {
		return err
	}

	// Track if initialization of gogit directories and files was successful
	// Default value: false
	var initSuccess bool

	// Clean up any directories/files in the case that repository initialization failed
	// If all resources got created successfully clean-up is not executed
	defer func() {
		if !initSuccess {
			cleanupRepository(gogitDir)
		}
	}()

	if err := createDirectoryStructure(gogitDir); err != nil {
		return err
	}

	if err := createHeadFile(gogitDir); err != nil {
		return err
	}

	initSuccess = true
	return nil
}

// checkRepositoryDoesNotExist verifies .gogit directory doesn't already exist.
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
		slog.Debug("Cleaning up partial repository initialization", "path", gogitDir)

		if err := os.RemoveAll(gogitDir); err != nil {
			slog.Warn("Failed to cleanup repository directory",
				"path", gogitDir,
				"error", err)
		} else {
			slog.Debug("Successfully cleaned up repository directory", "path", gogitDir)
		}
	}
}

// createDirectoryStructure creates required repository directories.
func createDirectoryStructure(gogitDir string) error {
	directories := []string{
		gogitDir,
		filepath.Join(gogitDir, constants.Objects),
		filepath.Join(gogitDir, constants.Refs),
		filepath.Join(gogitDir, constants.Refs, constants.Heads),
		filepath.Join(gogitDir, constants.Refs, constants.Tags),
	}

	for _, directory := range directories {
		if err := os.MkdirAll(directory, constants.DirPerms); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", directory, err)
		}
	}

	return nil
}

// createHeadFile writes HEAD file pointing to default branch.
func createHeadFile(gogitDir string) error {
	headFile := filepath.Join(gogitDir, constants.Head)
	headContent := constants.DefaultRefPrefix + constants.DefaultBranch + "\n"

	if err := os.WriteFile(headFile, []byte(headContent), constants.FilePerms); err != nil {
		return fmt.Errorf("failed to create %s file: %w", constants.Head, err)
	}

	return nil
}
