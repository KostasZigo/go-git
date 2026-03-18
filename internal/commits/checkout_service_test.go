package commits

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/objects/objectstestutils"
	"github.com/KostasZigo/gogit/testutils"
)

// TestCheckout_ResolveTarget_Branch verifies resolution of a valid branch name
// to the commit hash stored in its ref file.
func TestCheckout_ResolveTarget_Branch(t *testing.T) {
	target := testutils.RandomString(10)
	branchRefContent := []byte(testutils.RandomHash() + "\n")

	repoPath := testutils.SetupTestRepoWithInit(t)
	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads)
	testutils.CreateTestFile(t, refPath, target, branchRefContent)

	resolvedTarget, err := ResolveTarget(repoPath, target)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !resolvedTarget.IsBranch {
		t.Fatal("Expected target to be found from branches")
	}

	commitHash := resolvedTarget.Hash
	if len(commitHash) != constants.HashStringLength {
		t.Fatalf("Hash is malformed, expected length [%d], got [%d]", constants.HashStringLength, len(commitHash))
	}

	expectedCommitHash := strings.TrimSpace(string(branchRefContent))
	if commitHash != expectedCommitHash {
		t.Fatalf("Exepcted commit hash to be [%s], got [%s]", expectedCommitHash, commitHash)
	}
}

// TestCheckout_ResolveTarget_CommitHash verifies resolution of a valid commit hash
// directly from the object store when no matching branch exists.
func TestCheckout_ResolveTarget_CommitHash(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	// store commit
	commit, err := objects.NewCommit(testutils.RandomHash(),
		"",
		testutils.RandomString(10),
		objects.DefaultAuthor())
	if err != nil {
		t.Fatalf("Failed to create commit: %v", err)
	}
	store := objects.NewObjectStore(repoPath)
	if err := store.Store(commit); err != nil {
		t.Fatalf("Failed to store commit: %v", err)
	}

	target := commit.Hash()
	resolvedTarget, err := ResolveTarget(repoPath, target)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resolvedTarget.IsBranch {
		t.Fatal("Expected commit hash to be resolved from commit objects.")
	}

	commitHash := resolvedTarget.Hash
	if len(commitHash) != constants.HashStringLength {
		t.Fatalf("Hash is malformed, expected length [%d], got [%d]", constants.HashStringLength, len(commitHash))
	}

	if commitHash != target {
		t.Fatalf("Exepcted commit hash to be [%s], got [%s]", target, commitHash)
	}
}

// TestCheckout_ResolveTarget_BranchNonExist_TargetNoSHA1 verifies error when target
// is not a valid SHA-1 hash and does not match any branch name.
func TestCheckout_ResolveTarget_BranchNonExist_TargetNoSHA1(t *testing.T) {
	target := testutils.RandomString(10)
	repoPath := testutils.SetupTestRepoWithInit(t)

	_, err := ResolveTarget(repoPath, target)
	if err == nil {
		t.Fatal("Expected error when branch is not existent")
	}

	expectedErrorMessage := fmt.Sprintf("checkout target [%s] not found as branch or commit", target)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestCheckout_ResolveTarget_BranchNonExist_CommitNonExist verifies error when target
// is a valid SHA-1 format but matches neither a branch nor a stored commit.
func TestCheckout_ResolveTarget_BranchNonExist_CommitNonExist(t *testing.T) {
	target := testutils.RandomHash()
	repoPath := testutils.SetupTestRepoWithInit(t)

	_, err := ResolveTarget(repoPath, target)
	if err == nil {
		t.Fatal("Expected error when commit is not existent")
	}

	expectedErrorMessage := fmt.Sprintf("checkout target [%s] not found as branch or commit", target)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestCheckout_ResolveTarget_EmptyString verifies error when target is an empty string.
func TestCheckout_ResolveTarget_EmptyString(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	_, err := ResolveTarget(repoPath, "")
	if err == nil {
		t.Fatal("Expected error when target is empty")
	}

	expectedErrorMessage := "checkout target cannot be empty"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestCheckout_ResolveTarget_BranchTakesPriorityOverCommitHash verifies that branch
// resolution takes priority over commit hash lookup when the target matches both.
func TestCheckout_ResolveTarget_BranchTakesPriorityOverCommitHash(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	// Store a commit in the object store
	commit, err := objects.NewCommit(testutils.RandomHash(),
		"",
		testutils.RandomString(10),
		objects.DefaultAuthor())
	if err != nil {
		t.Fatalf("Failed to create commit: %v", err)
	}
	store := objects.NewObjectStore(repoPath)
	if err := store.Store(commit); err != nil {
		t.Fatalf("Failed to store commit: %v", err)
	}

	// Create a branch ref file with the same name as the commit hash
	branchCommitHash := testutils.RandomHash()
	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads)
	testutils.CreateTestFile(t, refPath, commit.Hash(), []byte(branchCommitHash+"\n"))

	resolvedTarget, err := ResolveTarget(repoPath, commit.Hash())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !resolvedTarget.IsBranch {
		t.Fatal("Expected branch resolution to take priority over commit hash")
	}

	if resolvedTarget.Hash != branchCommitHash {
		t.Fatalf("Expected commit hash to be [%s], got [%s]", branchCommitHash, resolvedTarget.Hash)
	}
}

func TestCheckout_RestoreTree_SingleRootFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	store := objects.NewObjectStore(repoPath)

	// Create a blob
	blob := objects.NewBlob([]byte(testutils.RandomString(100)))
	if err := store.Store(blob); err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, testutils.RandomString(10), blob.Hash())
}
