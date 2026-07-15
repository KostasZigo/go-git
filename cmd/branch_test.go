package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestBranchCommand_Success creates an initial commit, executes branch command,
// and verifies command output plus created branch ref content.
func TestBranchCommand_Success(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	commitWithSingleRandomFile(t, repoPath, testutils.RandomString(20))
	currentCommitHash := testutils.ReadDefaultRefFile(t, repoPath)

	newBranchName := testutils.RandomString(8)
	stdout, err := executeBranchCommand(newBranchName)
	if err != nil {
		t.Fatalf("branch command failed: %v", err)
	}

	expectedOutput := fmt.Sprintf("created branch [%s]\n", newBranchName)
	if !strings.Contains(stdout, expectedOutput) {
		t.Fatalf("expected stdout to contain [%s], got [%s]", expectedOutput, stdout)
	}

	createdBranchRefPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, newBranchName)
	testutils.AssertFileContent(t, createdBranchRefPath, []byte(currentCommitHash+"\n"))
}

// TestBranchCommand_InvalidNumberOfArguments verifies exact-args validation
// for missing and extra branch command arguments.
func TestBranchCommand_InvalidNumberOfArguments(t *testing.T) {
	branchCommand := createTestRootCmd(branchCmd)
	captureStderr(branchCommand)
	captureStdout(branchCommand)

	// Execute hash-object command without any arguments
	branchCommand.SetArgs([]string{constants.BranchCmdName})
	err := branchCommand.Execute()
	if err == nil {
		t.Fatal("Expected error when no arguments provided")
	}

	// Verify error message matches argument validation error
	expectedErrorMessage := fmt.Sprintf("%s command requires exactly 1 argument, received 0", constants.BranchCmdName)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", expectedErrorMessage, err.Error())
	}

	// Execute hash-object command without more arguments than needed
	branchCommand.SetArgs([]string{
		constants.BranchCmdName,
		testutils.RandomString(5),
		testutils.RandomString(5),
	})
	err = branchCommand.Execute()
	if err == nil {
		t.Fatal("Expected error when more than 1 arguments provided")
	}

	// Verify error message matches argument validation error
	expectedErrorMessage = fmt.Sprintf("%s command requires exactly 1 argument, received 2", constants.BranchCmdName)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", expectedErrorMessage, err.Error())
	}
}

// TestBranchCommand_RepoNotFound verifies branch command failure when executed
// outside a repository that contains the .gogit directory.
func TestBranchCommand_RepoNotFound(t *testing.T) {
	newBranchName := testutils.RandomString(8)
	_, err := executeBranchCommand(newBranchName)
	if err == nil {
		t.Fatal("Expected error when repository is not found.")
	}

	expectedOutput := fmt.Sprintf("%s directory not found", constants.Gogit)
	if !strings.Contains(err.Error(), expectedOutput) {
		t.Fatalf("expected stdout to contain [%s], got [%s]", expectedOutput, err.Error())
	}
}

// TestBranchCommand_Failure_BranchAlreadyExists verifies command failure when
// attempting to create a branch that already exists.
func TestBranchCommand_Failure_BranchAlreadyExists(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)
	commitWithSingleRandomFile(t, repoPath, testutils.RandomString(20))

	newBranchName := constants.DefaultBranch
	_, err := executeBranchCommand(newBranchName)
	if err == nil {
		t.Fatal("Expected error when trying to create existing branch.")
	}

	expectedOutput := fmt.Sprintf("branch [%s] already exists", newBranchName)
	if !strings.Contains(err.Error(), expectedOutput) {
		t.Fatalf("expected stdout to contain [%s], got [%s]", expectedOutput, err.Error())
	}
}

// TestBranchCommand_Success_BranchNameIsTrimmed verifies that user input with
// surrounding spaces is normalized and branch creation/output use trimmed name.
func TestBranchCommand_Success_BranchNameIsTrimmed(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	commitWithSingleRandomFile(t, repoPath, testutils.RandomString(20))
	currentCommitHash := testutils.ReadDefaultRefFile(t, repoPath)

	newBranchName := " " + testutils.RandomString(8) + " "
	stdout, err := executeBranchCommand(newBranchName)
	if err != nil {
		t.Fatalf("branch command failed: %v", err)
	}

	newBranchNameTrimmed := strings.TrimSpace(newBranchName)
	expectedOutput := fmt.Sprintf("created branch [%s]\n", newBranchNameTrimmed)
	if !strings.Contains(stdout, expectedOutput) {
		t.Fatalf("expected stdout to contain [%s], got [%s]", expectedOutput, stdout)
	}

	createdBranchRefPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, newBranchNameTrimmed)
	testutils.AssertFileContent(t, createdBranchRefPath, []byte(currentCommitHash+"\n"))
}
