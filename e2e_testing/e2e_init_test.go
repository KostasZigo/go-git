package e2etesting

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestE2E_InitCommand verifies repository initialization creates correct structure.
func TestE2E_InitCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Create test repo directory
	repoPath := setupTestRepo(t)

	// Test the binary like a real user
	cmd := newGogitCmd(t, constants.InitCmdName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary execution failed: %v\nOutput: %s", err, output)
	}

	// Verify output
	outputStr := string(output)
	expectedMsg := fmt.Sprintf("Initialized empty GoGit repository in %s\n", "."+string(filepath.Separator)+constants.Gogit+string(filepath.Separator))
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("expected output to contain %q, got: %s", expectedMsg, outputStr)
	}

	// Verify filesystem changes
	gogitDir := filepath.Join(repoPath, constants.Gogit)
	testutils.AssertDirExists(t, gogitDir)
	testutils.AssertRepositoryStructure(t, repoPath)

	// Test error case - init again
	cmd = newGogitCmd(t, constants.InitCmdName)
	cmd.Dir = repoPath
	output, err = cmd.CombinedOutput()

	if err == nil {
		t.Errorf("expected error when running %s twice", constants.InitCmdName)
	}

	expectedErrorMsg := "Error: failed to initialize repository - repository already exists at .gogit\n"
	if !strings.Contains(string(output), expectedErrorMsg) {
		t.Errorf("expected error to contain %q, got: %q", expectedErrorMsg, string(output))
	}
}

// TestE2E_HelpCommand verifies help output contains expected sections.
func TestE2E_HelpCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Test help
	cmd := newGogitCmd(t, "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("help command failed: %v", err)
	}

	expectedTexts := []string{
		"GoGit is a simplified Git Implementation",
		"Available Commands:",
		constants.InitCmdName,
		constants.HashObjectCmdName,
		"Flags:",
		"-h, --help",
	}

	outputStr := string(output)
	for _, text := range expectedTexts {
		if !strings.Contains(outputStr, text) {
			t.Errorf("help output missing %q, got: %s", text, outputStr)
		}
	}
}

// TestE2E_InvalidCommand verifies error for unknown commands.
func TestE2E_InvalidCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Test invalid command
	cmd := newGogitCmd(t, "nonexistent")
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("expected error for invalid command")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "unknown command") {
		t.Errorf("expected 'unknown command' error, got: %s", outputStr)
	}
}
