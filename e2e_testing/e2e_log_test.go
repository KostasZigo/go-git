package e2etesting

import (
	"os/exec"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/utils"
)

// TestE2E_LogCommand_SingleCommit executes the full init → add → commit → log
// workflow through the compiled binary. Reads the stored commit object to
// extract the hash, message, author, and timestamp, builds the expected
// formatted log line, and verifies the binary's stdout matches exactly.
func TestE2E_LogCommand_SingleCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	commitWithSingleFile(t, repoPath)
	commit := readCommitFromDefaultRef(t, repoPath)

	expectedOutput := utils.FormatCommitLogEntry(
		commit.Hash(),
		commit.Message(),
		commit.Author().String(),
		commit.Author().Time(),
	)

	// Execute log command
	logCmd := exec.Command(sharedBinaryPath, constants.LogCmdName)
	logCmd.Dir = repoPath
	output, err := logCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("log command failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if outputStr != expectedOutput {
		t.Fatalf("Expected output to be [%s], got [%s]", expectedOutput, outputStr)
	}
}
