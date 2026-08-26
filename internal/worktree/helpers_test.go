package worktree

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/objects/objectstest"
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

// newWorktreeService loads the persisted index into an operation-scoped service.
func newWorktreeService(t *testing.T, repoPath string) *Service {
	t.Helper()

	service, err := NewService(repoPath)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}
	return service
}

// addRollbackIndexEntry stores content as a blob and adds its recovery metadata
// to idx without creating the corresponding worktree file.
func addRollbackIndexEntry(t *testing.T, store *objects.ObjectStore, idx *index.Index, logicalPath string, content []byte, mode objects.FileMode) string {
	t.Helper()

	blob := objectstest.CreateAndStoreBlob(t, store, content)
	addIndexEntryWithContent(t, idx, mode, blob.Hash(), logicalPath, content, time.Now().Truncate(time.Second))
	return blob.Hash()
}

// readPersistedIndex returns the exact serialized index bytes.
func readPersistedIndex(t *testing.T, repoPath string) []byte {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(repoPath, constants.Gogit, constants.Index))
	if err != nil {
		t.Fatalf("failed to read persisted index: %v", err)
	}
	return content
}

// failingIndexSave returns an index-save operation that always fails with err.
func failingIndexSave(err error) saveIndexFunc {
	return func(*index.Index) error { return err }
}

// createMaterializationIndexEntry writes one disk file and returns matching
// index metadata, with the supplied hash and index mode.
func createMaterializationIndexEntry(
	t *testing.T,
	repoPath string,
	targetPath string,
	content []byte,
	hash string,
	mode index.FileMode,
	permissions os.FileMode,
) *index.Entry {
	t.Helper()

	absolutePath := filepath.Join(repoPath, targetPath)
	if err := os.WriteFile(absolutePath, content, permissions); err != nil {
		t.Fatalf("failed to write target file: %v", err)
	}
	fileInfo, err := os.Stat(absolutePath)
	if err != nil {
		t.Fatalf("failed to stat target file: %v", err)
	}

	entry, err := index.NewEntry(
		mode,
		hash,
		targetPath,
		fileInfo.Size(),
		fileInfo.ModTime().Truncate(time.Second),
	)
	if err != nil {
		t.Fatalf("failed to create index entry: %v", err)
	}
	return entry
}

// assertPlannedFileLists asserts that the expected plannedFileList is equal in order
// and content to the actual one.
func assertPlannedFileLists(t *testing.T, expected, actual []plannedFile) {
	t.Helper()

	if len(expected) != len(actual) {
		t.Fatalf("expected [%d] files, got [%d]", len(expected), len(actual))
	}

	for i := range expected {
		assertPlannedFiles(t, expected[i], actual[i])
	}
}

// assertPlannedFiles asserts that two planned files share the same content.
func assertPlannedFiles(t *testing.T, expected, actual plannedFile) {
	t.Helper()
	if expected.path != actual.path {
		t.Fatalf("expected planned file path [%s], got [%s]", expected.path, actual.path)
	}
	if expected.hash != actual.hash {
		t.Fatalf("expected blob hash [%s], got [%s]", expected.hash, actual.hash)
	}
	if expected.mode != actual.mode {
		t.Fatalf("expected mode [%s], got [%s]", expected.mode, actual.mode)
	}
	if expected.permissions != actual.permissions {
		t.Fatalf("expected permissions [%o], got [%o]", expected.permissions, actual.permissions)
	}
	if !bytes.Equal(expected.content, actual.content) {
		t.Fatal("expected content does not match actual content")
	}
	if expected.writeRequired != actual.writeRequired {
		t.Fatalf("expected writeRequired [%t], got [%t]", expected.writeRequired, actual.writeRequired)
	}
}

// testIdxEntry contains the target object identity asserted for one rebuilt
// index entry.
type testIdxEntry struct {
	hash string
	mode index.FileMode
}

// repositoryReferencesSnapshot captures HEAD and selected branch refs exactly.
type repositoryReferencesSnapshot struct {
	head []byte
	refs map[string][]byte
}

