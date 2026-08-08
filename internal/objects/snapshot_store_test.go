package objects_test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/objects/objectstest"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestObjectStore_ReadTreeSnapshot_RootFile verifies flattening a root-level leaf.
func TestObjectStore_ReadTreeSnapshot_RootFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	blobContent := testutils.RandomBytes(20)
	blob := objectstest.CreateAndStoreBlob(t, store, blobContent)

	treeEntryName := testutils.RandomString(10)
	treeEntry := objectstest.CreateTreeEntry(
		t,
		objects.ModeExecutable,
		treeEntryName,
		blob.Hash(),
	)
	rootTree := objectstest.CreateAndStoreTree(t, store, []objects.TreeEntry{treeEntry})

	snapshot, err := store.ReadTreeSnapshot(rootTree.Hash())
	if err != nil {
		t.Fatalf("failed to read tree snapshot: %v", err)
	}
	if len(snapshot) != 1 {
		t.Fatalf("expected snapshot entry count to be 1. got [%d]", len(snapshot))
	}

	assertSnapshotEntry(t, snapshot, treeEntryName, objects.ModeExecutable, blob.Hash())
}

// TestObjectStore_ReadTreeSnapshot_MixedNestedFiles verifies recursive
// flattening while preserving root and deeply nested logical paths.
func TestObjectStore_ReadTreeSnapshot_MixedNestedFiles(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	nestedBlobContent := testutils.RandomByteSlice(50)
	nestedBlob := objectstest.CreateAndStoreBlob(t, store, nestedBlobContent)

	nestedFileName := testutils.RandomString(10)
	nestedTreeEntry := objectstest.CreateTreeEntry(t, objects.ModeExecutable, nestedFileName, nestedBlob.Hash())
	nestedTree := objectstest.CreateAndStoreTree(t, store, []objects.TreeEntry{nestedTreeEntry})

	subDirName := testutils.RandomString(10)
	subDirTreeEntry := objectstest.CreateTreeEntry(t, objects.ModeDirectory, subDirName, nestedTree.Hash())
	subDirTree := objectstest.CreateAndStoreTree(t, store, []objects.TreeEntry{subDirTreeEntry})

	rootBlobContent := testutils.RandomByteSlice(50)
	rootBlob := objectstest.CreateAndStoreBlob(t, store, rootBlobContent)
	rootFileName := testutils.RandomString(10)
	rootDirEntryName := testutils.RandomString(10)

	rootTreeEntries := []objects.TreeEntry{
		objectstest.CreateTreeEntry(t, objects.ModeRegularFile, rootFileName, rootBlob.Hash()),
		objectstest.CreateTreeEntry(t, objects.ModeDirectory, rootDirEntryName, subDirTree.Hash()),
	}
	rootTree := objectstest.CreateAndStoreTree(t, store, rootTreeEntries)

	snapshot, err := store.ReadTreeSnapshot(rootTree.Hash())
	if err != nil {
		t.Fatalf("failed to read tree snapshot: %v", err)
	}

	if len(snapshot) != 2 {
		t.Fatalf("expected snapshot to contain 2 entries, got [%d]", len(snapshot))
	}

	assertSnapshotEntry(t, snapshot, rootFileName, objects.ModeRegularFile, rootBlob.Hash())
	assertSnapshotEntry(
		t,
		snapshot,
		path.Join(rootDirEntryName, subDirName, nestedFileName),
		objects.ModeExecutable,
		nestedBlob.Hash())

	if _, exists := snapshot[rootDirEntryName]; exists {
		t.Fatalf("snapshot contains implied directory path [%s]", rootDirEntryName)
	}

	improperDirEtry := path.Join(rootDirEntryName, subDirName)
	if _, exists := snapshot[improperDirEtry]; exists {
		t.Fatalf("snapshot contains implied directory path [%s]", improperDirEtry)
	}
}

// TestObjectStore_ReadTreeSnapshot_PreservesFileModes verifies that mode is
// part of a snapshot entry even when two entries reference the same blob.
func TestObjectStore_ReadTreeSnapshot_PreservesFileModes(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	blob := objectstest.CreateAndStoreBlob(t, store, []byte("shared content"))
	fileName := testutils.RandomString(10)
	secondFileName := testutils.RandomString(20)

	treeEntries := []objects.TreeEntry{
		objectstest.CreateTreeEntry(t, objects.ModeRegularFile, fileName, blob.Hash()),
		objectstest.CreateTreeEntry(t, objects.ModeExecutable, secondFileName, blob.Hash()),
	}
	tree := objectstest.CreateAndStoreTree(t, store, treeEntries)

	snapshot, err := store.ReadTreeSnapshot(tree.Hash())
	if err != nil {
		t.Fatalf("failed to read tree snapshot: %v", err)
	}

	assertSnapshotEntry(t, snapshot, fileName, objects.ModeRegularFile, blob.Hash())
	assertSnapshotEntry(t, snapshot, secondFileName, objects.ModeExecutable, blob.Hash())
}

