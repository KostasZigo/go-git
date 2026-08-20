// Package gogitpath validates repository-relative logical Git paths.
package gogitpath

import (
	"fmt"
	"path"
	"strings"
)

// Validate verifies that entryPath is relative, forward-slash separated, and
// contains only canonical path segments.
func Validate(entryPath string) error {
	if entryPath == "" {
		return fmt.Errorf("path cannot be empty")
	}
	if path.IsAbs(entryPath) || isWindowsAbsolute(entryPath) {
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

// isWindowsAbsolute reports whether entryPath is a drive-qualified Windows
// path written with forward slashes.
func isWindowsAbsolute(entryPath string) bool {
	if len(entryPath) < 3 {
		return false
	}

	driveLetter := entryPath[0]
	isLetter := driveLetter >= 'A' && driveLetter <= 'Z' ||
		driveLetter >= 'a' && driveLetter <= 'z'

	return isLetter && entryPath[1] == ':' && entryPath[2] == '/'
}
