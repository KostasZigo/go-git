package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/testutils"
	"github.com/KostasZigo/gogit/utils"
	"github.com/agiledragon/gomonkey/v2"
)

// TestHashObjectCommand_Success_NoStorage verifies hash computation without storage.
func TestHashObjectCommand_Success_NoStorage(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)

	// Change to repo directory
	changeToRepoDir(t, repoPath)

	// Create test file
	testFileName := "test.txt"
	testFileContent := []byte("hello world\nHave a nice day")
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	testRootCmd := createTestRootCmd(hashObjectCmd)
	stdout := captureStdout(testRootCmd)

	// Execute hash-object command without -w flag
	testRootCmd.SetArgs([]string{constants.HashObjectCmdName, testFileName})

	// Verify command succeeded
	if err := testRootCmd.Execute(); err != nil {
		t.Fatalf("%s command failed: %v", constants.HashObjectCmdName, err)
	}

	// Verify hash output
	outputHash := strings.TrimSpace(stdout.String())
	expectedHash, err := utils.ComputeHash(testFileContent, utils.BlobObjectType)
	if err != nil {
		t.Fatalf("Failed to compute hash: %v", err)
	}

	if expectedHash != outputHash {
		t.Fatalf("Expected hash %s, got %s", expectedHash, outputHash)
	}

	// Verify object was NOT created (no -w flag)
	objectPath := filepath.Join(repoPath, outputHash[:constants.HashDirPrefixLength], outputHash[constants.HashDirPrefixLength:])
	if _, err := os.Stat(objectPath); !errors.Is(err, fs.ErrNotExist) {
		t.Error("Object should not be created without -w flag")
	}
}

// TestHashObjectCommand_Success_WithStorage verifies hash computation with storage.
func TestHashObjectCommand_Success_WithStorage(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)

	testFileName := "test.txt"
	testFileContent := []byte("hello world\nHave a nice day")
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	changeToRepoDir(t, repoPath)

	testRootCmd := createTestRootCmd(hashObjectCmd)
	stdout := captureStdout(testRootCmd)

	// Execute hash-object command with -w flag
	testRootCmd.SetArgs([]string{constants.HashObjectCmdName, testFileName, "-w"})
	if err := testRootCmd.Execute(); err != nil {
		t.Fatalf("%s command failed: %v", constants.HashObjectCmdName, err)
	}

	// Verify hash output
	expectedHash, err := utils.ComputeHash(testFileContent, utils.BlobObjectType)
	if err != nil {
		t.Fatalf("Failed to compute hash: %v", err)
	}
	outputHash := strings.TrimSpace(stdout.String())

	if expectedHash != outputHash {
		t.Fatalf("Expected hash %s, got %s", expectedHash, outputHash)
	}

	// Verify object was created
	objectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, outputHash[:constants.HashDirPrefixLength], outputHash[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, objectPath)

	// Verify object can be read back
	store := objects.NewObjectStore(repoPath)
	blob, err := store.ReadBlob(expectedHash)
	if err != nil {
		t.Errorf("Failed to read stored blob: %v", err)
	}

	if blob.Hash() != expectedHash {
		t.Errorf("Stored blob hash mismatch: expected %q, got %q", expectedHash, blob.Hash())
	}
	if !bytes.Equal(blob.Content(), testFileContent) {
		t.Errorf("Stored blob content mismatch: expected %q, got %q", testFileContent, blob.Content())
	}
}

// TestHashObject_FileNotFound verifies error for non-existent file.
func TestHashObject_FileNotFound(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	changeToRepoDir(t, repoPath)

	dummyFileName := "dummy.txt"

	testRootCmd := createTestRootCmd(hashObjectCmd)
	captureStderr(testRootCmd)

	// Execute hash-object command with -w flag
	testRootCmd.SetArgs([]string{constants.HashObjectCmdName, dummyFileName})
	err := testRootCmd.Execute()
	if err == nil {
		t.Fatalf("%s command SHOULD fail", constants.HashObjectCmdName)
	}

	// Verify error message mentions the file
	expectedErrorMessage := fmt.Sprintf("failed to read file %s", dummyFileName)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", expectedErrorMessage, err.Error())
	}
}