// TestObjectStore_ReadTreeSnapshot_EmptyTree verifies that an empty Git tree
// becomes a non-nil empty snapshot.
func TestObjectStore_ReadTreeSnapshot_EmptyTree(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	tree := objectstest.CreateAndStoreTree(t, store, nil)

	snapshot, err := store.ReadTreeSnapshot(tree.Hash())
	if err != nil {
		t.Fatalf("failed to read tree snapshot: %v", err)
	}

	if len(snapshot) != 0 {
		t.Fatalf("expected 0 snapshot entries for empty tree, got [%d]", len(snapshot))
	}
}

// TestObjectStore_ReadTreeSnapshot_MissingSubtree verifies that recursive read
// errors identify the logical path of the unavailable subtree.
func TestObjectStore_ReadTreeSnapshot_MissingSubtree(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	missingTreeHash := testutils.RandomHash()
	missingTreeDirName := testutils.RandomString(10)
	missingTreeEntry := objectstest.CreateTreeEntry(t, objects.ModeDirectory, missingTreeDirName, missingTreeHash)
	rootTree := objectstest.CreateAndStoreTree(t, store, []objects.TreeEntry{missingTreeEntry})

	_, err := store.ReadTreeSnapshot(rootTree.Hash())
	if err == nil {
		t.Fatal("expected missing subtree to return an error")
	}

	if !strings.Contains(err.Error(), missingTreeDirName) {
		t.Fatalf("expected error to contain relative path [%s], got [%s]", missingTreeDirName, err.Error())
	}
}

// TestObjectStore_StoreTreeSnapshot_RootFile verifies that a root-level
// snapshot entry becomes a root-level Git tree entry.
func TestObjectStore_StoreTreeSnapshot_RootFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	fileName := testutils.RandomString(10)
	fileHash := testutils.RandomHash()
	fileMode := objects.ModeRegularFile
	snapshot := objects.TreeSnapshot{
		fileName: {
			Mode: fileMode,
			Hash: fileHash,
		},
	}

	treeHash, err := store.StoreTreeSnapshot(snapshot)
	if err != nil {
		t.Fatalf("failed to store tree from snapshot: %v", err)
	}

	tree, err := store.ReadTree(treeHash)
	if err != nil {
		t.Fatalf("failed to read tree with hash[%s]: %v", treeHash, err)
	}

	assertStoredTreeEntry(t, tree, fileName, fileHash, fileMode)
}

