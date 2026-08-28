package commits

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/index/indextest"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/objects/objectstest"
	"github.com/KostasZigo/gogit/internal/testutils"
	"github.com/KostasZigo/gogit/internal/worktree"
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
		t.Fatalf("unexpected error: %v", err)
	}

	if !resolvedTarget.IsBranch {
		t.Fatal("expected target to be found from branches")
	}

	commitHash := resolvedTarget.Hash
	if len(commitHash) != constants.HashStringLength {
		t.Fatalf("hash is malformed, expected length [%d], got [%d]", constants.HashStringLength, len(commitHash))
	}

	expectedCommitHash := strings.TrimSpace(string(branchRefContent))
	if commitHash != expectedCommitHash {
		t.Fatalf("exepcted commit hash to be [%s], got [%s]", expectedCommitHash, commitHash)
	}
}

// TestCheckout_ResolveTarget_CommitHash verifies resolution of a valid commit hash
// directly from the object store when no matching branch exists.
func TestCheckout_ResolveTarget_CommitHash(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	// store commit
	commit, err := objects.NewInitialCommit(testutils.RandomHash(),
		testutils.RandomString(10),
		objects.DefaultAuthor())
	if err != nil {
		t.Fatalf("failed to create commit: %v", err)
	}
	store := objects.NewObjectStore(repoPath)
	if err := store.Store(commit); err != nil {
		t.Fatalf("failed to store commit: %v", err)
	}

	target := commit.Hash()
	resolvedTarget, err := ResolveTarget(repoPath, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolvedTarget.IsBranch {
		t.Fatal("expected commit hash to be resolved from commit objects")
	}

	commitHash := resolvedTarget.Hash
	if len(commitHash) != constants.HashStringLength {
		t.Fatalf("hash is malformed, expected length [%d], got [%d]", constants.HashStringLength, len(commitHash))
	}

	if commitHash != target {
		t.Fatalf("exepcted commit hash to be [%s], got [%s]", target, commitHash)
	}
}

// TestCheckout_ResolveTarget_BranchNonExist_TargetNoSHA1 verifies error when target
// is not a valid SHA-1 hash and does not match any branch name.
func TestCheckout_ResolveTarget_BranchNonExist_TargetNoSHA1(t *testing.T) {
	target := testutils.RandomString(10)
	repoPath := testutils.SetupTestRepoWithInit(t)

	_, err := ResolveTarget(repoPath, target)
	if err == nil {
		t.Fatal("expected error when branch is not existent")
	}

	expectedErrorMessage := fmt.Sprintf("checkout target [%s] not found as branch or commit", target)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestCheckout_ResolveTarget_BranchNonExist_CommitNonExist verifies error when target
// is a valid SHA-1 format but matches neither a branch nor a stored commit.
func TestCheckout_ResolveTarget_BranchNonExist_CommitNonExist(t *testing.T) {
	target := testutils.RandomHash()
	repoPath := testutils.SetupTestRepoWithInit(t)

	_, err := ResolveTarget(repoPath, target)
	if err == nil {
		t.Fatal("expected error when commit is not existent")
	}

	expectedErrorMessage := fmt.Sprintf("checkout target [%s] not found as branch or commit", target)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestCheckout_ResolveTarget_EmptyString verifies error when target is an empty string.
func TestCheckout_ResolveTarget_EmptyString(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	_, err := ResolveTarget(repoPath, "")
	if err == nil {
		t.Fatal("expected error when target is empty")
	}

	expectedErrorMessage := "checkout target cannot be empty"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestCheckout_ResolveTarget_BranchTakesPriorityOverCommitHash verifies that branch
// resolution takes priority over commit hash lookup when the target matches both.
func TestCheckout_ResolveTarget_BranchTakesPriorityOverCommitHash(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	// Store a commit in the object store
	commit, err := objects.NewInitialCommit(testutils.RandomHash(),
		testutils.RandomString(10),
		objects.DefaultAuthor())
	if err != nil {
		t.Fatalf("failed to create commit: %v", err)
	}
	store := objects.NewObjectStore(repoPath)
	if err := store.Store(commit); err != nil {
		t.Fatalf("failed to store commit: %v", err)
	}

	// Create a branch ref file with the same name as the commit hash
	branchCommitHash := testutils.RandomHash()
	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads)
	testutils.CreateTestFile(t, refPath, commit.Hash(), []byte(branchCommitHash+"\n"))

	resolvedTarget, err := ResolveTarget(repoPath, commit.Hash())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resolvedTarget.IsBranch {
		t.Fatal("expected branch resolution to take priority over commit hash")
	}

	if resolvedTarget.Hash != branchCommitHash {
		t.Fatalf("expected commit hash to be [%s], got [%s]", branchCommitHash, resolvedTarget.Hash)
	}
}

// TestCheckout_Orchestrate_BranchCheckout verifies the full checkout workflow
// for checking out a  branch. Constructs a two-commit history, populates the working tree
// with creates a branch pointing at the first commit
// the second commit's content, then checks out the branch.
// Asserts that the working tree is restored to the first commit's content,
// and HEAD is updated accordingly.
func TestCheckout_Orchestrate_BranchCheckout(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	fileName := testutils.RandomString(10)
	originalContent := []byte(testutils.RandomString(50))
	updatedContent := []byte(testutils.RandomString(50))

	firstTree, firstBlobs := objectstest.StoreBlobTreeWithContent(t, store, map[string][]byte{fileName: originalContent})
	firstCommit := objectstest.CreateAndStoreCommit(t, store, firstTree.Hash(), "", testutils.RandomString(20))

	secondTree, _ := objectstest.StoreBlobTreeWithContent(t, store, map[string][]byte{fileName: updatedContent})
	secondCommit := objectstest.CreateAndStoreCommit(t, store, secondTree.Hash(), firstCommit.Hash(), testutils.RandomString(20))

	// Point main at secondCommit and populate working tree + index to match
	testutils.WriteRefFile(t, repoPath, constants.DefaultBranch, secondCommit.Hash())
	applyStoredTreeSnapshot(t, repoPath, store, secondTree.Hash())
	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), updatedContent)

	featureBranch := testutils.RandomString(10)
	testutils.WriteRefFile(t, repoPath, featureBranch, firstCommit.Hash())

	if err := OrchestrateCheckoutExecution(repoPath, featureBranch, false); err != nil {
		t.Fatalf("failed to checkout branch [%s]: %v", featureBranch, err)
	}

	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), originalContent)

	expectedHEAD := constants.DefaultRefPrefix + featureBranch + "\n"
	if head := testutils.ReadHEADFile(t, repoPath); head != expectedHEAD {
		t.Fatalf("expected HEAD to be [%s], got [%s]", expectedHEAD, head)
	}

	idxManager := index.NewManager(repoPath)
	idx, err := idxManager.Load()
	if err != nil {
		t.Fatalf("failed to load index: %v", err)
	}

	entries := idx.GetEntryList()
	if len(entries) != len(firstBlobs) {
		t.Fatalf("expected index to have [%d] entry, got %d", len(firstBlobs), len(entries))
	}
	if entries[0].Hash() != firstBlobs[fileName].Hash() {
		t.Fatalf("expected index entry hash [%s], got [%s]", firstBlobs[fileName].Hash(), entries[0].Hash())
	}
	if entries[0].Path() != fileName {
		t.Fatalf("expected index entry path [%s], got [%s]", fileName, entries[0].Path())
	}
}

