package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/testutils"
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

// assertRepositoryStructure validates complete .gogit directory structure.
// Verifies objects/, refs/heads/, refs/tags/ exist and HEAD contains correct branch reference.
// Fatal error if any validation fails.
func assertRepositoryStructure(t *testing.T, repoPath string) {
	t.Helper()

	gogitDir := filepath.Join(repoPath, constants.Gogit)
	testutils.AssertDirExists(t, gogitDir)

	expectedDirs := []string{
		constants.Objects,
		constants.Refs,
		filepath.Join(constants.Refs, constants.Heads),
		filepath.Join(constants.Refs, constants.Tags),
	}
	for _, dir := range expectedDirs {
		testutils.AssertDirExists(t, filepath.Join(gogitDir, dir))
	}

	headPath := filepath.Join(gogitDir, constants.Head)
	testutils.AssertFileExists(t, headPath)

	content, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("Failed to read %s file: %v", constants.Head, err)
	}

	expectedContent := constants.DefaultRefPrefix + constants.DefaultBranch + "\n"
	if string(content) != expectedContent {
		t.Errorf("%s content = %q, want %q", constants.Head, content, expectedContent)
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
