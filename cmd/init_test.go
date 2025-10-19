package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/spf13/cobra"
)

func TestInitCommand_Success(t *testing.T) {
	// Create temporary directory and change to it
	tempDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Capture output
	var stdout, stderr bytes.Buffer

	// Create a new root command for testing
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(initCmd)
	testRootCmd.SetOut(&stdout)
	testRootCmd.SetErr(&stderr)

	// Execute init command
	testRootCmd.SetArgs([]string{"init"})
	err = testRootCmd.Execute()

	// Verify command succeeded
	if err != nil {
		t.Fatalf("Init command failed: %v", err)
	}

	// Verify output message
	output := stdout.String()
	expectedMsg := "Initialized empty GoGit repository in ./.gogit/\n"
	if !strings.Contains(output, expectedMsg) {
		t.Errorf("Expected output to contain %q, got: %s", expectedMsg, output)
	}

	// Verify .gogit directory was created
	gogitDirectory := filepath.Join(tempDir, ".gogit")
	if _, err := os.Stat(gogitDirectory); os.IsNotExist(err) {
		t.Error(".gogit directory was not created by CLI command")
	}

	// Check required subdirectories exist
	expectedDirectories := []string{"objects", "refs", "refs/heads", "refs/tags"}
	for _, dir := range expectedDirectories {
		dirPath := filepath.Join(gogitDirectory, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Errorf("Required directory %s was not created: %v", dir, err)
		}
	}

	// Verify HEAD file exists and has correct content
	headPath := filepath.Join(gogitDirectory, "HEAD")
	content, err := os.ReadFile(headPath)
	if err != nil {
		t.Errorf("HEAD file was not created: %v", err)
	} else {
		expected := "ref: refs/heads/main\n"
		if string(content) != expected {
			t.Errorf("HEAD file content = %q, want %q", string(content), expected)
		}
	}
}

func TestInitCommand_WithDirectory_Success(t *testing.T) {
	tempDir := t.TempDir()
	targetDirectory := filepath.Join(tempDir, "my-project")

	var stdout bytes.Buffer
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(initCmd)
	testRootCmd.SetOut(&stdout)

	// Execute init with directory argument
	testRootCmd.SetArgs([]string{"init", targetDirectory})
	err := testRootCmd.Execute()

	if err != nil {
		t.Fatalf("Init command with directory failed: %v", err)
	}

	// Verify .gogit directory was created in target directory
	gogitDirectory := filepath.Join(targetDirectory, ".gogit")
	if _, err := os.Stat(gogitDirectory); os.IsNotExist(err) {
		t.Error(".gogit directory was not created in target directory")
	}

	// Check required subdirectories exist
	expectedDirectories := []string{"objects", "refs", "refs/heads", "refs/tags"}
	for _, dir := range expectedDirectories {
		dirPath := filepath.Join(gogitDirectory, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Errorf("Required directory %s was not created: %v", dir, err)
		}
	}

	// Verify HEAD file exists and has correct content
	headPath := filepath.Join(gogitDirectory, "HEAD")
	content, err := os.ReadFile(headPath)
	if err != nil {
		t.Errorf("HEAD file was not created: %v", err)
	} else {
		expected := "ref: refs/heads/main\n"
		if string(content) != expected {
			t.Errorf("HEAD file content = %q, want %q", string(content), expected)
		}
	}
}

func TestInitCommand_AlreadyExists(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize once
	var stdout1 bytes.Buffer
	testRootCmd1 := &cobra.Command{Use: "gogit"}
	testRootCmd1.AddCommand(initCmd)
	testRootCmd1.SetOut(&stdout1)
	testRootCmd1.SetArgs([]string{"init", tempDir})

	err := testRootCmd1.Execute()
	if err != nil {
		t.Fatalf("First init failed: %v", err)
	}

	// Try to initialize again
	var stdout2, stderr2 bytes.Buffer
	testRootCmd2 := &cobra.Command{Use: "gogit"}
	testRootCmd2.AddCommand(initCmd)
	testRootCmd2.SetOut(&stdout2)
	testRootCmd2.SetErr(&stderr2)
	testRootCmd2.SetArgs([]string{"init", tempDir})

	err = testRootCmd2.Execute()
	if err == nil {
		t.Error("Expected error when repository already exists")
	}

	// Verify error message mentions repository exists
	expectedErrorMsg := fmt.Sprintf("failed to initialize repository - repository already exists at %s\\.gogit", tempDir)
	if !strings.Contains(err.Error(), expectedErrorMsg) {
		t.Errorf("Expected error to contain %q, got: %q", expectedErrorMsg, err.Error())
	}

	stdErrExpectedMsg := "Error: " + expectedErrorMsg + "\n"
	if !strings.Contains(stderr2.String(), stdErrExpectedMsg) {
		t.Errorf("Expected error output (stdErr) to contain %q, got: %q", stdErrExpectedMsg, err.Error())
	}
}

func TestInitCommand_TooManyArguments(t *testing.T) {
	var stdout bytes.Buffer
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(initCmd)
	testRootCmd.SetOut(&stdout)

	// Execute init with too many arguments
	testRootCmd.SetArgs([]string{"init", "dir1", "dir2"})
	err := testRootCmd.Execute()

	// Should not return error but should show usage
	if err != nil {
		t.Errorf("Expected no error for too many args, got: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "usage: gogit init [<directory>]") {
		t.Errorf("Expected usage message [usage: gogit init [<directory>]] , got: %s", output)
	}
}

func TestInitCommand_Fail(t *testing.T) {
	tempDir := t.TempDir()

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

	var stdout, stdErr bytes.Buffer
	testRootCmd := &cobra.Command{Use: "gogit"}
	testRootCmd.AddCommand(initCmd)
	testRootCmd.SetOut(&stdout)
	testRootCmd.SetErr(&stdErr)
	testRootCmd.SetArgs([]string{"init", tempDir})

	err := testRootCmd.Execute()

	if err == nil {
		t.Error("Expected error since InitRepository mocked to fail")
	}

	if !errors.Is(err, mockError) {
		t.Errorf("Expected error to wrap the mock error %v, but got: %v", mockError, err)
	}

	// Verify cleanup was called
	gogitDirectory := filepath.Join(tempDir, ".gogit")
	if _, err := os.Stat(gogitDirectory); err == nil {
		t.Error("Expected .gogit directory to be cleaned up after failure")
	}
}