// TestHashObjectCommand_NoArguments verifies error when no arguments provided.
func TestHashObjectCommand_NoArguments(t *testing.T) {
	testRootCmd := createTestRootCmd(hashObjectCmd)
	captureStderr(testRootCmd)
	captureStdout(testRootCmd)

	// Execute hash-object command without any arguments
	testRootCmd.SetArgs([]string{constants.HashObjectCmdName})
	err := testRootCmd.Execute()

	if err == nil {
		t.Fatal("Expected error when no arguments provided")
	}

	// Verify error message matches argument validation error
	expectedErrorMessage := fmt.Sprintf("%s command requires exactly 1 argument (filepath), received 0", constants.HashObjectCmdName)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", expectedErrorMessage, err.Error())
	}
}

// TestHashObjectCommand_TooManyArguments verifies error when too many arguments provided.
func TestHashObjectCommand_TooManyArguments(t *testing.T) {
	testRootCmd := createTestRootCmd(hashObjectCmd)
	captureStderr(testRootCmd)
	captureStdout(testRootCmd)

	// Execute hash-object command with too many arguments
	testRootCmd.SetArgs([]string{constants.HashObjectCmdName, "a.txt", "b.txt"})
	err := testRootCmd.Execute()

	if err == nil {
		t.Fatal("Expected error when too many arguments are provided")
	}

	// Verify error message matches argument validation error
	expectedErrorMessage := fmt.Sprintf("%s command requires exactly 1 argument (filepath), received 2", constants.HashObjectCmdName)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", expectedErrorMessage, err.Error())
	}
}

// TestHashObjectCommand_FileNotInRepository verifies error when file outside repository.
func TestHashObjectCommand_FileNotInRepository(t *testing.T) {
	repoPath := t.TempDir()
	changeToRepoDir(t, repoPath)

	testFileName := "test.txt"
	testFileContent := []byte("Pikachu I choose you !")
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	testRootCmd := createTestRootCmd(hashObjectCmd)
	captureStderr(testRootCmd)
	captureStdout(testRootCmd)

	// Execute hash-object command with write directive
	// File not in repo error only appears if we are storing the blob
	testRootCmd.SetArgs([]string{constants.HashObjectCmdName, testFileName, "-w"})
	err := testRootCmd.Execute()

	if err == nil {
		t.Fatal("Expected error when file is not inside a repository")
	}

	expectedErrorMessage := fmt.Sprintf("%s directory not found", constants.Gogit)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", expectedErrorMessage, err.Error())
	}
}

// TestHashObjectCommand_StoreFailure verifies error handling when storage fails.
func TestHashObjectCommand_StoreFailure(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	changeToRepoDir(t, repoPath)

	// Create file
	testFileName := "test.txt"
	testFileContent := []byte("Charmander user Ember !")
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	// Mock ObjectStore.Store failure
	mockError := errors.New("failed to store blob to .gogit/objects")
	patches := gomonkey.ApplyMethod(&objects.ObjectStore{}, "Store",
		func(_ *objects.ObjectStore, _ objects.Object) error {
			return mockError
		})
	defer patches.Reset()

	testRootCmd := createTestRootCmd(hashObjectCmd)
	captureStderr(testRootCmd)
	captureStdout(testRootCmd)

	// Execute hash-object command with write directive
	// Store is only executed when we are storing the blob
	testRootCmd.SetArgs([]string{constants.HashObjectCmdName, testFileName, "-w"})
	err := testRootCmd.Execute()

	if err == nil {
		t.Fatalf("Expected %s command to fail according to mocking", constants.HashObjectCmdName)
	}

	expectedErrorMessage := "failed to store object: " + mockError.Error()
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", expectedErrorMessage, err.Error())
	}
}

// TestHashObjectCommand_NewBlobFromFileFailure verifies error handling when blob creation fails.
func TestHashObjectCommand_NewBlobFromFileFailure(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	changeToRepoDir(t, repoPath)

	// Create file
	testFileName := "test.txt"
	testFileContent := []byte("Charmander user Ember !")
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	// Mock failure
	mockError := errors.New("failed to create new blob from file")
	patches := gomonkey.ApplyFunc(objects.NewBlobFromFile,
		func(_ string) (*objects.Blob, error) {
			return nil, mockError
		})
	defer patches.Reset()

	testRootCmd := createTestRootCmd(hashObjectCmd)
	captureStderr(testRootCmd)
	captureStdout(testRootCmd)

	// Execute hash-object command with write directive
	// Store is only executed when we are storing the blob
	testRootCmd.SetArgs([]string{constants.HashObjectCmdName, testFileName, "-w"})
	err := testRootCmd.Execute()

	if err == nil {
		t.Fatalf("Expected %s command to fail according to mocking", constants.HashObjectCmdName)
	}
	if !strings.Contains(err.Error(), mockError.Error()) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", mockError.Error(), err.Error())
	}
}

