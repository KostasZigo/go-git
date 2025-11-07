package objects

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/KostasZigo/gogit/testutils"
)

// BLOB STORAGE TESTS

// TestObjectStore_StoreBlob verifies blob storage creates correct file structure.
func TestObjectStore_StoreBlob(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := NewObjectStore(repoPath)
	blob := NewBlob([]byte("test content\n"))

	// Store the blob
	err := store.Store(blob)
	if err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	// Verify file was created
	hash := blob.Hash()
	objectPath := filepath.Join(repoPath, ".gogit", "objects", hash[:2], hash[2:])
	testutils.AssertFileExists(t, objectPath)
}

// TestObjectStore_Compression verifies zlib compression reduces storage size.
func TestObjectStore_Compression(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := NewObjectStore(repoPath)

	// Use larger content to ensure compression is effective
	largeContent := bytes.Repeat([]byte("This is repeated content. "), 100)
	blob := NewBlob(largeContent)

	// Store the blob
	if err := store.Store(blob); err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	// Read the raw file to verify compression
	hash := blob.Hash()
	objectPath := filepath.Join(repoPath, ".gogit", "objects", hash[:2], hash[2:])
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
		t.Errorf("Hash mismatch: expected [%s], got [%s]",
			blob.Hash(), readBlob.Hash())
	}

}

// TestObjectStore_StoreIdempotent verifies storing same blob twice is safe.
func TestObjectStore_StoreIdempotent(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := NewObjectStore(repoPath)
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
	objectPath := filepath.Join(repoPath, ".gogit", "objects", hash[:2], hash[2:])

	info, err := os.Stat(objectPath)
	if err != nil {
		t.Fatalf("Object file should exist: %v", err)
	}

	// Verify it's a regular file (not multiple files)
	if !info.Mode().IsRegular() {
		t.Error("Object should be a regular file")
	}
}

