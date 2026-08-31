package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestIsPathMissing verifies that missing paths and Unix ENOTDIR errors are
// classified as absent while other filesystem errors are preserved.
func TestIsPathMissing(t *testing.T) {
	testCases := []struct {
		name    string
		err     error
		missing bool
	}{
		{name: "not exist", err: &os.PathError{Op: "lstat", Path: "missing", Err: fs.ErrNotExist}, missing: true},
		{name: "ancestor is not directory", err: &os.PathError{Op: "lstat", Path: filepath.Join("parent", "child"), Err: syscall.ENOTDIR}, missing: true},
		{name: "permission denied", err: &os.PathError{Op: "lstat", Path: "denied", Err: syscall.EACCES}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if missing := IsPathMissing(testCase.err); missing != testCase.missing {
				t.Fatalf("expected missing classification [%t], got [%t] for [%v]", testCase.missing, missing, testCase.err)
			}
		})
	}
}
