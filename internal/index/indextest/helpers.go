// Package indextest provides shared helpers for constructing index
// entries and temporary repository structures in index-related tests.
package indextest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/hasher"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// CreateTrackedFile creates a file with random content on disk inside the given
// directory, computes its blob hash, and registers a corresponding index entry.
// Delegates to CreateTrackedFileContent
func CreateTrackedFile(t *testing.T, repoPath, dir, fileName string, idx *index.Index) string {
	t.Helper()
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

	hash, _ := hasher.ComputeHash(content, hasher.Blob)
	entry, err := index.NewEntry(index.ModeRegularFile, hash, filepath.ToSlash(relPath), int64(len(content)), time.Now())
	if err != nil {
		t.Fatalf("failed to create index entry for %s: %v", relPath, err)
	}

	if err := idx.AddEntry(entry); err != nil {
		t.Fatalf("failed to add index entry for %s: %v", relPath, err)
	}

	return absPath
}

// AssertIndexEntryPaths loads the index from disk and verifies that the entry count
// matches expectedCount and that every path in expectedPaths appears in the index.
func AssertIndexEntryPaths(t *testing.T, repoPath string, expectedCount int, expectedPaths []string) {
	t.Helper()

	idxManager := index.NewManager(repoPath)
	idx, err := idxManager.Load()
	if err != nil {
		t.Fatalf("failed to load index: %v", err)
	}

	entries := idx.GetEntryList()
	if len(entries) != expectedCount {
		t.Fatalf("expected index to have %d entries, got %d", expectedCount, len(entries))
	}

	actualPaths := make([]string, len(entries))
	for i, e := range entries {
		actualPaths[i] = e.Path()
	}

	for _, expected := range expectedPaths {
		if !slices.Contains(actualPaths, expected) {
			t.Fatalf("expected index entry path [%s] to exist in %v", expected, actualPaths)
		}
	}
}