// TestObjectStore_StoreTreeSnapshot_MixedNestedFiles verifies that snapshot
// paths are rebuilt as recursive trees while root-level files remain in root.
func TestObjectStore_StoreTreeSnapshot_MixedNestedFiles(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	// file under root folder /
	rootFileName := testutils.RandomString(10)
	rootFileHash := testutils.RandomHash()
	rootFileMode := objects.ModeRegularFile

	// dir config under root folder /
	configDirectoryName := testutils.RandomString(10)

	// file under config folder /config/
	configFileName := testutils.RandomString(10)
	configFileHash := testutils.RandomHash()
	configFileMode := objects.ModeRegularFile

	// dir docs under root folder /
	docsDirectoryName := testutils.RandomString(10)

	// dir guides under docs /docs/
	guidesDirectoryName := testutils.RandomString(10)

	// file under guides folder /docs/guides/
	guideFileName := testutils.RandomString(10)
	guideFileHash := testutils.RandomHash()
	guideFileMode := objects.ModeExecutable

	snapshot := objects.TreeSnapshot{
		rootFileName: {
			Mode: rootFileMode,
			Hash: rootFileHash,
		},
		path.Join(configDirectoryName, configFileName): {
			Mode: configFileMode,
			Hash: configFileHash,
		},
		path.Join(docsDirectoryName, guidesDirectoryName, guideFileName): {
			Mode: guideFileMode,
			Hash: guideFileHash,
		},
	}

	treeHash, err := store.StoreTreeSnapshot(snapshot)
	if err != nil {
		t.Fatalf("failed to store tree from snapshot: %v", err)
	}

	// Read and assert root tree
	tree, err := store.ReadTree(treeHash)
	if err != nil {
		t.Fatalf("failed to read tree with hash[%s]: %v", treeHash, err)
	}
	assertStoredTreeEntry(t, tree, rootFileName, rootFileHash, rootFileMode)

	// Read and assert docs tree
	docsTreeEntry := getStoredTreeEntry(t, tree, docsDirectoryName)
	if docsTreeEntry.Mode() != objects.ModeDirectory {
		t.Fatalf("expected entry to be of mode [%s], got [%s]", objects.ModeDirectory, docsTreeEntry.Mode())
	}

	docsTree, err := store.ReadTree(docsTreeEntry.Hash())
	if err != nil {
		t.Fatalf("failed to read tree [%s]: %v", docsTreeEntry.Hash(), err)
	}

	// read and assert guides tree
	guidesTreeEntry := getStoredTreeEntry(t, docsTree, guidesDirectoryName)
	if guidesTreeEntry.Mode() != objects.ModeDirectory {
		t.Fatalf("expected entry to be of mode [%s], got [%s]", objects.ModeDirectory, guidesTreeEntry.Mode())
	}

	guidesTree, err := store.ReadTree(guidesTreeEntry.Hash())
	if err != nil {
		t.Fatalf("failed to read tree [%s]: %v", guidesTreeEntry.Hash(), err)
	}
	assertStoredTreeEntry(t, guidesTree, guideFileName, guideFileHash, guideFileMode)

	// read and assert config tree
	configTreeEntry := getStoredTreeEntry(t, tree, configDirectoryName)
	if configTreeEntry.Mode() != objects.ModeDirectory {
		t.Fatalf("expected entry to be of mode [%s], got [%s]", objects.ModeDirectory, configTreeEntry.Mode())
	}

	configTree, err := store.ReadTree(configTreeEntry.Hash())
	if err != nil {
		t.Fatalf("failed to read tree [%s]: %v", configTreeEntry.Hash(), err)
	}
	assertStoredTreeEntry(t, configTree, configFileName, configFileHash, configFileMode)
}

// TestObjectStore_StoreTreeSnapshot_EmptySnapshot verifies that an empty
// snapshot stores and returns the canonical empty Git tree.
func TestObjectStore_StoreTreeSnapshot_EmptySnapshot(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	treeHash, err := store.StoreTreeSnapshot(nil)
	if err != nil {
		t.Fatalf("failed to create an empty tree given an empty snapshot: %v", err)
	}

	if treeHash != testutils.CanonicalEmptyTreeHash {
		t.Fatalf(
			"empty snapshot tree hash = %s, want %s",
			treeHash,
			testutils.CanonicalEmptyTreeHash,
		)
	}

	if !store.Exists(testutils.CanonicalEmptyTreeHash) {
		t.Fatal("canonical empty tree was not stored")
	}
}

// TestObjectStore_StoreTreeSnapshot_DeterministicHash verifies that equivalent
// snapshots produce the same root tree hash regardless of insertion order.
func TestObjectStore_StoreTreeSnapshot_DeterministicHash(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	firstFileName := testutils.RandomString(10)
	firstFileHash := testutils.RandomHash()

	secondFileName := testutils.RandomString(20)
	secondFileHash := testutils.RandomHash()

	firstSnapshot := objects.TreeSnapshot{
		firstFileName: {
			Mode: objects.ModeRegularFile,
			Hash: firstFileHash,
		},
		secondFileName: {
			Mode: objects.ModeRegularFile,
			Hash: secondFileHash,
		},
	}
	secondSnapshot := objects.TreeSnapshot{
		secondFileName: {
			Mode: objects.ModeRegularFile,
			Hash: secondFileHash,
		},
		firstFileName: {
			Mode: objects.ModeRegularFile,
			Hash: firstFileHash,
		},
	}

	firstTreeHash, err := store.StoreTreeSnapshot(firstSnapshot)
	if err != nil {
		t.Fatalf("failed to store first snapshot: %v", err)
	}
	secondTreeHash, err := store.StoreTreeSnapshot(secondSnapshot)
	if err != nil {
		t.Fatalf("failed to store second snapshot: %v", err)
	}

	if firstTreeHash != secondTreeHash {
		t.Fatalf("equivalent snapshots produced different hashes [%s] and [%s]", firstTreeHash, secondTreeHash)
	}
}

