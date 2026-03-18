package objects_test

import (
	"path/filepath"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/objects/objectstestutils"
	"github.com/KostasZigo/gogit/testutils"
)

// TREE STORAGE TESTS

// TestObjectStore_StoreAndReadTree verifies tree storage with single entry.
func TestObjectStore_StoreAndReadTree(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	// Create a blob
	blob := objects.NewBlob([]byte("test content"))
	if err := store.Store(blob); err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	// Create Tree with blob entry
	treeEntry := objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, "file.txt", blob.Hash())
	entries := []objects.TreeEntry{
		treeEntry,
	}
	tree := objectstestutils.CreateAndStoreTree(t, store, entries)

	// Verify file was created
	hash := tree.Hash()
	objectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, hash[:constants.HashDirPrefixLength], hash[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, objectPath)

	// Read tree back
	retrievedTree, err := store.ReadTree(tree.Hash())
	if err != nil {
		t.Fatalf("Failed to read tree: %v", err)
	}

	// Verify hash matches
	if retrievedTree.Hash() != tree.Hash() {
		t.Errorf("Hash mismatch: expected %s, got %s",
			tree.Hash(), retrievedTree.Hash())
	}

	// Verify entries match
	if len(retrievedTree.Entries()) != len(tree.Entries()) {
		t.Errorf("Entry count mismatch: expected %d, got %d",
			len(tree.Entries()), len(retrievedTree.Entries()))
	}

	// Verify entry details
	objectstestutils.AssertTreeEntryEqual(t, retrievedTree.Entries()[0], treeEntry)
}

// TestObjectStore_ReadTree_MultipleEntries verifies tree with multiple files.
func TestObjectStore_ReadTree_MultipleEntries(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	// Create multiple blobs
	blob1 := objects.NewBlob([]byte("content 1\n"))
	blob2 := objects.NewBlob([]byte("content 2\n"))
	store.Store(blob1)
	store.Store(blob2)

	// Create tree with multiple entries
	entries := []objects.TreeEntry{
		objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, "file1.txt", blob1.Hash()),
		objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, "file2.txt", blob2.Hash()),
	}

	// Create and store tree
	tree := objectstestutils.CreateAndStoreTree(t, store, entries)

	// Read tree back
	retrievedTree, err := store.ReadTree(tree.Hash())
	if err != nil {
		t.Fatalf("Failed to read tree: %v", err)
	}

	// Verify hash matches
	if retrievedTree.Hash() != tree.Hash() {
		t.Errorf("Hash mismatch: expected %s, got %s",
			tree.Hash(), retrievedTree.Hash())
	}

	// Verify all entries
	if len(retrievedTree.Entries()) != len(entries) {
		t.Errorf("Expected %d entries, got %d", len(entries), len(retrievedTree.Entries()))
	}

	// Entries should be sorted
	if retrievedTree.Entries()[0].Name() != entries[0].Name() {
		t.Errorf("Expected first entry %s, got %s", entries[0].Name(), retrievedTree.Entries()[0].Name())
	}
	if retrievedTree.Entries()[1].Name() != entries[1].Name() {
		t.Errorf("Expected second entry %s, got %s", entries[1].Name(), retrievedTree.Entries()[1].Name())
	}
}

// TestObjectStore_ReadTree_NestedTree verifies nested directory structure storage.
func TestObjectStore_ReadTree_NestedTree(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := objects.NewObjectStore(repoPath)

	// Create a blob
	blob := objects.NewBlob([]byte("nested content\n"))
	if err := store.Store(blob); err != nil {
		t.Fatalf("Failed to store nested blob:%x", err)
	}

	// Create subtree
	subTreeEntry := objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, "nested.txt", blob.Hash())
	subTreeEntries := []objects.TreeEntry{
		subTreeEntry,
	}
	subTree := objectstestutils.CreateAndStoreTree(t, store, subTreeEntries)

	// Create root tree with directory entry
	rootBlob := objects.NewBlob([]byte("root content\n"))
	if err := store.Store(rootBlob); err != nil {
		t.Fatalf("Failed to strore root blob:%x", err)
	}

	rootEntryFile := objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, "root.txt", rootBlob.Hash())
	rootEntryDir := objectstestutils.CreateTreeEntry(t, objects.ModeDirectory, "subdir", subTree.Hash())
	rootEntries := []objects.TreeEntry{
		rootEntryFile,
		rootEntryDir,
	}
	rootTree := objectstestutils.CreateAndStoreTree(t, store, rootEntries)

	// Read root tree back
	retrievedRootTree, err := store.ReadTree(rootTree.Hash())
	if err != nil {
		t.Fatalf("Failed to read root tree: %v", err)
	}

	// Verify hash matches
	if retrievedRootTree.Hash() != rootTree.Hash() {
		t.Fatalf("Hash mismatch: expected %s, got %s",
			rootTree.Hash(), retrievedRootTree.Hash())
	}

	// Verify file entry details
	fileEntry := retrievedRootTree.Entries()[0]
	objectstestutils.AssertTreeEntryEqual(t, fileEntry, rootEntryFile)

	// Verify directory entry
	dirEntry := retrievedRootTree.Entries()[1]
	objectstestutils.AssertTreeEntryEqual(t, dirEntry, rootEntryDir)

	// Read subtree
	retrievedSubTree, err := store.ReadTree(dirEntry.Hash())
	if err != nil {
		t.Fatalf("Failed to read subtree: %v", err)
	}

	if len(retrievedSubTree.Entries()) != len(subTreeEntries) {
		t.Fatalf("Expected %d entry in subtree, got %d", len(subTreeEntries), len(retrievedSubTree.Entries()))
	}

	// Verify nested File tree entry
	nestedEntry := retrievedSubTree.Entries()[0]
	objectstestutils.AssertTreeEntryEqual(t, nestedEntry, subTreeEntry)
}
