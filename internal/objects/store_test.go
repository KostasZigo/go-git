package objects

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/testutils"
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

// Commit Storage tests

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

func TestParseCommitConten(t *testing.T) {
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

func TestObjectStore_StoreAndreadChildCommitInitialCommit(t *testing.T) {
	tempDir := t.TempDir()

	goGitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(goGitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit/objects: %v", err)
	}

	store := NewObjectStore(tempDir)

	// Create a commit
	author := Author{
		Name:      "Giannis Antetokounbo",
		Email:     "g.ante43@gmail.com",
		Timestamp: time.Now().UTC().Truncate(time.Second),
	}

	childCommit, err := NewInitialCommit(testutils.RandomHash(), testutils.RandomString(50), author)
	if err != nil {
		t.Fatalf("Expected initial commit to be created: %v", err)
	}

	// Store commit
	if err := store.Store(childCommit); err != nil {
		t.Fatalf("Failed to store commit: %v", err)
	}

	// Read commit back
	readChildCommit, err := store.ReadCommit(childCommit.hash)
	if err != nil {
		t.Fatalf("Failed to read commit: %v", err)
	}

	// Verify
	if readChildCommit.hash != childCommit.hash {
		t.Fatalf("Expected hash to be: [%s], got: [%s]", childCommit.hash, readChildCommit.hash)
	}
	if !readChildCommit.IsInitialCommit() {
		t.Fatal("Expected hash commit to be the initial commit")
	}
	if readChildCommit.treeHash != childCommit.treeHash {
		t.Fatalf("Expected tree hash to be: [%s], got: [%s]", childCommit.treeHash, readChildCommit.treeHash)
	}
	if readChildCommit.message != childCommit.message {
		t.Fatalf("Expected message to be: [%s], got: [%s]", childCommit.message, readChildCommit.message)
	}
	if readChildCommit.author.String() != childCommit.author.String() {
		t.Fatalf("Expected author to be: [%s], got: [%s]", childCommit.author.String(), readChildCommit.author.String())
	}
	if !readChildCommit.author.Timestamp.Equal(childCommit.author.Timestamp) {
		t.Errorf("Expected author timestamp to be %s,  but got %s", childCommit.author.Timestamp.Format("2006-01-02 15:04:05 -0700"),
			readChildCommit.author.Timestamp.Format("2006-01-02 15:04:05 -0700"))
	}
	if readChildCommit.author.Timestamp.Format("2006-01-02 15:04:05 -0700") != childCommit.author.Timestamp.Format("2006-01-02 15:04:05 -0700") {
		t.Fatalf("Expected author timestamp to be %s,  but got %s", childCommit.author.Timestamp.Format("2006-01-02 15:04:05 -0700"),
			readChildCommit.author.Timestamp.Format("2006-01-02 15:04:05 -0700"))
	}
}

func TestObjectStore_StoreAndreadChildCommit_WithParent(t *testing.T) {

	tempDir := t.TempDir()

	goGitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(goGitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit/objects: %v", err)
	}

	store := NewObjectStore(tempDir)

	// Create a commit
	author := Author{
		Name:      "Giannis Antetokounbo",
		Email:     "g.ante43@gmail.com",
		Timestamp: time.Now().UTC().Truncate(time.Second),
	}

	parentCommit, err := NewInitialCommit(testutils.RandomHash(), testutils.RandomString(50), author)
	if err != nil {
		t.Fatalf("Expected initial commit to be created: %v", err)
	}

	// Store commit
	if err := store.Store(parentCommit); err != nil {
		t.Fatalf("Failed to store commit: %v", err)
	}

	// Create new commit with parent
	childCommit, err := NewCommit(testutils.RandomHash(), parentCommit.hash, testutils.RandomString(50), author)
	if err != nil {
		t.Fatalf("Expected commit with parent to be created: %v", err)
	}

	// Store commit with parent
	if err := store.Store(childCommit); err != nil {
		t.Fatalf("Failed to store commit with parent: %v", err)
	}

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
	if readChildCommit.hash != childCommit.hash {
		t.Fatalf("Expected hash to be: [%s], got: [%s]", childCommit.hash, readChildCommit.hash)
	}
	if readChildCommit.treeHash != childCommit.treeHash {
		t.Fatalf("Expected tree hash to be: [%s], got: [%s]", childCommit.treeHash, readChildCommit.treeHash)
	}
	if readChildCommit.message != childCommit.message {
		t.Fatalf("Expected message to be: [%s], got: [%s]", childCommit.message, readChildCommit.message)
	}
	if readChildCommit.author.String() != childCommit.author.String() {
		t.Fatalf("Expected author to be: [%s], got: [%s]", childCommit.author.String(), readChildCommit.author.String())
	}
	if !readChildCommit.author.Timestamp.Equal(childCommit.author.Timestamp) {
		t.Errorf("Expected author timestamp to be %s,  but got %s", childCommit.author.Timestamp.Format("2006-01-02 15:04:05 -0700"),
			readChildCommit.author.Timestamp.Format("2006-01-02 15:04:05 -0700"))
	}
	if readChildCommit.author.Timestamp.Format("2006-01-02 15:04:05 -0700") != childCommit.author.Timestamp.Format("2006-01-02 15:04:05 -0700") {
		t.Fatalf("Expected author timestamp to be %s,  but got %s", childCommit.author.Timestamp.Format("2006-01-02 15:04:05 -0700"),
			readChildCommit.author.Timestamp.Format("2006-01-02 15:04:05 -0700"))
	}
}
