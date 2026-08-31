// Package filesystem provides shared filesystem inspection policies.
package filesystem

import (
	"errors"
	"io/fs"
	"syscall"
)

// IsPathMissing reports whether an inspected path is absent, including the
// Unix case where an ancestor exists but is not a directory.
func IsPathMissing(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}
