package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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

// TestE2E_AddCommand_SingleFile verifies staging single file creates blob and updates index.
func TestE2E_AddCommand_SingleFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	testFileName := testutils.RandomString(10)
	testFileContent := []byte(testutils.RandomString(100))
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	cmd := exec.Command(sharedBinaryPath, constants.AddCmdName, testFileName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Add command failed: %v\nOutput: %s", err, output)
	}

	assertAddCommandOutputAndObjectCreation(t, testFileName, output, testFileContent, repoPath)

	expectedFiles := map[string][]byte{
		testFileName: testFileContent,
	}
	assertIndexCreationAndContent(t, repoPath, expectedFiles)
}

// TestE2E_AddCommand_MultipleFiles verifies staging multiple files in single command.
func TestE2E_AddCommand_MultipleFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	files := []struct {
		name    string
		content []byte
	}{
		{testutils.RandomString(10), []byte(testutils.RandomString(100))},
		{testutils.RandomString(10), []byte(testutils.RandomString(100))},
		{testutils.RandomString(10), []byte(testutils.RandomString(100))},
	}

	for _, file := range files {
		testutils.CreateTestFile(t, repoPath, file.name, file.content)
	}

	args := []string{constants.AddCmdName}
	for _, file := range files {
		args = append(args, file.name)
	}

	cmd := exec.Command(sharedBinaryPath, args...)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Add command failed: %v\nOutput: %s", err, output)
	}

	for _, file := range files {
		assertAddCommandOutputAndObjectCreation(t, file.name, output, file.content, repoPath)
	}

	expectedFiles := map[string][]byte{}
	for _, file := range files {
		expectedFiles[file.name] = file.content
	}
	assertIndexCreationAndContent(t, repoPath, expectedFiles)
}

// TestE2E_AddCommand_FileNotFound verifies error for nonexistent file.
func TestE2E_AddCommand_FileNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	testFileName := testutils.RandomString(10)
	cmd := exec.Command(sharedBinaryPath, constants.AddCmdName, testFileName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("Expected error when trying to add a non-existing file.")
	}

	outputStr := string(output)
	expectedErrorMessage := "Error: failed to add file " + testFileName + ": failed to stat file "
	if !strings.Contains(outputStr, expectedErrorMessage) {
		t.Errorf("Expected [%s] error, got: %v", expectedErrorMessage, outputStr)
	}
}

// TestE2E_AddCommand_NotInRepository verifies error when outside repository.
func TestE2E_AddCommand_NotInRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)

	testFileName := testutils.RandomString(10)
	testFileContent := []byte(testutils.RandomString(100))
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	cmd := exec.Command(sharedBinaryPath, constants.AddCmdName, testFileName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("Expected error when trying to add a file that does not belong to an initialized repository.")
	}

	outputStr := string(output)
	expectedErrorMessage := constants.Gogit + " directory not found"
	if !strings.Contains(outputStr, expectedErrorMessage) {
		t.Errorf("Expected [%s] error, got: %v", expectedErrorMessage, outputStr)
	}
}

// TestE2E_AddCommand_UpdateExistingFile verifies updating already-staged file.
func TestE2E_AddCommand_UpdateExistingFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	testFileName := testutils.RandomString(10)
	testFileContent := []byte(testutils.RandomString(100))
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	cmd := exec.Command(sharedBinaryPath, constants.AddCmdName, testFileName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Add command failed: %v\nOutput: %s", err, output)
	}

	assertAddCommandOutputAndObjectCreation(t, testFileName, output, testFileContent, repoPath)

	expectedFiles := map[string][]byte{
		testFileName: testFileContent,
	}
	assertIndexCreationAndContent(t, repoPath, expectedFiles)

	// update existing file's content and add it to the index again
	testFileContentUpdated := []byte(testutils.RandomString(1000))
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContentUpdated)

	cmd = exec.Command(sharedBinaryPath, constants.AddCmdName, testFileName)
	cmd.Dir = repoPath
	output, err = cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Add command failed: %v\nOutput: %s", err, output)
	}

	assertAddCommandOutputAndObjectCreation(t, testFileName, output, testFileContentUpdated, repoPath)

	expectedFiles = map[string][]byte{
		testFileName: testFileContentUpdated,
	}
	assertIndexCreationAndContent(t, repoPath, expectedFiles)
}

