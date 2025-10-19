package repository

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
)

func TestInitRepository(t *testing.T) {
	// Create temporary directory for testing
	tempDirectory := t.TempDir()

	err := InitRepository(tempDirectory)
	if err != nil {
		t.Fatalf("InitRepository failed: %v", err)
	}

	gogitDirectory := filepath.Join(tempDirectory, ".gogit")

	// Check .gogit directory exists
	if _, err := os.Stat(gogitDirectory); os.IsNotExist(err) {
		t.Errorf(".gogit directory was not created: %v", err)
	}

	// Check required subdirectories exist
	expectedDirectories := []string{"objects", "refs", "refs/heads", "refs/tags"}
	for _, dir := range expectedDirectories {
		dirPath := filepath.Join(gogitDirectory, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Errorf("Required directory %s was not created: %v", dir, err)
		}
	}

	// Check HEAD file exists and has correct content
	headPath := filepath.Join(gogitDirectory, "HEAD")
	content, err := os.ReadFile(headPath)
	if err != nil {
		t.Errorf("HEAD file was not created: %v", err)
	} else {
		expected := "ref: refs/heads/main\n"
		if string(content) != expected {
			t.Errorf("HEAD file content = %q, want %q", string(content), expected)
		}
	}

}

func TestInitRepository_AlreadyExists(t *testing.T) {
	tempDirectory := t.TempDir()

	// Initialize once
	err := InitRepository(tempDirectory)
	if err != nil {
		t.Fatalf("First initialization failed: %v", err)
	}

	// Try to initialize again - should fail
	err = InitRepository(tempDirectory)
	if err == nil {
		t.Error("Expected error when repository already exists, but got nil")
	}
}

func TestInitRepository_MkdirAllFailure(t *testing.T) {
	tempDir := t.TempDir()

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

	err := InitRepository(tempDir)
	if err == nil {
		t.Error("Expected error when os.MkdirAll fails, but got nil")
	}

	if !errors.Is(err, mockError) {
		t.Errorf("Expected error to wrap the mock error, but got: %v", err)
	}

	// Verify cleanup was called
	gogitDirectory := filepath.Join(tempDir, ".gogit")
	if _, err := os.Stat(gogitDirectory); err == nil {
		t.Error("Expected .gogit directory to be cleaned up after failure")
	}
}
