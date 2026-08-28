package index

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/gogitpath"
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
	if err := ValidatePath(path); err != nil {
		return nil, fmt.Errorf("invalid index path %q: %w", path, err)
	}

	return &Entry{
		mode:         mode,
		hash:         hash,
		path:         path,
		fileSize:     fileSize,
		lastModified: lastModified,
	}, nil
}

// ValidatePath verifies that entryPath is a canonical repository-relative
// logical path and does not address gogit's internal metadata directory.
func ValidatePath(entryPath string) error {
	if err := gogitpath.Validate(entryPath); err != nil {
		return err
	}

	if entryPath == constants.Gogit || strings.HasPrefix(entryPath, constants.Gogit+"/") {
		return fmt.Errorf("path cannot address repository metadata")
	}

	return nil
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

// FromObjectFileMode converts a supported leaf tree mode to the corresponding
// index file mode.
func FromObjectFileMode(mode objects.FileMode) (FileMode, error) {
	switch mode {
	case objects.ModeRegularFile:
		return ModeRegularFile, nil
	case objects.ModeExecutable:
		return ModeExecutable, nil
	case objects.ModeSymlink:
		return ModeSymlink, nil
	case objects.ModeSubmodule:
		return ModeSubmodule, nil
	default:
		return 0, fmt.Errorf("unsupported tree leaf mode: %s", mode)
	}
}

// ToOsFileMOde converts an indexs' package FileMode to the
// corresponding os.FileMode.
func (m FileMode) ToOsFileMOde() (os.FileMode, error) {
	switch m {
	case ModeRegularFile:
		return constants.FilePerms, nil
	case ModeExecutable:
		return constants.ExecutableFilePerms, nil
	default:
		return 0, fmt.Errorf("unsuported file mode for conversion [%v]", m)
	}
}
