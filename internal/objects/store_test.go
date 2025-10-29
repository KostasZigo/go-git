package objects

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// BLOB STORAGE TESTS

func TestObjectStore_StoreBlob(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize .gogit structure
	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit/objects: %v", err)
	}

	objectStore := NewObjectStore(tempDir)
	blob := NewBlob([]byte("test content\n"))

	// Store the blob
	err := objectStore.Store(blob)
	if err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	// Verify file was created
	hash := blob.Hash()
	objectPath := filepath.Join(tempDir, ".gogit", "objects", hash[:2], hash[2:])

	if _, err := os.Stat(objectPath); errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Object file was not created at %s", objectPath)
	}
}

func TestObjectStore_Compression(t *testing.T) {
	tempDir := t.TempDir()

	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit/objects: %v", err)
	}

	store := NewObjectStore(tempDir)

	// Use larger content to ensure compression is effective
	largeContent := bytes.Repeat([]byte("This is repeated content. "), 100)
	blob := NewBlob(largeContent)

	// Store the blob
	if err := store.Store(blob); err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	// Read the raw file to verify compression
	hash := blob.Hash()
	objectPath := filepath.Join(tempDir, ".gogit", "objects", hash[:2], hash[2:])
	compressedData, err := os.ReadFile(objectPath)
	if err != nil {
		t.Fatalf("Failed to read stored object: %v", err)
	}

	// Verify data is actually compressed (should be smaller than original)
	originalSize := len(blob.Data())
	compressedSize := len(compressedData)

	if compressedSize >= originalSize {
		t.Errorf("Data doesn't appear to be compressed: compressed size (%d) >= original size (%d)",
			compressedSize, originalSize)
	}

	t.Logf("Compression effective: %d bytes -> %d bytes (%.1f%% reduction)",
		originalSize, compressedSize, 100*(1-float64(compressedSize)/float64(originalSize)))

	// Read it back
	readBlob, err := store.ReadBlob(blob.Hash())
	if err != nil {
		t.Fatalf("Failed to read blob: %v", err)
	}

	// Verify content matches
	if string(readBlob.Content()) != string(largeContent) {
		t.Errorf("Content mismatch: expected %q, got %q",
			largeContent, readBlob.Content())
	}

	// Verify hash matches
	if readBlob.Hash() != blob.Hash() {
		t.Errorf("Hash mismatch: expected %s, got %s",
			blob.Hash(), readBlob.Hash())
	}

}

func TestObjectStore_StoreIdempotent(t *testing.T) {
	tempDir := t.TempDir()

	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit/objects: %v", err)
	}

	store := NewObjectStore(tempDir)
	blob := NewBlob([]byte("test\n"))

	// Store twice, second time a debug log should appear
	if err := store.Store(blob); err != nil {
		t.Fatalf("First store failed: %v", err)
	}

	if err := store.Store(blob); err != nil {
		t.Fatalf("Second store failed: %v", err)
	}

	// Verify only one file was created (no duplicates)
	hash := blob.Hash()
	objectPath := filepath.Join(tempDir, ".gogit", "objects", hash[:2], hash[2:])

	info, err := os.Stat(objectPath)
	if err != nil {
		t.Fatalf("Object file should exist: %v", err)
	}

	// Verify it's a regular file (not multiple files)
	if !info.Mode().IsRegular() {
		t.Error("Object should be a regular file")
	}
}

func TestObjectStore_Exists(t *testing.T) {
	tempDir := t.TempDir()

	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit/objects: %v", err)
	}

	store := NewObjectStore(tempDir)
	blob := NewBlob([]byte("test\n"))

	// Should not exist initially
	if store.Exists(blob.Hash()) {
		t.Error("Blob should not exist before storing")
	}

	// Store it
	if err := store.Store(blob); err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	// Should exist now
	if !store.Exists(blob.Hash()) {
		t.Error("Blob should exist after storing")
	}
}

func TestObjectStore_ReadNonExistentBlob(t *testing.T) {
	tempDir := t.TempDir()

	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit/objects: %v", err)
	}

	store := NewObjectStore(tempDir)

	// Try to read a non-existent hash
	fakeHash := "0000000000000000000000000000000000000000"
	_, err := store.ReadBlob(fakeHash)

	if err == nil {
		t.Fatal("Expected error when reading non-existent object")
	}

	if !os.IsNotExist(errors.Unwrap(err)) {
		t.Errorf("Expected file not found error, got: %v", err)
	}
}

// TREE STORAGE TESTS

func TestObjectStore_StoreAndReadTree(t *testing.T) {
	tempDir := t.TempDir()

	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit/objects: %v", err)
	}

	store := NewObjectStore(tempDir)

	// Create a blob
	blob := NewBlob([]byte("test content"))
	if err := store.Store(blob); err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	// Create Tree with blob entry
	treeEntry, _ := NewTreeEntry(ModeRegularFile, "file.txt", blob.Hash())
	tree, err := NewTree([]TreeEntry{*treeEntry})
	if err != nil {
		t.Fatalf("Failed to create new tree: %v", err)
	}
	if err := store.Store(tree); err != nil {
		t.Fatalf("Failed to store tree: %v", err)
	}

	// Verify file was created
	hash := tree.Hash()
	objectPath := filepath.Join(tempDir, ".gogit", "objects", hash[:2], hash[2:])

	if _, err := os.Stat(objectPath); errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Tree file was not created at %s", objectPath)
	}

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
	retrievedEntry := retrievedTree.Entries()[0]
	if retrievedEntry.Name() != treeEntry.Name() {
		t.Errorf("Entry name mismatch: expected %s, got %s", treeEntry.Name(), retrievedEntry.Name())
	}
	if retrievedEntry.Hash() != treeEntry.Hash() {
		t.Errorf("Entry hash mismatch: expected %s, got %s", blob.Hash(), retrievedEntry.Hash())
	}
	if retrievedEntry.Mode() != treeEntry.Mode() {
		t.Errorf("Entry mode mismatch: expected %s, got %s", ModeRegularFile, retrievedEntry.Mode())
	}
}

