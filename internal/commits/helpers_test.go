package commits

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/objects/objectstest"
	"github.com/KostasZigo/gogit/internal/testutils"
	"github.com/KostasZigo/gogit/internal/worktree"
	"github.com/agiledragon/gomonkey/v2"
)

// checkoutFile describes one file stored in a checkout fixture snapshot.
type checkoutFile struct {
	content []byte
	mode    objects.FileMode
}

// checkoutFixture contains a repository with distinct current and target commits.
type checkoutFixture struct {
	repoPath       string
	headSnapshot   objects.TreeSnapshot
	targetSnapshot objects.TreeSnapshot
	headCommit     *objects.Commit
	targetBranch   string
}

// checkoutMetadataSnapshot captures metadata that checkout must not mutate on failure.
type checkoutMetadataSnapshot struct {
	head  []byte
	index []byte
	refs  map[string][]byte
}

// newCheckoutFixture creates two commits, checks out the first snapshot, and
// creates a target branch pointing to the second commit.
func newCheckoutFixture(t *testing.T, headFiles, targetFiles map[string]checkoutFile) *checkoutFixture {
	t.Helper()

	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)
	headSnapshot, headTreeHash := storeCheckoutSnapshot(t, store, headFiles)
	targetSnapshot, targetTreeHash := storeCheckoutSnapshot(t, store, targetFiles)
	headCommit := objectstest.CreateAndStoreCommit(t, store, headTreeHash, "", testutils.RandomString(10))
	targetCommit := objectstest.CreateAndStoreCommit(t, store, targetTreeHash, headCommit.Hash(), testutils.RandomString(11))
	targetBranch := testutils.RandomString(12)

	testutils.WriteRefFile(t, repoPath, constants.DefaultBranch, headCommit.Hash())
	applyStoredTreeSnapshot(t, repoPath, store, headTreeHash)
	testutils.WriteRefFile(t, repoPath, targetBranch, targetCommit.Hash())

	return &checkoutFixture{
		repoPath:       repoPath,
		headSnapshot:   headSnapshot,
		targetSnapshot: targetSnapshot,
		headCommit:     headCommit,
		targetBranch:   targetBranch,
	}
}

// storeCheckoutSnapshot stores every file blob and the resulting recursive tree.
func storeCheckoutSnapshot(t *testing.T, store *objects.ObjectStore, files map[string]checkoutFile) (objects.TreeSnapshot, string) {
	t.Helper()

	snapshot := make(objects.TreeSnapshot, len(files))
	for logicalPath, file := range files {
		blob := objectstest.CreateAndStoreBlob(t, store, file.content)
		snapshot[logicalPath] = objects.SnapshotEntry{Hash: blob.Hash(), Mode: file.mode}
	}

	treeHash, err := store.StoreTreeSnapshot(snapshot)
	if err != nil {
		t.Fatalf("failed to store checkout fixture snapshot: %v", err)
	}
	return snapshot, treeHash
}

// captureCheckoutMetadata reads HEAD, the index, and the requested branch refs.
func captureCheckoutMetadata(t *testing.T, repoPath string, branches ...string) checkoutMetadataSnapshot {
	t.Helper()

	refs := make(map[string][]byte, len(branches))
	for _, branch := range branches {
		refs[branch] = readRepositoryFile(t, branchRefPath(repoPath, branch))
	}

	return checkoutMetadataSnapshot{
		head:  readRepositoryFile(t, filepath.Join(repoPath, constants.Gogit, constants.Head)),
		index: readRepositoryFile(t, filepath.Join(repoPath, constants.Gogit, constants.Index)),
		refs:  refs,
	}
}

// assertCheckoutMetadataUnchanged verifies that HEAD, the index, and branch refs
// remain byte-for-byte equal to a previously captured state.
func assertCheckoutMetadataUnchanged(t *testing.T, repoPath string, expected checkoutMetadataSnapshot) {
	t.Helper()

	assertRepositoryFileContent(t, filepath.Join(repoPath, constants.Gogit, constants.Head), expected.head)
	assertCheckoutIndexAndRefsUnchanged(t, repoPath, expected)
}

// assertCheckoutIndexAndRefsUnchanged verifies that the index and branch refs
// remain byte-for-byte equal while allowing HEAD to change.
func assertCheckoutIndexAndRefsUnchanged(t *testing.T, repoPath string, expected checkoutMetadataSnapshot) {
	t.Helper()

	assertRepositoryFileContent(t, filepath.Join(repoPath, constants.Gogit, constants.Index), expected.index)
	for branch, content := range expected.refs {
		assertRepositoryFileContent(t, branchRefPath(repoPath, branch), content)
	}
}

// assertCheckoutRefsUnchanged verifies exact branch-ref contents while allowing
// HEAD, the index, and worktree files to change.
func assertCheckoutRefsUnchanged(t *testing.T, repoPath string, expected checkoutMetadataSnapshot) {
	t.Helper()

	for branch, content := range expected.refs {
		assertRepositoryFileContent(t, branchRefPath(repoPath, branch), content)
	}
}