// TestE2E_AddCommand_NoArguments verifies error when no files specified.
func TestE2E_AddCommand_NoArguments(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	cmd := exec.Command(sharedBinaryPath, constants.AddCmdName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("Expected error when no arguments provided")
	}

	outputStr := string(output)
	expectedMsg := fmt.Sprintf("%s command accepts at least %d arg(s), received %d", constants.AddCmdName, 1, 0)
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected error to contain %q, got: %s", expectedMsg, outputStr)
	}
}

// TestE2E_AddCommand_FileInSubdirectory verifies staging file in nested directory.
func TestE2E_AddCommand_FileInSubdirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	subDir := filepath.Join("src", "pkg")
	if err := os.MkdirAll(filepath.Join(repoPath, subDir), constants.DirPerms); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	testFileName := filepath.Join(subDir, "module.go")
	testFileContent := []byte(testutils.RandomString(100))
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	cmd := exec.Command(sharedBinaryPath, constants.AddCmdName, testFileName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Add command failed: %v\nOutput: %s", err, output)
	}

	assertAddCommandOutputAndObjectCreation(t, testFileName, output, testFileContent, repoPath)

	expectedFiles := map[string][]byte{
		testFileName: testFileContent,
	}
	assertIndexCreationAndContent(t, repoPath, expectedFiles)
}

// TestE2E_AddCommand_SameContentDifferentFiles verifies identical content produces same hash.
func TestE2E_AddCommand_SameContentDifferentFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	sharedContent := []byte(testutils.RandomString(1000))
	file1 := testutils.RandomString(10)
	file2 := testutils.RandomString(10)

	testutils.CreateTestFile(t, repoPath, file1, sharedContent)
	testutils.CreateTestFile(t, repoPath, file2, sharedContent)

	cmd := exec.Command(sharedBinaryPath, constants.AddCmdName, file1, file2)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Add command failed: %v\nOutput: %s", err, output)
	}

	fileNames := []string{file1, file2}
	for _, fileName := range fileNames {
		assertAddCommandOutputAndObjectCreation(t, fileName, output, sharedContent, repoPath)
	}

	expectedFiles := map[string][]byte{}
	for _, fileName := range fileNames {
		expectedFiles[fileName] = sharedContent
	}
	assertIndexCreationAndContent(t, repoPath, expectedFiles)
}

// TestE2E_AddCommand_IdempotentAdd verifies adding same file twice is idempotent.
func TestE2E_AddCommand_IdempotentAdd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	testFileName := testutils.RandomString(10)
	testFileContent := []byte(testutils.RandomString(1000))
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	cmd1 := exec.Command(sharedBinaryPath, constants.AddCmdName, testFileName)
	cmd1.Dir = repoPath
	if _, err := cmd1.CombinedOutput(); err != nil {
		t.Fatalf("First add failed: %v", err)
	}

	cmd2 := exec.Command(sharedBinaryPath, constants.AddCmdName, testFileName)
	cmd2.Dir = repoPath
	output, err := cmd2.CombinedOutput()
	if err != nil {
		t.Fatalf("Second add failed: %v", err)
	}

	assertAddCommandOutputAndObjectCreation(t, testFileName, output, testFileContent, repoPath)

	expectedFiles := map[string][]byte{
		testFileName: testFileContent,
	}
	assertIndexCreationAndContent(t, repoPath, expectedFiles)
}

// TestE2E_AddCommand_AddAll verifies staging all files with "."
func TestE2E_AddCommand_AddAll(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	// Create file structure
	files := []struct {
		name    string
		content []byte
	}{
		{testutils.RandomString(10), []byte(testutils.RandomString(100))},
		{testutils.RandomString(10), []byte(testutils.RandomString(100))},
		{filepath.Join(testutils.RandomString(10), testutils.RandomString(10)), []byte(testutils.RandomString(100))},
		{filepath.Join(testutils.RandomString(10), testutils.RandomString(10)), []byte(testutils.RandomString(100))},
	}

	for _, file := range files {
		dir := filepath.Dir(file.name)
		if dir != "." {
			if err := os.MkdirAll(filepath.Join(repoPath, dir), 0755); err != nil {
				t.Fatalf("Failed to create directory: %v", err)
			}
		}
		testutils.CreateTestFile(t, repoPath, file.name, file.content)
	}

	cmd := exec.Command(sharedBinaryPath, constants.AddCmdName, ".")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("add . failed: %v\nOutput: %s", err, output)
	}

	for _, file := range files {
		assertAddCommandOutputAndObjectCreation(t, file.name, output, file.content, repoPath)
	}

	expectedFiles := map[string][]byte{}
	for _, file := range files {
		expectedFiles[file.name] = file.content
	}
	assertIndexCreationAndContent(t, repoPath, expectedFiles)
}