// TestHashObjectCommand_MultipleFiles_SameContent verifies content-addressable storage.
func TestHashObjectCommand_MultipleFiles_SameContent(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	changeToRepoDir(t, repoPath)

	// Create two files with identical content
	content := []byte("identical content\n")
	file1_name := "file1.txt"
	file2_name := "file2.txt"

	testutils.CreateTestFile(t, repoPath, file1_name, content)
	testutils.CreateTestFile(t, repoPath, file2_name, content)

	// Hash file 1
	testRootCmd1 := createTestRootCmd(hashObjectCmd)
	stdout1 := captureStdout(testRootCmd1)
	testRootCmd1.SetArgs([]string{constants.HashObjectCmdName, "-w", file1_name})
	if err := testRootCmd1.Execute(); err != nil {
		t.Fatalf("Failed to hash file1: %v", err)
	}
	hash1 := strings.TrimSpace(stdout1.String())

	// Hash file2
	testRootCmd2 := createTestRootCmd(hashObjectCmd)
	stdout2 := captureStdout(testRootCmd2)
	testRootCmd2.SetArgs([]string{constants.HashObjectCmdName, "-w", file2_name})
	if err := testRootCmd2.Execute(); err != nil {
		t.Fatalf("Failed to hash file2: %v", err)
	}
	hash2 := strings.TrimSpace(stdout2.String())

	// Verify both files produce the same hash
	if hash1 != hash2 {
		t.Errorf("Identical content should produce same hash: %s != %s", hash1, hash2)
	}

	// Verify only one object was created (content-addressable)
	objectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, hash1[:constants.HashDirPrefixLength], hash1[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, objectPath)
}

// TestHashObjectCommand_EmptyFile verifies hash computation for empty file.
func TestHashObjectCommand_EmptyFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	changeToRepoDir(t, repoPath)

	// Create empty file
	emptyFile := "empty.txt"
	testutils.CreateTestFile(t, repoPath, emptyFile, []byte{})

	testRootCmd := createTestRootCmd(hashObjectCmd)
	stdout := captureStdout(testRootCmd)

	// Execute hash-object
	testRootCmd.SetArgs([]string{constants.HashObjectCmdName, "-w", emptyFile})
	if err := testRootCmd.Execute(); err != nil {
		t.Fatalf("%s should succeed for empty file: %v", constants.HashObjectCmdName, err)
	}

	// Verify hash for empty
	outputHash := strings.TrimSpace(stdout.String())
	expectedHash, err := utils.ComputeHash([]byte{}, utils.BlobObjectType)
	if err != nil {
		t.Fatalf("Failed to compute hash: %v", err)
	}

	if outputHash != expectedHash {
		t.Errorf("Expected empty file hash %s, got %s", expectedHash, outputHash)
	}
}

// TestHashObjectCommand_LargeFile verifies hash computation for large file.
func TestHashObjectCommand_LargeFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	changeToRepoDir(t, repoPath)

	// Create large file (1MB)
	largeFileName := "large.bin"
	largeContent := bytes.Repeat([]byte("A"), 1024*1024) // 1MB of 'A's
	testutils.CreateTestFile(t, repoPath, largeFileName, largeContent)

	testRootCmd := createTestRootCmd(hashObjectCmd)
	stdout := captureStdout(testRootCmd)

	// Execute hash-object with -w
	testRootCmd.SetArgs([]string{constants.HashObjectCmdName, "-w", largeFileName})
	if err := testRootCmd.Execute(); err != nil {
		t.Fatalf("%s should succeed for large file: %v", constants.HashObjectCmdName, err)
	}

	// Verify hash was printed
	outputHash := strings.TrimSpace(stdout.String())
	expectedHash, err := utils.ComputeHash(largeContent, utils.BlobObjectType)
	if err != nil {
		t.Fatalf("Failed to compute hash: %v", err)
	}

	if len(outputHash) != 40 {
		t.Errorf("Expected 40-char hash, got: %s", outputHash)
	}

	if expectedHash != outputHash {
		t.Fatalf("Expected hash %s, got %s", expectedHash, outputHash)
	}

	// Verify object was stored
	objectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, outputHash[:constants.HashDirPrefixLength], outputHash[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, objectPath)
}
