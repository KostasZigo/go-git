package objects_test

import (
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/objects/objectstestutils"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TREE ENTRY TESTS

// TestNewTreeEntry verifies tree entry creation with valid mode, name, and hash.
func TestNewTreeEntry(t *testing.T) {
	entryName := "test.txt"
	hash := testutils.RandomHash()
	entry, err := objects.NewTreeEntry(objects.ModeRegularFile, entryName, hash)

	if err != nil {
		t.Fatal("Expected New Tree Entry to be created")
	}

	if entry.Mode() != objects.ModeRegularFile {
		t.Errorf("Expected mode [%s], got [%s]", objects.ModeRegularFile, entry.Mode())
	}

	if entry.Name() != entryName {
		t.Errorf("Expected name [%s], got [%s]", entryName, entry.Name())
	}

	if entry.Hash() != hash {
		t.Errorf("Expected hash [%s], got [%s]", hash, entry.Hash())
	}
}

// TestTreeEntry_IsDirectory verifies directory vs file mode detection.
func TestTreeEntry_IsDirectory(t *testing.T) {
	dirEntry := objectstestutils.CreateTreeEntry(t, objects.ModeDirectory, "src", testutils.RandomHash())
	fileEntry := objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, "main.go", testutils.RandomHash())

	if !dirEntry.IsDirectory() {
		t.Fatal("Expected directory entry to be identified as directory")
	}

	if fileEntry.IsDirectory() {
		t.Fatal("Expected file entry not to be identified as directory")
	}
}

// TREE TESTS

// TestNewTree_EmptyTree verifies empty tree creation and hash computation.
func TestNewTree_EmptyTree(t *testing.T) {
	_, err := objects.NewTree([]objects.TreeEntry{})
	if err == nil {
		t.Fatalf("Expected to fail when creating empty tree: %v", err)
	}

	expectedErrorMessage := "tree must contain at least one entry"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestNewTree_SingleEntry verifies tree with single file entry.
func TestNewTree_SingleEntry(t *testing.T) {
	// Create a blob first
	blob := objects.NewBlob([]byte("test content\n"))
	entry := objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, "test.txt", blob.Hash())

	entries := []objects.TreeEntry{entry}
	tree := objectstestutils.CreateTree(t, entries)

	if tree.Hash() == "" {
		t.Error("Tree hash should not be empty")
	}

	if len(tree.Entries()) != len(entries) {
		t.Errorf("Expected %d entry, got %d", len(entries), len(tree.Entries()))
	}
}

// TestNewTree_MultipleEntries verifies tree with multiple file entries.
func TestNewTree_MultipleEntries(t *testing.T) {
	blob1 := objects.NewBlob([]byte("content1\n"))
	blob2 := objects.NewBlob([]byte("content2\n"))

	entries := []objects.TreeEntry{
		objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, "file1.txt", blob1.Hash()),
		objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, "file2.txt", blob2.Hash()),
	}

	tree := objectstestutils.CreateTree(t, entries)

	if len(tree.Entries()) != len(entries) {
		t.Errorf("Expected %d entries, got %d", len(entries), len(tree.Entries()))
	}
}

func TestNewTree_SortsEntries(t *testing.T) {
	// Add entries in wrong order
	entries := []objects.TreeEntry{
		objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, "z.txt", testutils.RandomHash()),
		objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, "a.txt", testutils.RandomHash()),
		objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, "m.txt", testutils.RandomHash()),
	}

	tree := objectstestutils.CreateTree(t, entries)

	sortedEntries := tree.Entries()
	expectedOrder := []string{"a.txt", "m.txt", "z.txt"}

	for i, expected := range expectedOrder {
		if sortedEntries[i].Name() != expected {
			t.Errorf("Expected entry %d to be '%s', got '%s'", i, expected, sortedEntries[i].Name())
		}
	}

}

// TestTree_NestedStructure verifies tree with nested directory structure.
func TestTree_NestedStructure(t *testing.T) {
	// Create blobs for files
	mainBlob := objects.NewBlob([]byte("package main\n"))
	readmeBlob := objects.NewBlob([]byte("# Project\n"))

	// Create subtree for src/ directory
	srcTree := objectstestutils.CreateTree(t, []objects.TreeEntry{
		objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, "main.go", mainBlob.Hash()),
	})

	// Create root tree
	rootEntries := []objects.TreeEntry{
		objectstestutils.CreateTreeEntry(t, objects.ModeRegularFile, "README.md", readmeBlob.Hash()),
		objectstestutils.CreateTreeEntry(t, objects.ModeDirectory, "src", srcTree.Hash()),
	}
	rootTree := objectstestutils.CreateTree(t, rootEntries)

	// Verify structure
	if len(rootTree.Entries()) != len(rootEntries) {
		t.Errorf("Expected %d entries in root tree, got %d", len(rootEntries), len(rootTree.Entries()))
	}

	// Find the src directory entry
	srcEntry, found := rootTree.FindEntry("src")
	if !found {
		t.Fatal("src directory not found in root tree")
	}
	if !srcEntry.IsDirectory() {
		t.Error("src entry not identified as directory")
	}
	if srcEntry.Hash() != srcTree.Hash() {
		t.Errorf("Expected src entry hash %s, got %s", srcTree.Hash(), srcEntry.Hash())
	}
}
