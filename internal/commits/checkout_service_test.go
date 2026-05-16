package commits

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/index/indextest"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/objects/objectstest"
	"github.com/KostasZigo/gogit/internal/testutils"
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
	tree, blobs := objectstest.StoreBlobTree(t, store, fileName)

	err := RestoreTreeAndRebuildIndex(repoPath, tree.Hash())
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
	subtree, blobs := objectstest.StoreBlobTree(t, store, fileName)

	// Root Tree
	rootTreeEntry := objectstest.CreateTreeEntry(t, objects.ModeDirectory, dirName, subtree.Hash())
	entries := []objects.TreeEntry{
		rootTreeEntry,
	}
	rootTree := objectstest.CreateAndStoreTree(t, store, entries)

	err := RestoreTreeAndRebuildIndex(repoPath, rootTree.Hash())
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
	subTree, subBlobs := objectstest.StoreBlobTree(t, store, subFile1, subFile2)

	// Create a root-level file
	rootFileName := testutils.RandomString(10)
	rootBlob := objectstest.CreateAndStoreBlob(t, store, []byte(testutils.RandomString(100)))

	// Build root tree combining the subdirectory and root-level file
	dirEntry := objectstest.CreateTreeEntry(t, objects.ModeDirectory, dirName, subTree.Hash())
	fileEntry := objectstest.CreateTreeEntry(t, objects.ModeRegularFile, rootFileName, rootBlob.Hash())
	rootTree := objectstest.CreateAndStoreTree(t, store, []objects.TreeEntry{dirEntry, fileEntry})

	err := RestoreTreeAndRebuildIndex(repoPath, rootTree.Hash())
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

	err := RestoreTreeAndRebuildIndex(repoPath, testutils.RandomHash())
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

	treeEntry := objectstest.CreateTreeEntry(t, objects.ModeRegularFile, testutils.RandomString(10), testutils.RandomHash())
	rootTree := objectstest.CreateAndStoreTree(t, store, []objects.TreeEntry{treeEntry})

	err := RestoreTreeAndRebuildIndex(repoPath, rootTree.Hash())
	if err == nil {
		t.Fatal("Expected error when tree a references non existent blob")
	}

	expectedErrorMessage := "TBD"
	if strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestCheckout_DeleteIndexFiles_SingleFile verifies that CleanWorkingTree
// removes a single tracked file from the repository root.
func TestCheckout_DeleteIndexFiles_SingleFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	idx := index.NewIndex()
	filePath := indextest.CreateTrackedFile(t, repoPath, repoPath, testutils.RandomString(10), idx)

	if err := CleanWorkingTree(repoPath, idx.GetEntryList()); err != nil {
		t.Fatalf("Failed to clean working directory: %v", err)
	}

	testutils.AssertFileNotExists(t, filePath)
}

// TestCheckout_DeleteIndexFiles_NestedFiles verifies that CleanWorkingTree
// removes tracked files inside a subdirectory and prunes the now-empty parent directory.
func TestCheckout_DeleteIndexFiles_NestedFiles(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	idx := index.NewIndex()
	dir := filepath.Join(repoPath, testutils.RandomString(10))

	filePaths := make([]string, 2)
	for i := range filePaths {
		filePaths[i] = indextest.CreateTrackedFile(t, repoPath, dir, testutils.RandomString(10), idx)
	}

	if err := CleanWorkingTree(repoPath, idx.GetEntryList()); err != nil {
		t.Fatalf("Failed to clean working directory: %v", err)
	}

	for _, filePath := range filePaths {
		testutils.AssertFileNotExists(t, filePath)
	}
	testutils.AssertDirNotExists(t, dir)
}

