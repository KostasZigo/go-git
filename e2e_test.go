package main

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/testutils"
	"github.com/KostasZigo/gogit/utils"
)

// sharedBinaryPath stores compiled gogit binary path built once in TestMain.
// All E2E tests execute this binary to verify end-to-end behavior.
// Binary persists for test suite duration, cleaned up after all tests complete
var sharedBinaryPath string

// TestMain executes before all tests to build gogit binary once.
// Binary stored in temporary directory, removed after test suite completes.
//
// Execution flow:
//  1. Create temporary directory for binary storage
//  2. Build gogit binary with platform-specific extension
//  3. Store binary path in package-level sharedBinaryPath variable
//  4. Execute all Test* functions via m.Run()
//  5. Clean up temporary directory and binary
//  6. Exit with test suite status code
func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "gogit-e2e-*")
	if err != nil {
		panic("Failed to create temp directory: " + err.Error())
	}
	defer os.RemoveAll(tempDir)

	binaryName := "gogit"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	sharedBinaryPath = filepath.Join(tempDir, binaryName)

	buildCmd := exec.Command("go", "build", "-o", sharedBinaryPath, ".")
	if err := buildCmd.Run(); err != nil {
		panic("Failed to build binary: " + err.Error())
	}

	os.Exit(m.Run())
}

// TestE2E_InitCommand verifies repository initialization creates correct structure.
func TestE2E_InitCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Create test repo directory
	repoPath := setupTestRepo(t)

	// Test the binary like a real user
	cmd := exec.Command(sharedBinaryPath, constants.InitCmdName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Binary execution failed: %v\nOutput: %s", err, output)
	}

	// Verify output
	outputStr := string(output)
	expectedMsg := fmt.Sprintf("Initialized empty GoGit repository in %s\n", utils.BuildDirPath(".", constants.Gogit))
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected output to contain %q, got: %s", expectedMsg, outputStr)
	}

	// Verify filesystem changes
	gogitDir := filepath.Join(repoPath, constants.Gogit)
	testutils.AssertDirExists(t, gogitDir)
	testutils.AssertRepositoryStructure(t, repoPath)

	// Test error case - init again
	cmd = exec.Command(sharedBinaryPath, constants.InitCmdName)
	cmd.Dir = repoPath
	output, err = cmd.CombinedOutput()

	if err == nil {
		t.Errorf("Expected error when running %s twice", constants.InitCmdName)
	}

	expectedErrorMsg := "Error: failed to initialize repository - repository already exists at .gogit\n"
	if !strings.Contains(string(output), expectedErrorMsg) {
		t.Errorf("Expected error to contain %q, got: %q", expectedErrorMsg, string(output))
	}
}

// TestE2E_HelpCommand verifies help output contains expected sections.
func TestE2E_HelpCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Test help
	cmd := exec.Command(sharedBinaryPath, "--help")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Help command failed: %v", err)
	}

	expectedTexts := []string{
		"GoGit is a simplified Git Implementation",
		"Available Commands:",
		constants.InitCmdName,
		constants.HashObjectCmdName,
		"Flags:",
		"-h, --help",
	}

	outputStr := string(output)
	for _, text := range expectedTexts {
		if !strings.Contains(outputStr, text) {
			t.Errorf("Help output missing %q, got: %s", text, outputStr)
		}
	}
}

// TestE2E_InvalidCommand verifies error for unknown commands.
func TestE2E_InvalidCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Test invalid command
	cmd := exec.Command(sharedBinaryPath, "nonexistent")
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("Expected error for invalid command")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "unknown command") {
		t.Errorf("Expected 'unknown command' error, got: %s", outputStr)
	}
}

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

	// Build binary and run `gogit init` command
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
	decompressedContent := decompressBlobObject(t, objectPath)
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

// Helper Methods

// setupTestRepo creates test directory.
func setupTestRepo(t *testing.T) (repoPath string) {
	t.Helper()

	repoPath = filepath.Join(t.TempDir(), "test-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create test repo dir: %v", err)
	}

	return repoPath
}

// initializeRepository runs gogit init in test directory.
func initializeRepository(t *testing.T, repoPath string) {
	t.Helper()

	cmd := exec.Command(sharedBinaryPath, constants.InitCmdName)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}
}

// decompressBlobObject reads and decompresses blob object file.
func decompressBlobObject(t *testing.T, objectPath string) []byte {
	t.Helper()

	compressedData, err := os.ReadFile(objectPath)
	if err != nil {
		t.Fatalf("Failed to read object file: %v", err)
	}

	reader, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		t.Fatalf("Failed to create zlib reader: %v", err)
	}
	defer reader.Close()

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(reader); err != nil {
		t.Fatalf("Failed to read decompressed data: %v", err)
	}

	return buffer.Bytes()
}

// assertBlobContent verifies blob object format and content.
func assertBlobContent(t *testing.T, decompressedData, expectedContent []byte) {
	t.Helper()

	if !bytes.HasPrefix(decompressedData, []byte("blob ")) {
		t.Fatal("Object is not a blob")
	}

	nullByteIndex := bytes.IndexByte(decompressedData, 0)
	if nullByteIndex == -1 {
		t.Fatal("Invalid blob format: no null byte found")
	}

	content := decompressedData[nullByteIndex+1:]
	if !bytes.Equal(content, expectedContent) {
		t.Errorf("Content mismatch: expected %q, got %q", expectedContent, content)
	}
}
