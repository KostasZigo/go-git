package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
	"github.com/spf13/cobra"
)

// createTestRootCmd creates fresh root command with subcommand.
func createTestRootCmd(cmd *cobra.Command) *cobra.Command {
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(cmd)
	return testRootCmd
}

// captureStdout returns command stdout output as buffer.
func captureStdout(cmd *cobra.Command) *bytes.Buffer {
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	return &stdout
}

// captureStderr returns command stderr output as buffer.
func captureStderr(cmd *cobra.Command) *bytes.Buffer {
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	return &stderr
}

// changeToRepoDir changes working directory to repo path and registers cleanup.
func changeToRepoDir(t *testing.T, repoPath string) {
	t.Helper()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("Failed to change to directory %s: %v", repoPath, err)
	}

	t.Cleanup(func() {
		os.Chdir(oldDir)
	})
}

// stageRandomFile creates a file with random name and content, then stages it
// via the add command. Convenience wrapper around stageFile for tests that do
// not need control over file identity.
func stageRandomFile(t *testing.T, fileDir string) {
	t.Helper()
	stageFile(t, testutils.RandomString(10), fileDir, testutils.RandomByteSlice(100))
}

// stageFile creates a file with the given name and content under fileDir, then
// stages it via the add command. Changes the working directory to fileDir for
// the add invocation and restores it afterward.
func stageFile(t *testing.T, fileName, fileDir string, fileContent []byte) {
	t.Helper()
	// Stage file with add command
	command := createTestRootCmd(addCmd)
	captureStdout(command)
	testutils.CreateTestFile(t, fileDir, fileName, fileContent)

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(fileDir); err != nil {
		t.Fatalf("Failed to change to directory %s: %v", fileDir, err)
	}

	command.SetArgs([]string{constants.AddCmdName, fileName})
	if err := command.Execute(); err != nil {
		t.Fatalf("add command failed: %v", err)
	}
}

// treeEntryNames extracts names from a slice of tree entries for use in error messages.
func treeEntryNames(entries []objects.TreeEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

// commitWithSingleRandomFile stages a random file and commits it with the
// given message. Fails the test if either the add or commit command fails.
func commitWithSingleRandomFile(t *testing.T, fileDir, commitMessage string) {
	t.Helper()
	stageRandomFile(t, fileDir)
	executeCommitCommand(t, commitMessage)
}

// commitWithSingleFile stages a specific file with given content and commits
// it with the provided message. Fails the test if either operation fails.
func commitWithSingleFile(t *testing.T, fileName, fileDir string, fileContent []byte, commitMessage string) {
	t.Helper()
	stageFile(t, fileName, fileDir, fileContent)
	executeCommitCommand(t, commitMessage)
}

// executeCommitCommand creates a fresh commit command and executes it with the
// given message. Fails the test if the commit command returns an error.
func executeCommitCommand(t *testing.T, commitMessage string) {
	t.Helper()
	command := createTestRootCmd(commitCmd)
	captureStdout(command)

	command.SetArgs([]string{constants.CommitCmdName, "-m", commitMessage})
	if err := command.Execute(); err != nil {
		t.Fatalf("commit command failed: %v", err)
	}
}