// TestCheckout_DeleteIndexFiles_UntrackedFilesRemain verifies that CleanWorkingTree
// only removes files tracked in the index, leaving untracked files and their parent
// directories intact — even when tracked and untracked files share the same directory.
func TestCheckout_DeleteIndexFiles_UntrackedFilesRemain(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	idx := index.NewIndex()
	dir := filepath.Join(repoPath, testutils.RandomString(10))

	// Tracked files: registered in the index, expected to be deleted
	trackedPaths := []string{
		indextest.CreateTrackedFile(t, repoPath, dir, testutils.RandomString(10), idx),
		indextest.CreateTrackedFile(t, repoPath, dir, testutils.RandomString(10), idx),
	}

	// Untracked files: NOT in the index, expected to survive cleanup
	untrackedPaths := []string{
		testutils.CreateTestFile(t, dir, testutils.RandomString(10), []byte(testutils.RandomString(10))),
		testutils.CreateTestFile(t, repoPath, testutils.RandomString(10), []byte(testutils.RandomString(10))),
	}

	if err := CleanWorkingTree(repoPath, idx.GetEntryList()); err != nil {
		t.Fatalf("Failed to clean working directory: %v", err)
	}

	for _, filePath := range trackedPaths {
		testutils.AssertFileNotExists(t, filePath)
	}
	for _, filePath := range untrackedPaths {
		testutils.AssertFileExists(t, filePath)
	}
	testutils.AssertDirExists(t, dir)
}

// TestCheckout_DeleteIndexFiles_EmptyIndex verifies that CleanWorkingTree
// returns no error and leaves existing files untouched when the index is empty.
func TestCheckout_DeleteIndexFiles_EmptyIndex(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	filePath := testutils.CreateTestFile(t, repoPath, testutils.RandomString(10), []byte(testutils.RandomString(10)))

	idx := index.NewIndex()

	if err := CleanWorkingTree(repoPath, idx.GetEntryList()); err != nil {
		t.Fatalf("Failed to clean working directory: %v", err)
	}
	testutils.AssertFileExists(t, filePath)
}

// TestCheckout_DeleteIndexFiles_FileAlreadyMissing verifies that CleanWorkingTree
// does not error when an index entry references a file that no longer exists on disk.
func TestCheckout_DeleteIndexFiles_FileAlreadyMissing(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	idx := index.NewIndex()

	fileName := testutils.RandomString(10)
	entry, err := index.NewEntry(index.ModeRegularFile, testutils.RandomHash(), filepath.ToSlash(fileName), testutils.RandomInt(100), time.Now())
	if err != nil {
		t.Fatalf("failed to create index entry for %s: %v", fileName, err)
	}

	if err := idx.AddEntry(entry); err != nil {
		t.Fatalf("failed to add index entry for %s: %v", fileName, err)
	}

	if err := CleanWorkingTree(repoPath, idx.GetEntryList()); err != nil {
		t.Fatalf("Failed to clean working directory: %v", err)
	}

	testutils.AssertFileNotExists(t, filepath.Join(repoPath, fileName))
}

func TestCheckout_RebuildIndex_SingleFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	store := objects.NewObjectStore(repoPath)

	fileName := testutils.RandomString(10)
	tree, blobs := objectstest.StoreBlobTree(t, store, fileName)

	err := RestoreTreeAndRebuildIndex(repoPath, tree.Hash())
	if err != nil {
		t.Fatalf("Failed to restore tree: %v", err)
	}

	idxManager := index.NewManager(repoPath)
	idx, err := idxManager.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	if len(idx.GetEntryList()) != len(blobs) {
		t.Fatalf("Expected index entries length to be [%d], got [%d]", len(blobs), len(idx.GetEntryList()))
	}

	blob := blobs[fileName]
	indexEntry := idx.GetEntryList()[0]
	if indexEntry.Hash() != blob.Hash() {
		t.Fatalf("Expected index entry's hash to be [%s], got [%s]", blob.Hash(), indexEntry.Hash())
	}
	if indexEntry.Path() != fileName {
		t.Fatalf("Expected index entry's rel path to be [%s], got [%s]", fileName, indexEntry.Path())
	}
	if indexEntry.Mode() != index.ModeRegularFile {
		t.Fatalf("Expected file mode to be [%v], got [%v]", index.ModeRegularFile, indexEntry.Mode())
	}
}