// Commit e2e testing

// TestE2E_CommitCommand_FirstCommit verifies the full init → add → commit workflow.
// Stages a file and commits with a message. Verifies exit code 0, stdout contains
// the short commit hash and message, ref file is created, and the commit object
// is stored with correct tree reference.
func TestE2E_CommitCommand_FirstCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	// Create and stage a file
	testFileName := testutils.RandomString(10)
	testFileContent := []byte(testutils.RandomString(100))
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	addCmd := exec.Command(sharedBinaryPath, constants.AddCmdName, testFileName)
	addCmd.Dir = repoPath
	if output, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("add command failed: %v\nOutput: %s", err, output)
	}

	// Execute commit
	commitMessage := "initial commit"
	commitCmd := exec.Command(sharedBinaryPath, constants.CommitCmdName, "-m", commitMessage)
	commitCmd.Dir = repoPath
	output, err := commitCmd.CombinedOutput()

	if err != nil {
		t.Fatalf("commit command failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)

	// Verify output contains the commit message
	if !strings.Contains(outputStr, commitMessage) {
		t.Fatalf("Expected output to contain message [%s], got: [%s]", commitMessage, outputStr)
	}

	// Read ref file and verify it contains a valid hash
	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, constants.DefaultBranch)
	refContent, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("Failed to read ref file: %v", err)
	}
	commitHash := strings.TrimSpace(string(refContent))

	if len(commitHash) != constants.HashStringLength {
		t.Fatalf("Expected %d-char hash in ref file, got %d: %s", constants.HashStringLength, len(commitHash), commitHash)
	}

	// Verify output contains the short hash
	shortHash := commitHash[:7]
	if !strings.Contains(outputStr, shortHash) {
		t.Fatalf("Expected output to contain short hash %q, got: %s", shortHash, outputStr)
	}

	// Verify commit object exists
	commitObjectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, commitHash[:constants.HashDirPrefixLength], commitHash[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, commitObjectPath)

	commitData := decompressObject(t, commitObjectPath)
	assertCommitObjectContent(t, commitData, commitMessage)

	// Verify tree object exists and has correct type header
	commitBody := extractObjectBody(t, commitData)
	treeHash := extractFieldFromCommitBody(t, commitBody, "tree")
	if len(treeHash) != constants.HashStringLength {
		t.Fatalf("Expected %d-char tree hash, got %d: %q", constants.HashStringLength, len(treeHash), treeHash)
	}

	treeObjectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, treeHash[:constants.HashDirPrefixLength], treeHash[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, treeObjectPath)

	treeData := decompressObject(t, treeObjectPath)
	if !bytes.HasPrefix(treeData, []byte("tree ")) {
		t.Fatalf("Tree object has wrong type header: %q", string(treeData[:20]))
	}

	// Verify blob object for the staged file exists
	expectedBlobHash, err := utils.ComputeHash(testFileContent, utils.BlobObjectType)
	if err != nil {
		t.Fatalf("Failed to compute expected blob hash: %v", err)
	}
	blobObjectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, expectedBlobHash[:constants.HashDirPrefixLength], expectedBlobHash[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, blobObjectPath)
}