// requirePreflightError verifies the typed checkout rejection and returns its state.
func requirePreflightError(t *testing.T, err error) worktree.State {
	t.Helper()

	if !errors.Is(err, worktree.ErrPreflight) {
		t.Fatalf("expected worktree preflight error, got [%v]", err)
	}

	var preflightError *worktree.PreflightError
	if !errors.As(err, &preflightError) {
		t.Fatalf("expected typed preflight error, got [%T]", err)
	}
	return preflightError.State
}

// assertIndexMatchesSnapshot verifies identity and filesystem metadata for every
// persisted index entry against the authoritative snapshot and current disk.
func assertIndexMatchesSnapshot(t *testing.T, repoPath string, expected objects.TreeSnapshot) {
	t.Helper()

	idx, err := index.NewManager(repoPath).Load()
	if err != nil {
		t.Fatalf("failed to load index: %v", err)
	}
	if idx.CountEntries() != len(expected) {
		t.Fatalf("expected [%d] index entries, got [%d]", len(expected), idx.CountEntries())
	}

	for logicalPath, snapshotEntry := range expected {
		idxEntry := idx.GetEntry(logicalPath)
		if idxEntry == nil {
			t.Fatalf("expected index entry for path [%s]", logicalPath)
		}
		expectedMode, err := index.FromObjectFileMode(snapshotEntry.Mode)
		if err != nil {
			t.Fatalf("failed to convert expected mode for path [%s]: %v", logicalPath, err)
		}
		if idxEntry.Hash() != snapshotEntry.Hash || idxEntry.Mode() != expectedMode {
			t.Fatalf("index identity mismatch for path [%s]", logicalPath)
		}

		localPath, err := filepath.Localize(logicalPath)
		if err != nil {
			t.Fatalf("failed to localize path [%s]: %v", logicalPath, err)
		}
		fileInfo, err := os.Stat(filepath.Join(repoPath, localPath))
		if err != nil {
			t.Fatalf("failed to stat indexed path [%s]: %v", logicalPath, err)
		}
		if idxEntry.FileSize() != fileInfo.Size() {
			t.Fatalf("expected index size [%d] for path [%s], got [%d]", fileInfo.Size(), logicalPath, idxEntry.FileSize())
		}
		if !idxEntry.LastModified().Equal(fileInfo.ModTime().Truncate(time.Second)) {
			t.Fatalf("expected index timestamp [%s] for path [%s], got [%s]", fileInfo.ModTime().Truncate(time.Second), logicalPath, idxEntry.LastModified())
		}
	}
}

// injectIndexSaveFailure replaces index persistence for the duration of one test.
func injectIndexSaveFailure(t *testing.T, saveErr error) {
	t.Helper()

	patches := gomonkey.ApplyMethod(&index.Manager{}, "Save", func(_ *index.Manager, _ *index.Index) error { return saveErr })
	t.Cleanup(patches.Reset)
}

// removeStoredObject deletes one object to make rollback restoration fail.
func removeStoredObject(t *testing.T, repoPath, hash string) {
	t.Helper()

	objectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, hash[:constants.HashDirPrefixLength], hash[constants.HashDirPrefixLength:])
	if err := os.Remove(objectPath); err != nil {
		t.Fatalf("failed to remove stored object [%s]: %v", hash, err)
	}
}

// readRepositoryFile returns a defensive copy of one repository file.
func readRepositoryFile(t *testing.T, filePath string) []byte {
	t.Helper()

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read repository file [%s]: %v", filePath, err)
	}
	return bytes.Clone(content)
}

// assertRepositoryFileContent verifies exact repository file contents.
func assertRepositoryFileContent(t *testing.T, filePath string, expected []byte) {
	t.Helper()

	actual := readRepositoryFile(t, filePath)
	if !bytes.Equal(actual, expected) {
		t.Fatalf("repository file content changed for [%s]", filePath)
	}
}

// branchRefPath returns the filesystem path for one branch ref.
func branchRefPath(repoPath, branch string) string {
	return filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, filepath.FromSlash(branch))
}

// applyStoredTreeSnapshot materializes a stored tree and persists its
// replacement index through the reusable worktree application API.
func applyStoredTreeSnapshot(t *testing.T, repoPath string, store *objects.ObjectStore, treeHash string) {
	t.Helper()

	targetSnapshot, err := store.ReadTreeSnapshot(treeHash)
	if err != nil {
		t.Fatalf("failed to read setup tree snapshot: %v", err)
	}

	worktreeService, err := worktree.NewService(repoPath)
	if err != nil {
		t.Fatalf("failed to create setup worktree service: %v", err)
	}

	if err := worktreeService.ApplySnapshot(store, objects.TreeSnapshot{}, targetSnapshot); err != nil {
		t.Fatalf("failed to apply setup tree snapshot: %v", err)
	}
}
