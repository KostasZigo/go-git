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

	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/utils"
	"github.com/agiledragon/gomonkey/v2"
	"github.com/spf13/cobra"
)

func TestHashObjectCommand_Success_NoStorage(t *testing.T) {
	// Create temp repo
	tempDir := t.TempDir()
	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit: %v", err)
	}

	// Change to repo directory
	oldDir, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldDir)

	// Create test file
	testFileName := "test.txt"
	testFileContent := []byte("hello world\nHave a nice day")
	if err := os.WriteFile(testFileName, testFileContent, 0755); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Capture output
	var stdout bytes.Buffer
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(hashObjectCmd)
	testRootCmd.SetOut(&stdout)

	// Execute hash-object command without -w flag
	testRootCmd.SetArgs([]string{"hash-object", testFileName})
	err := testRootCmd.Execute()

	// Verify command succeeded
	if err != nil {
		t.Fatalf("hash-object command failed: %v", err)
	}

	// Verify hash output
	expectedHash, _ := utils.ComputeHash(testFileContent, utils.BlobObjectType)
	outputHash := strings.TrimSpace(stdout.String())

	if expectedHash != outputHash {
		t.Fatalf("Expected hash %s, got %s", expectedHash, outputHash)
	}

	// Verify object was NOT created (no -w flag)
	objectPath := filepath.Join(gogitDir, outputHash[:2], outputHash[2:])
	if _, err := os.Stat(objectPath); !errors.Is(err, fs.ErrNotExist) {
		t.Error("Object should not be created without -w flag")
	}
}

func TestHashObjectCommand_Success_WithStorage(t *testing.T) {
	// Create temp repo
	tempDir := t.TempDir()
	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit: %v", err)
	}

	// Create test file
	testFileName := "test.txt"
	testFile := filepath.Join(tempDir, testFileName)
	testFileContent := []byte("hello world\nHave a nice day")
	if err := os.WriteFile(testFile, testFileContent, 0755); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Change to repo directory
	oldDir, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldDir)

	// Capture output
	var stdout bytes.Buffer
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(hashObjectCmd)
	testRootCmd.SetOut(&stdout)

	// Execute hash-object command with -w flag
	testRootCmd.SetArgs([]string{"hash-object", testFileName, "-w"})
	err := testRootCmd.Execute()

	// Verify command succeeded
	if err != nil {
		t.Fatalf("hash-object command failed: %v", err)
	}

	// Verify hash output
	expectedHash, _ := utils.ComputeHash(testFileContent, utils.BlobObjectType)
	outputHash := strings.TrimSpace(stdout.String())

	if expectedHash != outputHash {
		t.Fatalf("Expected hash %s, got %s", expectedHash, outputHash)
	}

	// Verify object was created
	objectPath := filepath.Join(gogitDir, outputHash[:2], outputHash[2:])
	if _, err := os.Stat(objectPath); errors.Is(err, fs.ErrNotExist) {
		t.Error("Object should have been created with -w flag")
	}

	// Verify object can be read back
	store := objects.NewObjectStore(tempDir)
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

func TestHashObject_FileNotFound(t *testing.T) {
	// Create temp repo
	tempDir := t.TempDir()
	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit: %v", err)
	}

	// Change to repo directory
	oldDir, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldDir)

	dummyFileName := "dummy.txt"

	// Capture output
	var stderr bytes.Buffer
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(hashObjectCmd)
	testRootCmd.SetErr(&stderr)

	// Execute hash-object command with -w flag
	testRootCmd.SetArgs([]string{"hash-object", dummyFileName})
	err := testRootCmd.Execute()

	// Verify command failed
	if err == nil {
		t.Fatalf("hash-object command SHOULD fail")
	}

	// Verify error message mentions the file
	expectedErrorMessage := fmt.Sprintf("failed to read file %s", dummyFileName)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", expectedErrorMessage, err.Error())
	}
}

func TestHashObjectCommand_NoArguments(t *testing.T) {
	var stderr, stdout bytes.Buffer
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(hashObjectCmd)
	testRootCmd.SetErr(&stderr)
	testRootCmd.SetOut(&stdout)

	// Execute hash-object command without any arguments
	testRootCmd.SetArgs([]string{"hash-object"})
	err := testRootCmd.Execute()

	if err == nil {
		t.Fatal("Expected error when no arguments provided")
	}

	// Verify error message matches argument validation error
	expectedErrorMessage := "hash-object command requires exactly 1 argument (filepath), received 0"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", expectedErrorMessage, err.Error())
	}
}

