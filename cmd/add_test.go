package cmd

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestAddCommand_NotInRepository verifies error when outside repository.
func TestAddCommand_NotInRepository(t *testing.T) {
	tempDir := t.TempDir()
	testutils.ChangeToDir(t, tempDir)

	testFileName := testutils.RandomString(10)
	testutils.CreateTestFile(t, tempDir, testFileName, []byte(testutils.RandomString(100)))

	testRootCmd := createTestRootCmd(addCmd)
	stderr := captureStderr(testRootCmd)

	testRootCmd.SetArgs([]string{constants.AddCmdName, testFileName})
	if err := testRootCmd.Execute(); err == nil {
		t.Fatal("expected error when file not in a repository")
	}

	expectedErrorMessage := constants.Gogit + " directory not found"
	if !strings.Contains(stderr.String(), expectedErrorMessage) {
		t.Errorf("expected [%s] error, got: %v", expectedErrorMessage, stderr.String())
	}
}

// TestAddCommand_OutputFormat verifies the CLI prints staged file paths.
func TestAddCommand_OutputFormat(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	testFileName := testutils.RandomString(10)
	testutils.CreateTestFile(t, repoPath, testFileName, []byte(testutils.RandomString(100)))

	testRootCmd := createTestRootCmd(addCmd)
	stdout := captureStdout(testRootCmd)

	testRootCmd.SetArgs([]string{constants.AddCmdName, testFileName})
	if err := testRootCmd.Execute(); err != nil {
		t.Fatalf("add command failed: %v", err)
	}

	expectedOutput := "add '" + testFileName + "'"
	if !strings.Contains(stdout.String(), expectedOutput) {
		t.Errorf("expected output to contain %q, got: %q", expectedOutput, stdout.String())
	}
}

// TestAddCommand_DeletionOutputFormat verifies that explicitly staging a
// tracked deletion prints the canonical deleted-path result.
func TestAddCommand_DeletionOutputFormat(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	fileName := testutils.RandomString(10)
	filePath := testutils.CreateTestFile(t, repoPath, fileName, testutils.RandomBytes(20))
	stageCommand := createTestRootCmd(addCmd)
	captureStdout(stageCommand)
	stageCommand.SetArgs([]string{constants.AddCmdName, fileName})
	if err := stageCommand.Execute(); err != nil {
		t.Fatalf("failed to stage initial file: %v", err)
	}
	if err := os.Remove(filePath); err != nil {
		t.Fatalf("failed to remove tracked file: %v", err)
	}

	deleteCommand := createTestRootCmd(addCmd)
	stdout := captureStdout(deleteCommand)
	deleteCommand.SetArgs([]string{constants.AddCmdName, fileName})
	if err := deleteCommand.Execute(); err != nil {
		t.Fatalf("failed to stage deletion: %v", err)
	}

	expectedOutput := "deleted '" + fileName + "'\n"
	if stdout.String() != expectedOutput {
		t.Fatalf("expected output [%q], got [%q]", expectedOutput, stdout.String())
	}
}

// TestAddCommand_OutputFormat_MulitpleFiles verifies the CLI prints staged file paths correctly.
func TestAddCommand_OutputFormat_MulitpleFiles(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	fileNames := make([]string, 0, 3)
	for range 3 {
		testFileName := testutils.RandomString(10)
		testutils.CreateTestFile(t, repoPath, testFileName, []byte(testutils.RandomString(100)))
		fileNames = append(fileNames, testFileName)
	}
	slices.Sort(fileNames)

	testRootCmd := createTestRootCmd(addCmd)
	stdout := captureStdout(testRootCmd)

	testRootCmd.SetArgs([]string{constants.AddCmdName, "."})
	if err := testRootCmd.Execute(); err != nil {
		t.Fatalf("add command failed: %v", err)
	}

	var expectedOutput strings.Builder
	for _, fileName := range fileNames {
		expectedOutput.WriteString("add '")
		expectedOutput.WriteString(fileName)
		expectedOutput.WriteString("'\n")
	}
	if !strings.Contains(stdout.String(), expectedOutput.String()) {
		t.Errorf("expected output to contain %q, got: %q", expectedOutput.String(), stdout.String())
	}
}
