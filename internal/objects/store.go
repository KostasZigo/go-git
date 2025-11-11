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
	"strconv"
	"strings"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/utils"
)

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
	objectPath := store.objectPath(hash)

	// Check if object already exists (content-addressable)
	_, err := os.Stat(objectPath)
	if err == nil {
		slog.Debug("Object with this hash already exists",
			"hash", hash)
		return nil
	}
	if !(errors.Is(err, fs.ErrNotExist)) {
		return fmt.Errorf("failed to check object existence: %w", err)
	}

	// Create directory if it doesn't exist
	objectDir := filepath.Dir(objectPath)
	if err := os.MkdirAll(objectDir, constants.DirPerms); err != nil {
		return fmt.Errorf("failed to create object directory: %w", err)
	}

	// Compress object content
	compressedData, err := store.compressData(obj.Data())
	if err != nil {
		return fmt.Errorf("failed to compress object: %w", err)
	}

	// Write compressed object data to file
	if err := os.WriteFile(objectPath, compressedData, constants.FilePerms); err != nil {
		return fmt.Errorf("failed to write object file: %w", err)
	}

	return nil
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

// ReadCommit reads a commit from storage by hash
func (store *ObjectStore) ReadCommit(hash string) (*Commit, error) {
	data, err := store.readObject(hash)
	if err != nil {
		return nil, err
	}

	return parseCommitData(data, hash)
}

// Exists checks if an object exists in storage
func (store *ObjectStore) Exists(hash string) bool {
	_, err := os.Stat(store.objectPath(hash))
	return err == nil
}

// objectPath constructs filesystem path for object hash.
func (s *ObjectStore) objectPath(hash string) string {
	return filepath.Join(s.repoPath, constants.Gogit, constants.Objects, hash[:constants.HashDirPrefixLength], hash[constants.HashDirPrefixLength:])
}

// compressData compresses byte slice using zlib.
func (store *ObjectStore) compressData(data []byte) ([]byte, error) {
	// Compress with zlib
	var buffer bytes.Buffer
	// Crete a new writer that compresses and writes data to the buffer
	writer := zlib.NewWriter(&buffer)

	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return nil, err
	}

	// Call Close in order to flush any buffered data
	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// readObject is a private helper that reads and decompresses any object
// It returns the raw decompressed data without parsing
func (store *ObjectStore) readObject(hash string) ([]byte, error) {
	// Read compressed file
	compressedData, err := os.ReadFile(store.objectPath(hash))
	if err != nil {
		return nil, fmt.Errorf("failed to read object file %s: %w", hash, err)
	}

	return decompressData(compressedData)
}

