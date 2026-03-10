package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/testutils"
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

func stageRandomFile(t *testing.T, repoPath string) {
	stageFile(t, testutils.RandomString(10), testutils.RandomString(100), repoPath)
}

func stageFile(t *testing.T, fileName, fileContent, filePath string) {
	// Stage file with add command
	command := createTestRootCmd(addCmd)
	captureStdout(command)
	testutils.CreateTestFile(t, filePath, fileName, []byte(fileContent))

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(filePath); err != nil {
		t.Fatalf("Failed to change to directory %s: %v", filePath, err)
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

// commitWithSingleFile stages a random file and commits it with the
// given message. Fails the test if either the add or commit command fails.
func commitWithSingleFile(t *testing.T, repoPath, commitMessage string) {
	stageRandomFile(t, repoPath)

	command := createTestRootCmd(commitCmd)
	captureStdout(command)

	command.SetArgs([]string{constants.CommitCmdName, "-m", commitMessage})
	if err := command.Execute(); err != nil {
		t.Fatalf("commit command failed: %v", err)
	}
}
