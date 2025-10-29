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

// Store saves a GoGit Object to .gogit/objects/<first 2 chars>/<rest>
// Returns nil if object already exists
func (store *ObjectStore) Store(obj Object) error {
	hash := obj.Hash()

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
	compressedData, err := store.compressObject(obj)
	if err != nil {
		return fmt.Errorf("failed to compress object: %w", err)
	}

	// Write compressed object data to file
	if err := os.WriteFile(objectFile, compressedData, 0755); err != nil {
		return fmt.Errorf("failed to write object file: %w", err)
	}

	return nil
}

func (store *ObjectStore) compressObject(obj Object) ([]byte, error) {
	data := obj.Data()

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

// ReadBlob reads a blob from storage by hash
func (store *ObjectStore) ReadBlob(hash string) (*Blob, error) {
	data, err := store.readObject(hash)
	if err != nil {
		return nil, err
	}

	return parseBlobData(data, hash)
}

// ReadTree reads a tree from storage by hash
func (store *ObjectStore) ReadTree(hash string) (*Tree, error) {
	data, err := store.readObject(hash)
	if err != nil {
		return nil, err
	}

	return parseTreeData(data, hash)
}

// readObject is a private helper that reads and decompresses any object
// It returns the raw decompressed data without parsing
func (store *ObjectStore) readObject(hash string) ([]byte, error) {
	objectFile := filepath.Join(store.repoPath, objectsRelativeFilePath, hash[:2], hash[2:])

	// Read compressed file
	compressedData, err := os.ReadFile(objectFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read object file %s: %w", hash, err)
	}

	// Decompress
	reader, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, fmt.Errorf("failed to create reader for decompressed data: %w", err)
	}
	defer reader.Close()

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("failed to read decompressed data: %w", err)
	}

	return buffer.Bytes(), nil
}

// parseBlobData parses decompressed blob data and returns a Blob object
func parseBlobData(data []byte, expectedHash string) (*Blob, error) {
	// Verify object type is blob
	if !bytes.HasPrefix(data, []byte("blob ")) {
		return nil, fmt.Errorf("object %s is not a blob", expectedHash)
	}

	// Find null byte separator (end of header)
	nullByteIndex := bytes.IndexByte(data, 0)
	if nullByteIndex == -1 {
		return nil, fmt.Errorf("invalid blob format: no null byte found")
	}

	// Extract content (after null byte)
	content := data[nullByteIndex+1:]

	// Create blob from content
	blob := NewBlob(content)

	// Verify hash matches
	if blob.Hash() != expectedHash {
		return nil, fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, blob.Hash())
	}

	return blob, nil
}

// parseTreeData parses decompressed tree data and returns a Tree object
func parseTreeData(data []byte, expectedHash string) (*Tree, error) {
	// Verify object type is tree
	if !bytes.HasPrefix(data, []byte("tree ")) {
		return nil, fmt.Errorf("object %s is not a tree", expectedHash)
	}

	// Find null byte separator (end of header)
	nullByteIndex := bytes.IndexByte(data, 0)
	if nullByteIndex == -1 {
		return nil, fmt.Errorf("invalid tree format: no null byte found")
	}

	// Extract tree content (after null byte)
	content := data[nullByteIndex+1:]

	// Parse tree entries from binary content
	entries, err := parseTreeEntries(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tree entries: %w", err)
	}

	// Create tree from entries
	tree, err := NewTree(entries)
	if err != nil {
		return nil, fmt.Errorf("failed to create tree from entries: %v", err)
	}

	// Verify hash matches
	if tree.Hash() != expectedHash {
		return nil, fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, tree.Hash())
	}

	return tree, nil
}

// parseTreeEntries parses binary tree content into a slice of TreeEntry
// Format: <mode> <name>\0<20-byte binary SHA>
func parseTreeEntries(content []byte) ([]TreeEntry, error) {
	var entries []TreeEntry
	offset := 0

	for offset < len(content) {
		// 1. Find space separator (between mode and name)
		spaceIndex := bytes.IndexByte(content[offset:], ' ')
		if spaceIndex == -1 {
			// No more entries
			break
		}

		// 2. Extract mode (e.g., "100644", "040000")
		modeStr := string(content[offset : offset+spaceIndex])
		mode := FileMode(modeStr)
		offset += spaceIndex + 1

		// 3. Find null byte (end of name)
		nullIndex := bytes.IndexByte(content[offset:], 0)
		if nullIndex == -1 {
			return nil, fmt.Errorf("invalid tree entry: no null byte after name")
		}

		// 4. Extract name
		name := string(content[offset : offset+nullIndex])
		offset += nullIndex + 1

		// 5. Extract 20-byte binary hash
		if offset+20 > len(content) {
			return nil, fmt.Errorf("invalid tree entry: incomplete hash for %s", name)
		}
		hashBytes := content[offset : offset+20]

		// 6. Convert binary hash to hex string (40 chars)
		hash := fmt.Sprintf("%x", hashBytes)
		offset += 20

		// 7. Create entry
		entry, err := NewTreeEntry(mode, name, hash)
		if err != nil {
			return nil, fmt.Errorf("failed to create tree entry: %v", err)
		}
		entries = append(entries, *entry)
	}

	return entries, nil
}

// Exists checks if an object exists in storage
func (s *ObjectStore) Exists(hash string) bool {
	objectFile := filepath.Join(s.repoPath, ".gogit", "objects", hash[:2], hash[2:])
	_, err := os.Stat(objectFile)
	return !(errors.Is(err, fs.ErrNotExist))
}
