package objects

import (
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/testutils"
)

// TREE ENTRY TESTS

// TestNewTreeEntry verifies tree entry creation with valid mode, name, and hash.
func TestNewTreeEntry(t *testing.T) {
	entryName := "test.txt"
	hash := testutils.RandomHash()
	entry, err := NewTreeEntry(ModeRegularFile, entryName, hash)

	if err != nil {
		t.Fatal("Expected New Tree Entry to be created")
	}

	if entry.Mode() != ModeRegularFile {
		t.Errorf("Expected mode [%s], got [%s]", ModeRegularFile, entry.Mode())
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
	dirEntry := createTreeEntry(t, ModeDirectory, "src", testutils.RandomHash())
	fileEntry := createTreeEntry(t, ModeRegularFile, "main.go", testutils.RandomHash())

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
	_, err := NewTree([]TreeEntry{})
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
	blob := NewBlob([]byte("test content\n"))
	entry := createTreeEntry(t, ModeRegularFile, "test.txt", blob.Hash())

	entries := []TreeEntry{entry}
	tree := createTree(t, entries)

	if tree.Hash() == "" {
		t.Error("Tree hash should not be empty")
	}

	if len(tree.Entries()) != len(entries) {
		t.Errorf("Expected %d entry, got %d", len(entries), len(tree.Entries()))
	}
}

// TestNewTree_MultipleEntries verifies tree with multiple file entries.
func TestNewTree_MultipleEntries(t *testing.T) {
	blob1 := NewBlob([]byte("content1\n"))
	blob2 := NewBlob([]byte("content2\n"))

	entries := []TreeEntry{
		createTreeEntry(t, ModeRegularFile, "file1.txt", blob1.Hash()),
		createTreeEntry(t, ModeRegularFile, "file2.txt", blob2.Hash()),
	}

	tree := createTree(t, entries)

	if len(tree.Entries()) != len(entries) {
		t.Errorf("Expected %d entries, got %d", len(entries), len(tree.Entries()))
	}
}

func TestNewTree_SortsEntries(t *testing.T) {
	// Add entries in wrong order
	entries := []TreeEntry{
		createTreeEntry(t, ModeRegularFile, "z.txt", testutils.RandomHash()),
		createTreeEntry(t, ModeRegularFile, "a.txt", testutils.RandomHash()),
		createTreeEntry(t, ModeRegularFile, "m.txt", testutils.RandomHash()),
	}

	tree := createTree(t, entries)

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
	mainBlob := NewBlob([]byte("package main\n"))
	readmeBlob := NewBlob([]byte("# Project\n"))

	// Create subtree for src/ directory
	srcTree := createTree(t, []TreeEntry{
		createTreeEntry(t, ModeRegularFile, "main.go", mainBlob.Hash()),
	})

	// Create root tree
	rootEntries := []TreeEntry{
		createTreeEntry(t, ModeRegularFile, "README.md", readmeBlob.Hash()),
		createTreeEntry(t, ModeDirectory, "src", srcTree.Hash()),
	}
	rootTree := createTree(t, rootEntries)

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
