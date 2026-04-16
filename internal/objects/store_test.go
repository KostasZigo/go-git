package objects

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/testutils"
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
	objectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, hash[:constants.HashDirPrefixLength], hash[constants.HashDirPrefixLength:])
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
	objectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, hash[:constants.HashDirPrefixLength], hash[constants.HashDirPrefixLength:])
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
	objectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, hash[:constants.HashDirPrefixLength], hash[constants.HashDirPrefixLength:])

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
	fakeHash := testutils.RandomHash()
	_, err := store.ReadBlob(fakeHash)

	if err == nil {
		t.Fatal("Expected error when reading non-existent object")
	}

	if !os.IsNotExist(errors.Unwrap(err)) {
		t.Errorf("Expected file not found error, got: %v", err)
	}
}

// COMMIT STORAGE TESTS

// TestParseAuthorLine verifies author metadata parsing from commit format.
func TestParseAuthorLine(t *testing.T) {
	authorLine := "John Doe <john@example.com> 1698765432 -0500"

	author, err := parseAuthor(authorLine)
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

	timezone := calculateTimezone(author.Timestamp)
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

	timezone := calculateTimezone(commit.author.Timestamp)
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
