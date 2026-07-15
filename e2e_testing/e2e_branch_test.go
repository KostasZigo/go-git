package e2etesting

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestE2E_BranchCommand_SymbolicHEAD verifies that `gogit branch`
// creates a new refs/heads entry pointing to the same commit as current
// symbolic HEAD, without mutating HEAD itself.
func TestE2E_BranchCommand_SymbolicHEAD(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	// Ensure current HEAD branch has a commit to point to.
	commitWithSingleFile(t, repoPath)
	expectedCommitHash := testutils.ReadDefaultRefFile(t, repoPath)

	newBranchName := filepath.ToSlash(filepath.Join(testutils.RandomString(8), testutils.RandomString(8)))
	output, err := runBranchCommand(t, repoPath, newBranchName)
	if err != nil {
		t.Fatalf("branch command failed: %v\nOutput: %s", err, output)
	}

	assertBranchOutput(t, string(output), newBranchName)

	actualBranchHash := readBranchRefHash(t, repoPath, newBranchName)
	if actualBranchHash != expectedCommitHash {
		t.Fatalf("Expected branch [%s] ref hash [%s], got [%s]", newBranchName, expectedCommitHash, actualBranchHash)
	}
}

// TestE2E_BranchCommand_DetachedHEAD verifies that creating a branch while
// HEAD is detached points the new branch ref to the detached commit hash.
func TestE2E_BranchCommand_DetachedHEAD(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	// 1st commit in main branch
	commitWithSingleFile(t, repoPath)
	expectedCommitHash := testutils.ReadDefaultRefFile(t, repoPath)

	// 2nd commit in main branch
	commitWithSingleFile(t, repoPath)

	// Checkout first commit by hash (detached HEAD)
	cmd := newGogitCmd(t, constants.CheckoutCmdName, expectedCommitHash)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("detached checkout failed: %v\nOutput: %s", err, output)
	}

	// Create branch from Detached Head's commit hash - first commit
	newBranchName := testutils.RandomString(8)
	output, err = runBranchCommand(t, repoPath, newBranchName)
	if err != nil {
		t.Fatalf("branch command failed: %v\nOutput: %s", err, output)
	}
	assertBranchOutput(t, string(output), newBranchName)

	actualBranchHash := readBranchRefHash(t, repoPath, newBranchName)
	if actualBranchHash != expectedCommitHash {
		t.Fatalf("Expected branch [%s] ref hash [%s], got [%s]", newBranchName, expectedCommitHash, actualBranchHash)
	}
}

// TestE2E_BranchCommand_BranchAlreadyExists verifies that attempting to create
// the same branch twice returns an "already exists" validation error.
func TestE2E_BranchCommand_BranchAlreadyExists(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	commitWithSingleFile(t, repoPath)
	expectedCommitHash := testutils.ReadDefaultRefFile(t, repoPath)

	newBranchName := filepath.ToSlash(filepath.Join(testutils.RandomString(8), testutils.RandomString(8)))
	output, err := runBranchCommand(t, repoPath, newBranchName)
	if err != nil {
		t.Fatalf("branch command failed: %v\nOutput: %s", err, output)
	}
	assertBranchOutput(t, string(output), newBranchName)

	actualBranchHash := readBranchRefHash(t, repoPath, newBranchName)
	if actualBranchHash != expectedCommitHash {
		t.Fatalf("Expected branch [%s] ref hash [%s], got [%s]", newBranchName, expectedCommitHash, actualBranchHash)
	}

	// Create second branch with the same name
	output, err = runBranchCommand(t, repoPath, newBranchName)
	if err == nil {
		t.Fatal("Expected error when the branch already exists.")
	}

	expectedErrorMessage := fmt.Sprintf("branch [%s] already exists", newBranchName)
	if !strings.Contains(string(output), expectedErrorMessage) {
		t.Fatalf("Expected branch output to contain [%s], got: [%s]", expectedErrorMessage, output)
	}
}

// TestE2E_BranchCommand_InvalidName verifies that names violating the current
// validation policy are rejected by the CLI with a non-zero exit code and an
// appropriate error message.
func TestE2E_BranchCommand_InvalidName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)
	commitWithSingleFile(t, repoPath)

	invalidNames := []string{
		".",
		"abc..def",
		"name.lock",
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			output, err := runBranchCommand(t, repoPath, name)
			if err == nil {
				t.Fatalf("Expected error for invalid branch name [%s], but command succeeded.\nOutput: %s", name, output)
			}

			if !strings.Contains(string(output), "invalid branch name") {
				t.Fatalf("Expected error output to contain 'invalid branch name', got: [%s]", output)
			}
		})
	}
}

// TestE2E_BranchCommand_NestedRefMaterialization verifies that a branch with a
// multi-level hierarchical name (e.g. release/2026/q3) results in the nested
// directory path being created under refs/heads/ and the ref file containing
// the correct commit hash.
func TestE2E_BranchCommand_NestedRefMaterialization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)
	commitWithSingleFile(t, repoPath)

	expectedCommitHash := testutils.ReadDefaultRefFile(t, repoPath)

	nestedBranchName := filepath.ToSlash(filepath.Join("release", testutils.RandomString(4), testutils.RandomString(2)))
	output, err := runBranchCommand(t, repoPath, nestedBranchName)
	if err != nil {
		t.Fatalf("branch command failed: %v\nOutput: %s", err, output)
	}

	assertBranchOutput(t, string(output), nestedBranchName)

	// Assert the nested directory structure exists on disk
	nestedRefDir := filepath.Join(
		repoPath,
		constants.Gogit,
		constants.Refs,
		constants.Heads,
		filepath.Dir(filepath.FromSlash(nestedBranchName)),
	)
	testutils.AssertDirExists(t, nestedRefDir)

	// Assert the ref file contains the correct commit hash
	actualBranchHash := readBranchRefHash(t, repoPath, nestedBranchName)
	if actualBranchHash != expectedCommitHash {
		t.Fatalf("Expected branch [%s] ref hash [%s], got [%s]", nestedBranchName, expectedCommitHash, actualBranchHash)
	}
}

// TestE2E_BranchCommand_NoSideEffectsOnFailure verifies that a failed branch
// creation does not leave a partial ref file on disk.
func TestE2E_BranchCommand_NoSideEffectsOnFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)
	commitWithSingleFile(t, repoPath)

	invalidName := testutils.RandomString(6) + ".lock"
	output, err := runBranchCommand(t, repoPath, invalidName)
	if err == nil {
		t.Fatalf("Expected error for invalid branch name [%s], but command succeeded.\nOutput: %s", invalidName, output)
	}

	// Assert no ref file was created at the expected path
	expectedRefPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, invalidName)
	testutils.AssertFileNotExists(t, expectedRefPath)
}