func TestCheckout_RebuildIndex_ManyFiles_DifferentLevels(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	// Create subdirectory as a flat tree with two files
	dirName := testutils.RandomString(10)
	subFile1 := testutils.RandomString(10)
	subFile2 := testutils.RandomString(10)
	subTree, subBlobs := objectstest.StoreBlobTree(t, store, subFile1, subFile2)

	// Create a root-level file
	rootFileName := testutils.RandomString(10)
	rootBlob := objectstest.CreateAndStoreBlob(t, store, []byte(testutils.RandomString(100)))

	// Build root tree combining the subdirectory and root-level file
	dirEntry := objectstest.CreateTreeEntry(t, objects.ModeDirectory, dirName, subTree.Hash())
	fileEntry := objectstest.CreateTreeEntry(t, objects.ModeRegularFile, rootFileName, rootBlob.Hash())
	rootTree := objectstest.CreateAndStoreTree(t, store, []objects.TreeEntry{dirEntry, fileEntry})

	err := RestoreTreeAndRebuildIndex(repoPath, rootTree.Hash())
	if err != nil {
		t.Fatalf("Failed to restore tree: %v", err)
	}

	idxManager := index.NewManager(repoPath)
	idx, err := idxManager.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	// collect all blobs
	blobMap := make(map[string]*objects.Blob, len(subBlobs)+1)
	for k, v := range subBlobs {
		relPath := filepath.ToSlash(filepath.Join(dirName, k))
		blobMap[relPath] = v
	}
	blobMap[rootFileName] = rootBlob

	// Assertions
	if len(idx.GetEntryList()) != len(blobMap) {
		t.Fatalf("Expected index entries length to be [%d], got [%d]", len(blobMap), len(idx.GetEntryList()))
	}

	for _, entry := range idx.GetEntryList() {
		blob, exist := blobMap[entry.Path()]
		if !exist {
			t.Fatalf("Expected index entry with relative path [%s] to exist in the list of created blobs [%v]", entry.Path(), blobMap)
		}
		if entry.Hash() != blob.Hash() {
			t.Fatalf("Expected index entry's hash to be [%s], got [%s]", blob.Hash(), entry.Hash())
		}
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
	if err := RestoreTreeAndRebuildIndex(repoPath, secondTree.Hash()); err != nil {
		t.Fatalf("failed to set up working tree: %v", err)
	}
	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), updatedContent)

	featureBranch := testutils.RandomString(10)
	testutils.WriteRefFile(t, repoPath, featureBranch, firstCommit.Hash())

	if err := OrchestrateCheckoutExecution(repoPath, featureBranch, false); err != nil {
		t.Fatalf("Failed to checkout branch [%s]: %v", featureBranch, err)
	}

	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), originalContent)

	expectedHEAD := constants.DefaultRefPrefix + featureBranch + "\n"
	if head := testutils.ReadHEADFile(t, repoPath); head != expectedHEAD {
		t.Fatalf("Expected HEAD to be [%s], got [%s]", expectedHEAD, head)
	}

	idxManager := index.NewManager(repoPath)
	idx, err := idxManager.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	entries := idx.GetEntryList()
	if len(entries) != len(firstBlobs) {
		t.Fatalf("Expected index to have [%d] entry, got %d", len(firstBlobs), len(entries))
	}
	if entries[0].Hash() != firstBlobs[fileName].Hash() {
		t.Fatalf("Expected index entry hash [%s], got [%s]", firstBlobs[fileName].Hash(), entries[0].Hash())
	}
	if entries[0].Path() != fileName {
		t.Fatalf("Expected index entry path [%s], got [%s]", fileName, entries[0].Path())
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
	if err := RestoreTreeAndRebuildIndex(repoPath, secondTree.Hash()); err != nil {
		t.Fatalf("failed to set up working tree: %v", err)
	}
	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), updatedContent)

	firstCommitHash := firstCommit.Hash()
	if err := OrchestrateCheckoutExecution(repoPath, firstCommitHash, false); err != nil {
		t.Fatalf("Failed to checkout commit [%s]: %v", firstCommit.Hash(), err)
	}
	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), originalContent)

	expectedHEAD := firstCommitHash + "\n"
	if head := testutils.ReadHEADFile(t, repoPath); head != expectedHEAD {
		t.Fatalf("Expected HEAD to be [%s], got [%s]", expectedHEAD, head)
	}

	idxManager := index.NewManager(repoPath)
	idx, err := idxManager.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	entries := idx.GetEntryList()
	if len(entries) != len(firstBlob) {
		t.Fatalf("Expected index to have [%d] entry, got %d", len(firstBlob), len(entries))
	}
	if entries[0].Hash() != firstBlob[fileName].Hash() {
		t.Fatalf("Expected index entry hash [%s], got [%s]", firstBlob[fileName].Hash(), entries[0].Hash())
	}
	if entries[0].Path() != fileName {
		t.Fatalf("Expected index entry path [%s], got [%s]", fileName, entries[0].Path())
	}
}

