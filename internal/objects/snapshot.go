package objects

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/KostasZigo/gogit/internal/hasher"
)

// TreeSnapshot represents one repository tree as entries keyed by logical Git paths.
type TreeSnapshot map[string]SnapshotEntry

// SnapshotEntry is the object metadata stored for one logical file path.
type SnapshotEntry struct {
	Mode FileMode
	Hash string
}

// Validate verifies that all snapshot paths and entries can be represented
// as leaf entries in a Git tree.
func (ts TreeSnapshot) Validate() error {
	relativePaths := make([]string, 0, len(ts))
	for relativePath := range ts {
		relativePaths = append(relativePaths, relativePath)
	}

	// Sorting makes the first validation failure deterministic
	slices.Sort(relativePaths)

	for _, relativePath := range relativePaths {
		if err := validateSnapshotPath(relativePath); err != nil {
			return fmt.Errorf("invalid snapshot path %s: %w", relativePath, err)
		}
		if err := validateSnapshotEntry(ts[relativePath]); err != nil {
			return fmt.Errorf("invalid snapshot entry for %q: %w", relativePath, err)
		}
	}

	// Validate that there are no file/directory collisions such as `docs` and `docs/readme.md`
	for _, relativePath := range relativePaths {
		for parentPath := path.Dir(relativePath); parentPath != "."; parentPath = path.Dir(parentPath) {
			if _, exists := ts[parentPath]; exists {
				return fmt.Errorf(
					"snapshot path collision: %q is both a file and a directory",
					parentPath,
				)
			}
		}
	}

	return nil
}

// validateSnapshotEntry verifies that a snapshot leaf has a supported
// non-directory mode and a valid SHA-1 object hash.
func validateSnapshotEntry(entry SnapshotEntry) error {
	if !entry.Mode.isValid() {
		return fmt.Errorf("unsupported file mode %s", entry.Mode)
	}
	if entry.Mode == ModeDirectory {
		return fmt.Errorf("directory mode is not valid for a snapshot entry")
	}
	if !hasher.IsValidSHA1(entry.Hash) {
		return fmt.Errorf("invalid SHA-1 hash %q", entry.Hash)
	}
	return nil
}

// validateSnapshotPath verifies that a logical Git path is relative,
// forward-slash separated, and contains only valid path segments.
func validateSnapshotPath(entryPath string) error {
	if entryPath == "" {
		return fmt.Errorf("path cannot be empty")
	}
	if path.IsAbs(entryPath) || isWindowsAbsolutePath(entryPath) {
		return fmt.Errorf("path cannot be absolute")
	}
	if strings.Contains(entryPath, `\`) {
		return fmt.Errorf("path cannot contain backslashes")
	}
	if strings.HasSuffix(entryPath, "/") {
		return fmt.Errorf("path cannot have a trailing slash")
	}

	for _, segment := range strings.Split(entryPath, "/") {
		switch segment {
		case "":
			return fmt.Errorf("path cannot contain empty segments")
		case ".", "..":
			return fmt.Errorf("path cannot contain %q segments", segment)
		}
	}

	return nil
}

// isWindowsAbsolutePath reports whether entryPath uses a drive-qualified
// Windows absolute path written with forward slashes.
func isWindowsAbsolutePath(entryPath string) bool {
	if len(entryPath) < 3 {
		return false
	}

	driveLetter := entryPath[0]
	isLetter := driveLetter >= 'A' && driveLetter <= 'Z' ||
		driveLetter >= 'a' && driveLetter <= 'z'

	return isLetter && entryPath[1] == ':' && entryPath[2] == '/'
}
