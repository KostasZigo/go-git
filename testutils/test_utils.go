package testutils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// RandomString generates a random hex string of n bytes
func RandomString(n int) string {
	bytes := make([]byte, n)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// RandomHash generates a random 40-character SHA-1 hash
func RandomHash() string {
	return RandomString(20)
}

// SetupTestRepo creates a temporary directory for testing.
// The directory is automatically cleaned up when the test completes.
func SetupTestRepo(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// SetupTestRepoWithGogitDir creates a temporary directory with .gogit/objects structure.
// This is useful for tests that need the repository structure but not full initialization.
func SetupTestRepoWithGogitDir(t *testing.T) string {
	t.Helper()

	repoPath := SetupTestRepo(t)
	gogitDir := filepath.Join(repoPath, ".gogit", "objects")

	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit/objects: %v", err)
	}

	return repoPath
}

// SetupTestRepoWithInit creates a fully initialized .gogit repository structure.
// This includes objects/, refs/heads/, refs/tags/, and HEAD file.
func SetupTestRepoWithInit(t *testing.T) string {
	t.Helper()

	repoPath := SetupTestRepo(t)
	gogitDir := filepath.Join(repoPath, ".gogit")

	// Create directory structure
	dirs := []string{
		filepath.Join(gogitDir, "objects"),
		filepath.Join(gogitDir, "refs", "heads"),
		filepath.Join(gogitDir, "refs", "tags"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create HEAD file
	headPath := filepath.Join(gogitDir, "HEAD")
	headContent := []byte("ref: refs/heads/main\n")
	if err := os.WriteFile(headPath, headContent, 0644); err != nil {
		t.Fatalf("Failed to create HEAD file: %v", err)
	}

	return repoPath
}

// CreateTestFile creates a file with given content in the specified directory.
// Returns the full path to the created file.
func CreateTestFile(t *testing.T, dir, filename string, content []byte) string {
	t.Helper()

	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("Failed to create test file %s: %v", filename, err)
	}

	return filePath
}

// AssertFileExists checks that a file exists at the given path.
// Fails the test if the file doesn't exist.
func AssertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Expected file to exist at %s", path)
	}
}

// AssertFileNotExists checks that a file does NOT exist at the given path.
// Fails the test if the file exists.
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Errorf("Expected file to NOT exist at %s", path)
	}
}

// AssertDirExists checks that a directory exists at the given path.
// Fails the test if the directory doesn't exist.
func AssertDirExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Expected directory to exist at %s", path)
		return
	}
	if err != nil {
		t.Errorf("Failed to stat directory %s: %v", path, err)
		return
	}
	if !info.IsDir() {
		t.Errorf("Expected %s to be a directory, but it's a file", path)
	}
}
