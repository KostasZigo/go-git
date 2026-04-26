package index

import (
	"fmt"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
)

// FileMode represents Git index modes (uint32 for binary serialization)
type FileMode uint32

// FileMode constants define the standard Unix permission and type
// values used in Git Index object entries.
const (
	ModeRegularFile FileMode = 0o100644
	ModeExecutable  FileMode = 0o100755
	ModeSymlink     FileMode = 0o120000
	ModeSubmodule   FileMode = 0o160000
)

// IsValid verifies file mode matches Git specification.
func (m FileMode) isValid() bool {
	switch m {
	case ModeRegularFile, ModeExecutable, ModeSymlink, ModeSubmodule:
		return true
	default:
		return false
	}
}

// Entry represents a single entry in the staging area
type Entry struct {
	mode         FileMode  // File permissions
	hash         string    // SHA-1 hash of blob object
	path         string    // Relative filepath from repository root
	fileSize     int64     // File size in bytes
	lastModified time.Time // Last modified timestamp
}

// NewEntry creates index entry from file metadata.
func NewEntry(mode FileMode, hash, path string, fileSize int64, lastModified time.Time) (*Entry, error) {
	if !mode.isValid() {
		return nil, fmt.Errorf("invalid file mode: %v", mode)
	}
	if len(hash) != constants.HashStringLength {
		return nil, fmt.Errorf("invalid hash length expected [%d], got [%d]", constants.HashStringLength, len(hash))
	}
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	return &Entry{
		mode:         mode,
		hash:         hash,
		path:         path,
		fileSize:     fileSize,
		lastModified: lastModified,
	}, nil
}

// Mode returns the Unix file permission and type bits for this entry.
func (indexEntry *Entry) Mode() FileMode {
	return indexEntry.mode
}

// Hash returns the SHA-1 blob hash stored for this entry.
func (indexEntry *Entry) Hash() string {
	return indexEntry.hash
}

// Path returns the forward-slash relative path from the repository root.
func (indexEntry *Entry) Path() string {
	return indexEntry.path
}

// FileSize returns the file size in bytes at the time of staging.
func (indexEntry *Entry) FileSize() int64 {
	return indexEntry.fileSize
}

// LastModified returns the file's modification timestamp at the time of staging.
func (indexEntry *Entry) LastModified() time.Time {
	return indexEntry.lastModified
}

// ToObjectFileMode converts index FileMode to objects.FileMode for tree creation
func ToObjectFileMode(m FileMode) objects.FileMode {
	switch m {
	case ModeRegularFile:
		return objects.ModeRegularFile
	case ModeExecutable:
		return objects.ModeExecutable
	case ModeSymlink:
		return objects.ModeSymlink
	case ModeSubmodule:
		return objects.ModeSubmodule
	default:
		// Should not occur if index validates modes properly
		return objects.ModeRegularFile
	}
}
