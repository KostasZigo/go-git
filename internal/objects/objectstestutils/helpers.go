// Package objectstestutils provides shared helpers for creating and
// persisting blobs, trees, and commits in object-store-related tests.
package objectstestutils

import (
	"testing"

	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// CreateTreeEntry creates tree entry and fails test on error.
func CreateTreeEntry(t *testing.T, mode objects.FileMode, name, hash string) objects.TreeEntry {
	t.Helper()

	entry, err := objects.NewTreeEntry(mode, name, hash)
	if err != nil {
		t.Fatalf("Failed to create tree entry: %v", err)
	}

	return *entry
}

// CreateTree creates tree from entries and fails test on error.
func CreateTree(t *testing.T, entries []objects.TreeEntry) *objects.Tree {
	t.Helper()

	tree, err := objects.NewTree(entries)
	if err != nil {
		t.Fatalf("Failed to create tree: %v", err)
	}

	return tree
}

// CreateAndStoreTree creates tree from entries, stores it, and returns tree.
func CreateAndStoreTree(t *testing.T, store *objects.ObjectStore, entries []objects.TreeEntry) *objects.Tree {
	t.Helper()

	tree := CreateTree(t, entries)
	if err := store.Store(tree); err != nil {
		t.Fatalf("Failed to store tree: %v", err)
	}

	return tree
}

// AssertTreeEntryEqual verifies two tree entries match.
func AssertTreeEntryEqual(t *testing.T, actual, expected objects.TreeEntry) {
	t.Helper()

	if actual.Name() != expected.Name() {
		t.Errorf("Entry name mismatch: expected %s, got %s", expected.Name(), actual.Name())
	}
	if actual.Hash() != expected.Hash() {
		t.Errorf("Entry hash mismatch: expected %s, got %s", expected.Hash(), actual.Hash())
	}
	if actual.Mode() != expected.Mode() {
		t.Errorf("Entry mode mismatch: expected %s, got %s", expected.Mode(), actual.Mode())
	}
}

// CreateAndStoreBlob creates a blob from content, stores it in the object store,
// and returns the blob. Fails the test on error.
func CreateAndStoreBlob(t *testing.T, store *objects.ObjectStore, content []byte) *objects.Blob {
	t.Helper()

	blob := objects.NewBlob(content)
	if err := store.Store(blob); err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	return blob
}

// StoreBlobTree creates a flat tree containing one blob per provided file name.
// Each blob is created with random content and stored in the object store.
// Returns the stored root tree and a map of file name to blob for assertion lookup.
func StoreBlobTree(t *testing.T, store *objects.ObjectStore, fileNames ...string) (*objects.Tree, map[string]*objects.Blob) {
	t.Helper()

	fileContents := make(map[string][]byte, len(fileNames))
	for _, name := range fileNames {
		fileContents[name] = []byte(testutils.RandomString(100))
	}

	return StoreBlobTreeWithContent(t, store, fileContents)
}

// StoreBlobTreeWithContent creates a flat tree containing one blob per entry in
// the provided fileContents map (fileName → content). Each blob is stored in the
// object store with the given content. Returns the stored root tree and a map of
// file name to blob for assertion lookup.
func StoreBlobTreeWithContent(t *testing.T, store *objects.ObjectStore, fileContents map[string][]byte) (*objects.Tree, map[string]*objects.Blob) {
	t.Helper()

	blobMap := make(map[string]*objects.Blob, len(fileContents))
	entries := make([]objects.TreeEntry, 0, len(fileContents))

	for name, content := range fileContents {
		blob := CreateAndStoreBlob(t, store, content)
		blobMap[name] = blob

		entry := CreateTreeEntry(t, objects.ModeRegularFile, name, blob.Hash())
		entries = append(entries, entry)
	}

	tree := CreateAndStoreTree(t, store, entries)
	return tree, blobMap
}

// CreateAndStoreCommit creates a commit object referencing the given tree hash
// and parent hash, stores it in the object store, and returns the commit.
// Pass an empty parentHash for an initial commit.
func CreateAndStoreCommit(t *testing.T, store *objects.ObjectStore, treeHash, parentHash, message string) *objects.Commit {
	t.Helper()

	commit, err := objects.NewCommit(treeHash, parentHash, message, objects.DefaultAuthor())
	if err != nil {
		t.Fatalf("Failed to create commit: %v", err)
	}

	if err := store.Store(commit); err != nil {
		t.Fatalf("Failed to store commit: %v", err)
	}

	return commit
}
