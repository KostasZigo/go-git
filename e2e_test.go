package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E_InitCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// 1. Build the binary
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "gogit.exe")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = "."
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}

	// 2. Create test directory
	testRepoDir := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(testRepoDir, 0755); err != nil {
		t.Fatalf("Failed to create test repo dir: %v", err)
	}

	// 3. Test the binary like a real user
	cmd := exec.Command(binaryPath, "init")
	cmd.Dir = testRepoDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Binary execution failed: %v\nOutput: %s", err, output)
	}

	// 4. Verify output
	outputStr := string(output)
	expectedMsg := "Initialized empty GoGit repository in ./.gogit/\n"
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected output to contain %q, got: %s", expectedMsg, outputStr)
	}

	// 5. Verify filesystem changes
	gogitDir := filepath.Join(testRepoDir, ".gogit")
	if _, err := os.Stat(gogitDir); os.IsNotExist(err) {
		t.Error("Binary didn't create .gogit directory")
	}

	// Check required subdirectories exist
	expectedDirectories := []string{"objects", "refs", "refs/heads", "refs/tags"}
	for _, dir := range expectedDirectories {
		dirPath := filepath.Join(gogitDir, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Errorf("Required directory %s was not created: %v", dir, err)
		}
	}

	// Verify HEAD file exists and has correct content
	headPath := filepath.Join(gogitDir, "HEAD")
	content, err := os.ReadFile(headPath)
	if err != nil {
		t.Errorf("HEAD file was not created: %v", err)
	} else {
		expected := "ref: refs/heads/main\n"
		if string(content) != expected {
			t.Errorf("HEAD file content = %q, want %q", string(content), expected)
		}
	}

	// 6. Test error case - init again
	cmd = exec.Command(binaryPath, "init")
	cmd.Dir = testRepoDir
	output, err = cmd.CombinedOutput()

	if err == nil {
		t.Error("Expected error when running init twice")
	}

	expectedErrorMsg := "Error: failed to initialize repository - repository already exists at .gogit\n"
	if !strings.Contains(string(output), expectedErrorMsg) {
		t.Errorf("Expected error to contain %q, got: %q", expectedErrorMsg, string(output))
	}
}

func TestE2E_HelpCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Build binary
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "gogit.exe")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}

	// Test help
	cmd := exec.Command(binaryPath, "--help")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Help command failed: %v", err)
	}

	outputStr := string(output)
	expectedTexts := []string{
		"GoGit is a simplified Git Implementation",
		"Available Commands:",
		"init",
		"Flags:",
		"-h, --help",
	}

	for _, text := range expectedTexts {
		if !strings.Contains(outputStr, text) {
			t.Errorf("Help output missing %q, got: %s", text, outputStr)
		}
	}
}

func TestE2E_InvalidCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Build binary
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "gogit.exe")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}

	// Test invalid command
	cmd := exec.Command(binaryPath, "nonexistent")
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("Expected error for invalid command")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "unknown command") {
		t.Errorf("Expected 'unknown command' error, got: %s", outputStr)
	}
}
