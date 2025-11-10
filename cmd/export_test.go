package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

// createTestRootCmd creates fresh root command with init subcommand.
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
