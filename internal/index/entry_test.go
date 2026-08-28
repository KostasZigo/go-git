package index

import (
	"strings"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestNewEntry_PathValidation verifies that index entries accept canonical
// logical paths and reject paths outside the worktree or inside .gogit.
func TestNewEntry_PathValidation(t *testing.T) {
	testCases := []struct {
		name          string
		path          string
		expectedError string
	}{
		{name: "root file", path: "README.md"},
		{name: "nested file", path: "docs/README.md"},
		{name: "empty path", path: "", expectedError: "path cannot be empty"},
		{name: "absolute POSIX path", path: "/README.md", expectedError: "path cannot be absolute"},
		{name: "absolute Windows path", path: "C:/README.md", expectedError: "path cannot be absolute"},
		{name: "backslash separator", path: `docs\README.md`, expectedError: "path cannot contain backslashes"},
		{name: "current directory segment", path: "docs/./README.md", expectedError: `path cannot contain "." segments`},
		{name: "parent directory segment", path: "docs/../README.md", expectedError: `path cannot contain ".." segments`},
		{name: "empty segment", path: "docs//README.md", expectedError: "path cannot contain empty segments"},
		{name: "trailing slash", path: "docs/", expectedError: "path cannot have a trailing slash"},
		{name: "metadata directory", path: ".gogit", expectedError: "path cannot address repository metadata"},
		{name: "metadata descendant", path: ".gogit/HEAD", expectedError: "path cannot address repository metadata"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewEntry(
				ModeRegularFile,
				testutils.RandomHash(),
				testCase.path,
				testutils.RandomInt(100),
				time.Now(),
			)

			if testCase.expectedError == "" {
				if err != nil {
					t.Fatalf("NewEntry() returned an unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing [%s]", testCase.expectedError)
			}
			if !strings.Contains(err.Error(), testCase.expectedError) {
				t.Fatalf("expected error to contain [%s], got [%s]", testCase.expectedError, err)
			}
		})
	}
}