// TestObjectStore_StoreTreeSnapshot_FileModesAffectHash verifies that file mode
// participates in Git tree identity.
func TestObjectStore_StoreTreeSnapshot_FileModesAffectHash(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	fileName := testutils.RandomString(10)
	fileHash := testutils.RandomHash()

	regularTreeHash, err := store.StoreTreeSnapshot(objects.TreeSnapshot{
		fileName: {
			Mode: objects.ModeRegularFile,
			Hash: fileHash,
		},
	})
	if err != nil {
		t.Fatalf("failed to store regular-file tree snapshot: %v", err)
	}

	executableTreeHash, err := store.StoreTreeSnapshot(objects.TreeSnapshot{
		fileName: {
			Mode: objects.ModeExecutable,
			Hash: fileHash,
		},
	})
	if err != nil {
		t.Fatalf("failed to store executable-file tree snapshot: %v", err)
	}

	if regularTreeHash == executableTreeHash {
		t.Fatal("different file modes produced the same hash")
	}
}

// TestObjectStore_StoreTreeSnapshot_InvalidSnapshotDoesNotStoreTrees verifies
// validation occurs before the writer creates any tree objects.
func TestObjectStore_StoreTreeSnapshot_InvalidSnapshotDoesNotStoreTrees(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	// .. is not allowed in the relative path
	invalidSnapshot := objects.TreeSnapshot{
		"docs/../README.md": {
			Mode: objects.ModeRegularFile,
			Hash: testutils.RandomHash(),
		},
	}

	_, err := store.StoreTreeSnapshot(invalidSnapshot)
	if err == nil {
		t.Fatal("expected invalid snapshot to return an error")
	}

	expectedErrorMessage := fmt.Sprintf("path cannot contain %q segments", "..")
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%q], got [%q]", expectedErrorMessage, err.Error())
	}

	objectsDirectory := filepath.Join(
		repoPath,
		constants.Gogit,
		constants.Objects,
	)
	entries, err := os.ReadDir(objectsDirectory)
	if err != nil {
		t.Fatalf("failed to read objects directory: %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf(
			"invalid snapshot created %d object directories, want 0",
			len(entries),
		)
	}
}

// TestObjectStore_TreeSnapshotRoundTrip_NestedSnapshot verifies that a nested
// snapshot survives storage and flattening without losing paths, modes, or hashes.
func TestObjectStore_TreeSnapshotRoundTrip_NestedSnapshot(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	// file under root folder /
	rootFileName := testutils.RandomString(10)
	rootFileHash := testutils.RandomHash()
	rootFileMode := objects.ModeRegularFile

	// dir config under root folder /
	configDirectoryName := testutils.RandomString(10)

	// file under config folder /config/
	configFileName := testutils.RandomString(10)
	configFileHash := testutils.RandomHash()
	configFileMode := objects.ModeRegularFile

	// dir docs under root folder /
	docsDirectoryName := testutils.RandomString(10)

	// dir guides under docs /docs/
	guidesDirectoryName := testutils.RandomString(10)

	// file under guides folder /docs/guides/
	guideFileName := testutils.RandomString(10)
	guideFileHash := testutils.RandomHash()
	guideFileMode := objects.ModeExecutable

	snapshot := objects.TreeSnapshot{
		rootFileName: {
			Mode: rootFileMode,
			Hash: rootFileHash,
		},
		path.Join(configDirectoryName, configFileName): {
			Mode: configFileMode,
			Hash: configFileHash,
		},
		path.Join(docsDirectoryName, guidesDirectoryName, guideFileName): {
			Mode: guideFileMode,
			Hash: guideFileHash,
		},
	}

	rootTreeHash, err := store.StoreTreeSnapshot(snapshot)
	if err != nil {
		t.Fatalf("failed to store tree from snapshot: %v", err)
	}

	readSnapshot, err := store.ReadTreeSnapshot(rootTreeHash)
	if err != nil {
		t.Fatalf("failed to read tree snapshot for hash [%s]: %v", rootTreeHash, err)
	}

	assertTreeSnapshotsEqual(t, snapshot, readSnapshot)
}

// TestObjectStore_TreeSnapshotRoundTrip_EmptySnapshot verifies that an empty
// snapshot round-trips through the canonical empty Git tree.
func TestObjectStore_TreeSnapshotRoundTrip_EmptySnapshot(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	treeHash, err := store.StoreTreeSnapshot(nil)
	if err != nil {
		t.Fatalf("failed to store tree from empty snapshot: %v", err)
	}

	readSnapshot, err := store.ReadTreeSnapshot(treeHash)
	if err != nil {
		t.Fatalf("failed to read empty tree snapshot: %v", err)
	}

	assertTreeSnapshotsEqual(t, nil, readSnapshot)
}

