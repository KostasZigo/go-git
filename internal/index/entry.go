package index

import (
	"fmt"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
)

// FileMode represents Git index modes (uint32 for binary serialization)
type FileMode uint32

const (
	ModeRegularFile FileMode = 0o100644
	ModeExecutable  FileMode = 0o100755
	ModeSymlink     FileMode = 0o120000
	ModeDirectory   FileMode = 0o040000
	ModeSubmodule   FileMode = 0o160000
)

// IsValid verifies file mode matches Git specification.
func (m FileMode) isValid() bool {
	switch m {
	case ModeRegularFile, ModeExecutable, ModeSymlink, ModeDirectory, ModeSubmodule:
		return true
	default:
		return false
	}
}

// IndexEntry represents a single entry in the staging area
type IndexEntry struct {
	mode         FileMode  // File permissions
	hash         string    // SHA-1 hash of blob object
	path         string    // Relative filepath from repository root
	fileSize     int64     // File size in bytes
	lastModified time.Time // Last modified timestamp
}

// NewEntry creates index entry from file metadata.
func NewEntry(mode FileMode, hash, path string, fileSize int64, lastModified time.Time) (*IndexEntry, error) {
	if !mode.isValid() {
		return nil, fmt.Errorf("invalid file mode: %v", mode)
	}
	if len(hash) != constants.HashStringLength {
		return nil, fmt.Errorf("invalid hash length expectes[%d], got [%d]", constants.HashStringLength, len(hash))
	}
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	return &IndexEntry{
		mode:         mode,
		hash:         hash,
		path:         path,
		fileSize:     fileSize,
		lastModified: lastModified,
	}, nil
}

func (indexEntry *IndexEntry) Mode() FileMode {
	return indexEntry.mode
}

func (indexEntry *IndexEntry) Hash() string {
	return indexEntry.hash
}

func (indexEntry *IndexEntry) Path() string {
	return indexEntry.path
}

func (indexEntry *IndexEntry) FileSize() int64 {
	return indexEntry.fileSize
}

func (indexEntry *IndexEntry) LastModified() time.Time {
	return indexEntry.lastModified
}
