package commits

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/branches"
	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// Test_BuildDirectoryTree_MixedRootAndNestedFiles verifies that a flat
// list of index entries with paths at multiple depths
// is correctly decomposed into a tree with root-level files, first-level
// children, and deeper nested children.
func Test_BuildDirectoryTree_MixedRootAndNestedFiles(t *testing.T) {
	folder := testutils.RandomString(3)
	subfolder := testutils.RandomString(3)
	filePaths := []string{
		testutils.RandomString(10),
		filepath.ToSlash(filepath.Join(folder, testutils.RandomString(10))),
		filepath.ToSlash(filepath.Join(folder, testutils.RandomString(10))),
		filepath.ToSlash(filepath.Join(folder, subfolder, testutils.RandomString(10))),
	}

	entries := make([]*index.Entry, len(filePaths))
	for i, filePath := range filePaths {
		indexEntry, err := index.NewEntry(
			index.ModeExecutable,
			testutils.RandomHash(),
			filePath,
			testutils.RandomInt(50),
			time.Now(),
		)
		if err != nil {
			t.Fatalf("failed to create index entry: %v", err)
		}
		entries[i] = indexEntry
	}

	// Root should have exactly 1 child directory
	rootNode := buildDirectoryTree(entries)
	if len(rootNode.files) != 1 {
		t.Fatalf("expected a single root level file but got [%d]", len(rootNode.files))
	}

	expectedRootFileName := filepath.Base(filePaths[0])
	if rootNode.files[0].name != expectedRootFileName {
		t.Fatalf("expected root file to be named [%s] but got [%s]", expectedRootFileName, rootNode.files[0].name)
	}

	if len(rootNode.children) != 1 {
		t.Fatalf("expected 1 child directory at root, got %d", len(rootNode.children))
	}

	// First-level directory should contain 2 files
	firstLevelNode := rootNode.children[folder]
	if len(firstLevelNode.files) != 2 {
		t.Fatalf("expected 2 first directory level files but got [%d]", len(firstLevelNode.files))
	}

	expectedFirstLevelFiles := []string{
		filepath.Base(filePaths[1]),
		filepath.Base(filePaths[2]),
	}
	actualFirstLevelFiles := []string{
		firstLevelNode.files[0].name,
		firstLevelNode.files[1].name,
	}

	for _, expected := range expectedFirstLevelFiles {
		if !slices.Contains(actualFirstLevelFiles, expected) {
			t.Fatalf("%s directory missing expected file: %q (found: %v)", folder, expected, actualFirstLevelFiles)
		}
	}

	if len(firstLevelNode.children) != 1 {
		t.Fatalf("expected 1 child directory in %q, got %d", folder, len(firstLevelNode.children))
	}

	// Second-level directory should contain 1 file
	secondLevelNode := firstLevelNode.children[subfolder]
	if len(secondLevelNode.files) != 1 {
		t.Fatalf("expected a single second directory level file but got [%d]", len(rootNode.files))
	}

	expextedSecondLevelFileName := filepath.Base(filePaths[3])
	if secondLevelNode.files[0].name != expextedSecondLevelFileName {
		t.Fatalf("expected second level directory file to be named [%s] but got [%s]", expextedSecondLevelFileName, secondLevelNode.files[0].name)
	}

	if len(secondLevelNode.children) != 0 {
		t.Fatalf("expected 0 children in %s/%s, got %d", folder, subfolder, len(secondLevelNode.children))
	}
}

// Test_BuildDirectoryTree_EmptyEntries
// verifies that an empty input produces a valid root node with no files
// and no children, without panicking.
func Test_BuildDirectoryTree_EmptyEntries(t *testing.T) {
	var entries []*index.Entry
	rootNode := buildDirectoryTree(entries)

	if len(rootNode.files) != 0 {
		t.Fatalf("expected 0 root files when no entries exist but got [%d]", len(rootNode.files))
	}
	if len(rootNode.children) != 0 {
		t.Fatalf("expected 0 children nodes when no entries exist but got [%d]", len(rootNode.children))
	}
}

