package objectstestutils

import (
	"testing"

	"github.com/KostasZigo/gogit/internal/objects"
)

// createTreeEntry creates tree entry and fails test on error.
func CreateTreeEntry(t *testing.T, mode objects.FileMode, name, hash string) objects.TreeEntry {
	t.Helper()

	entry, err := objects.NewTreeEntry(mode, name, hash)
	if err != nil {
		t.Fatalf("Failed to create tree entry: %v", err)
	}

	return *entry
}

// createTree creates tree from entries and fails test on error.
func CreateTree(t *testing.T, entries []objects.TreeEntry) *objects.Tree {
	t.Helper()

	tree, err := objects.NewTree(entries)
	if err != nil {
		t.Fatalf("Failed to create tree: %v", err)
	}

	return tree
}

// createAndStoreTree creates tree from entries, stores it, and returns tree.
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
