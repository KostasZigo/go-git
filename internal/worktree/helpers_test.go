package worktree

import (
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// addIndexEntryWithContent creates and adds one index entry using the supplied tree mode and file content.
func addIndexEntryWithContent(t *testing.T, idx *index.Index, fileMode objects.FileMode, hash, path string, fileContent []byte, modTime time.Time) {
	t.Helper()

	mode, err := index.FromObjectFileMode(fileMode)
	if err != nil {
		t.Fatalf("failed to convert file mode: %v", err)
	}

	entry, err := index.NewEntry(mode, hash, path, int64(len(fileContent)), modTime)
	if err != nil {
		t.Fatalf("failed to create new index entry: %v", err)
	}
	idx.AddEntry(entry)
}

// addIndexEntry creates and adds one index entry using the supplied tree mode.
func addIndexEntry(t *testing.T, idx *index.Index, fileMode objects.FileMode, hash, path string, modTime time.Time) {
	t.Helper()

	addIndexEntryWithContent(t, idx, fileMode, hash, path, testutils.RandomBytes(100), modTime)
}

// saveIndex persists idx as the repository staging index.
func saveIndex(t *testing.T, repoPath string, idx *index.Index) {
	t.Helper()

	idxManager := index.NewManager(repoPath)
	if err := idxManager.Save(idx); err != nil {
		t.Fatalf("failed to save index: %v", err)
	}
}
