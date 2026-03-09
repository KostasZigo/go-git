package cmd

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/testutils"
	"github.com/KostasZigo/gogit/utils"
)

// TestLog_SingleCommit_Sucess stages and commits a single file, then
// executes the log command. Verifies that stdout contains the commit
// hash, message, author, and date in the expected format.
func TestLog_SingleCommit_Sucess(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	changeToRepoDir(t, repoPath)

	// Create commit history
	message := testutils.RandomString(100)
	commitWithSingleFile(t, repoPath, message)

	commitHash := readRefFile(t, repoPath)

	store := objects.NewObjectStore(repoPath)
	commit, err := store.ReadCommit(commitHash)
	if err != nil {
		t.Fatalf("Failed to read commit object: %v", err)
	}

	// Execute Log command
	command := createTestRootCmd(logCmd)
	stdout := captureStdout(command)
	command.SetArgs([]string{constants.LogCmdName})
	if err := command.Execute(); err != nil {
		t.Fatalf("log command failed: %v", err)
	}

	output := stdout.String()
	expectedOutput := utils.FormatCommitLogEntry(
		commit.Hash(),
		commit.Message(),
		commit.Author().String(),
		commit.Author().Time(),
	)

	if output != expectedOutput {
		t.Fatalf("Expected commit history output to be [%s], got [%s]", expectedOutput, output)
	}
}

// TestLog_CommitChain_Sucess creates three sequential commits, then
// executes the log command. Verifies that stdout contains all three
// entries in reverse chronological order (most recent first) with
// correct hash, message, author, and date for each.
func TestLog_CommitChain_Sucess(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	changeToRepoDir(t, repoPath)

	// Create commit history
	store := objects.NewObjectStore(repoPath)
	commitLogEntries := make([]string, 3)
	for range 3 {
		message := testutils.RandomString(100)
		commitWithSingleFile(t, repoPath, message)

		commitHash := readRefFile(t, repoPath)
		commit, err := store.ReadCommit(commitHash)
		if err != nil {
			t.Fatalf("Failed to read commit object: %v", err)
		}
		commitLogEntries = append(commitLogEntries, utils.FormatCommitLogEntry(
			commit.Hash(),
			commit.Message(),
			commit.Author().String(),
			commit.Author().Time(),
		))
	}

	// Reverse commit order as they meant to appear from most recent to least recent
	slices.Reverse(commitLogEntries)
	var expectedOutput strings.Builder
	for _, entry := range commitLogEntries {
		expectedOutput.WriteString(entry)
	}

	// Execute Log command
	command := createTestRootCmd(logCmd)
	stdout := captureStdout(command)
	command.SetArgs([]string{constants.LogCmdName})
	if err := command.Execute(); err != nil {
		t.Fatalf("log command failed: %v", err)
	}

	output := stdout.String()
	if output != expectedOutput.String() {
		t.Fatalf("Expected commit history output to be [%s], got [%s]", expectedOutput.String(), output)
	}
}

// TestLog_EmptyRepository_NoCommits initializes a repository without
// creating any commits and executes the log command. Verifies that the
// command fails with an error indicating the ref file could not be read.
func TestLog_EmptyRepository_NoCommits(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	changeToRepoDir(t, repoPath)

	command := createTestRootCmd(logCmd)
	captureStdout(command)
	errOut := captureStderr(command)
	command.SetArgs([]string{constants.LogCmdName})
	if err := command.Execute(); err == nil {
		t.Fatal("Expected log command to fail on empty repository with no commits")
	}

	errorOutput := errOut.String()
	expectedErrorMessage := fmt.Sprintf("failed to read commit hash from [%s]",
		filepath.Join(repoPath, constants.Gogit, "refs", "heads", "main"))
	if !strings.Contains(errorOutput, expectedErrorMessage) {
		t.Fatalf("Expected error output to contain [%s], got [%s]", expectedErrorMessage, errorOutput)
	}
}

// TestLog_OutsideRepository executes the log command from a temporary
// directory that contains no .gogit structure. Verifies that the command
// fails with an error indicating the repository was not found.
func TestLog_OutsideRepository(t *testing.T) {
	tempDir := t.TempDir()
	changeToRepoDir(t, tempDir)

	command := createTestRootCmd(logCmd)
	captureStdout(command)
	errOut := captureStderr(command)
	command.SetArgs([]string{constants.LogCmdName})
	if err := command.Execute(); err == nil {
		t.Fatal("Expected log command to fail when executed outside a repository")
	}

	expectedErrorMessage := constants.Gogit + " directory not found"
	errorOutput := errOut.String()
	if !strings.Contains(errorOutput, expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, errorOutput)
	}
}
