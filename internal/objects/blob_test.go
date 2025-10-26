package objects

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewBlob(t *testing.T) {
	content := []byte("Hello, World!\n")
	blob := NewBlob(content)

	expectedHash := computeBlobHash(content)

	if blob.Hash() != expectedHash {
		t.Fatalf("Expected hash %s, got %s", expectedHash, blob.Hash())
	}

	if blob.Size() != len(content) {
		t.Fatalf("Expected size 14, got %d", blob.Size())
	}

	if string(blob.Content()) != string(content) {
		t.Errorf("Content mismatch")
	}
}

func TestNewBlobFromFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	content := []byte("test content\n")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	blob, err := NewBlobFromFile(testFile)
	if err != nil {
		t.Fatalf("Failed to create blob from file: %v", err)
	}

	if string(blob.Content()) != string(content) {
		t.Errorf("Content mismatch: expected %q, got %q", content, blob.Content())
	}

	if blob.Size() != len(content) {
		t.Errorf("Size mismatch: expected %d, got %d", len(content), blob.Size())
	}

	// Verify hash is correct
	expectedHash := computeExpectedHash(content)
	if blob.Hash() != expectedHash {
		t.Errorf("Hash mismatch: expected %s, got %s", expectedHash, blob.Hash())
	}
}

func TestNewBlobFromFile_NonExistent(t *testing.T) {
	_, err := NewBlobFromFile("/nonexistent/file.txt")

	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}

	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected error message about reading file, got: %v", err)
	}
}

func TestBlob_EmptyContent(t *testing.T) {
	blob := NewBlob([]byte(""))

	// Verify empty blob hash
	expectedHash := computeExpectedHash([]byte(""))

	if blob.Hash() != expectedHash {
		t.Fatalf("Expected empty blob hash %s, got %s", expectedHash, blob.Hash())
	}

	if blob.Size() != 0 {
		t.Fatalf("Expected size 0 for empty blob, got %d", blob.Size())
	}
}

func TestBlob_HashConsistency(t *testing.T) {
	content := []byte("test content")

	blob1 := NewBlob(content)
	blob2 := NewBlob(content)

	if blob1.Hash() != blob2.Hash() {
		t.Fatal("Same content should produce same hash")
	}
}

func TestBlob_DifferentContentDifferentHash(t *testing.T) {
	blob1 := NewBlob([]byte("content A"))
	blob2 := NewBlob([]byte("content B"))

	if blob1.Hash() == blob2.Hash() {
		t.Fatal("Different content should produce different hashes")
	}
}

// Helper function to compute expected hash in the same way as the blob
// This makes the test self-documenting and verifiable
func computeExpectedHash(content []byte) string {
	header := fmt.Sprintf("blob %d\x00", len(content))
	data := append([]byte(header), content...)
	hash := sha1.Sum(data)
	return fmt.Sprintf("%x", hash)
}