func TestCheckout_Orchestrate_CommitCheckout(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	fileName := testutils.RandomString(10)
	originalContent := []byte(testutils.RandomString(50))
	updatedContent := []byte(testutils.RandomString(50))

	firstTree, firstBlob := objectstest.StoreBlobTreeWithContent(t, store, map[string][]byte{fileName: originalContent})
	firstCommit := objectstest.CreateAndStoreCommit(t, store, firstTree.Hash(), "", testutils.RandomString(20))

	secondTree, _ := objectstest.StoreBlobTreeWithContent(t, store, map[string][]byte{fileName: updatedContent})
	secondCommit := objectstest.CreateAndStoreCommit(t, store, secondTree.Hash(), firstCommit.Hash(), testutils.RandomString(20))

	// Point main at secondCommit and populate working tree + index to match
	testutils.WriteRefFile(t, repoPath, constants.DefaultBranch, secondCommit.Hash())
	applyStoredTreeSnapshot(t, repoPath, store, secondTree.Hash())
	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), updatedContent)

	firstCommitHash := firstCommit.Hash()
	if err := OrchestrateCheckoutExecution(repoPath, firstCommitHash, false); err != nil {
		t.Fatalf("failed to checkout commit [%s]: %v", firstCommit.Hash(), err)
	}
	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), originalContent)

	expectedHEAD := firstCommitHash + "\n"
	if head := testutils.ReadHEADFile(t, repoPath); head != expectedHEAD {
		t.Fatalf("expected HEAD to be [%s], got [%s]", expectedHEAD, head)
	}

	idxManager := index.NewManager(repoPath)
	idx, err := idxManager.Load()
	if err != nil {
		t.Fatalf("failed to load index: %v", err)
	}

	entries := idx.GetEntryList()
	if len(entries) != len(firstBlob) {
		t.Fatalf("expected index to have [%d] entry, got %d", len(firstBlob), len(entries))
	}
	if entries[0].Hash() != firstBlob[fileName].Hash() {
		t.Fatalf("expected index entry hash [%s], got [%s]", firstBlob[fileName].Hash(), entries[0].Hash())
	}
	if entries[0].Path() != fileName {
		t.Fatalf("expected index entry path [%s], got [%s]", fileName, entries[0].Path())
	}
}

