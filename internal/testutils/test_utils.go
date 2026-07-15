// Package testutils provides shared test helpers for repository setup,
// random data generation, file assertions, and HEAD/ref file manipulation.
package testutils

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"math/big"
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

// RandomBytes generates a random hex bytes array of n bytes
func RandomBytes(n int) []byte {
	bytes := make([]byte, n)
	rand.Read(bytes)
	return hex.AppendEncode(nil, bytes)
}

// RandomByteSlice generates a cryptographically random byte slice of length n.
func RandomByteSlice(n int) []byte {
	bytes := make([]byte, n)
	rand.Read(bytes)
	return bytes
}

// RandomInt generates a cryptographically random int64 in the range [0, max).
func RandomInt(upperBound int) int64 {
	bigInt := big.NewInt(int64(upperBound))
	number, err := rand.Int(rand.Reader, bigInt)
	if err != nil {
		panic(err)
	}
	return number.Int64()
}

// RandomHash generates a random 40-character SHA-1 hash
func RandomHash() string {
	return RandomString(constants.HashByteLength)
}

// RandomByteHash generates a random 40-character SHA-1 byte hash
func RandomByteHash() []byte {
	return RandomBytes(constants.HashByteLength)
}

// ChangeToDir changes the working directory to dir and registers a cleanup
// to restore the original directory when the test finishes.
func ChangeToDir(t *testing.T, dir string) {
	t.Helper()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change to directory %s: %v", dir, err)
	}

	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})
}

// SetupTestRepoWithGogitDir creates a temporary directory with .gogit/objects structure.
// This is useful for tests that need the repository structure but not full initialization.
func SetupTestRepoWithGogitDir(t *testing.T) string {
	t.Helper()

	repoPath := t.TempDir()
	gogitDir := filepath.Join(repoPath, constants.Gogit, constants.Objects)

	if err := os.MkdirAll(gogitDir, constants.DirPerms); err != nil {
		t.Fatalf("failed to create %s/%s: %v", constants.Gogit, constants.Objects, err)
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
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	// Create HEAD file
	headPath := filepath.Join(gogitDir, constants.Head)
	headContent := []byte(constants.DefaultRefPrefix + constants.DefaultBranch + "\n")
	if err := os.WriteFile(headPath, headContent, constants.FilePerms); err != nil {
		t.Fatalf("failed to create %s file: %v", constants.Head, err)
	}

	return repoPath
}

// CreateTestFile creates a file with given content in the specified directory.
// Returns the full path to the created file. If file already exists, it content is
// updated.
func CreateTestFile(t *testing.T, dir, filename string, content []byte) string {
	t.Helper()

	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, content, constants.FilePerms); err != nil {
		t.Fatalf("failed to create test file %s: %v", filename, err)
	}

	return filePath
}

// AssertFileExists checks that a file exists at the given path.
// Fails the test if the file doesn't exist.
func AssertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected file to exist at %s", path)
	}
}

// AssertFileNotExists checks that a file does NOT exist at the given path.
// Fails the test if the file exists.
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected file to NOT exist at %s", path)
	}
}

// AssertDirExists checks that a directory exists with the given path.
// Fails the test if the directory doesn't exist.
func AssertDirExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected directory to exist at %s", path)
		return
	}
	if err != nil {
		t.Errorf("failed to stat directory %s: %v", path, err)
		return
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be a directory, but it's a file", path)
	}
}

// AssertDirNotExists checks that a directory does not exist at the given path.
// Fails the test if the directory exists.
func AssertDirNotExists(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected directory to not exist at %s", path)
		return
	}
}

// AssertRepositoryStructure validates complete .gogit directory structure.
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
		t.Fatalf("failed to read %s file: %v", constants.Head, err)
	}

	expectedContent := constants.DefaultRefPrefix + constants.DefaultBranch + "\n"
	if string(content) != expectedContent {
		t.Errorf("%s content = %q, want %q", constants.Head, content, expectedContent)
	}
}

// ReadDefaultRefFile reads the default branch ref file and returns the trimmed commit hash.
// Fails the test if the file cannot be read.
func ReadDefaultRefFile(t *testing.T, repoPath string) string {
	t.Helper()
	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, constants.DefaultBranch)
	content, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("failed to read ref file: %v", err)
	}
	return string(bytes.TrimSpace(content))
}

// AssertFileContent reads the file at the given path and verifies its content
// matches the expected byte slice. Fails the test on read error or content mismatch.
func AssertFileContent(t *testing.T, filePath string, expectedContent []byte) {
	t.Helper()

	actualContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", filePath, err)
	}

	if string(actualContent) != string(expectedContent) {
		t.Fatalf("file content mismatch at %s:\n  expected: [%s]\n  got: [%s]", filePath, expectedContent, actualContent)
	}
}

// WriteRefFile writes a commit hash into the branch ref file at refs/heads/<branchName>.
func WriteRefFile(t *testing.T, repoPath, branchName, commitHash string) {
	t.Helper()

	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, branchName)
	if err := os.WriteFile(refPath, []byte(commitHash+"\n"), constants.FilePerms); err != nil {
		t.Fatalf("failed to write ref file for branch %s: %v", branchName, err)
	}
}

// ReadHEADFile reads and returns the raw content of .gogit/HEAD.
func ReadHEADFile(t *testing.T, repoPath string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(repoPath, constants.Gogit, constants.Head))
	if err != nil {
		t.Fatalf("failed to read HEAD file: %v", err)
	}
	return string(content)
}

// WriteHEADFile overwrites .gogit/HEAD with the given content.
func WriteHEADFile(t *testing.T, repoPath string, content []byte) {
	t.Helper()

	path := filepath.Join(repoPath, constants.Gogit, constants.Head)
	if err := os.WriteFile(path, content, constants.FilePerms); err != nil {
		t.Fatalf("failed to write HEAD file: %v", err)
	}
}

// AssertHEADContent reads .gogit/HEAD and verifies its content matches the expected string.
func AssertHEADContent(t *testing.T, repoPath, expectedContent string) {
	t.Helper()

	head := ReadHEADFile(t, repoPath)
	if head != expectedContent {
		t.Fatalf("expected HEAD to be [%s], got [%s]", expectedContent, head)
	}
}