func TestCheckout_Orchestrate_DirtyDir(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	fileName := testutils.RandomString(10)
	tree, blob := objectstest.StoreBlobTreeWithContent(t, store, map[string][]byte{fileName: []byte(testutils.RandomString(10))})
	commit := objectstest.CreateAndStoreCommit(t, store, tree.Hash(), "", testutils.RandomString(20))

	idx := index.NewIndex()
	filePath := indextest.CreateTrackedFileContent(t, repoPath, repoPath, fileName, blob[fileName].Content(), idx)

	idxManager := index.NewManager(repoPath)
	if err := idxManager.Save(idx); err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	updatedContent := slices.Concat(originalContent, []byte(" new content "))
	if err := os.WriteFile(filePath, updatedContent, constants.FilePerms); err != nil {
		t.Fatalf("Failed to write updated file: %v", err)
	}

	err = OrchestrateCheckoutExecution(repoPath, commit.Hash(), false)
	if err == nil {
		t.Fatal("Expected error when directory is dirty")
	}

	expectedErrorMessage := fmt.Sprintf("working directory contains dirty files:\n\ndirty: [%s] file was modified", fileName)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

func TestCheckout_Orchestrate_NonExistingTarget(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	hash := testutils.RandomHash()
	err := OrchestrateCheckoutExecution(repoPath, hash, false)
	if err == nil {
		t.Fatal("Expected error target reference does not exist.")
	}

	expectedErrorMessage := fmt.Sprintf("failure while resolving target: checkout target [%s] not found as branch or commit", hash)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
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
	if err := RestoreTreeAndRebuildIndex(repoPath, tree.Hash()); err != nil {
		t.Fatalf("failed to set up working tree: %v", err)
	}

	idxManager := index.NewManager(repoPath)
	idxBefore, err := idxManager.Load()
	if err != nil {
		t.Fatalf("Failed to load index before checkout: %v", err)
	}
	entriesBefore := idxBefore.GetEntryList()

	if err := OrchestrateCheckoutExecution(repoPath, constants.DefaultBranch, false); err != nil {
		t.Fatalf("Failed to checkout current branch: %v", err)
	}

	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), content)
	expectedHEAD := constants.DefaultRefPrefix + constants.DefaultBranch + "\n"
	if head := testutils.ReadHEADFile(t, repoPath); head != expectedHEAD {
		t.Fatalf("Expected HEAD to be [%s], got [%s]", expectedHEAD, head)
	}

	idxAfter, err := idxManager.Load()
	if err != nil {
		t.Fatalf("Failed to load index after checkout: %v", err)
	}
	entriesAfter := idxAfter.GetEntryList()
	if len(entriesAfter) != len(entriesBefore) {
		t.Fatalf("Expected index to have [%d] entries, got %d", len(entriesBefore), len(entriesAfter))
	}
	if entriesAfter[0].Hash() != blobs[fileName].Hash() {
		t.Fatalf("Expected index entry hash [%s], got [%s]", blobs[fileName].Hash(), entriesAfter[0].Hash())
	}
	if entriesAfter[0].Path() != fileName {
		t.Fatalf("Expected index entry path [%s], got [%s]", fileName, entriesAfter[0].Path())
	}
}
