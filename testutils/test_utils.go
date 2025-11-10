package testutils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
)

// RandomString generates a random hex string of n bytes
func RandomString(n int) string {
	bytes := make([]byte, n)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// RandomHash generates a random 40-character SHA-1 hash
func RandomHash() string {
	return RandomString(constants.HashByteLength)
}

// SetupTestRepoWithGogitDir creates a temporary directory with .gogit/objects structure.
// This is useful for tests that need the repository structure but not full initialization.
func SetupTestRepoWithGogitDir(t *testing.T) string {
	t.Helper()

	repoPath := t.TempDir()
	gogitDir := filepath.Join(repoPath, constants.Gogit, constants.Objects)

	if err := os.MkdirAll(gogitDir, constants.DirPerms); err != nil {
		t.Fatalf("Failed to create %s/%s: %v", constants.Gogit, constants.Objects, err)
	}

	return repoPath
}

// SetupTestRepoWithInit creates a fully initialized .gogit repository structure.
// This includes objects/, refs/heads/, refs/tags/, and HEAD file.
func SetupTestRepoWithInit(t *testing.T) string {
	t.Helper()

	repoPath := t.TempDir()
	gogitDir := filepath.Join(repoPath, constants.Gogit)

	// Create directory structure
	dirs := []string{
		filepath.Join(gogitDir, constants.Objects),
		filepath.Join(gogitDir, constants.Refs, constants.Heads),
		filepath.Join(gogitDir, constants.Refs, constants.Tags),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, constants.DirPerms); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create HEAD file
	headPath := filepath.Join(gogitDir, constants.Head)
	headContent := []byte(constants.DefaultRefPrefix + constants.DefaultBranch + "\n")
	if err := os.WriteFile(headPath, headContent, constants.FilePerms); err != nil {
		t.Fatalf("Failed to create %s file: %v", constants.Head, err)
	}

	return repoPath
}

// CreateTestFile creates a file with given content in the specified directory.
// Returns the full path to the created file.
func CreateTestFile(t *testing.T, dir, filename string, content []byte) string {
	t.Helper()

	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, content, constants.FilePerms); err != nil {
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

// assertRepositoryStructure validates complete .gogit directory structure.
// Verifies objects/, refs/heads/, refs/tags/ exist and HEAD contains correct branch reference.
// Fatal error if any validation fails.
func AssertRepositoryStructure(t *testing.T, repoPath string) {
	t.Helper()

	gogitDir := filepath.Join(repoPath, constants.Gogit)
	AssertDirExists(t, gogitDir)

	expectedDirs := []string{
		constants.Objects,
		constants.Refs,
		filepath.Join(constants.Refs, constants.Heads),
		filepath.Join(constants.Refs, constants.Tags),
	}
	for _, dir := range expectedDirs {
		AssertDirExists(t, filepath.Join(gogitDir, dir))
	}

	headPath := filepath.Join(gogitDir, constants.Head)
	AssertFileExists(t, headPath)

	content, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("Failed to read %s file: %v", constants.Head, err)
	}

	expectedContent := constants.DefaultRefPrefix + constants.DefaultBranch + "\n"
	if string(content) != expectedContent {
		t.Errorf("%s content = %q, want %q", constants.Head, content, expectedContent)
	}
}