// Test_WriteTree_WithFilesAndSubdirectory
// Builds a directoryNode with a root-level blob and a subdirectory containing
// another blob. Verifies both tree objects are stored, the root tree contains
// two entries (blob + subtree), and the subtree is readable with the correct
// child entry.
func Test_WriteTree_WithFilesAndSubdirectory(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	blobRoot := objects.NewBlob([]byte(testutils.RandomString(100)))
	blobSubDir := objects.NewBlob([]byte(testutils.RandomString(100)))

	if err := store.Store(blobRoot); err != nil {
		t.Fatalf("failed to store root blob: %v", err)
	}
	if err := store.Store(blobSubDir); err != nil {
		t.Fatalf("failed to store sub dir blob: %v", err)
	}

	rootFileName := testutils.RandomString(10)
	subDirName := testutils.RandomString(3)
	subDirFileName := testutils.RandomString(10)
	rootNode := &directoryNode{
		files: []fileEntry{
			{
				name: rootFileName,
				mode: objects.ModeExecutable,
				hash: blobRoot.Hash(),
			},
		},
		children: map[string]*directoryNode{
			subDirName: {
				files: []fileEntry{
					{
						name: subDirFileName,
						mode: objects.ModeExecutable,
						hash: blobSubDir.Hash(),
					},
				},
				children: make(map[string]*directoryNode),
			},
		},
	}

	rootHash, err := writeTree(rootNode, store)
	if err != nil {
		t.Fatalf("failed to write tree: %v", err)
	}

	if len(rootHash) != constants.HashStringLength {
		t.Fatalf("expected %d-char hash, got %d chars: %s", constants.HashStringLength, len(rootHash), rootHash)
	}

	// Verify root tree is readable and has 2 entries (1 root file + 1 sub dir )
	rootTree, err := store.ReadTree(rootHash)
	if err != nil {
		t.Fatalf("failed to read root tree: %v", err)
	}

	expectedNoRootTreeEntries := (len(rootNode.files) + len(rootNode.children))
	actualNoRootTreeEntries := len(rootTree.Entries())
	if actualNoRootTreeEntries != expectedNoRootTreeEntries {
		t.Fatalf("expected %d entries but got [%d]", expectedNoRootTreeEntries, actualNoRootTreeEntries)
	}

	entryNames := make([]string, actualNoRootTreeEntries)
	for i, e := range rootTree.Entries() {
		entryNames[i] = e.Name()
	}
	if !slices.Contains(entryNames, rootFileName) {
		t.Fatalf("root tree missing [%s] entry (found: %v)", rootFileName, entryNames)
	}
	if !slices.Contains(entryNames, subDirName) {
		t.Fatalf("root tree missing [%s] entry (found: %v)", subDirName, entryNames)
	}

	// Verify the subtree is stored and readable
	subTreeEntry, found := rootTree.FindEntry(subDirName)
	if !found {
		t.Fatalf("expected [%s] to exist in root tree", subDirName)
	}

	subTree, err := store.ReadTree(subTreeEntry.Hash())
	if err != nil {
		t.Fatalf("failed to read sub tree: %v", err)
	}

	subEntries := subTree.Entries()
	if len(subEntries) != 1 {
		t.Fatalf("expected 1 entry in subtree, got %d", len(subEntries))
	}
	if subEntries[0].Name() != subDirFileName {
		t.Fatalf("expected [%s] in subtree, got %q", subDirFileName, subEntries[0].Name())
	}
	if subEntries[0].Hash() != blobSubDir.Hash() {
		t.Fatalf("expected [%s] hash [%s], got [%s]", subDirFileName, blobSubDir.Hash(), subEntries[0].Hash())
	}
}