func TestObjectStore_ReadTree_MultipleEntries(t *testing.T) {
	tempDir := t.TempDir()

	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit/objects: %v", err)
	}

	store := NewObjectStore(tempDir)

	// Create multiple blobs
	blob1 := NewBlob([]byte("content 1\n"))
	blob2 := NewBlob([]byte("content 2\n"))
	store.Store(blob1)
	store.Store(blob2)

	// Create tree with multiple entries
	treeEntry1, _ := NewTreeEntry(ModeRegularFile, "file1.txt", blob1.Hash())
	treeEntry2, _ := NewTreeEntry(ModeRegularFile, "file2.txt", blob2.Hash())
	entries := []TreeEntry{
		*treeEntry1,
		*treeEntry2,
	}

	// Create and store tree
	tree, err := NewTree(entries)
	if err != nil {
		t.Fatalf("Failed to create new tree: %v", err)
	}
	if err := store.Store(tree); err != nil {
		t.Fatalf("Failed to store tree: %v", err)
	}

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
	if retrievedTree.Entries()[0].Name() != treeEntry1.Name() {
		t.Errorf("Expected first entry %s, got %s", treeEntry1.Name(), retrievedTree.Entries()[0].Name())
	}
	if retrievedTree.Entries()[1].Name() != treeEntry2.Name() {
		t.Errorf("Expected second entry %s, got %s", treeEntry2.Name(), retrievedTree.Entries()[1].Name())
	}
}

func TestObjectStore_ReadTree_NestedTree(t *testing.T) {
	tempDir := t.TempDir()

	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit/objects: %v", err)
	}

	store := NewObjectStore(tempDir)

	// Create a blob
	blob := NewBlob([]byte("nested content\n"))
	if err := store.Store(blob); err != nil {
		t.Fatalf("Failed to store nested blob:%x", err)
	}

	// Create subtree
	subTreeEntry, _ := NewTreeEntry(ModeRegularFile, "nested.txt", blob.Hash())
	subTreeEntries := []TreeEntry{
		*subTreeEntry,
	}
	subTree, _ := NewTree(subTreeEntries)
	if err := store.Store(subTree); err != nil {
		t.Fatalf("Failed to store subtree:%x", err)
	}

	// Create root tree with directory entry
	rootBlob := NewBlob([]byte("root content\n"))
	if err := store.Store(rootBlob); err != nil {
		t.Fatalf("Failed to strore root blob:%x", err)
	}
	rootEntryFile, _ := NewTreeEntry(ModeRegularFile, "root.txt", rootBlob.Hash())
	rootEntryDir, _ := NewTreeEntry(ModeDirectory, "subdir", subTree.Hash())
	rootEntries := []TreeEntry{
		*rootEntryFile,
		*rootEntryDir,
	}
	rootTree, _ := NewTree(rootEntries)
	if err := store.Store(rootTree); err != nil {
		t.Fatalf("Failed to store root tree:%x", err)
	}

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
	if fileEntry.Name() != rootEntryFile.Name() {
		t.Errorf("Entry name mismatch: expected %s, got %s", rootEntryFile.Name(), fileEntry.Name())
	}
	if fileEntry.Hash() != rootEntryFile.Hash() {
		t.Errorf("Entry hash mismatch: expected %s, got %s", rootBlob.Hash(), fileEntry.Hash())
	}
	if fileEntry.Mode() != rootEntryFile.Mode() {
		t.Errorf("Entry mode mismatch: expected %s, got %s", rootEntryFile.Mode(), fileEntry.Mode())
	}

	// Verify directory entry
	dirEntry := retrievedRootTree.Entries()[1]
	if dirEntry.Name() != rootEntryDir.Name() {
		t.Fatalf("Expected first entry %s, got %s", rootEntryDir.Name(), dirEntry.Name())
	}
	if dirEntry.Mode() != ModeDirectory {
		t.Fatalf("Expected directory mode, got %s", dirEntry.Mode())
	}
	if dirEntry.Hash() != subTree.Hash() {
		t.Fatalf("Directory hash mismatch: expected %s, got %s", subTree.Hash(), dirEntry.Hash())
	}

	// Read subtree
	retrievedSubTree, err := store.ReadTree(dirEntry.Hash())
	if err != nil {
		t.Fatalf("Failed to read subtree: %v", err)
	}

	if len(retrievedSubTree.Entries()) != 1 {
		t.Fatalf("Expected 1 entry in subtree, got %d", len(retrievedSubTree.Entries()))
	}

	// Verify nested File tree entry
	nestedEntry := retrievedSubTree.Entries()[0]
	if nestedEntry.Name() != subTreeEntry.Name() {
		t.Errorf("Entry name mismatch: expected %s, got %s", subTreeEntry.Name(), nestedEntry.Name())
	}
	if nestedEntry.Hash() != subTreeEntry.Hash() {
		t.Errorf("Entry hash mismatch: expected %s, got %s", blob.Hash(), nestedEntry.Hash())
	}
	if nestedEntry.Mode() != subTreeEntry.Mode() {
		t.Errorf("Entry mode mismatch: expected %s, got %s", subTreeEntry.Mode(), nestedEntry.Mode())
	}
}