// TestObjectStore_Exists verifies object existence detection.
func TestObjectStore_Exists(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := NewObjectStore(repoPath)
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

// TestObjectStore_ReadNonExistentBlob verifies error for missing objects.
func TestObjectStore_ReadNonExistentBlob(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := NewObjectStore(repoPath)

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

// TestObjectStore_StoreAndReadTree verifies tree storage with single entry.
func TestObjectStore_StoreAndReadTree(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := NewObjectStore(repoPath)

	// Create a blob
	blob := NewBlob([]byte("test content"))
	if err := store.Store(blob); err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	// Create Tree with blob entry
	treeEntry := createTreeEntry(t, ModeRegularFile, "file.txt", blob.Hash())
	tree := createAndStoreTree(t, store, []TreeEntry{treeEntry})

	// Verify file was created
	hash := tree.Hash()
	objectPath := filepath.Join(repoPath, ".gogit", "objects", hash[:2], hash[2:])
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
	assertTreeEntryEqual(t, retrievedTree.Entries()[0], treeEntry)
}

// TestObjectStore_ReadTree_MultipleEntries verifies tree with multiple files.
func TestObjectStore_ReadTree_MultipleEntries(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := NewObjectStore(repoPath)

	// Create multiple blobs
	blob1 := NewBlob([]byte("content 1\n"))
	blob2 := NewBlob([]byte("content 2\n"))
	store.Store(blob1)
	store.Store(blob2)

	// Create tree with multiple entries
	entries := []TreeEntry{
		createTreeEntry(t, ModeRegularFile, "file1.txt", blob1.Hash()),
		createTreeEntry(t, ModeRegularFile, "file2.txt", blob2.Hash()),
	}

	// Create and store tree
	tree := createAndStoreTree(t, store, entries)

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
	store := NewObjectStore(repoPath)

	// Create a blob
	blob := NewBlob([]byte("nested content\n"))
	if err := store.Store(blob); err != nil {
		t.Fatalf("Failed to store nested blob:%x", err)
	}

	// Create subtree
	subTreeEntry := createTreeEntry(t, ModeRegularFile, "nested.txt", blob.Hash())
	subTreeEntries := []TreeEntry{
		subTreeEntry,
	}
	subTree := createAndStoreTree(t, store, subTreeEntries)

	// Create root tree with directory entry
	rootBlob := NewBlob([]byte("root content\n"))
	if err := store.Store(rootBlob); err != nil {
		t.Fatalf("Failed to strore root blob:%x", err)
	}

	rootEntryFile := createTreeEntry(t, ModeRegularFile, "root.txt", rootBlob.Hash())
	rootEntryDir := createTreeEntry(t, ModeDirectory, "subdir", subTree.Hash())
	rootEntries := []TreeEntry{
		rootEntryFile,
		rootEntryDir,
	}
	rootTree := createAndStoreTree(t, store, rootEntries)

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
	assertTreeEntryEqual(t, fileEntry, rootEntryFile)

	// Verify directory entry
	dirEntry := retrievedRootTree.Entries()[1]
	assertTreeEntryEqual(t, dirEntry, rootEntryDir)

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
	assertTreeEntryEqual(t, nestedEntry, subTreeEntry)
}

// COMMIT STORAGE TESTS

// TestParseAuthorLine verifies author metadata parsing from commit format.
func TestParseAuthorLine(t *testing.T) {
	authorLine := "John Doe <john@example.com> 1698765432 -0500"

	author, err := parseCommitAuthorLine(authorLine)
	if err != nil {
		t.Fatalf("Failed to parse author line: %v", err)
	}

	if author.Name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got %q", author.Name)
	}

	if author.Email != "john@example.com" {
		t.Errorf("Expected email 'john@example.com', got %q", author.Email)
	}

	if author.Timestamp.Unix() != 1698765432 {
		t.Errorf("Expected timestamp 1698765432, got %d", author.Timestamp.Unix())
	}

	_, timeZoneOffset := author.Timestamp.Zone()
	timezone := calculateTimezone(timeZoneOffset)
	if timezone != "-0500" {
		t.Errorf("Expected timezone -0500, got %s", timezone)
	}
}

// TestParseCommitContent verifies commit object parsing from Git format.
func TestParseCommitContent(t *testing.T) {
	commitContent := `tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904
parent abc123def456
author Alexander the Great <alexander@great.com> 1698765432 +0000
committer Alexander the Great <alexander@great.com> 1698765432 +0000

Initial commit message
`

	commit, err := parseCommitContent(commitContent)
	if err != nil {
		t.Fatal("expected commit to be parsed successfully")
	}

	if commit.treeHash != "4b825dc642cb6eb9a060e54bf8d69288fbee4904" {
		t.Errorf("Unexpected tree hash: %s", commit.treeHash)
	}

	if commit.parentHash != "abc123def456" {
		t.Errorf("Unexpected parent hash: %s", commit.parentHash)
	}

	if commit.message != "Initial commit message" {
		t.Errorf("Unexpected message: %q", commit.message)
	}

	if commit.author.Name != "Alexander the Great" {
		t.Errorf("Expected name 'Alexander the Great', got %q", commit.author.Name)
	}

	if commit.author.Email != "alexander@great.com" {
		t.Errorf("Expected email 'alexander@great.com', got %q", commit.author.Email)
	}

	if commit.author.Timestamp.Unix() != 1698765432 {
		t.Errorf("Expected timestamp 1698765432, got %d", commit.author.Timestamp.Unix())
	}

	_, timeZoneOffset := commit.author.Timestamp.Zone()
	timezone := calculateTimezone(timeZoneOffset)
	if timezone != "+0000" {
		t.Errorf("Expected timezone +0000, got %s", timezone)
	}

}

// TestObjectStore_StoreAndReadInitialCommit verifies initial commit storage and retrieval.
func TestObjectStore_StoreAndReadInitialCommit(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := NewObjectStore(repoPath)

	commit := createAndStoreInitialCommit(t, store)

	readCommit, err := store.ReadCommit(commit.hash)
	if err != nil {
		t.Fatalf("Failed to read commit: %v", err)
	}

	assertCommitEqual(t, readCommit, commit)
	if !readCommit.IsInitialCommit() {
		t.Fatal("Expected hash commit to be the initial commit")
	}
}

// TestObjectStore_StoreAndReadCommit_WithParent verifies commit with parent storage.
func TestObjectStore_StoreAndreadChildCommit_WithParent(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)
	store := NewObjectStore(repoPath)

	parentCommit := createAndStoreInitialCommit(t, store)
	childCommit := createAndStoreCommit(t, parentCommit.Hash(), store)

	// Read child back
	readChildCommit, err := store.ReadCommit(childCommit.Hash())
	if err != nil {
		t.Fatalf("Failed to read child commit: %v", err)
	}

	// Verify
	if readChildCommit.parentHash != parentCommit.Hash() {
		t.Errorf("Parent hash mismatch: expected %s, got %s",
			parentCommit.Hash(), readChildCommit.parentHash)
	}
	if readChildCommit.IsInitialCommit() {
		t.Error("Child commit should not be initial commit")
	}
	assertCommitEqual(t, readChildCommit, childCommit)
}
