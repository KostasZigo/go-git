package indextestutils

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/testutils"
	"github.com/KostasZigo/gogit/internal/utils"
)

// CreateTrackedFile creates a file with random content on disk inside the given
// directory, computes its blob hash, and registers a corresponding index entry.
// Delegates to CreateTrackedFileContent
func CreateTrackedFile(t *testing.T, repoPath, dir, fileName string, idx *index.Index) string {
	return CreateTrackedFileContent(t, repoPath, dir, fileName, []byte(testutils.RandomString(100)), idx)
}

// CreateTrackedFileContent creates a file with the specified content on disk
// inside the given directory and registers a corresponding index entry in the
// provided Index. The entry stores a forward-slash relative path from repoPath,
// the blob hash of content, and the actual file size. Creates parent directories
// if needed. Returns the absolute path of the created file.
func CreateTrackedFileContent(t *testing.T, repoPath, dir, fileName string, content []byte, idx *index.Index) string {
	t.Helper()

	if err := os.MkdirAll(dir, constants.DirPerms); err != nil {
		t.Fatalf("failed to create directory %s: %v", dir, err)
	}

	absPath := testutils.CreateTestFile(t, dir, fileName, content)

	relPath, err := filepath.Rel(repoPath, absPath)
	if err != nil {
		t.Fatalf("failed to compute relative path for %s: %v", absPath, err)
	}

	hash, _ := utils.ComputeHash(content, utils.BlobObjectType)
	entry, err := index.NewEntry(index.ModeRegularFile, hash, filepath.ToSlash(relPath), int64(len(content)), time.Now())
	if err != nil {
		t.Fatalf("failed to create index entry for %s: %v", relPath, err)
	}

	if err := idx.AddEntry(entry); err != nil {
		t.Fatalf("failed to add index entry for %s: %v", relPath, err)
	}

	return absPath
}
