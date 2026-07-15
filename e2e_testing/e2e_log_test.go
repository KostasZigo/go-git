package e2etesting

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/commits"
	"github.com/KostasZigo/gogit/internal/constants"
)

// TestE2E_LogCommand_SingleCommit verifies the log command output after a
// single init → add → commit cycle.
func TestE2E_LogCommand_SingleCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	commitWithSingleFile(t, repoPath)
	commit := readCommitFromDefaultRef(t, repoPath)

	expectedOutput := commits.FormatLogEntry(
		commit.Hash(),
		commit.Message(),
		commit.Author().String(),
		commit.Author().Time(),
	)

	// Execute log command
	logCmd := newGogitCmd(t, constants.LogCmdName)
	logCmd.Dir = repoPath
	output, err := logCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("log command failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if outputStr != expectedOutput {
		t.Fatalf("expected output to be [%s], got [%s]", expectedOutput, outputStr)
	}
}

// TestE2E_LogCommand_CommitChain verifies the log command correctly renders
// a chain of three sequential commits in reverse chronological order.
func TestE2E_LogCommand_CommitChain(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	commitEntryLogs := make([]string, 0, 3)
	for range 3 {
		commitWithSingleFile(t, repoPath)
		commit := readCommitFromDefaultRef(t, repoPath)

		commitEntryLogs = append(commitEntryLogs, commits.FormatLogEntry(
			commit.Hash(),
			commit.Message(),
			commit.Author().String(),
			commit.Author().Time(),
		))
	}

	slices.Reverse(commitEntryLogs)
	var expectedOutput strings.Builder
	for _, logEntry := range commitEntryLogs {
		expectedOutput.WriteString(logEntry)
	}

	// Execute log command
	logCmd := newGogitCmd(t, constants.LogCmdName)
	logCmd.Dir = repoPath
	output, err := logCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("log command failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if outputStr != expectedOutput.String() {
		t.Fatalf("expected output to be [%s], got [%s]", expectedOutput.String(), outputStr)
	}
}

// TestE2E_LogCommand_NoRepository verifies that running log outside of any
// initialised repository produces an error message
func TestE2E_LogCommand_NoRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)

	// Execute log command
	logCmd := newGogitCmd(t, constants.LogCmdName)
	logCmd.Dir = repoPath
	output, err := logCmd.CombinedOutput()
	if err == nil {
		t.Fatal("log command expected to fail when executed outside of a repository")
	}

	outputStr := string(output)
	expectedOutput := constants.Gogit + " directory not found"
	if !strings.Contains(outputStr, expectedOutput) {
		t.Fatalf("expected output to contain [%s], got [%s]", expectedOutput, outputStr)
	}
}

// TestE2E_LogCommand_NoCommits verifies that running log on a freshly
// initialised repository (zero commits) produces an error message
func TestE2E_LogCommand_NoCommits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	// Execute log command
	logCmd := newGogitCmd(t, constants.LogCmdName)
	logCmd.Dir = repoPath
	output, err := logCmd.CombinedOutput()
	if err == nil {
		t.Fatal("log command expected to fail when no commits exist in the repository")
	}

	outputStr := string(output)
	expectedErrorMessage := fmt.Sprintf("failed to read commit hash from [%s]",
		filepath.Join(repoPath, constants.Gogit, "refs", "heads", "main"))
	if !strings.Contains(outputStr, expectedErrorMessage) {
		t.Fatalf("expected output to contain [%s], got [%s]", expectedErrorMessage, outputStr)
	}
}