// TestE2E_CommitCommand_FullWorkflow verifies the multi-commit workflow:
// init → add → commit → modify → add → commit. Verifies two distinct commits
// are created, the second commit's parent references the first, both commit
// objects exist in the object store, and the ref file points to the latest commit.
func TestE2E_CommitCommand_FullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	testFileName := testutils.RandomString(10)
	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, constants.DefaultBranch)

	// First: create, stage, commit
	testutils.CreateTestFile(t, repoPath, testFileName, []byte("version 1"))

	addCmd1 := exec.Command(sharedBinaryPath, constants.AddCmdName, testFileName)
	addCmd1.Dir = repoPath
	if output, err := addCmd1.CombinedOutput(); err != nil {
		t.Fatalf("first add failed: %v\nOutput: %s", err, output)
	}

	commitCmd1 := exec.Command(sharedBinaryPath, constants.CommitCmdName, "-m", "first")
	commitCmd1.Dir = repoPath
	if output, err := commitCmd1.CombinedOutput(); err != nil {
		t.Fatalf("first commit failed: %v\nOutput: %s", err, output)
	}

	firstHashBytes, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("Failed to read ref after first commit: %v", err)
	}
	firstHash := strings.TrimSpace(string(firstHashBytes))

	// Second: modify, re-stage, commit
	testutils.CreateTestFile(t, repoPath, testFileName, []byte("version 2"))

	addCmd2 := exec.Command(sharedBinaryPath, constants.AddCmdName, testFileName)
	addCmd2.Dir = repoPath
	if output, err := addCmd2.CombinedOutput(); err != nil {
		t.Fatalf("second add failed: %v\nOutput: %s", err, output)
	}

	commitCmd2 := exec.Command(sharedBinaryPath, constants.CommitCmdName, "-m", "second")
	commitCmd2.Dir = repoPath
	output, err := commitCmd2.CombinedOutput()
	if err != nil {
		t.Fatalf("second commit failed: %v\nOutput: %s", err, output)
	}

	secondHashBytes, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("Failed to read ref after second commit: %v", err)
	}
	secondHash := strings.TrimSpace(string(secondHashBytes))

	// Hashes must differ
	if firstHash == secondHash {
		t.Fatal("First and second commit hashes must differ")
	}

	// Both commit objects must exist
	firstObjectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, firstHash[:constants.HashDirPrefixLength], firstHash[constants.HashDirPrefixLength:])
	secondObjectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, secondHash[:constants.HashDirPrefixLength], secondHash[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, firstObjectPath)
	testutils.AssertFileExists(t, secondObjectPath)

	// Verify second commit references first as parent
	secondCommitData := decompressObject(t, secondObjectPath)
	secondCommitBody := extractObjectBody(t, secondCommitData)

	if !strings.Contains(secondCommitBody, "parent "+firstHash) {
		t.Fatalf("Second commit missing parent reference to first commit.\nExpected parent: %s\nCommit body:\n%s", firstHash, secondCommitBody)
	}

	// Verify first commit has no parent
	firstCommitData := decompressObject(t, firstObjectPath)
	firstCommitBody := extractObjectBody(t, firstCommitData)

	if strings.Contains(firstCommitBody, "parent ") {
		t.Fatalf("First commit should have no parent.\nCommit body:\n%s", firstCommitBody)
	}

	// Extract tree hashes from both commits and verify they differ
	firstTreeHash := extractFieldFromCommitBody(t, firstCommitBody, "tree")
	secondTreeHash := extractFieldFromCommitBody(t, secondCommitBody, "tree")

	if firstTreeHash == secondTreeHash {
		t.Fatal("Tree hashes must differ between commits with different file content")
	}

	// Verify both tree objects exist and have correct type header
	for _, treeHash := range []string{firstTreeHash, secondTreeHash} {
		treeObjectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, treeHash[:constants.HashDirPrefixLength], treeHash[constants.HashDirPrefixLength:])
		testutils.AssertFileExists(t, treeObjectPath)

		treeData := decompressObject(t, treeObjectPath)
		if !bytes.HasPrefix(treeData, []byte("tree ")) {
			t.Fatalf("Object %s is not a tree: %q", treeHash[:7], string(treeData[:20]))
		}
	}

	// Verify ref file points to second commit
	if secondHash != strings.TrimSpace(string(secondHashBytes)) {
		t.Fatalf("Ref file should point to second commit %s", secondHash)
	}
}

