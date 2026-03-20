package commits

import (
	"fmt"
	"os"
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

// TestCheckout_RestoreTree_SingleRootFile verifies that a tree with a single blob entry
// restores the file to disk with correct content and no extra files.
func TestCheckout_RestoreTree_SingleRootFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	fileName := testutils.RandomString(10)
	tree, blobs := objectstestutils.StoreBlobTree(t, store, fileName)

	err := RestoreTree(repoPath, tree.Hash())
	if err != nil {
		t.Fatalf("Failed to restore tree: %v", err)
	}

	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), blobs[fileName].Content())

	// Verify no extra files were created in repo root (only .gogit dir and restored file)
	dirEntries, err := os.ReadDir(repoPath)
	if err != nil {
		t.Fatalf("Failed to read repo directory: %v", err)
	}

	for _, entry := range dirEntries {
		if entry.Name() == constants.Gogit || entry.Name() == fileName {
			continue
		}
		t.Fatalf("Unexpected restored file [%s]", entry.Name())
	}
}

// TestCheckout_RestoreTree_NestedDirectory verifies that a tree referencing a subtree
// creates the directory and restores files inside with correct content.
func TestCheckout_RestoreTree_NestedDirectory(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	dirName := testutils.RandomString(10)
	fileName := testutils.RandomString(10)
	subtree, blobs := objectstestutils.StoreBlobTree(t, store, fileName)

	// Root Tree
	rootTreeEntry := objectstestutils.CreateTreeEntry(t, objects.ModeDirectory, dirName, subtree.Hash())
	entries := []objects.TreeEntry{
		rootTreeEntry,
	}
	rootTree := objectstestutils.CreateAndStoreTree(t, store, entries)

	err := RestoreTree(repoPath, rootTree.Hash())
	if err != nil {
		t.Fatalf("Failed to restore tree: %v", err)
	}

	testutils.AssertDirExists(t, filepath.Join(repoPath, dirName))
	testutils.AssertFileContent(t, filepath.Join(repoPath, dirName, fileName), blobs[fileName].Content())
}

// TestCheckout_RestoreTree_ManyFiles_DifferentLevels verifies that a tree with
// multiple files at root level and inside a subdirectory restores all files correctly.
func TestCheckout_RestoreTree_ManyFiles_DifferentLevels(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	// Create subdirectory as a flat tree with two files
	dirName := testutils.RandomString(10)
	subFile1 := testutils.RandomString(10)
	subFile2 := testutils.RandomString(10)
	subTree, subBlobs := objectstestutils.StoreBlobTree(t, store, subFile1, subFile2)

	// Create a root-level file
	rootFileName := testutils.RandomString(10)
	rootBlob := objectstestutils.CreateAndStoreBlob(t, store, []byte(testutils.RandomString(100)))

	// Build root tree combining the subdirectory and root-level file
	dirEntry := objectstestutils.CreateTreeEntry(t, objects.ModeDirectory, dirName, subTree.Hash())
	fileEntry := objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, rootFileName, rootBlob.Hash())
	rootTree := objectstestutils.CreateAndStoreTree(t, store, []objects.TreeEntry{dirEntry, fileEntry})

	err := RestoreTree(repoPath, rootTree.Hash())
	if err != nil {
		t.Fatalf("Failed to restore tree: %v", err)
	}

	// Verify root-level file
	testutils.AssertFileContent(t, filepath.Join(repoPath, rootFileName), rootBlob.Content())

	// Verify subdirectory and its files
	testutils.AssertDirExists(t, filepath.Join(repoPath, dirName))
	testutils.AssertFileContent(t, filepath.Join(repoPath, dirName, subFile1), subBlobs[subFile1].Content())
	testutils.AssertFileContent(t, filepath.Join(repoPath, dirName, subFile2), subBlobs[subFile2].Content())
}

// TestCheckout_RestoreTree_UnknowTreeHash verifies error when the provided
// tree hash does not exist in the object store.
func TestCheckout_RestoreTree_UnknowTreeHash(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	err := RestoreTree(repoPath, testutils.RandomHash())
	if err == nil {
		t.Fatal("Expected error when tree hash to restore doesn't exist")
	}

	expectedErrorMessage := "TBD"
	if strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestCheckout_RestoreTree_UnknowBlobHash_ReferencedByTree verifies error when a tree
// entry references a blob hash that does not exist in the object store.
func TestCheckout_RestoreTree_UnknowBlobHash_ReferencedByTree(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	treeEntry := objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, testutils.RandomString(10), testutils.RandomHash())
	rootTree := objectstestutils.CreateAndStoreTree(t, store, []objects.TreeEntry{treeEntry})

	err := RestoreTree(repoPath, rootTree.Hash())
	if err == nil {
		t.Fatal("Expected error when tree a references non existent blob")
	}

	expectedErrorMessage := "TBD"
	if strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}
