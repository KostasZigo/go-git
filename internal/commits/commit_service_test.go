package commits

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/branches"
	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestBuildTreeSnapshot verifies that index entries retain their logical paths,
// object hashes, and Git file modes in the commit tree snapshot.
func TestBuildTreeSnapshot(t *testing.T) {
	rootFilePath := testutils.RandomString(10)
	rootFileHash := testutils.RandomHash()

	nestedFilePath := path.Join(testutils.RandomString(8), testutils.RandomString(8))
	nestedFileHash := testutils.RandomHash()

	lastModified := time.Now()

	rootEntry, err := index.NewEntry(
		index.ModeRegularFile,
		rootFileHash,
		rootFilePath,
		testutils.RandomInt(50),
		lastModified,
	)
	if err != nil {
		t.Fatalf("failed to create root index entry: %v", err)
	}

	nestedEntry, err := index.NewEntry(
		index.ModeExecutable,
		nestedFileHash,
		nestedFilePath,
		testutils.RandomInt(50),
		lastModified,
	)
	if err != nil {
		t.Fatalf("failed to create nested index entry: %v", err)
	}

	idx := index.NewIndex()
	_ = idx.AddEntry(rootEntry)
	_ = idx.AddEntry(nestedEntry)
	snapshot, err := idx.ToTreeSnapshot()
	if err != nil {
		t.Fatalf("failed to convert index  to snapshot: %v", err)
	}

	expectedSnapshot := objects.TreeSnapshot{
		rootFilePath: {
			Mode: objects.ModeRegularFile,
			Hash: rootFileHash,
		},
		nestedFilePath: {
			Mode: objects.ModeExecutable,
			Hash: nestedFileHash,
		},
	}

	if len(snapshot) != len(expectedSnapshot) {
		t.Fatalf("expected snapshot entry count should be [%d], got [%d]", len(expectedSnapshot), len(snapshot))
	}

	for relativePath, expectedEntry := range expectedSnapshot {
		actualEntry, exists := snapshot[relativePath]
		if !exists {
			t.Fatalf("snapshot is missing path [%q]", relativePath)
		}

		if actualEntry != expectedEntry {
			t.Fatalf("snapshot entry %q = [%#v], got [%#v]", relativePath, expectedEntry, actualEntry)
		}
	}
}

// TestOrchestrateCommitExecution_SnapshotTreeRestoresThroughCheckout verifies
// that commit orchestration creates snapshot-backed trees that checkout can
// restore into the working tree and index.
func TestOrchestrateCommitExecution_SnapshotTreeRestoresThroughCheckout(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	rootFileName := testutils.RandomString(10)
	rootFileContent := testutils.RandomByteSlice(50)
	rootBlob := objects.NewBlob(rootFileContent)
	if err := store.Store(rootBlob); err != nil {
		t.Fatalf("failed to store root blob: %v", err)
	}

	directoryName := testutils.RandomString(10)
	nestedFileName := testutils.RandomString(10)
	nestedFilePath := path.Join(directoryName, nestedFileName)
	nestedFileContent := testutils.RandomByteSlice(50)
	nestedBlob := objects.NewBlob(nestedFileContent)
	if err := store.Store(nestedBlob); err != nil {
		t.Fatalf("failed to store nested blob: %v", err)
	}

	lastModified := time.Now()
	rootIndexEntry, err := index.NewEntry(
		index.ModeRegularFile,
		rootBlob.Hash(),
		rootFileName,
		int64(len(rootFileContent)),
		lastModified,
	)
	if err != nil {
		t.Fatalf("failed to create root index entry: %v", err)
	}

	nestedIndexEntry, err := index.NewEntry(
		index.ModeExecutable,
		nestedBlob.Hash(),
		nestedFilePath,
		int64(len(nestedFileContent)),
		lastModified,
	)
	if err != nil {
		t.Fatalf("failed to create nested index entry: %v", err)
	}

	stagedIndex := index.NewIndex()
	if err := stagedIndex.AddEntry(rootIndexEntry); err != nil {
		t.Fatalf("failed to add root index entry: %v", err)
	}
	if err := stagedIndex.AddEntry(nestedIndexEntry); err != nil {
		t.Fatalf("failed to add nested index entry: %v", err)
	}
	if err := index.NewManager(repoPath).Save(stagedIndex); err != nil {
		t.Fatalf("failed to save staged index: %v", err)
	}

	commitHash, err := OrchestrateCommitExecution(
		repoPath,
		testutils.RandomString(20),
		objects.DefaultAuthor(),
	)
	if err != nil {
		t.Fatalf("failed to create commit: %v", err)
	}

	commit, err := store.ReadCommit(commitHash)
	if err != nil {
		t.Fatalf("failed to read commit: %v", err)
	}

	if err := RestoreTreeAndRebuildIndex(repoPath, commit.TreeHash()); err != nil {
		t.Fatalf("failed to restore committed tree: %v", err)
	}

	testutils.AssertFileContent(t, filepath.Join(repoPath, rootFileName), rootFileContent)
	testutils.AssertFileContent(
		t,
		filepath.Join(repoPath, directoryName, nestedFileName),
		nestedFileContent,
	)

	rebuiltIndex, err := index.NewManager(repoPath).Load()
	if err != nil {
		t.Fatalf("failed to load rebuilt index: %v", err)
	}

	expectedEntries := map[string]struct {
		hash string
		mode index.FileMode
	}{
		rootFileName: {
			hash: rootBlob.Hash(),
			mode: index.ModeRegularFile,
		},
		nestedFilePath: {
			hash: nestedBlob.Hash(),
			mode: index.ModeExecutable,
		},
	}

	actualEntries := rebuiltIndex.GetEntryList()
	if len(actualEntries) != len(expectedEntries) {
		t.Fatalf(
			"expected rebuilt index entry count to be [%d], got [%d]",
			len(expectedEntries),
			len(actualEntries),
		)
	}

	for _, entry := range actualEntries {
		expectedEntry, exists := expectedEntries[entry.Path()]
		if !exists {
			t.Fatalf("unexpected rebuilt index path [%s]", entry.Path())
		}

		if entry.Hash() != expectedEntry.hash {
			t.Fatalf(
				"expected rebuilt index hash for [%s] to be [%s], got [%s]",
				entry.Path(),
				expectedEntry.hash,
				entry.Hash(),
			)
		}

		if entry.Mode() != expectedEntry.mode {
			t.Fatalf(
				"expected rebuilt index mode for [%s] to be [%v], got [%v]",
				entry.Path(),
				expectedEntry.mode,
				entry.Mode(),
			)
		}
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