func TestHashObjectCommand_TooManyArguments(t *testing.T) {
	var stderr, stdout bytes.Buffer
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(hashObjectCmd)
	testRootCmd.SetErr(&stderr)
	testRootCmd.SetOut(&stdout)

	// Execute hash-object command with too many arguments
	testRootCmd.SetArgs([]string{"hash-object", "a.txt", "b.txt"})
	err := testRootCmd.Execute()

	if err == nil {
		t.Fatal("Expected error when too many arguments are provided")
	}

	// Verify error message matches argument validation error
	expectedErrorMessage := "hash-object command requires exactly 1 argument (filepath), received 2"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", expectedErrorMessage, err.Error())
	}
}

func TestHashObjectCommand_FileNotInRepository(t *testing.T) {
	tempDir := t.TempDir()

	testFileName := "test.txt"
	testFileContent := []byte("Pikachu I choose you !")

	oldDir, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal("Failed to change directory")
	}
	defer os.Chdir(oldDir)

	if err := os.WriteFile(testFileName, testFileContent, 0755); err != nil {
		t.Fatal("Failed to write file")
	}

	var stderr, stdout bytes.Buffer
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(hashObjectCmd)
	testRootCmd.SetErr(&stderr)
	testRootCmd.SetOut(&stdout)

	// Execute hash-object command with write directive
	// File not in repo error only appears if we are storing the blob
	testRootCmd.SetArgs([]string{"hash-object", testFileName, "-w"})
	err := testRootCmd.Execute()

	if err == nil {
		t.Fatal("Expected error when file is not inside a repository")
	}

	expectedErrorMessage := ".gogit directory not found in this directory (or any parent up to mount point)"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", expectedErrorMessage, err.Error())
	}
}

func TestHashObjectCommand_StoreFailure(t *testing.T) {
	tempDir := t.TempDir()
	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatal("Failed to create .gogit directory")
	}

	// Change repo directory
	oldDir, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal("Failed to change directory")
	}
	defer os.Chdir(oldDir)

	// Create file
	testFileName := "test.txt"
	testFileContent := []byte("Charmander user Ember !")
	if err := os.WriteFile(testFileName, testFileContent, 0755); err != nil {
		t.Fatal("Failed to create test file")
	}

	// Mock ObjectStore.Store failure
	mockError := errors.New("failed to store blob to .gogit/objects")
	patches := gomonkey.ApplyMethod(&objects.ObjectStore{}, "Store",
		func(_ *objects.ObjectStore, _ objects.Object) error {
			return mockError
		})
	defer patches.Reset()

	var stderr, stdout bytes.Buffer
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(hashObjectCmd)
	testRootCmd.SetErr(&stderr)
	testRootCmd.SetOut(&stdout)

	// Execute hash-object command with write directive
	// Store is only executed when we are storing the blob
	testRootCmd.SetArgs([]string{"hash-object", testFileName, "-w"})
	err := testRootCmd.Execute()

	if err == nil {
		t.Fatal("Expected hash-object command to fail according to mocking")
	}

	expectedErrorMessage := "failed to store object: " + mockError.Error()
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", expectedErrorMessage, err.Error())
	}
}

func TestHashObjectCommand_NewBlobFromFileFailure(t *testing.T) {
	tempDir := t.TempDir()
	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatal("Failed to create .gogit directory")
	}

	// Change repo directory
	oldDir, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal("Failed to change directory")
	}
	defer os.Chdir(oldDir)

	// Create file
	testFileName := "test.txt"
	testFileContent := []byte("Charmander user Ember !")
	if err := os.WriteFile(testFileName, testFileContent, 0755); err != nil {
		t.Fatal("Failed to create test file")
	}

	// Mock EKGJEWORPIHG[]ERKHEJWTPHITWOKHJEPKHJRTHK[ESTHMEPSHKPEJHPKPREHNSKEPGRHKMCF;aGNEPGNJAWKN;AE'GKF] failure
	mockError := errors.New("failed to create new blob from file")
	patches := gomonkey.ApplyFunc(objects.NewBlobFromFile,
		func(_ string) (*objects.Blob, error) {
			return nil, mockError
		})
	defer patches.Reset()

	var stderr, stdout bytes.Buffer
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(hashObjectCmd)
	testRootCmd.SetErr(&stderr)
	testRootCmd.SetOut(&stdout)

	// Execute hash-object command with write directive
	// Store is only executed when we are storing the blob
	testRootCmd.SetArgs([]string{"hash-object", testFileName, "-w"})
	err := testRootCmd.Execute()

	if err == nil {
		t.Fatal("Expected hash-object command to fail according to mocking")
	}
	if !strings.Contains(err.Error(), mockError.Error()) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", mockError.Error(), err.Error())
	}
}

