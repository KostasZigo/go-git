package branches

import (
	"errors"
	"fmt"
	"os"
	"path"
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

	newBranchName := testutils.RandomString(8)
	err := OrchestrateBranchCreation(repoPath, newBranchName)
	if err != nil {
		t.Fatalf("unexpected error creating branch [%s]: %v", newBranchName, err)
	}

	assertBranchRefHash(t, repoPath, newBranchName, currentCommitHash)
}

// TestBranch_OrchestrateBranchCreation_HierarchicalSymbolicHEAD verifies that
// branch creation resolves a hierarchical current branch from symbolic HEAD.
func TestBranch_OrchestrateBranchCreation_HierarchicalSymbolicHEAD(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	currentBranchName := path.Join(testutils.RandomString(8), testutils.RandomString(8))
	currentCommitHash := testutils.RandomHash()
	headContent := constants.DefaultRefPrefix + currentBranchName + "\n"
	testutils.WriteHEADFile(t, repoPath, []byte(headContent))
	writeBranchRefFixture(t, repoPath, currentBranchName, []byte(currentCommitHash+"\n"))
	newBranchName := testutils.RandomString(8)

	if err := OrchestrateBranchCreation(repoPath, newBranchName); err != nil {
		t.Fatalf("unexpected error creating branch [%s]: %v", newBranchName, err)
	}

	assertBranchRefHash(t, repoPath, newBranchName, currentCommitHash)
	testutils.AssertHEADContent(t, repoPath, headContent)
}

// TestBranch_OrchestrateBranchCreation_DetachedHead verifies that creating a branch
// while HEAD is in detached state copies the commit hash from HEAD into
// refs/heads/<new-branch>.
func TestBranch_OrchestrateBranchCreation_DetachedHead(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	currentCommitHash := testutils.RandomHash()
	testutils.WriteHEADFile(t, repoPath, []byte(currentCommitHash+"\n"))

	newBranchName := testutils.RandomString(8)
	err := OrchestrateBranchCreation(repoPath, newBranchName)
	if err != nil {
		t.Fatalf("unexpected error creating branch [%s]: %v", newBranchName, err)
	}

	assertBranchRefHash(t, repoPath, newBranchName, currentCommitHash)
}

// TestBranch_OrchestrateBranchCreation_InvalidDetachedHEAD verifies that an
// invalid detached HEAD does not create a branch ref or leave a lock file.
func TestBranch_OrchestrateBranchCreation_InvalidDetachedHEAD(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	invalidHEADContent := testutils.RandomString(8) + "\n"
	testutils.WriteHEADFile(t, repoPath, []byte(invalidHEADContent))
	newBranchName := testutils.RandomString(8)

	err := OrchestrateBranchCreation(repoPath, newBranchName)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HEAD contains invalid commit hash") {
		t.Fatalf("expected invalid HEAD hash error, got [%v]", err)
	}

	refPath, pathErr := branchRefPath(repoPath, newBranchName)
	if pathErr != nil {
		t.Fatalf("failed to resolve branch ref path: %v", pathErr)
	}
	testutils.AssertFileNotExists(t, refPath)
	assertBranchRefLockNotExists(t, repoPath, newBranchName)
	testutils.AssertHEADContent(t, repoPath, invalidHEADContent)
}

// TestBranch_OrchestrateBranchCreation_HierarchicalBranchName verifies that
// creating a branch with hierarchical naming (path-like) succeeds and writes
// the expected commit hash into the branch ref.
func TestBranch_OrchestrateBranchCreation_HierarchicalBranchName(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	currentCommitHash := testutils.RandomHash()
	testutils.WriteHEADFile(t, repoPath, []byte(currentCommitHash+"\n"))

	dir := testutils.RandomString(8)
	name := testutils.RandomString(8)
	newBranchName := path.Join(dir, name)
	err := OrchestrateBranchCreation(repoPath, newBranchName)
	if err != nil {
		t.Fatalf("unexpected error creating branch [%s]: %v", newBranchName, err)
	}

	assertBranchRefHash(t, repoPath, newBranchName, currentCommitHash)
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
		t.Fatal("expected error when the branch already exists")
	}

	expectedErrorMessage := fmt.Sprintf("branch [%s] already exists", branchName)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestBranch_OrchestrateBranchCreation_HeadRefNotExists verifies that branch
// creation fails when HEAD points to a symbolic ref that does not exist.
func TestBranch_OrchestrateBranchCreation_HeadRefNotExists(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	branchName := testutils.RandomString(8)

	err := OrchestrateBranchCreation(repoPath, branchName)
	if err == nil {
		t.Fatal("expected error when the branch already exists")
	}

	expectedErrorMessage := "failed to read current ref:"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestBranch_OrchestrateBranchCreation_EmptyBranchName verifies that empty and
// whitespace-only branch names are rejected.
func TestBranch_OrchestrateBranchCreation_EmptyBranchName(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	err := OrchestrateBranchCreation(repoPath, "")
	if err == nil {
		t.Fatal("expected error when the branch name is empty")
	}

	expectedErrorMessage := "branch name cannot be empty"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestBranch_OrchestrateBranchCreation_InvalidBranchName verifies that
// malformed branch names are rejected by validation rules.
func TestBranch_OrchestrateBranchCreation_InvalidBranchName(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	expectedErrorMessage := "invalid branch name"

	err := OrchestrateBranchCreation(repoPath, "aa..bb")
	if err == nil {
		t.Fatal("expected error when the branch name is invalid")
	}
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}

	err = OrchestrateBranchCreation(repoPath, ".")
	if err == nil {
		t.Fatal("expected error when the branch name is invalid")
	}
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}

	err = OrchestrateBranchCreation(repoPath, "a\\b")
	if err == nil {
		t.Fatal("expected error when the branch name is invalid")
	}
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}

	err = OrchestrateBranchCreation(repoPath, "a.lock")
	if err == nil {
		t.Fatal("expected error when the branch name is invalid")
	}
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestBranch_OrchestrateBranchCreation_LockedRef verifies that branch creation
// respects an existing ref lock without creating or overwriting the branch.
func TestBranch_OrchestrateBranchCreation_LockedRef(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.WriteRefFile(t, repoPath, constants.DefaultBranch, testutils.RandomHash())
	branchName := testutils.RandomString(8)
	refPath, err := branchRefPath(repoPath, branchName)
	if err != nil {
		t.Fatalf("failed to resolve branch ref path: %v", err)
	}
	lockPath := refPath + ".lock"
	lockContent := testutils.RandomByteSlice(20)
	if err := os.WriteFile(lockPath, lockContent, constants.FilePerms); err != nil {
		t.Fatalf("failed to create branch ref lock: %v", err)
	}

	err = OrchestrateBranchCreation(repoPath, branchName)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrReferenceLocked) {
		t.Fatalf("expected reference locked error, got [%v]", err)
	}

	testutils.AssertFileNotExists(t, refPath)
	testutils.AssertFileContent(t, lockPath, lockContent)
}
