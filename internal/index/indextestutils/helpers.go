package indextestutils

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/testutils"
)

// CreateTrackedFile creates a file on disk inside the given directory and registers
// a corresponding index entry in the provided Index. The entry's path is stored as
// a relative path from repoPath (using forward slashes), matching how the real
// index tracks files. Creates the parent directory if it does not already exist.
// Returns the absolute path of the created file.
func CreateTrackedFile(t *testing.T, repoPath, dir, fileName string, idx *index.Index) string {
	t.Helper()

	if err := os.MkdirAll(dir, constants.DirPerms); err != nil {
		t.Fatalf("failed to create directory %s: %v", dir, err)
	}

	absPath := testutils.CreateTestFile(t, dir, fileName, []byte(testutils.RandomString(10)))

	relPath, err := filepath.Rel(repoPath, absPath)
	if err != nil {
		t.Fatalf("failed to compute relative path for %s: %v", absPath, err)
	}

	entry, err := index.NewEntry(index.ModeRegularFile, testutils.RandomHash(), filepath.ToSlash(relPath), testutils.RandomInt(100), time.Now())
	if err != nil {
		t.Fatalf("failed to create index entry for %s: %v", relPath, err)
	}

	if err := idx.AddEntry(entry); err != nil {
		t.Fatalf("failed to add index entry for %s: %v", relPath, err)
	}

	return absPath
}
