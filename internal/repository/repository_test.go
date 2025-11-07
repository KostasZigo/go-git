package repository

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/KostasZigo/gogit/testutils"
	"github.com/agiledragon/gomonkey/v2"
)

// TestInitRepository verifies successful repository initialization.
func TestInitRepository(t *testing.T) {
	repoPath := testutils.SetupTestRepo(t)

	if err := InitRepository(repoPath); err != nil {
		t.Fatalf("InitRepository failed: %v", err)
	}

	gogitDirectory := filepath.Join(repoPath, ".gogit")
	testutils.AssertDirExists(t, gogitDirectory)

	// Check required subdirectories exist
	expectedDirectories := []string{"objects", "refs", "refs/heads", "refs/tags"}
	for _, dir := range expectedDirectories {
		testutils.AssertDirExists(t, filepath.Join(gogitDirectory, dir))
	}

	// Check HEAD file exists and has correct content
	headPath := filepath.Join(gogitDirectory, "HEAD")
	content, err := os.ReadFile(headPath)
	if err != nil {
		t.Errorf("Failed to read HEAD file: %v", err)
	} else {
		expected := "ref: refs/heads/main\n"
		if string(content) != expected {
			t.Errorf("HEAD file content = %q, want %q", string(content), expected)
		}
	}

}

// TestInitRepository_AlreadyExists verifies error when repository exists.
func TestInitRepository_AlreadyExists(t *testing.T) {
	repoPath := testutils.SetupTestRepo(t)

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
	repoPath := testutils.SetupTestRepo(t)

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
	gogitDirectory := filepath.Join(repoPath, ".gogit")
	testutils.AssertFileNotExists(t, gogitDirectory)
}