// TestObjectStore_TreeSnapshotRoundTrip_Deterministic verifies that repeated
// round trips produce the same root hash and equivalent snapshot.
func TestObjectStore_TreeSnapshotRoundTrip_Deterministic(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	snapshot := objects.TreeSnapshot{
		testutils.RandomString(10): {
			Mode: objects.ModeRegularFile,
			Hash: testutils.RandomHash(),
		},
		testutils.RandomString(15): {
			Mode: objects.ModeExecutable,
			Hash: testutils.RandomHash(),
		},
		testutils.RandomString(20): {
			Mode: objects.ModeRegularFile,
			Hash: testutils.RandomHash(),
		},
	}

	expectedTreeHash, err := store.StoreTreeSnapshot(snapshot)
	if err != nil {
		t.Fatalf("failed to store initial tree snapshot: %v", err)
	}

	// store 20 times the same snapshot. Expect same has every time
	for range 20 {
		treeHash, err := store.StoreTreeSnapshot(snapshot)
		if err != nil {
			t.Fatalf("failed to store tree snapshot: %v", err)
		}
		if treeHash != expectedTreeHash {
			t.Fatalf("expected tree hash [%s], got %s", expectedTreeHash, treeHash)
		}

		restoredSnapshot, err := store.ReadTreeSnapshot(treeHash)
		if err != nil {
			t.Fatalf("failed to read tree snapshot: %v", err)
		}

		assertTreeSnapshotsEqual(t, snapshot, restoredSnapshot)
	}
}

// assertSnapshotEntry verifies one snapshot leaf's mode and object hash.
func assertSnapshotEntry(t *testing.T,
	snapshot objects.TreeSnapshot,
	logicalPath string,
	expectedMode objects.FileMode,
	expectedHash string,
) {
	t.Helper()

	entry, exists := snapshot[logicalPath]
	if !exists {
		t.Fatalf("snapshot entry [%s] does not exist", logicalPath)
	}

	if entry.Mode != expectedMode {
		t.Errorf(
			"snapshot entry [%s] expected mode = [%s], got [%s]",
			logicalPath,
			expectedMode,
			entry.Mode,
		)
	}

	if entry.Hash != expectedHash {
		t.Errorf(
			"snapshot entry [%s] expected hash [%s], got [%s]",
			logicalPath,
			expectedHash,
			entry.Hash,
		)
	}
}

// getStoredTreeEntry returns the tree entry named `name` or fails the test.
func getStoredTreeEntry(t *testing.T, tree *objects.Tree, name string) *objects.TreeEntry {
	t.Helper()

	entry, exists := tree.FindEntry(name)
	if !exists {
		t.Fatalf("tree entry [%s] does not exist", name)
	}
	return entry
}

// assertStoredTreeEntry verifies a tree entry's mode and referenced object hash.
func assertStoredTreeEntry(t *testing.T, tree *objects.Tree, name, expectedHash string, expectedMode objects.FileMode) {
	t.Helper()

	entry := getStoredTreeEntry(t, tree, name)
	if entry.Mode() != expectedMode {
		t.Fatalf("expected tree entry mode for [%s] to be [%s], got [%s]", name, expectedMode, entry.Mode())
	}

	if entry.Hash() != expectedHash {
		t.Fatalf("expected tree entry hash for [%s] to be [%s], got [%s]", name, expectedHash, entry.Hash())
	}
}

// assertTreeSnapshotsEqual verifies that two snapshots have identical logical
// paths and identical entries, including object hashes and file modes.
func assertTreeSnapshotsEqual(t *testing.T, expectedSnapshot, actualSnapshot objects.TreeSnapshot) {
	t.Helper()

	if len(actualSnapshot) != len(expectedSnapshot) {
		t.Fatalf("expected snapshot entry count [%d], got [%d]", len(expectedSnapshot), len(actualSnapshot))
	}

	for relativePath, expectedEntry := range expectedSnapshot {
		actualEntry, exists := actualSnapshot[relativePath]
		if !exists {
			t.Fatalf("snapshot is missing path [%s]", relativePath)
		}

		if actualEntry != expectedEntry {
			t.Fatalf("snapshot entry [%s] expected to be [%#v], got [%#v]", relativePath, expectedEntry, actualEntry)
		}
	}
}