// assertIndexEntries verifies that idx contains exactly the expected paths,
// hashes, and platform-independent index modes.
func assertIndexEntries(t *testing.T, idx *index.Index, expected map[string]testIdxEntry) {
	t.Helper()

	if idx.CountEntries() != len(expected) {
		t.Fatalf("expected [%d] index entries, got [%d]", len(expected), idx.CountEntries())
	}

	for _, idxEntry := range idx.GetEntryList() {
		expectedEntry, exists := expected[idxEntry.Path()]
		if !exists {
			t.Fatalf("unexpected index entry with path [%s]", idxEntry.Path())
		}
		if expectedEntry.mode != idxEntry.Mode() {
			t.Fatalf("expected entry mode to be [%o], got [%o]", expectedEntry.mode, idxEntry.Mode())
		}
		if expectedEntry.hash != idxEntry.Hash() {
			t.Fatalf("expected entry hash to be [%s], got [%s]", expectedEntry.hash, idxEntry.Hash())
		}
	}
}

// assertIndexMetadataMatchesDisk verifies that every index entry records the
// exact size and second-precision modification time of its worktree file.
func assertIndexMetadataMatchesDisk(t *testing.T, repoPath string, idx *index.Index) {
	t.Helper()

	for _, idxEntry := range idx.GetEntryList() {
		localPath, err := filepath.Localize(idxEntry.Path())
		if err != nil {
			t.Fatalf("failed to localize indexed path [%s]: %v", idxEntry.Path(), err)
		}
		fileInfo, err := os.Stat(filepath.Join(repoPath, localPath))
		if err != nil {
			t.Fatalf("failed to stat indexed path [%s]: %v", idxEntry.Path(), err)
		}
		if idxEntry.FileSize() != fileInfo.Size() {
			t.Fatalf("expected index size [%d] for path [%s], got [%d]", fileInfo.Size(), idxEntry.Path(), idxEntry.FileSize())
		}
		expectedModTime := fileInfo.ModTime().Truncate(time.Second)
		if !idxEntry.LastModified().Equal(expectedModTime) {
			t.Fatalf("expected index timestamp [%s] for path [%s], got [%s]", expectedModTime, idxEntry.Path(), idxEntry.LastModified())
		}
	}
}

// captureRepositoryReferences reads HEAD and selected branch refs exactly.
func captureRepositoryReferences(t *testing.T, repoPath string, branches ...string) repositoryReferencesSnapshot {
	t.Helper()

	headPath := filepath.Join(repoPath, constants.Gogit, constants.Head)
	head, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("failed to read HEAD: %v", err)
	}
	refs := make(map[string][]byte, len(branches))
	for _, branch := range branches {
		refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, filepath.FromSlash(branch))
		refs[branch], err = os.ReadFile(refPath)
		if err != nil {
			t.Fatalf("failed to read branch ref [%s]: %v", branch, err)
		}
	}
	return repositoryReferencesSnapshot{head: head, refs: refs}
}

// assertRepositoryReferencesUnchanged verifies exact HEAD and branch-ref bytes.
func assertRepositoryReferencesUnchanged(t *testing.T, repoPath string, expected repositoryReferencesSnapshot) {
	t.Helper()

	head, err := os.ReadFile(filepath.Join(repoPath, constants.Gogit, constants.Head))
	if err != nil {
		t.Fatalf("failed to read HEAD: %v", err)
	}
	if !bytes.Equal(head, expected.head) {
		t.Fatal("expected worktree operation to preserve HEAD")
	}
	for branch, expectedContent := range expected.refs {
		refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, filepath.FromSlash(branch))
		actualContent, err := os.ReadFile(refPath)
		if err != nil {
			t.Fatalf("failed to read branch ref [%s]: %v", branch, err)
		}
		if !bytes.Equal(actualContent, expectedContent) {
			t.Fatalf("expected worktree operation to preserve branch ref [%s]", branch)
		}
	}
}
