package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/KostasZigo/gogit/testutils"
	"github.com/spf13/cobra"
)

// createTestRootCmd creates fresh root command with init subcommand.
func createTestRootCmd(cmd *cobra.Command) *cobra.Command {
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(cmd)
	return testRootCmd
}

// captureStdout returns command stdout output as string.
func captureStdout(cmd *cobra.Command) *bytes.Buffer {
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	return &stdout
}

// captureStderr returns command stderr output as string.
func captureStderr(cmd *cobra.Command) *bytes.Buffer {
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	return &stderr
}

// assertRepositoryStructure verifies .gogit directory structure and HEAD file.
func assertRepositoryStructure(t *testing.T, repoPath string) {
	t.Helper()

	gogitDir := filepath.Join(repoPath, ".gogit")
	testutils.AssertDirExists(t, gogitDir)

	expectedDirs := []string{"objects", "refs", "refs/heads", "refs/tags"}
	for _, dir := range expectedDirs {
		testutils.AssertDirExists(t, filepath.Join(gogitDir, dir))
	}

	headPath := filepath.Join(gogitDir, "HEAD")
	testutils.AssertFileExists(t, headPath)

	content, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("Failed to read HEAD file: %v", err)
	}

	expectedContent := "ref: refs/heads/main\n"
	if string(content) != expectedContent {
		t.Errorf("HEAD content = %q, want %q", content, expectedContent)
	}
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
