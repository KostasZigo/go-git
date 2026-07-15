package e2etesting

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/testutils"
)

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

	cmd := newGogitCmd(t, constants.AddCmdName, testFileName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("add command failed: %v\nOutput: %s", err, output)
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

	args := make([]string, 0, len(files)+1)
	args = append(args, constants.AddCmdName)
	for _, file := range files {
		args = append(args, file.name)
	}

	cmd := newGogitCmd(t, args...)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("add command failed: %v\nOutput: %s", err, output)
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
	cmd := newGogitCmd(t, constants.AddCmdName, testFileName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected error when trying to add a non-existing file")
	}

	outputStr := string(output)
	expectedErrorMessage := "Error: failed to add file " + testFileName + ": failed to stat file "
	if !strings.Contains(outputStr, expectedErrorMessage) {
		t.Errorf("expected [%s] error, got: %v", expectedErrorMessage, outputStr)
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

	cmd := newGogitCmd(t, constants.AddCmdName, testFileName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected error when trying to add a file that does not belong to an initialized repository")
	}

	outputStr := string(output)
	expectedErrorMessage := constants.Gogit + " directory not found"
	if !strings.Contains(outputStr, expectedErrorMessage) {
		t.Errorf("expected [%s] error, got: %v", expectedErrorMessage, outputStr)
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

	cmd := newGogitCmd(t, constants.AddCmdName, testFileName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("add command failed: %v\nOutput: %s", err, output)
	}

	assertAddCommandOutputAndObjectCreation(t, testFileName, output, testFileContent, repoPath)

	expectedFiles := map[string][]byte{
		testFileName: testFileContent,
	}
	assertIndexCreationAndContent(t, repoPath, expectedFiles)

	// update existing file's content and add it to the index again
	testFileContentUpdated := []byte(testutils.RandomString(1000))
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContentUpdated)

	cmd = newGogitCmd(t, constants.AddCmdName, testFileName)
	cmd.Dir = repoPath
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("add command failed: %v\nOutput: %s", err, output)
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

	cmd := newGogitCmd(t, constants.AddCmdName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("expected error when no arguments provided")
	}

	outputStr := string(output)
	expectedMsg := fmt.Sprintf("%s command accepts at least %d arg(s), received %d", constants.AddCmdName, 1, 0)
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("expected error to contain %q, got: %s", expectedMsg, outputStr)
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
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	testFileName := filepath.Join(subDir, "module.go")
	testFileContent := []byte(testutils.RandomString(100))
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	cmd := newGogitCmd(t, constants.AddCmdName, testFileName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("add command failed: %v\nOutput: %s", err, output)
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

	cmd := newGogitCmd(t, constants.AddCmdName, file1, file2)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("add command failed: %v\nOutput: %s", err, output)
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

	cmd1 := newGogitCmd(t, constants.AddCmdName, testFileName)
	cmd1.Dir = repoPath
	if _, err := cmd1.CombinedOutput(); err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	cmd2 := newGogitCmd(t, constants.AddCmdName, testFileName)
	cmd2.Dir = repoPath
	_, err := cmd2.CombinedOutput()
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}

	assertAddCommandObjectCreation(t, testFileName, testFileContent, repoPath)

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
			if err := os.MkdirAll(filepath.Join(repoPath, dir), 0o755); err != nil {
				t.Fatalf("failed to create directory: %v", err)
			}
		}
		testutils.CreateTestFile(t, repoPath, file.name, file.content)
	}

	cmd := newGogitCmd(t, constants.AddCmdName, ".")
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
