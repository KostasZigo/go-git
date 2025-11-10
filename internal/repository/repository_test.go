package repository

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/testutils"
	"github.com/agiledragon/gomonkey/v2"
)

// TestInitRepository verifies successful repository initialization.
func TestInitRepository(t *testing.T) {
	repoPath := t.TempDir()

	if err := InitRepository(repoPath); err != nil {
		t.Fatalf("InitRepository failed: %v", err)
	}

	gogitDirectory := filepath.Join(repoPath, constants.Gogit)
	testutils.AssertDirExists(t, gogitDirectory)

	testutils.AssertRepositoryStructure(t, repoPath)
}

// TestInitRepository_AlreadyExists verifies error when repository exists.
func TestInitRepository_AlreadyExists(t *testing.T) {
	repoPath := t.TempDir()

	// Initialize once
	if err := InitRepository(repoPath); err != nil {
		t.Fatalf("First initialization failed: %v", err)
	}

	// Try to initialize again - should fail
	if err := InitRepository(repoPath); err == nil {
		t.Error("Expected error when repository already exists, but got nil")
	}
}

// TestInitRepository_MkdirAllFailure verifies cleanup on directory creation failure.
func TestInitRepository_MkdirAllFailure(t *testing.T) {
	repoPath := t.TempDir()
	// Mock os.MkdirAll to fail after first call
	mockError := errors.New("mocked mkdir failure")
	callCount := 0
	patches := gomonkey.ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
		callCount++
		if callCount > 1 {
			return mockError
		}
		// Let first call succeed (creates .gogit directory)
		return os.MkdirAll(path, perm)
	})
	defer patches.Reset()

	err := InitRepository(repoPath)
	if err == nil {
		t.Error("Expected error when os.MkdirAll fails, but got nil")
	}

	if !errors.Is(err, mockError) {
		t.Errorf("Expected error to wrap the mock error, but got: %v", err)
	}

	// Verify cleanup was called
	gogitDirectory := filepath.Join(repoPath, constants.Gogit)
	testutils.AssertFileNotExists(t, gogitDirectory)
}