func TestHashObjectCommand_MultipleFiles_SameContent(t *testing.T) {
	// Create temp repo
	tempDir := t.TempDir()
	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit: %v", err)
	}

	// Change to repo directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create two files with identical content
	content := []byte("identical content\n")
	file1_name := "file1.txt"
	file2_name := "file2.txt"

	if err := os.WriteFile(file1_name, content, 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2_name, content, 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Hash file1
	var stdout1 bytes.Buffer
	testRootCmd1 := &cobra.Command{Use: "gogit"}
	testRootCmd1.AddCommand(hashObjectCmd)
	testRootCmd1.SetOut(&stdout1)
	testRootCmd1.SetArgs([]string{"hash-object", "-w", file1_name})

	if err := testRootCmd1.Execute(); err != nil {
		t.Fatalf("Failed to hash file1: %v", err)
	}

	hash1 := strings.TrimSpace(stdout1.String())

	// Hash file2
	var stdout2 bytes.Buffer
	testRootCmd2 := &cobra.Command{Use: "gogit"}
	testRootCmd2.AddCommand(hashObjectCmd)
	testRootCmd2.SetOut(&stdout2)
	testRootCmd2.SetArgs([]string{"hash-object", "-w", file2_name})

	if err := testRootCmd2.Execute(); err != nil {
		t.Fatalf("Failed to hash file2: %v", err)
	}

	hash2 := strings.TrimSpace(stdout2.String())

	// Verify both files produce the same hash
	if hash1 != hash2 {
		t.Errorf("Identical content should produce same hash: %s != %s", hash1, hash2)
	}

	// Verify only one object was created (content-addressable)
	objectPath := filepath.Join(gogitDir, hash1[:2], hash1[2:])
	if _, err := os.Stat(objectPath); os.IsNotExist(err) {
		t.Error("Object should exist for both files")
	}
}

func TestHashObjectCommand_EmptyFile(t *testing.T) {
	// Create temp repo
	tempDir := t.TempDir()
	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit: %v", err)
	}

	// Change to repo directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create empty file
	emptyFile := "empty.txt"
	if err := os.WriteFile(emptyFile, []byte{}, 0755); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	var stdout bytes.Buffer
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(hashObjectCmd)
	testRootCmd.SetOut(&stdout)

	// Execute hash-object
	testRootCmd.SetArgs([]string{"hash-object", "-w", emptyFile})
	err := testRootCmd.Execute()

	// Should succeed
	if err != nil {
		t.Fatalf("hash-object should succeed for empty file: %v", err)
	}

	// Verify hash for empty file
	expectedHash, _ := utils.ComputeHash([]byte{}, utils.BlobObjectType)
	outputHash := strings.TrimSpace(stdout.String())

	if outputHash != expectedHash {
		t.Errorf("Expected empty file hash %s, got %s", expectedHash, outputHash)
	}
}

func TestHashObjectCommand_LargeFile(t *testing.T) {
	// Create temp repo
	tempDir := t.TempDir()
	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit: %v", err)
	}

	// Change to repo directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create large file (1MB)
	largeFileName := "large.bin"
	largeContent := bytes.Repeat([]byte("A"), 1024*1024) // 1MB of 'A's
	if err := os.WriteFile(largeFileName, largeContent, 0755); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	var stdout bytes.Buffer
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(hashObjectCmd)
	testRootCmd.SetOut(&stdout)

	// Execute hash-object with -w
	testRootCmd.SetArgs([]string{"hash-object", "-w", largeFileName})
	err := testRootCmd.Execute()

	// Should succeed
	if err != nil {
		t.Fatalf("hash-object should succeed for large file: %v", err)
	}

	// Verify hash was printed
	expectedHash, _ := utils.ComputeHash(largeContent, utils.BlobObjectType)
	outputHash := strings.TrimSpace(stdout.String())

	if len(outputHash) != 40 {
		t.Errorf("Expected 40-char hash, got: %s", outputHash)
	}

	if expectedHash != outputHash {
		t.Fatalf("Expected hash %s, got %s", expectedHash, outputHash)
	}

	// Verify object was stored
	objectPath := filepath.Join(gogitDir, outputHash[:2], outputHash[2:])
	if _, err := os.Stat(objectPath); os.IsNotExist(err) {
		t.Error("Large file object should be stored")
	}
}
