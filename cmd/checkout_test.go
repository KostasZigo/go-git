package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index/indextest"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestCheckoutCommand_BranchCheckout creates two commits on the default branch,
// creates a new branch ref pointing to the first commit, then checks out that
// branch. Verifies: stdout contains branch name, file content matches the first
// commit's snapshot, HEAD points to the branch, and the index contains one entry.
func TestCheckoutCommand_BranchCheckout(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	checkoutCommand := createTestRootCmd(checkoutCmd)
	stdout := captureStdout(checkoutCommand)

	// add to commits in default branch
	fileName := testutils.RandomString(10)
	fileContent := testutils.RandomByteSlice(100)
	commitWithSingleFile(t, fileName, repoPath, fileContent, testutils.RandomString(10))
	firstCommitHash := testutils.ReadDefaultRefFile(t, repoPath)

	updatedFileContent := testutils.RandomByteSlice(200)
	commitWithSingleFile(t, fileName, repoPath, updatedFileContent, testutils.RandomString(10))

	// create branch ref with fist commit
	branch := testutils.RandomString(10)
	testutils.WriteRefFile(t, repoPath, branch, firstCommitHash)

	// Execute checkout command
	checkoutCommand.SetArgs([]string{constants.CheckoutCmdName, branch})
	if err := checkoutCommand.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify stdout message
	output := stdout.String()
	expectedOutput := fmt.Sprintf("checked out [%s]\n", branch)
	if !strings.Contains(output, expectedOutput) {
		t.Fatalf("Expected output to contain branch [%s], got: [%s]", expectedOutput, output)
	}
	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), fileContent)
	testutils.AssertHEADContent(t, repoPath, constants.DefaultRefPrefix+branch+"\n")
	indextest.AssertIndexEntryPaths(t, repoPath, 1, []string{fileName})
}

// TestCheckoutCommand_NoArguments executes checkout without a target argument.
// Verifies: the command returns an error whose message matches the exactArgs
// validation format.
func TestCheckoutCommand_NoArguments(t *testing.T) {
	checkoutCommand := createTestRootCmd(checkoutCmd)
	captureStderr(checkoutCommand)
	captureStdout(checkoutCommand)

	// Execute hash-object command without any arguments
	checkoutCommand.SetArgs([]string{constants.CheckoutCmdName})
	err := checkoutCommand.Execute()

	if err == nil {
		t.Fatal("Expected error when no arguments provided")
	}

	// Verify error message matches argument validation error
	expectedErrorMessage := fmt.Sprintf("%s command requires exactly 1 argument (filepath), received 0", constants.CheckoutCmdName)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s] but got error message [%s]", expectedErrorMessage, err.Error())
	}
}

// TestCheckoutCommand_Force creates two commits, then modifies the tracked file
// on disk (dirty working tree) and checks out the first commit with the -f flag.
// Verifies: checkout succeeds despite dirty state, file content matches the first
// commit, HEAD is detached to the commit hash, and the index has one entry.
func TestCheckoutCommand_Force(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	checkoutCommand := createTestRootCmd(checkoutCmd)
	stdout := captureStdout(checkoutCommand)

	// add two commits in default branch
	fileName := testutils.RandomString(10)
	fileContent := testutils.RandomByteSlice(100)
	commitWithSingleFile(t, fileName, repoPath, fileContent, testutils.RandomString(10))
	firstCommitHash := testutils.ReadDefaultRefFile(t, repoPath)

	updatedFileContent := testutils.RandomByteSlice(200)
	commitWithSingleFile(t, fileName, repoPath, updatedFileContent, testutils.RandomString(10))

	// Introduce modification in file (dirty working tree)
	dirtyFileContent := testutils.RandomByteSlice(200)
	testutils.CreateTestFile(t, repoPath, fileName, dirtyFileContent)

	// Checkout to the first commit with "force" option
	checkoutCommand.SetArgs([]string{constants.CheckoutCmdName, firstCommitHash, "-f"})
	if err := checkoutCommand.Execute(); err != nil {
		t.Fatalf("Expected checkout to succeed with -f on dirty working directory, got: %v", err)
	}

	// Verify stdout message
	output := stdout.String()
	expectedOutput := fmt.Sprintf("checked out [%s]\n", firstCommitHash)
	if !strings.Contains(output, expectedOutput) {
		t.Fatalf("Expected stdout to contain [%s], got: [%s]", expectedOutput, output)
	}
	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), fileContent)
	testutils.AssertHEADContent(t, repoPath, firstCommitHash+"\n")
	indextest.AssertIndexEntryPaths(t, repoPath, 1, []string{fileName})
}

// TestCheckoutCommand_RoundTripBetweenBranches creates two commits on main (each
// adding a distinct file), creates a feature branch pointing to the first commit,
// checks out the feature branch, then checks out main again. Verifies: each
// checkout restores the correct file set, HEAD switches between branches, and the
// index entry count matches the target commit's tree.
func TestCheckoutCommand_RoundTripBetweenBranches(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	checkoutCommand := createTestRootCmd(checkoutCmd)
	stdout := captureStdout(checkoutCommand)

	// Add two commits to main branch
	firstFileName := testutils.RandomString(10)
	firstFileContent := testutils.RandomByteSlice(100)
	commitWithSingleFile(t, firstFileName, repoPath, firstFileContent, testutils.RandomString(10))
	firstCommitHash := testutils.ReadDefaultRefFile(t, repoPath)

	secondFileName := testutils.RandomString(10)
	secondFileContent := testutils.RandomByteSlice(100)
	commitWithSingleFile(t, secondFileName, repoPath, secondFileContent, testutils.RandomString(10))
	testutils.ReadDefaultRefFile(t, repoPath)

	// create feature branch pointing to first commit
	featureBranch := testutils.RandomString(10)
	testutils.WriteRefFile(t, repoPath, featureBranch, firstCommitHash)

	// checkout feature branch
	checkoutCommand.SetArgs([]string{constants.CheckoutCmdName, featureBranch})
	if err := checkoutCommand.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify stdout message
	output := stdout.String()
	expectedOutput := fmt.Sprintf("checked out [%s]\n", featureBranch)
	if !strings.Contains(output, expectedOutput) {
		t.Fatalf("Expected stdout to contain [%s], got: [%s]", expectedOutput, output)
	}
	testutils.AssertFileContent(t, filepath.Join(repoPath, firstFileName), firstFileContent)
	testutils.AssertHEADContent(t, repoPath, constants.DefaultRefPrefix+featureBranch+"\n")
	indextest.AssertIndexEntryPaths(t, repoPath, 1, []string{firstFileName})

	_, err := os.Stat(secondFileName)
	if err == nil || !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("Epexcted file from second commit to not exist")
	}

	// checkout back to main branch
	stdout.Reset()
	checkoutCommand.SetArgs([]string{constants.CheckoutCmdName, constants.DefaultBranch})
	if err := checkoutCommand.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify stdout message
	output = stdout.String()
	expectedOutput = fmt.Sprintf("checked out [%s]\n", constants.DefaultBranch)
	if !strings.Contains(output, expectedOutput) {
		t.Fatalf("Expected stdout to contain [%s], got: [%s]", expectedOutput, output)
	}
	testutils.AssertFileContent(t, filepath.Join(repoPath, firstFileName), firstFileContent)
	testutils.AssertFileContent(t, filepath.Join(repoPath, secondFileName), secondFileContent)
	testutils.AssertHEADContent(t, repoPath, constants.DefaultRefPrefix+constants.DefaultBranch+"\n")
	indextest.AssertIndexEntryPaths(t, repoPath, 2, []string{firstFileName, secondFileName})
}
