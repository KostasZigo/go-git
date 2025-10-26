package objects

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestObjectStore_Store(t *testing.T) {
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
	readBlob, err := store.Read(blob.Hash())
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

func TestObjectStore_ReadNonExistent(t *testing.T) {
	tempDir := t.TempDir()

	gogitDir := filepath.Join(tempDir, ".gogit", "objects")
	if err := os.MkdirAll(gogitDir, 0755); err != nil {
		t.Fatalf("Failed to create .gogit/objects: %v", err)
	}

	store := NewObjectStore(tempDir)

	// Try to read a non-existent hash
	fakeHash := "0000000000000000000000000000000000000000"
	_, err := store.Read(fakeHash)

	if err == nil {
		t.Fatal("Expected error when reading non-existent object")
	}

	if !os.IsNotExist(errors.Unwrap(err)) {
		t.Errorf("Expected file not found error, got: %v", err)
	}
}
