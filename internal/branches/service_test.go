package branches

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestBranch_OrchestrateBranchCreation_SymbolicHead verifies that creating a branch
// while HEAD points to a symbolic ref copies the pointed commit hash into
// refs/heads/<new-branch>.
func TestBranch_OrchestrateBranchCreation_SymbolicHead(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	currentCommitHash := testutils.RandomHash()
	testutils.WriteRefFile(t, repoPath, constants.DefaultBranch, currentCommitHash)

	currentCommitHash += "\n" // \n is added in WriteRefFile but not updating the currentCommitHash var
	newBranchName := testutils.RandomString(8)
	err := OrchestrateBranchCreation(repoPath, newBranchName)
	if err != nil {
		t.Fatalf("Unexpected error creating branch [%s]: %v", newBranchName, err)
	}

	createdBranchHash := readBranchRefHash(t, repoPath, newBranchName)
	if createdBranchHash != currentCommitHash {
		t.Fatalf("Expected branch [%s] hash to be [%s], got [%s]", newBranchName, currentCommitHash, createdBranchHash)
	}
}

// TestBranch_OrchestrateBranchCreation_DetachedHead verifies that creating a branch
// while HEAD is in detached state copies the commit hash from HEAD into
// refs/heads/<new-branch>.
func TestBranch_OrchestrateBranchCreation_DetachedHead(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	currentCommitHash := append(testutils.RandomByteHash(), '\n')
	testutils.WriteHEADFile(t, repoPath, currentCommitHash)

	newBranchName := testutils.RandomString(8)
	err := OrchestrateBranchCreation(repoPath, newBranchName)
	if err != nil {
		t.Fatalf("Unexpected error creating branch [%s]: %v", newBranchName, err)
	}

	createdBranchHash := readBranchRefHash(t, repoPath, newBranchName)
	if createdBranchHash != string(currentCommitHash) {
		t.Fatalf("Expected branch [%s] hash to be [%s], got [%s]", newBranchName, currentCommitHash, createdBranchHash)
	}
}

// TestBranch_OrchestrateBranchCreation_HierarchicalBranchName verifies that
// creating a branch with hierarchical naming (path-like) succeeds and writes
// the expected commit hash into the branch ref.
func TestBranch_OrchestrateBranchCreation_HierarchicalBranchName(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	currentCommitHash := append(testutils.RandomByteHash(), '\n')
	testutils.WriteHEADFile(t, repoPath, currentCommitHash)

	dir := testutils.RandomString(8)
	name := testutils.RandomString(8)
	newBranchName := filepath.ToSlash(filepath.Join(dir, name))
	err := OrchestrateBranchCreation(repoPath, newBranchName)
	if err != nil {
		t.Fatalf("Unexpected error creating branch [%s]: %v", newBranchName, err)
	}

	createdBranchHash := readBranchRefHash(t, repoPath, newBranchName)
	if createdBranchHash != string(currentCommitHash) {
		t.Fatalf("Expected branch [%s] hash to be [%s], got [%s]", newBranchName, currentCommitHash, createdBranchHash)
	}
}

// TestBranch_OrchestrateBranchCreation_AlreadyExists verifies that branch
// creation fails when the target branch ref already exists.
func TestBranch_OrchestrateBranchCreation_AlreadyExists(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	commitHash := testutils.RandomHash()
	branchName := testutils.RandomString(8)

	// Write Head file to point to this branch ref -
	// since default main ref is not created due to lack of commit
	content := constants.DefaultRefPrefix + branchName + "\n"
	testutils.WriteHEADFile(t, repoPath, []byte(content))

	testutils.WriteRefFile(t, repoPath, branchName, commitHash)
	err := OrchestrateBranchCreation(repoPath, branchName)
	if err == nil {
		t.Fatal("Expected error when the branch already exists.")
	}

	expectedErrorMessage := fmt.Sprintf("branch [%s] already exists", branchName)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestBranch_OrchestrateBranchCreation_HeadRefNotExists verifies that branch
// creation fails when HEAD points to a symbolic ref that does not exist.
func TestBranch_OrchestrateBranchCreation_HeadRefNotExists(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	branchName := testutils.RandomString(8)

	err := OrchestrateBranchCreation(repoPath, branchName)
	if err == nil {
		t.Fatal("Expected error when the branch already exists.")
	}

	expectedErrorMessage := "failed to read current ref:"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestBranch_OrchestrateBranchCreation_EmptyBranchName verifies that empty and
// whitespace-only branch names are rejected.
func TestBranch_OrchestrateBranchCreation_EmptyBranchName(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	err := OrchestrateBranchCreation(repoPath, "")
	if err == nil {
		t.Fatal("Expected error when the branch name is empty.")
	}

	expectedErrorMessage := "branch name cannot be empty"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestBranch_OrchestrateBranchCreation_InvalidBranchName verifies that
// malformed branch names are rejected by validation rules.
func TestBranch_OrchestrateBranchCreation_InvalidBranchName(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	expectedErrorMessage := "invalid branch name"

	err := OrchestrateBranchCreation(repoPath, "aa..bb")
	if err == nil {
		t.Fatal("Expected error when the branch name is invalid.")
	}
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}

	err = OrchestrateBranchCreation(repoPath, ".")
	if err == nil {
		t.Fatal("Expected error when the branch name is invalid.")
	}
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}

	err = OrchestrateBranchCreation(repoPath, "a\\b")
	if err == nil {
		t.Fatal("Expected error when the branch name is invalid.")
	}
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}

	err = OrchestrateBranchCreation(repoPath, "a.lock")
	if err == nil {
		t.Fatal("Expected error when the branch name is invalid.")
	}
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestBranch_writeRefFileExclusive_DoesNotOverwriteExistingRef verifies that
// exclusive creation fails with os.ErrExist when the target ref file already exists.
func TestBranch_writeRefFileExclusive_DoesNotOverwriteExistingRef(t *testing.T) {
	repoPath := t.TempDir()
	refPath := filepath.Join(repoPath, testutils.RandomString(5))

	initialContent := testutils.RandomBytes(10)
	if err := os.WriteFile(refPath, initialContent, constants.FilePerms); err != nil {
		t.Fatalf("Failed to create ref file: %v", err)
	}

	err := writeRefFileExclusive(refPath, []byte("new-content\n"))
	if err == nil {
		t.Fatal("Expected os.ErrExist when writing exclusively an existing ref file")
	}

	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("Expected [%v] error, got: [%v]", os.ErrExist, err)
	}
}

// readBranchRefHash reads refs/heads/<branchName> and returns the trimmed commit hash.
func readBranchRefHash(t *testing.T, repoPath, branchName string) string {
	t.Helper()

	branchRefPath := filepath.Join(
		repoPath,
		constants.Gogit,
		constants.Refs,
		constants.Heads,
		filepath.FromSlash(branchName),
	)
	content, err := os.ReadFile(branchRefPath)
	if err != nil {
		t.Fatalf("Failed to read ref file for branch [%s]: %v", branchName, err)
	}

	return string(content)
}