// TestCheckout_Orchestrate_ForceRemovesIndexOnlyPath verifies that forced
// checkout discards a staged addition absent from the target snapshot rather
// than leaving it behind as an untracked file.
func TestCheckout_Orchestrate_ForceRemovesIndexOnlyPath(t *testing.T) {
	// Arrange
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	trackedPath := testutils.RandomString(10)
	trackedContent := testutils.RandomBytes(20)
	headTree, _ := objectstest.StoreBlobTreeWithContent(t, store, map[string][]byte{trackedPath: trackedContent})
	headCommit := objectstest.CreateAndStoreCommit(t, store, headTree.Hash(), "", testutils.RandomString(11))
	targetCommit := objectstest.CreateAndStoreCommit(t, store, headTree.Hash(), headCommit.Hash(), testutils.RandomString(12))
	testutils.WriteRefFile(t, repoPath, constants.DefaultBranch, headCommit.Hash())

	idx := index.NewIndex()
	trackedFilePath := indextest.CreateTrackedFileContent(t, repoPath, repoPath, trackedPath, trackedContent, idx)
	indexOnlyPath := testutils.RandomString(13)
	indexOnlyFilePath := indextest.CreateTrackedFileContent(t, repoPath, repoPath, indexOnlyPath, testutils.RandomBytes(21), idx)
	if err := index.NewManager(repoPath).Save(idx); err != nil {
		t.Fatalf("failed to save pre-checkout index: %v", err)
	}

	// Act
	if err := OrchestrateCheckoutExecution(repoPath, targetCommit.Hash(), true); err != nil {
		t.Fatalf("failed forced checkout with staged addition: %v", err)
	}

	// Assert
	testutils.AssertFileContent(t, trackedFilePath, trackedContent)
	testutils.AssertFileNotExists(t, indexOnlyFilePath)
	testutils.AssertHEADContent(t, repoPath, targetCommit.Hash()+"\n")
	loadedIndex, err := index.NewManager(repoPath).Load()
	if err != nil {
		t.Fatalf("failed to load index after forced checkout: %v", err)
	}
	entries := loadedIndex.GetEntryList()
	if len(entries) != 1 || entries[0].Path() != trackedPath {
		t.Fatalf("expected replacement index to contain only [%s], got [%#v]", trackedPath, extractPathsFromIndexEntries(t, entries))
	}
}