// Test_WriteTree_Idempotency verifies that
// same input twice produces same root hash (idempotency)
func Test_WriteTree_Idempotency(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	blob := objects.NewBlob([]byte(testutils.RandomString(100)))
	if err := store.Store(blob); err != nil {
		t.Fatalf("failed to store blob: %v", err)
	}

	fileName := testutils.RandomString(100)
	buildNode := func() *directoryNode {
		return &directoryNode{
			files: []fileEntry{
				{name: fileName, mode: objects.ModeRegularFile, hash: blob.Hash()},
			},
			children: make(map[string]*directoryNode),
		}
	}

	hash1, err := writeTree(buildNode(), store)
	if err != nil {
		t.Fatalf("first writeTree failed: %v", err)
	}

	hash2, err := writeTree(buildNode(), store)
	if err != nil {
		t.Fatalf("second writeTree failed: %v", err)
	}

	if hash1 != hash2 {
		t.Fatalf("expected identical hashes for identical input, got [%s] and [%s]", hash1, hash2)
	}
}

// Test_CreateAndStoreCommit_RoundTrip
// creates a commit with a real tree reference, stores it, reads it back,
// and verifies all fields (tree hash, parent hash, message, author) match.
func Test_CreateAndStoreCommit_RoundTrip(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	// Create a real blob and tree to reference
	blob := objects.NewBlob([]byte(testutils.RandomString(100)))
	if err := store.Store(blob); err != nil {
		t.Fatalf("failed to store blob: %v", err)
	}

	treeEntry, err := objects.NewTreeEntry(objects.ModeRegularFile, testutils.RandomString(10), blob.Hash())
	if err != nil {
		t.Fatalf("failed to create tree entry: %v", err)
	}

	tree, err := objects.NewTree([]objects.TreeEntry{*treeEntry})
	if err != nil {
		t.Fatalf("failed to create tree: %v", err)
	}
	if err := store.Store(tree); err != nil {
		t.Fatalf("failed to store tree: %v", err)
	}

	author := objects.Author{
		Name:      testutils.RandomString(10),
		Email:     testutils.RandomString(15),
		Timestamp: time.Now(),
	}
	message := testutils.RandomString(10)
	parentHash := ""

	commitHash, err := createAndStoreCommit(tree.Hash(), parentHash, message, author, store)
	if err != nil {
		t.Fatalf("failed to  create and store commit: %v", err)
	}

	if len(commitHash) != constants.HashStringLength {
		t.Fatalf("expected %d-char commit hash, got %d chars: %s", constants.HashStringLength, len(commitHash), commitHash)
	}

	// Read back and verify all fields
	commit, err := store.ReadCommit(commitHash)
	if err != nil {
		t.Fatalf("failed to read commit back from store: %v", err)
	}

	if commit.TreeHash() != tree.Hash() {
		t.Fatalf("tree hash mismatch: expected [%s], got [%s]", tree.Hash(), commit.TreeHash())
	}

	if commit.ParentHash() != "" {
		t.Fatalf("expected empty parent hash for first commit, got [%s]", commit.ParentHash())
	}

	if commit.Message() != message {
		t.Fatalf("message mismatch: expected [%s], got [%s]", message, commit.Message())
	}

	if commit.Author().String() != author.String() {
		t.Fatalf("author mismatch: expected [%s], got [%s]", author.String(), commit.Author())
	}
}

// Test_OrchestrateCommitExecution_DetachedHEAD verifies that commit
// orchestration rejects detached HEAD without creating a branch ref.
func Test_OrchestrateCommitExecution_DetachedHEAD(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	stageCommitEntry(t, repoPath, testutils.RandomString(8), []byte(testutils.RandomString(50)))
	detachedHash := testutils.RandomHash()
	testutils.WriteHEADFile(t, repoPath, []byte(detachedHash+"\n"))

	_, err := OrchestrateCommitExecution(
		repoPath,
		testutils.RandomString(20),
		objects.DefaultAuthor(),
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, branches.ErrDetachedHEAD) {
		t.Fatalf("expected detached HEAD error, got [%v]", err)
	}

	defaultRefPath := filepath.Join(
		repoPath,
		constants.Gogit,
		constants.Refs,
		constants.Heads,
		constants.DefaultBranch,
	)
	testutils.AssertFileNotExists(t, defaultRefPath)
	testutils.AssertHEADContent(t, repoPath, detachedHash+"\n")
}

