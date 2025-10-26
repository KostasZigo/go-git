package objects

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

var objectsRelativeFilePath string = filepath.Join(".gogit", "objects")

// ObjectStore manages storage of Git objects
type ObjectStore struct {
	repoPath string // Path to repository root
}

func NewObjectStore(repoPath string) *ObjectStore {
	return &ObjectStore{
		repoPath: repoPath,
	}
}

// Store saves a blob to .gogit/objects/<first 2 chars>/<rest>
// Returns nil if object already exists
func (store *ObjectStore) Store(blob *Blob) error {
	hash := blob.Hash()

	// Calculate object path: .gogit/objects/ab/cdef123...
	objectDir := filepath.Join(store.repoPath, objectsRelativeFilePath, hash[:2])
	objectFile := filepath.Join(objectDir, hash[2:])

	// Check if object already exists (content-addressable)
	_, err := os.Stat(objectFile)
	if err == nil {
		slog.Debug("Object with this hash already exists",
			"hash", hash)
		return nil
	}
	if !(errors.Is(err, fs.ErrNotExist)) {
		return err
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(objectDir, 0755); err != nil {
		return fmt.Errorf("failed to create object directory: %w", err)
	}

	// Compress object content
	compressedData, err := store.compressObject(blob)
	if err != nil {
		return fmt.Errorf("failed to compress object: %w", err)
	}

	// Write compressed object data to file
	if err := os.WriteFile(objectFile, compressedData, 0755); err != nil {
		return fmt.Errorf("failed to write object file: %w", err)
	}

	return nil
}

func (store *ObjectStore) compressObject(blob *Blob) ([]byte, error) {
	data := blob.Data()

	// Compress with zlib
	var buffer bytes.Buffer
	// Crete a new writer that compresses and writes data to the buffer
	writer := zlib.NewWriter(&buffer)

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	// Call Close in order to flush any buffered data
	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// Read reads a blob from storage by hash
func (store *ObjectStore) Read(hash string) (*Blob, error) {
	objectFile := filepath.Join(store.repoPath, objectsRelativeFilePath, hash[:2], hash[2:])

	// Read compressed file
	compressedData, err := os.ReadFile(objectFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read object file %s: %w", hash, err)
	}

	// Decompress
	reader, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, fmt.Errorf("failed to create new reader for decompressed data: %w", err)
	}
	defer reader.Close()

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("failed to read decompressed data: %w", err)
	}

	data := buffer.Bytes()

	// Find null byte separator
	nullByteIndex := bytes.IndexByte(data, 0)
	if nullByteIndex == -1 {
		return nil, fmt.Errorf("invalid object format: no null byte found")
	}

	// Extract content (after null byte)
	content := data[nullByteIndex+1:]

	blob := NewBlob(content)

	if blob.Hash() != hash {
		return nil, fmt.Errorf("hash mismatch: expected %s, got %s", hash, blob.Hash())
	}

	return blob, nil
}

// Exists checks if an object exists in storage
func (s *ObjectStore) Exists(hash string) bool {
	objectFile := filepath.Join(s.repoPath, ".gogit", "objects", hash[:2], hash[2:])
	_, err := os.Stat(objectFile)
	return !(errors.Is(err, fs.ErrNotExist))
}
