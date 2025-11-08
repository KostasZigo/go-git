package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
)

// TestInitCommand_Success verifies successful repository initialization in current directory.
func TestInitCommand_Success(t *testing.T) {
	repoPath := t.TempDir()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	if err = os.Chdir(repoPath); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create a new root command for testing
	testRootCmd := createTestRootCmd(initCmd)
	stdout := captureStdout(testRootCmd)

	// Execute init command
	testRootCmd.SetArgs([]string{"init"})
	if err = testRootCmd.Execute(); err != nil {
		t.Fatalf("Init command failed: %v", err)
	}

	// Verify output message
	expectedMsg := "Initialized empty GoGit repository in ./.gogit/\n"
	if !strings.Contains(stdout.String(), expectedMsg) {
		t.Errorf("Expected output to contain %q, got: %s", expectedMsg, stdout.String())
	}

	assertRepositoryStructure(t, repoPath)
}

// TestInitCommand_WithDirectory_Success verifies initialization with explicit directory path.
func TestInitCommand_WithDirectory_Success(t *testing.T) {
	repoPath := t.TempDir()
	targetDirectory := filepath.Join(repoPath, "my-project")

	testRootCmd := createTestRootCmd(initCmd)
	captureStdout(testRootCmd)

	// Execute init with directory argument
	testRootCmd.SetArgs([]string{"init", targetDirectory})
	if err := testRootCmd.Execute(); err != nil {
		t.Fatalf("Init command with directory failed: %v", err)
	}

	assertRepositoryStructure(t, targetDirectory)
}

// TestInitCommand_AlreadyExists verifies error when repository already exists.
func TestInitCommand_AlreadyExists(t *testing.T) {
	repoPath := t.TempDir()

	// Initialize once
	testRootCmd1 := createTestRootCmd(initCmd)
	captureStdout(testRootCmd1)
	testRootCmd1.SetArgs([]string{"init", repoPath})

	if err := testRootCmd1.Execute(); err != nil {
		t.Fatalf("First init failed: %v", err)
	}

	// Try to initialize again
	testRootCmd2 := createTestRootCmd(initCmd)
	captureStderr(testRootCmd2)
	testRootCmd2.SetArgs([]string{"init", repoPath})

	err := testRootCmd2.Execute()
	if err == nil {
		t.Error("Expected error when repository already exists")
	}

	// Verify error message mentions repository exists
	expectedErrorMsg := fmt.Sprintf("failed to initialize repository - repository already exists at %s\\.gogit", repoPath)
	if !strings.Contains(err.Error(), expectedErrorMsg) {
		t.Errorf("Expected error to contain %q, got: %q", expectedErrorMsg, err.Error())
	}
}

// TestInitCommand_TooManyArguments verifies behavior with excessive arguments.
func TestInitCommand_TooManyArguments(t *testing.T) {
	testRootCmd := createTestRootCmd(initCmd)
	stdout := captureStdout(testRootCmd)
	testRootCmd.SetArgs([]string{"init", "dir1", "dir2"})

	// Should not return error but should show usage
	if err := testRootCmd.Execute(); err != nil {
		t.Errorf("Expected no error for too many args, got: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "usage: gogit init [<directory>]") {
		t.Errorf("Expected usage message [usage: gogit init [<directory>]] , got: %s", output)
	}
}

// TestInitCommand_Fail verifies cleanup on initialization failure.
func TestInitCommand_Fail(t *testing.T) {
	repoPath := t.TempDir()

	// Mock os.MkdirAll to fail after first call
	mockError := errors.New("mocked mkdir failure")
	callCount := 0
	patches := gomonkey.ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
		callCount++
		if callCount > 1 {
			return mockError
		}
		// Let first call succeed (creates .gogit directory)
		return os.MkdirAll(path, perm)
	})
	defer patches.Reset()

	testRootCmd := createTestRootCmd(initCmd)
	captureStdout(testRootCmd)
	captureStderr(testRootCmd)
	testRootCmd.SetArgs([]string{"init", repoPath})

	err := testRootCmd.Execute()

	if err == nil {
		t.Error("Expected error since InitRepository mocked to fail")
	}

	if !errors.Is(err, mockError) {
		t.Errorf("Expected error to wrap the mock error %v, but got: %v", mockError, err)
	}

	// Verify cleanup was called
	gogitDirectory := filepath.Join(repoPath, ".gogit")
	if _, err := os.Stat(gogitDirectory); err == nil {
		t.Error("Expected .gogit directory to be cleaned up after failure")
	}
}