// decompressData decompresses zlib-compressed byte slice.
func decompressData(compressed []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("failed to create zlib reader: %w", err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	return buf.Bytes(), nil
}

// parseBlobData parses decompressed blob data and returns a Blob object
func parseBlobData(data []byte, expectedHash string) (*Blob, error) {
	// Verify object type is blob
	if !bytes.HasPrefix(data, []byte(constants.BlobPrefix)) {
		return nil, fmt.Errorf("object %s is not a blob", expectedHash)
	}

	// Find null byte separator (end of header)
	nullByteIndex := bytes.IndexByte(data, constants.NullByte)
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
	if !bytes.HasPrefix(data, []byte(constants.TreePrefix)) {
		return nil, fmt.Errorf("object %s is not a tree", expectedHash)
	}

	// Find null byte separator (end of header)
	nullByteIndex := bytes.IndexByte(data, constants.NullByte)
	if nullByteIndex == -1 {
		return nil, fmt.Errorf("invalid tree format: no null byte found")
	}

	// Parse tree entries from binary content
	entries, err := parseTreeEntries(data[nullByteIndex+1:])
	if err != nil {
		return nil, fmt.Errorf("failed to parse tree entries: %w", err)
	}

	// Create tree from entries
	tree, err := NewTree(entries)
	if err != nil {
		return nil, fmt.Errorf("failed to create tree from entries: %w", err)
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
		mode := FileMode(content[offset : offset+spaceIndex])
		offset += spaceIndex + 1

		// 3. Find null byte (end of name)
		nullIndex := bytes.IndexByte(content[offset:], constants.NullByte)
		if nullIndex == -1 {
			return nil, fmt.Errorf("invalid tree entry: no null byte after name")
		}

		// 4. Extract name
		name := string(content[offset : offset+nullIndex])
		offset += nullIndex + 1

		// 5. Extract 20-byte binary hash
		if offset+constants.HashByteLength > len(content) {
			return nil, fmt.Errorf("invalid tree entry: incomplete hash for %s", name)
		}

		// 6. Convert binary hash to hex string (40 chars)
		hash := fmt.Sprintf("%x", content[offset:offset+constants.HashByteLength])
		offset += constants.HashByteLength

		// 7. Create entry
		entry, err := NewTreeEntry(mode, name, hash)
		if err != nil {
			return nil, fmt.Errorf("failed to create entry for %s: %w", name, err)
		}
		entries = append(entries, *entry)
	}

	return entries, nil
}

// parseCommitData parses decompressed commit data and validates hash.
func parseCommitData(data []byte, hash string) (*Commit, error) {
	if !bytes.HasPrefix(data, []byte(constants.CommitPrefix)) {
		return nil, fmt.Errorf("object %s is not a commit", hash)
	}

	// Find end of header
	nullByteIndex := bytes.IndexByte(data, constants.NullByte)
	if nullByteIndex == -1 {
		return nil, fmt.Errorf("invalid commit format: no null byte found")
	}

	commit, err := parseCommitContent(string(data[nullByteIndex+1:]))
	if err != nil {
		return nil, fmt.Errorf("failed to parse commit: %w", err)
	}

	if hash != commit.Hash() {
		return nil, fmt.Errorf("hash mismatch: expected %s , got %s", hash, commit.Hash())
	}

	return commit, nil
}

// parseCommitContent parses commit text content into Commit object.
func parseCommitContent(content string) (*Commit, error) {
	lines := strings.Split(content, "\n")

	var treeHash, parentHash string
	var author, committer Author
	var messageIndex int

	for i, line := range lines {
		if line == "" { // this is the blank line separating the message
			messageIndex = i + 1
			break
		}

		switch {
		case strings.HasPrefix(line, constants.TreePrefix):
			treeHash = strings.TrimPrefix(line, constants.TreePrefix)
		case strings.HasPrefix(line, constants.CommitParentPrefix):
			parentHash = strings.TrimPrefix(line, constants.CommitParentPrefix)
		case strings.HasPrefix(line, constants.CommitAuthorPrefix):
			var err error
			author, err = parseAuthor(strings.TrimPrefix(line, constants.CommitAuthorPrefix))
			if err != nil {
				return nil, fmt.Errorf("failed to parse author: %w", err)
			}
		case strings.HasPrefix(line, constants.CommitCommitterPrefix):
			var err error
			committer, err = parseAuthor(strings.TrimPrefix(line, constants.CommitCommitterPrefix))
			if err != nil {
				return nil, fmt.Errorf("failed to parse committer: %w", err)
			}
		}
	}

	// Validate required fields
	if treeHash == "" {
		return nil, fmt.Errorf("commit missing tree hash")
	}
	if author.Name == "" {
		return nil, fmt.Errorf("commit missing author")
	}
	if committer.Name == "" {
		return nil, fmt.Errorf("commit missing committer")
	}

	// Extract message
	message := strings.Join(lines[messageIndex:], "\n")
	message = strings.TrimRight(message, "\n")

	//Compute Hash
	builtContent := buildCommitContent(treeHash, parentHash, message, author)
	hash, err := utils.ComputeHash(builtContent, utils.CommitObjectType)
	if err != nil {
		return nil, fmt.Errorf("failed to compute commit hash: %w", err)
	}

	// Create commit
	return &Commit{
		hash:       hash,
		treeHash:   treeHash,
		parentHash: parentHash,
		author:     author,
		committer:  committer,
		message:    message,
	}, nil
}

// parseAuthor parses author/committer line format: Name <email> timestamp timezone
func parseAuthor(content string) (Author, error) {
	emailStartIndex := strings.Index(content, "<")
	if emailStartIndex == -1 {
		return Author{}, fmt.Errorf("invalid author format: no email")
	}

	name := strings.TrimSpace(content[:emailStartIndex])
	parts := strings.Fields(content[emailStartIndex:])

	if len(parts) < 3 {
		return Author{}, fmt.Errorf("invalid author format: missing fields")
	}

	email := strings.Trim(parts[0], "<>")

	unixTime, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return Author{}, fmt.Errorf("invalid timestamp: %w", err)
	}

	timezone := parts[2]
	if len(timezone) != 5 {
		return Author{}, fmt.Errorf("invalid timezone format: %s", timezone)
	}

	offsetHours, err := strconv.Atoi(timezone[1:3])
	if err != nil {
		return Author{}, fmt.Errorf("invalid timezone hours: %w", err)
	}

	offsetMinutes, err := strconv.Atoi(timezone[3:5])
	if err != nil {
		return Author{}, fmt.Errorf("invalid timezone minutes: %w", err)
	}

	offsetSeconds := (offsetHours * constants.SecondsPerHour) + (offsetMinutes * constants.SecondsPerMinute)

	if timezone[0] == '-' {
		offsetSeconds = -offsetSeconds
	}

	location := time.FixedZone("", offsetSeconds)
	timestamp := time.Unix(unixTime, 0).In(location)

	return Author{
		Name:      name,
		Email:     email,
		Timestamp: timestamp,
	}, nil
}