// TestCheckout_Orchestrate_ForceRejectsRecreatedStagedDeletion verifies that
// force cannot remove untracked content recreated at a path staged for deletion
// and preserves HEAD, branch refs, the index, and disk on rejection.
func TestCheckout_Orchestrate_ForceRejectsRecreatedStagedDeletion(t *testing.T) {
	// Arrange
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	deletedPath := testutils.RandomString(10)
	headContent := testutils.RandomBytes(20)
	headTree, _ := objectstest.StoreBlobTreeWithContent(t, store, map[string][]byte{deletedPath: headContent})
	headCommit := objectstest.CreateAndStoreCommit(t, store, headTree.Hash(), "", testutils.RandomString(11))

	targetPath := testutils.RandomString(12)
	targetContent := testutils.RandomBytes(21)
	targetTree, _ := objectstest.StoreBlobTreeWithContent(t, store, map[string][]byte{targetPath: targetContent})
	targetCommit := objectstest.CreateAndStoreCommit(t, store, targetTree.Hash(), headCommit.Hash(), testutils.RandomString(13))

	testutils.WriteRefFile(t, repoPath, constants.DefaultBranch, headCommit.Hash())
	applyStoredTreeSnapshot(t, repoPath, store, headTree.Hash())

	deletedFilePath := filepath.Join(repoPath, deletedPath)
	if err := os.Remove(deletedFilePath); err != nil {
		t.Fatalf("failed to remove tracked file before staging its deletion: %v", err)
	}
	if err := index.NewManager(repoPath).Save(index.NewIndex()); err != nil {
		t.Fatalf("failed to stage file deletion: %v", err)
	}

	recreatedContent := testutils.RandomBytes(22)
	testutils.CreateTestFile(t, repoPath, deletedPath, recreatedContent)
	metadataBefore := captureCheckoutMetadata(t, repoPath, constants.DefaultBranch)

	// Act
	err := OrchestrateCheckoutExecution(repoPath, targetCommit.Hash(), true)

	// Assert
	state := requirePreflightError(t, err)
	expectedCollisions := []worktree.Collision{{Path: deletedPath, Kind: worktree.CollisionUntrackedFile}}
	if !slices.Equal(state.Collisions, expectedCollisions) {
		t.Fatalf("expected collisions [%#v], got [%#v]", expectedCollisions, state.Collisions)
	}
	testutils.AssertFileContent(t, deletedFilePath, recreatedContent)
	testutils.AssertFileNotExists(t, filepath.Join(repoPath, targetPath))
	assertCheckoutMetadataUnchanged(t, repoPath, metadataBefore)
}

func TestCheckout_Orchestrate_NonExistingTarget(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	hash := testutils.RandomHash()
	err := OrchestrateCheckoutExecution(repoPath, hash, false)
	if err == nil {
		t.Fatal("expected error target reference does not exist")
	}

	expectedErrorMessage := fmt.Sprintf("failure while resolving target: checkout target [%s] not found as branch or commit", hash)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestCheckout_Orchestrate_CurrentBranchNoop verifies that checking out the branch
// HEAD already points to succeeds without modifying the working tree, HEAD, or index.
func TestCheckout_Orchestrate_CurrentBranchNoop(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	fileName := testutils.RandomString(10)
	content := []byte(testutils.RandomString(50))

	tree, blobs := objectstest.StoreBlobTreeWithContent(t, store, map[string][]byte{fileName: content})
	commit := objectstest.CreateAndStoreCommit(t, store, tree.Hash(), "", testutils.RandomString(20))

	// Point main at commit and populate working tree + index to match
	testutils.WriteRefFile(t, repoPath, constants.DefaultBranch, commit.Hash())
	applyStoredTreeSnapshot(t, repoPath, store, tree.Hash())

	idxManager := index.NewManager(repoPath)
	idxBefore, err := idxManager.Load()
	if err != nil {
		t.Fatalf("failed to load index before checkout: %v", err)
	}
	entriesBefore := idxBefore.GetEntryList()

	if err := OrchestrateCheckoutExecution(repoPath, constants.DefaultBranch, false); err != nil {
		t.Fatalf("failed to checkout current branch: %v", err)
	}

	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), content)
	expectedHEAD := constants.DefaultRefPrefix + constants.DefaultBranch + "\n"
	if head := testutils.ReadHEADFile(t, repoPath); head != expectedHEAD {
		t.Fatalf("expected HEAD to be [%s], got [%s]", expectedHEAD, head)
	}

	idxAfter, err := idxManager.Load()
	if err != nil {
		t.Fatalf("failed to load index after checkout: %v", err)
	}
	entriesAfter := idxAfter.GetEntryList()
	if len(entriesAfter) != len(entriesBefore) {
		t.Fatalf("expected index to have [%d] entries, got %d", len(entriesBefore), len(entriesAfter))
	}
	if entriesAfter[0].Hash() != blobs[fileName].Hash() {
		t.Fatalf("expected index entry hash [%s], got [%s]", blobs[fileName].Hash(), entriesAfter[0].Hash())
	}
	if entriesAfter[0].Path() != fileName {
		t.Fatalf("expected index entry path [%s], got [%s]", fileName, entriesAfter[0].Path())
	}
}

func extractPathsFromIndexEntries(t *testing.T, idxEntries []*index.Entry) []string {
	t.Helper()
	pathsToRemove := make([]string, 0, len(idxEntries))
	for _, entry := range idxEntries {
		pathsToRemove = append(pathsToRemove, entry.Path())
	}
	return pathsToRemove
}
