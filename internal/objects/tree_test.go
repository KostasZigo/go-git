package objects

import (
	"testing"

	"github.com/KostasZigo/gogit/utils"
)

// TREE ENTRY TESTS

func TestNewTreeEntry(t *testing.T) {
	entry, err := NewTreeEntry(ModeRegularFile, "test.txt", "abc123")

	if err != nil {
		t.Fatal("Expected New Tree Entry to be created")
	}

	if entry.Mode() != ModeRegularFile {
		t.Errorf("Expected mode %s, got %s", ModeRegularFile, entry.Mode())
	}

	if entry.Name() != "test.txt" {
		t.Errorf("Expected name 'test.txt', got %s", entry.Name())
	}

	if entry.Hash() != "abc123" {
		t.Errorf("Expected hash 'abc123', got %s", entry.Hash())
	}
}

func TestTreeEntry_IsDirectory(t *testing.T) {
	dirEntry, _ := NewTreeEntry(ModeDirectory, "src", "abc123")
	fileEntry, _ := NewTreeEntry(ModeRegularFile, "main.go", "def456")

	if !dirEntry.IsDirectory() {
		t.Fatal("Expected directory entry to be identified as directory")
	}

	if fileEntry.IsDirectory() {
		t.Fatal("Expected file entry not to be identified as directory")
	}
}

// TREE TESTS

func TestNewTree_EmptyTree(t *testing.T) {
	tree, err := NewTree([]TreeEntry{})
	if err != nil {
		t.Fatal("Expected Tree to be created")
	}

	// Hash for empty tree
	expectedHash, err := utils.ComputeHash([]byte(""), utils.TreeObjectType)
	if err != nil {
		t.Fatal("Expected hash to be computed")
	}

	if tree.Hash() != expectedHash {
		t.Errorf("Expected empty tree hash %s, got %s", expectedHash, tree.Hash())
	}
}

func TestNewTree_SingleEntry(t *testing.T) {
	// Create a blob first
	blob := NewBlob([]byte("test content\n"))

	entry, err := NewTreeEntry(ModeRegularFile, "test.txt", blob.Hash())
	if err != nil {
		t.Fatal("Expected FileMode to be valid")
	}

	entries := []TreeEntry{
		*entry,
	}

	tree, err := NewTree(entries)
	if err != nil {
		t.Fatalf("Expected tree to be created: %v", err)
	}

	if tree.Hash() == "" {
		t.Error("Tree hash should not be empty")
	}

	if len(tree.Entries()) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(tree.Entries()))
	}
}

func TestNewTree_MultipleEntries(t *testing.T) {
	blob1 := NewBlob([]byte("content1\n"))
	blob2 := NewBlob([]byte("content2\n"))

	firstEntry, _ := NewTreeEntry(ModeRegularFile, "file1.txt", blob1.Hash())
	secondEntry, _ := NewTreeEntry(ModeRegularFile, "file2.txt", blob2.Hash())
	entries := []TreeEntry{
		*firstEntry,
		*secondEntry,
	}

	tree, err := NewTree(entries)
	if err != nil {
		t.Fatalf("Expected tree to be created: %v", err)
	}

	if len(tree.Entries()) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(tree.Entries()))
	}
}

func TestNewTree_SortsEntries(t *testing.T) {
	// Add entries in wrong order
	firstEntry, _ := NewTreeEntry(ModeRegularFile, "z.txt", "hash1")
	secondEntry, _ := NewTreeEntry(ModeRegularFile, "a.txt", "hash2")
	thirdEntry, _ := NewTreeEntry(ModeRegularFile, "m.txt", "hash3")
	entries := []TreeEntry{
		*firstEntry,
		*secondEntry,
		*thirdEntry,
	}

	tree, err := NewTree(entries)
	if err != nil {
		t.Fatalf("Expected tree to be created: %v", err)
	}

	sortedEntries := tree.Entries()

	// Should be sorted alphabetically
	if sortedEntries[0].Name() != "a.txt" {
		t.Errorf("Expected first entry to be 'a.txt', got %s", sortedEntries[0].Name())
	}
	if sortedEntries[1].Name() != "m.txt" {
		t.Errorf("Expected second entry to be 'm.txt', got %s", sortedEntries[1].Name())
	}
	if sortedEntries[2].Name() != "z.txt" {
		t.Errorf("Expected third entry to be 'z.txt', got %s", sortedEntries[2].Name())
	}
}

func TestTree_NestedStructure(t *testing.T) {
	// Create blobs for files
	mainBlob := NewBlob([]byte("package main\n"))
	readmeBlob := NewBlob([]byte("# Project\n"))

	// Create subtree for src/ directory
	srcEntry, _ := NewTreeEntry(ModeRegularFile, "main.go", mainBlob.Hash())
	srcEntries := []TreeEntry{
		*srcEntry,
	}
	srcTree, err := NewTree(srcEntries)
	if err != nil {
		t.Fatalf("Expected tree to be created: %v", err)
	}

	// Create root tree
	fileEntry, _ := NewTreeEntry(ModeRegularFile, "README.md", readmeBlob.Hash())
	dirEntry, _ := NewTreeEntry(ModeDirectory, "src", srcTree.Hash())
	rootEntries := []TreeEntry{
		*fileEntry,
		*dirEntry,
	}
	rootTree, err := NewTree(rootEntries)
	if err != nil {
		t.Fatalf("Expected root tree to be created: %v", err)
	}

	// Verify structure
	if len(rootTree.Entries()) != 2 {
		t.Errorf("Expected 2 entries in root tree, got %d", len(rootTree.Entries()))
	}

	// Find the src directory entry
	srcEntry, found := rootTree.FindEntry("src")
	if !found {
		t.Error("Should find 'src' directory")
	}
	if !srcEntry.IsDirectory() {
		t.Error("'src' should be identified as directory")
	}
	if srcEntry.Hash() != srcTree.Hash() {
		t.Error("src entry hash should match src tree hash")
	}
}
