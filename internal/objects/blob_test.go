package objects

import (
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/testutils"
)

// TestNewBlob verifies blob creation from raw content.
func TestNewBlob(t *testing.T) {
	content := []byte("Hello, World!\n")
	blob := NewBlob(content)

	assertBlobHash(t, blob, content)
	assertBlobContent(t, blob, content)
}

// TestNewBlobFromFile verifies blob creation from filesystem file.
func TestNewBlobFromFile(t *testing.T) {
	repoPath := t.TempDir()
	content := []byte("test content\n")
	testFile := testutils.CreateTestFile(t, repoPath, "test.txt", content)

	blob, err := NewBlobFromFile(testFile)
	if err != nil {
		t.Fatalf("Failed to create blob from file: %v", err)
	}

	assertBlobHash(t, blob, content)
	assertBlobContent(t, blob, content)
}

// TestNewBlobFromFile_NonExistent verifies error handling for missing files.
func TestNewBlobFromFile_NonExistent(t *testing.T) {
	_, err := NewBlobFromFile("/nonexistent/file.txt")

	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}

	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected error message about reading file, got: %v", err)
	}
}

// TestBlob_EmptyContent verifies blob behavior with zero-length content.
// GoGit supports empty blobs; hash must be deterministic.
func TestBlob_EmptyContent(t *testing.T) {
	emptyContent := []byte("")
	blob := NewBlob(emptyContent)

	assertBlobHash(t, blob, emptyContent)
	assertBlobContent(t, blob, emptyContent)
}

// TestBlob_HashConsistency verifies content-addressable storage property.
// Identical content must produce identical hashes (idempotent).
func TestBlob_HashConsistency(t *testing.T) {
	content := []byte("test content")

	blob1 := NewBlob(content)
	blob2 := NewBlob(content)

	if blob1.Hash() != blob2.Hash() {
		t.Fatal("Same content should produce same hash")
	}
}

// TestBlob_DifferentContentDifferentHash verifies hash collision resistance.
// Different content must produce different hashes.
func TestBlob_DifferentContentDifferentHash(t *testing.T) {
	blob1 := NewBlob([]byte("content A"))
	blob2 := NewBlob([]byte("content B"))

	if blob1.Hash() == blob2.Hash() {
		t.Fatal("Different content should produce different hashes")
	}
}