// Test_OrchestrateCommitExecution_HierarchicalCurrentBranch verifies that
// initial and ordinary commits advance a hierarchical current branch.
func Test_OrchestrateCommitExecution_HierarchicalCurrentBranch(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	branchName := path.Join(testutils.RandomString(8), testutils.RandomString(8))
	headContent := constants.DefaultRefPrefix + branchName + "\n"
	testutils.WriteHEADFile(t, repoPath, []byte(headContent))
	fileName := testutils.RandomString(8)

	stageCommitEntry(t, repoPath, fileName, []byte(testutils.RandomString(50)))
	firstCommitHash, err := OrchestrateCommitExecution(
		repoPath,
		testutils.RandomString(20),
		objects.DefaultAuthor(),
	)
	if err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	stageCommitEntry(t, repoPath, fileName, []byte(testutils.RandomString(100)))
	secondCommitHash, err := OrchestrateCommitExecution(
		repoPath,
		testutils.RandomString(20),
		objects.DefaultAuthor(),
	)
	if err != nil {
		t.Fatalf("failed to create ordinary commit: %v", err)
	}

	refPath := filepath.Join(
		repoPath,
		constants.Gogit,
		constants.Refs,
		constants.Heads,
		filepath.FromSlash(branchName),
	)
	testutils.AssertFileContent(t, refPath, []byte(secondCommitHash+"\n"))
	testutils.AssertHEADContent(t, repoPath, headContent)

	secondCommit, err := objects.NewObjectStore(repoPath).ReadCommit(secondCommitHash)
	if err != nil {
		t.Fatalf("failed to read second commit: %v", err)
	}
	if secondCommit.ParentHash() != firstCommitHash {
		t.Fatalf("expected parent hash [%s], got [%s]", firstCommitHash, secondCommit.ParentHash())
	}
}

// Test_OrchestrateCommitExecution_LockedCurrentRef verifies that a current ref
// lock prevents branch advancement without changing the ref or HEAD.
func Test_OrchestrateCommitExecution_LockedCurrentRef(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	fileName := testutils.RandomString(8)
	stageCommitEntry(t, repoPath, fileName, []byte(testutils.RandomString(50)))
	firstCommitHash, err := OrchestrateCommitExecution(
		repoPath,
		testutils.RandomString(20),
		objects.DefaultAuthor(),
	)
	if err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	stageCommitEntry(t, repoPath, fileName, []byte(testutils.RandomString(100)))
	refPath := filepath.Join(
		repoPath,
		constants.Gogit,
		constants.Refs,
		constants.Heads,
		constants.DefaultBranch,
	)
	lockPath := refPath + ".lock"
	lockContent := testutils.RandomByteSlice(20)
	if err := os.WriteFile(lockPath, lockContent, constants.FilePerms); err != nil {
		t.Fatalf("failed to create current ref lock: %v", err)
	}
	originalHEADContent := testutils.ReadHEADFile(t, repoPath)

	_, err = OrchestrateCommitExecution(
		repoPath,
		testutils.RandomString(20),
		objects.DefaultAuthor(),
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, branches.ErrReferenceLocked) {
		t.Fatalf("expected reference locked error, got [%v]", err)
	}

	testutils.AssertFileContent(t, refPath, []byte(firstCommitHash+"\n"))
	testutils.AssertFileContent(t, lockPath, lockContent)
	testutils.AssertHEADContent(t, repoPath, originalHEADContent)
}

// stageCommitEntry stores a blob and writes a one-entry index for commit tests.
func stageCommitEntry(t *testing.T, repoPath, fileName string, content []byte) {
	t.Helper()

	store := objects.NewObjectStore(repoPath)
	blob := objects.NewBlob(content)
	if err := store.Store(blob); err != nil {
		t.Fatalf("failed to store blob: %v", err)
	}

	entry, err := index.NewEntry(
		index.ModeRegularFile,
		blob.Hash(),
		fileName,
		int64(len(content)),
		time.Now(),
	)
	if err != nil {
		t.Fatalf("failed to create index entry: %v", err)
	}
	idx := index.NewIndex()
	if err := idx.AddEntry(entry); err != nil {
		t.Fatalf("failed to add index entry: %v", err)
	}
	if err := index.NewManager(repoPath).Save(idx); err != nil {
		t.Fatalf("failed to save index: %v", err)
	}
}