// TestE2E_CommitCommand_NoStagedFiles verifies commit fails with non-zero exit code
// and appropriate error message when no files have been staged.
func TestE2E_CommitCommand_NoStagedFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	commitCmd := exec.Command(sharedBinaryPath, constants.CommitCmdName, "-m", "empty commit")
	commitCmd.Dir = repoPath
	output, err := commitCmd.CombinedOutput()

	if err == nil {
		t.Fatal("Expected non-zero exit code for commit with no staged files")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "nothing to commit") {
		t.Fatalf("Expected error containing 'nothing to commit', got: %s", outputStr)
	}

	// Verify ref file was NOT created
	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, constants.DefaultBranch)
	if _, err := os.Stat(refPath); err == nil {
		t.Fatal("Ref file should not exist when commit fails")
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

// decompressObject reads and decompresses any git object file.
// Returns the full decompressed content including the header.
func decompressObject(t *testing.T, objectPath string) []byte {
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
		t.Fatalf("Failed to decompress object: %v", err)
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

// assertAddCommandOutputAndObjectCreation verifies add command output and blob object creation and content.
func assertAddCommandOutputAndObjectCreation(t *testing.T, testFileName string, output []byte, testFileContent []byte, repoPath string) {
	expectedOutput := fmt.Sprintf("add '%s'", testFileName)
	if !strings.Contains(string(output), expectedOutput) {
		t.Errorf("Expected output to contain %q, got: %s", expectedOutput, string(output))
	}

	expectedHash, err := utils.ComputeHash(testFileContent, utils.BlobObjectType)
	if err != nil {
		t.Fatalf("Failed to compute hash: %v", err)
	}

	objectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, expectedHash[:constants.HashDirPrefixLength], expectedHash[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, objectPath)

	//Verify File content
	decompressedContent := decompressObject(t, objectPath)
	assertBlobContent(t, decompressedContent, testFileContent)
}

// assertIndexCreationAndContent verifies index cretion and content
func assertIndexCreationAndContent(t *testing.T, repoPath string, expectedFiles map[string][]byte) {
	indexPath := filepath.Join(repoPath, constants.Gogit, constants.Index)
	testutils.AssertFileExists(t, indexPath)

	assertIndexContent(t, indexPath, expectedFiles)
}

// assertIndexContent verifies index content
func assertIndexContent(t *testing.T, indexPath string, expectedFiles map[string][]byte) {
	t.Helper()

	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("Failed to read index file: %v", err)
	}

	reader := bytes.NewReader(indexData)

	readAndAssertIndexHeader(t, reader, expectedFiles)
	readAndAssertIndexEntries(t, reader, expectedFiles)
	_, err = reader.ReadByte()
	if err == nil {
		t.Fatal("Expeceted an error when trying to read form the index while its meant to have reached EOF")
	}
}

// readAndAssertIndexHeader verifies index header content
func readAndAssertIndexHeader(t *testing.T, reader *bytes.Reader, expectedFiles map[string][]byte) {
	signature := make([]byte, 4)
	if _, err := io.ReadFull(reader, signature); err != nil {
		t.Fatalf("Failed to read signature: %v", err)
	}
	if string(signature) != constants.IndexSignature {
		t.Fatalf("Invalid signature: expected %s, got %s", constants.IndexSignature, string(signature))
	}

	var version uint32
	if err := binary.Read(reader, binary.BigEndian, &version); err != nil {
		t.Fatalf("Failed to read version: %v", err)
	}
	if version != constants.IndexVersion {
		t.Fatalf("Invalid version: expected %d, got %d", constants.IndexVersion, version)
	}

	var entryCount uint32
	if err := binary.Read(reader, binary.BigEndian, &entryCount); err != nil {
		t.Fatalf("Failed to read entry count: %v", err)
	}
	if int(entryCount) != len(expectedFiles) {
		t.Fatalf("Entry count mismatch: expected %d, got %d", len(expectedFiles), entryCount)
	}
}

// readAndAssertIndexEntries verifies index entries content
func readAndAssertIndexEntries(t *testing.T, reader *bytes.Reader, expectedFiles map[string][]byte) {
	// Sort the map keys that correspond to the filepats as they are expected to be sorted inside the index
	keys := make([]string, 0, len(expectedFiles))
	for k := range expectedFiles {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, key := range keys {
		// Verify expected file was indexed
		expectedContent, _ := expectedFiles[key]
		parseAndAssertIndexEntry(t, reader, key, expectedContent)
	}
}

// parseIndexEntry reads single entry from binary stream.
func parseAndAssertIndexEntry(t *testing.T, reader *bytes.Reader, filepath string, expectedContent []byte) {
	t.Helper()

	// File mode (4 bytes)
	var fileMode uint32
	if err := binary.Read(reader, binary.BigEndian, &fileMode); err != nil {
		t.Fatalf("Failed to read file mode: %v", err)
	}

	// Object hash (20 bytes)
	hashBytes := make([]byte, constants.HashByteLength)
	if _, err := io.ReadFull(reader, hashBytes); err != nil {
		t.Fatalf("Failed to read hash: %v", err)
	}

	// Verify hash matches file content
	expectedHash, err := utils.ComputeHash(expectedContent, utils.BlobObjectType)
	if err != nil {
		t.Fatalf("Failed to compute expected hash for %s: %v", filepath, err)
	}

	hash := fmt.Sprintf("%x", hashBytes)
	if hash != expectedHash {
		t.Fatalf("Hash mismatch for %s: expected %s, got %s", filepath, expectedHash, hash)
	}

	// File size (8 bytes)
	var fileSize int64
	if err := binary.Read(reader, binary.BigEndian, &fileSize); err != nil {
		t.Fatalf("Failed to read file size: %v", err)
	}
	// Verify file size
	if fileSize != int64(len(expectedContent)) {
		t.Fatalf("Size mismatch for %s: expected %d, got %d", filepath, len(expectedContent), fileSize)
	}

	// Modified time (8 bytes)
	var lastModified int64
	if err := binary.Read(reader, binary.BigEndian, &lastModified); err != nil {
		t.Fatalf("Failed to read modified time: %v", err)
	}

	// Path length (2 bytes)
	var pathLength uint16
	if err := binary.Read(reader, binary.BigEndian, &pathLength); err != nil {
		t.Fatalf("Failed to read path length: %v", err)
	}

	// Path (N bytes)
	pathBytes := make([]byte, pathLength)
	if _, err := io.ReadFull(reader, pathBytes); err != nil {
		t.Fatalf("Failed to read path: %v", err)
	}
	// verify expected file path
	if string(pathBytes) != filepath {
		t.Fatalf("Expected file path to be [%s] but got [%s]", filepath, string(pathBytes))
	}

	// Null terminator (1 byte)
	var nullByte byte
	if err := binary.Read(reader, binary.BigEndian, &nullByte); err != nil {
		t.Fatalf("Failed to read null terminator: %v", err)
	}
	if nullByte != constants.NullByte {
		t.Errorf("Invalid null terminator: expected 0x00, got 0x%02x", nullByte)
	}
}

// extractObjectBody strips the git object header ("type size\0") and returns
// the body content as a string. Fails the test if the null separator is missing.
func extractObjectBody(t *testing.T, data []byte) string {
	t.Helper()

	nullIndex := bytes.IndexByte(data, 0)
	if nullIndex == -1 {
		t.Fatal("Invalid object format: no null byte separator found")
	}

	return string(data[nullIndex+1:])
}

// extractFieldFromCommitBody scans the commit body for a line starting with
// "<field> " and returns the value after the space.
// Returns empty string if the field is not found.
func extractFieldFromCommitBody(t *testing.T, body, field string) string {
	t.Helper()

	prefix := field + " "
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}

	return ""
}

// assertCommitObjectContent verifies a decompressed commit object has the correct
// type header and contains the expected commit message.
func assertCommitObjectContent(t *testing.T, decompressedData []byte, expectedMessage string) {
	t.Helper()

	if !bytes.HasPrefix(decompressedData, []byte("commit ")) {
		t.Fatalf("Object is not a commit, starts with: %q", string(decompressedData[:20]))
	}

	body := extractObjectBody(t, decompressedData)

	if !strings.Contains(body, "tree ") {
		t.Fatal("Commit object missing 'tree' field")
	}

	if !strings.Contains(body, "author ") {
		t.Fatal("Commit object missing 'author' field")
	}

	if !strings.Contains(body, expectedMessage) {
		t.Fatalf("Commit object missing message %q.\nCommit body:\n%s", expectedMessage, body)
	}
}
