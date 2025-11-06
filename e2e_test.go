package main

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/utils"
)

func TestE2E_InitCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// 1. Build the binary
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "gogit.exe")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = "."
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}

	// 2. Create test directory
	testRepoDir := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(testRepoDir, 0755); err != nil {
		t.Fatalf("Failed to create test repo dir: %v", err)
	}

	// 3. Test the binary like a real user
	cmd := exec.Command(binaryPath, "init")
	cmd.Dir = testRepoDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Binary execution failed: %v\nOutput: %s", err, output)
	}

	// 4. Verify output
	outputStr := string(output)
	expectedMsg := "Initialized empty GoGit repository in ./.gogit/\n"
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected output to contain %q, got: %s", expectedMsg, outputStr)
	}

	// 5. Verify filesystem changes
	gogitDir := filepath.Join(testRepoDir, ".gogit")
	if _, err := os.Stat(gogitDir); os.IsNotExist(err) {
		t.Error("Binary didn't create .gogit directory")
	}

	// Check required subdirectories exist
	expectedDirectories := []string{"objects", "refs", "refs/heads", "refs/tags"}
	for _, dir := range expectedDirectories {
		dirPath := filepath.Join(gogitDir, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Errorf("Required directory %s was not created: %v", dir, err)
		}
	}

	// Verify HEAD file exists and has correct content
	headPath := filepath.Join(gogitDir, "HEAD")
	content, err := os.ReadFile(headPath)
	if err != nil {
		t.Errorf("HEAD file was not created: %v", err)
	} else {
		expected := "ref: refs/heads/main\n"
		if string(content) != expected {
			t.Errorf("HEAD file content = %q, want %q", string(content), expected)
		}
	}

	// 6. Test error case - init again
	cmd = exec.Command(binaryPath, "init")
	cmd.Dir = testRepoDir
	output, err = cmd.CombinedOutput()

	if err == nil {
		t.Error("Expected error when running init twice")
	}

	expectedErrorMsg := "Error: failed to initialize repository - repository already exists at .gogit\n"
	if !strings.Contains(string(output), expectedErrorMsg) {
		t.Errorf("Expected error to contain %q, got: %q", expectedErrorMsg, string(output))
	}
}

func TestE2E_HelpCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Build binary
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "gogit.exe")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}

	// Test help
	cmd := exec.Command(binaryPath, "--help")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Help command failed: %v", err)
	}

	outputStr := string(output)
	expectedTexts := []string{
		"GoGit is a simplified Git Implementation",
		"Available Commands:",
		"init",
		"Flags:",
		"-h, --help",
	}

	for _, text := range expectedTexts {
		if !strings.Contains(outputStr, text) {
			t.Errorf("Help output missing %q, got: %s", text, outputStr)
		}
	}
}

func TestE2E_InvalidCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Build binary
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "gogit.exe")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}

	// Test invalid command
	cmd := exec.Command(binaryPath, "nonexistent")
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("Expected error for invalid command")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "unknown command") {
		t.Errorf("Expected 'unknown command' error, got: %s", outputStr)
	}
}

func TestE2E_HashObjectCommand_NoStorage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Build binary
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "gogit.exe")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}

	// Create test repo with .gogit
	testRepoDir := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(filepath.Join(testRepoDir, ".gogit", "objects"), 0755); err != nil {
		t.Fatalf("Failed to create test repo: %v", err)
	}

	// Create test file
	testFileName := "test.txt"
	testFile := filepath.Join(testRepoDir, testFileName)
	testFileContent := []byte("hello world\n")
	if err := os.WriteFile(testFile, testFileContent, 0755); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Run hash-object without -w
	cmd := exec.Command(binaryPath, "hash-object", testFileName)
	cmd.Dir = testRepoDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify hash is printed (40 hex chars + newline)
	expectedHash, _ := utils.ComputeHash(testFileContent, utils.BlobObjectType)
	outputHash := strings.TrimSpace(string(output))

	if len(outputHash) != 40 {
		t.Errorf("Expected 40-char hash, got: %s", outputHash)
	}

	if expectedHash != outputHash {
		t.Fatalf("Expected hash %s, got %s", expectedHash, outputHash)
	}

	// Verify object was NOT created (no -w flag)
	objectPath := filepath.Join(testRepoDir, outputHash[:2], outputHash[2:])
	if _, err := os.Stat(objectPath); !errors.Is(err, fs.ErrNotExist) {
		t.Error("Object should not be created without -w flag")
	}
}

func TestE2E_HashObjectCommand_WithStorage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Build binary
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "gogit.exe")

	// Execute build commnand
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}

	// Run gogit init <file-path> to initialize repository
	testRepoPath := filepath.Join(tempDir, "testRepo")
	initCmd := exec.Command(binaryPath, "init", testRepoPath)
	initCmd.Dir = tempDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("Failed to initialize repository with gogit init command: %v", err)
	}

	testFileName := "pokemon.txt"
	testFileContent := []byte("Charmander evolved into Charmeleon !")
	testFilePath := filepath.Join(testRepoPath, testFileName)
	if err := os.WriteFile(testFilePath, testFileContent, 0755); err != nil {
		t.Fatalf("Failed to write test file in test repo: %v", err)
	}

	// Run gogit hash-object file with write directive (-w)
	hashObjectCmd := exec.Command(binaryPath, "hash-object", testFileName, "-w")
	hashObjectCmd.Dir = testRepoPath

	output, err := hashObjectCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gogit hash-object command failed: %v", err)
	}

	// Verify hash was printed
	expectedHash, _ := utils.ComputeHash(testFileContent, utils.BlobObjectType)
	printedHash := strings.TrimSpace(string(output))

	if printedHash != expectedHash {
		t.Fatalf("Expected printed has to be [%s] but got [%s]", expectedHash, printedHash)
	}

	// Verify object file was created at correct path
	expectedFile := filepath.Join(testRepoPath, ".gogit", "objects", expectedHash[:2], expectedHash[2:])
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Required file %s was not created: %v", expectedFile, err)
	}

	// Verify object file is not empty (compressed data)
	info, err := os.Stat(expectedFile)
	if err != nil {
		t.Fatalf("Failed to stat object file: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Object file should not be empty")
	}

	//Verify File content
	readContent, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	// Decompress
	reader, err := zlib.NewReader(bytes.NewReader(readContent))
	if err != nil {
		t.Fatalf("failed to create reader for decompressed data: %v", err)
	}
	defer reader.Close()

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(reader); err != nil {
		t.Fatalf("failed to read decompressed data: %v", err)
	}

	// Verify object type is blob
	data := buffer.Bytes()
	if !bytes.HasPrefix(data, []byte("blob ")) {
		t.Fatalf("object %s is not a blob", expectedHash)
	}

	// Find null byte separator (end of header)
	nullByteIndex := bytes.IndexByte(data, 0)
	if nullByteIndex == -1 {
		t.Fatalf("invalid blob format: no null byte found")
	}

	// Extract content (after null byte)
	content := data[nullByteIndex+1:]

	if string(content) != string(testFileContent) {
		t.Fatalf("Expected file content to ve [%s] but got [%s]", testFileContent, content)
	}
}

func TestE2E_HashObjectCommand_InvalidArgs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Build binary
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "gogit.exe")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}

	// Test with no arguments
	cmd := exec.Command(binaryPath, "hash-object")
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("Expected error when no file argument provided")
	}

	outputStr := string(output)
	expectedMsg := "hash-object command requires exactly 1 argument (filepath), received 0"
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected error to contain %q, got: %s", expectedMsg, outputStr)
	}
}
