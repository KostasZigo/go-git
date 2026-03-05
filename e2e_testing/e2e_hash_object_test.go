package e2etesting

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/testutils"
	"github.com/KostasZigo/gogit/utils"
)

// TestE2E_HashObjectCommand_NoStorage verifies hash computation without storage.
func TestE2E_HashObjectCommand_NoStorage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Build binary and run `gogit init`
	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	// Create test file
	testFileName := "test.txt"
	testFileContent := []byte("hello world\n")
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	// Run hash-object without -w
	cmd := exec.Command(sharedBinaryPath, constants.HashObjectCmdName, testFileName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify hash is printed (40 hex chars + newline)
	outputHash := strings.TrimSpace(string(output))
	expectedHash, err := utils.ComputeHash(testFileContent, utils.BlobObjectType)
	if err != nil {
		t.Fatalf("Failed to compute hash: %v", err)
	}

	if len(outputHash) != 40 {
		t.Errorf("Expected 40-char hash, got: %s", outputHash)
	}

	if expectedHash != outputHash {
		t.Fatalf("Expected hash %s, got %s", expectedHash, outputHash)
	}

	// Verify object was NOT created (no -w flag)
	objectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, outputHash[:constants.HashDirPrefixLength], outputHash[constants.HashDirPrefixLength:])
	if _, err := os.Stat(objectPath); !errors.Is(err, fs.ErrNotExist) {
		t.Error("Object should not be created without -w flag")
	}
}

// TestE2E_HashObjectCommand_WithStorage verifies hash computation with storage.
func TestE2E_HashObjectCommand_WithStorage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	testFileName := "pokemon.txt"
	testFileContent := []byte("Charmander evolved into Charmeleon !")
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	// Run gogit hash-object file with write directive (-w)
	hashObjectCmd := exec.Command(sharedBinaryPath, constants.HashObjectCmdName, testFileName, "-w")
	hashObjectCmd.Dir = repoPath
	output, err := hashObjectCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gogit %s command failed: %v", constants.HashObjectCmdName, err)
	}

	// Verify hash was printed
	printedHash := strings.TrimSpace(string(output))
	expectedHash, err := utils.ComputeHash(testFileContent, utils.BlobObjectType)
	if err != nil {
		t.Fatalf("Failed to compute hash: %v", err)
	}

	if printedHash != expectedHash {
		t.Fatalf("Expected printed has to be [%s] but got [%s]", expectedHash, printedHash)
	}

	// Verify object file was created at correct path
	objectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, expectedHash[:constants.HashDirPrefixLength], expectedHash[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, objectPath)

	// Verify object file is not empty (compressed data)
	info, err := os.Stat(objectPath)
	if err != nil {
		t.Fatalf("Failed to stat object file: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Object file should not be empty")
	}

	//Verify File content
	decompressedContent := decompressObject(t, objectPath)
	assertBlobContent(t, decompressedContent, testFileContent)
}

// TestE2E_HashObjectCommand_InvalidArgs verifies error for missing arguments.
func TestE2E_HashObjectCommand_InvalidArgs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Test with no arguments
	cmd := exec.Command(sharedBinaryPath, constants.HashObjectCmdName)
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("Expected error when no file argument provided")
	}

	outputStr := string(output)
	expectedMsg := fmt.Sprintf("%s command requires exactly 1 argument (filepath), received 0", constants.HashObjectCmdName)
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected error to contain %q, got: %s", expectedMsg, outputStr)
	}
}
