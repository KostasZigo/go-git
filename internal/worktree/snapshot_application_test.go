package worktree

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/index/indextest"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/objects/objectstest"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestApplySnapshot_MixedFilesUpdatesWorktreeAndIndex verifies that applying a
// target snapshot retains unchanged files, rewrites changed files, removes
// deleted files, creates nested files, and persists an index matching the
// complete target snapshot.
func TestApplySnapshot_MixedFilesUpdatesWorktreeAndIndex(t *testing.T) {
	// Arrange
	repoPath := testutils.SetupTestRepoWithInit(t)
	idx := index.NewIndex()
	idxManager := index.NewManager(repoPath)
	store := objects.NewObjectStore(repoPath)
	fileMode := objects.ModeRegularFile

	// Create retained file blob/indexEntry/file
	retainedFileContent := testutils.RandomBytes(21)
	retainedFileLogicalPath := testutils.RandomString(10)
	retainedBlob := objectstest.CreateAndStoreBlob(t, store, retainedFileContent)
	retainedFileHash := retainedBlob.Hash()
	retainedFilePath := indextest.CreateTrackedFileContent(t, repoPath, repoPath, retainedFileLogicalPath, retainedFileContent, idx)
	retainedFileInfoBefore, err := os.Stat(retainedFilePath)
	if err != nil {
		t.Fatalf("failed to stat retained file before materialization: %v", err)
	}

	// Create existing file blob/indexEntry/file
	existingFileLogicalPath := testutils.RandomString(11)
	existingFileContent := testutils.RandomBytes(22)
	changedFileContent := testutils.RandomBytes(21)
	existingBlob := objectstest.CreateAndStoreBlob(t, store, existingFileContent)
	changedBlob := objectstest.CreateAndStoreBlob(t, store, changedFileContent)
	existingFileHash := existingBlob.Hash()
	changedFileHash := changedBlob.Hash()
	existingFilePath := indextest.CreateTrackedFileContent(t, repoPath, repoPath, existingFileLogicalPath, existingFileContent, idx)

	// Create deleted file blob/indexEntry/file
	deletedFileLogicalPath := testutils.RandomString(21)
	deletedFileContent := testutils.RandomBytes(23)
	deletedBlob := objectstest.CreateAndStoreBlob(t, store, deletedFileContent)
	deletedFileHash := deletedBlob.Hash()
	deletedFilePath := indextest.CreateTrackedFileContent(t, repoPath, repoPath, deletedFileLogicalPath, deletedFileContent, idx)

	saveIndex(t, repoPath, idx)

	original := objects.TreeSnapshot{
		retainedFileLogicalPath: {Hash: retainedFileHash, Mode: fileMode},
		deletedFileLogicalPath:  {Hash: deletedFileHash, Mode: fileMode},
		existingFileLogicalPath: {Hash: existingFileHash, Mode: fileMode},
	}

	// Create nested file blob
	dir := testutils.RandomString(12)
	nestedFileLogicalPath := path.Join(dir, testutils.RandomString(13))
	nestedFileContent := testutils.RandomBytes(24)
	nestedBlob := objectstest.CreateAndStoreBlob(t, store, nestedFileContent)
	nestedFileHash := nestedBlob.Hash()
	nestedFilePath := filepath.Join(repoPath, filepath.FromSlash(nestedFileLogicalPath))

	target := objects.TreeSnapshot{
		retainedFileLogicalPath: {
			Hash: retainedFileHash,
			Mode: fileMode,
		},
		existingFileLogicalPath: {
			Hash: changedFileHash,
			Mode: fileMode,
		},
		nestedFileLogicalPath: {
			Hash: nestedFileHash,
			Mode: fileMode,
		},
	}

	// Act
	if err := ApplySnapshot(repoPath, store, idx, original, target); err != nil {
		t.Fatalf("unexpected error during snapshot application: %v", err)
	}

	// Assert
	testutils.AssertFileExists(t, retainedFilePath)
	testutils.AssertFileContent(t, retainedFilePath, retainedFileContent)
	retainedFileInfoAfter, err := os.Stat(retainedFilePath)
	if err != nil {
		t.Fatalf("failed to stat retained file after materialization: %v", err)
	}
	if !retainedFileInfoAfter.ModTime().Equal(retainedFileInfoBefore.ModTime()) {
		t.Fatal("expected retained file modification time to remain unchanged")
	}

	testutils.AssertFileExists(t, existingFilePath)
	testutils.AssertFileContent(t, existingFilePath, changedFileContent)

	testutils.AssertFileNotExists(t, deletedFilePath)

	testutils.AssertDirExists(t, filepath.Join(repoPath, dir))
	testutils.AssertFileExists(t, nestedFilePath)
	testutils.AssertFileContent(t, nestedFilePath, nestedFileContent)

	loadedIndex, err := idxManager.Load()
	if err != nil {
		t.Fatalf("failed to load updated index: %v", err)
	}

	indexMode, _ := index.FromObjectFileMode(fileMode)
	expectedIndexEntries := map[string]testIdxEntry{
		retainedFileLogicalPath: {hash: retainedFileHash, mode: indexMode},
		nestedFileLogicalPath:   {hash: nestedFileHash, mode: indexMode},
		existingFileLogicalPath: {hash: changedFileHash, mode: indexMode},
	}
	assertIndexEntries(t, loadedIndex, expectedIndexEntries)
}

// TestApplySnapshot_PlanningFailureLeavesWorktreeAndIndexUnchanged verifies
// that a missing target blob fails before filesystem mutation and leaves the
// previously persisted index intact.
func TestApplySnapshot_PlanningFailureLeavesWorktreeAndIndexUnchanged(t *testing.T) {
	// Arrange
	repoPath := testutils.SetupTestRepoWithInit(t)
	idx := index.NewIndex()
	store := objects.NewObjectStore(repoPath)

	existingContent := testutils.RandomBytes(20)
	existingPath := testutils.RandomString(10)
	existingBlob := objectstest.CreateAndStoreBlob(t, store, existingContent)
	existingFilePath := indextest.CreateTrackedFileContent(t, repoPath, repoPath, existingPath, existingContent, idx)
	saveIndex(t, repoPath, idx)

	original := objects.TreeSnapshot{
		existingPath: {Hash: existingBlob.Hash(), Mode: objects.ModeRegularFile},
	}
	target := objects.TreeSnapshot{
		testutils.RandomString(11): {Hash: testutils.RandomHash(), Mode: objects.ModeRegularFile},
	}

	// Act
	err := ApplySnapshot(repoPath, store, idx, original, target)

	// Assert
	if err == nil {
		t.Fatal("expected snapshot application to fail when a target blob is missing")
	}

	expectedErrorMessage := "failed to build application plan"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], got [%v]", expectedErrorMessage, err)
	}
	testutils.AssertFileExists(t, existingFilePath)
	testutils.AssertFileContent(t, existingFilePath, existingContent)

	loadedIndex, err := index.NewManager(repoPath).Load()
	if err != nil {
		t.Fatalf("failed to load index after planning failure: %v", err)
	}
	assertIndexEntries(t, loadedIndex, map[string]testIdxEntry{existingPath: {hash: existingBlob.Hash(), mode: index.ModeRegularFile}})
}

// TestApplySnapshot_RemovalFailurePreservesUntrackedContentAndIndex verifies
// that removing a tracked path fails when it has become a non-empty directory,
// preserving its untracked descendant and the previously persisted index.
func TestApplySnapshot_RemovalFailurePreservesUntrackedContentAndIndex(t *testing.T) {
	// Arrange
	repoPath := testutils.SetupTestRepoWithInit(t)
	idx := index.NewIndex()
	store := objects.NewObjectStore(repoPath)

	trackedPath := testutils.RandomString(10)
	trackedContent := testutils.RandomBytes(20)
	trackedBlob := objectstest.CreateAndStoreBlob(t, store, trackedContent)
	trackedFilePath := indextest.CreateTrackedFileContent(t, repoPath, repoPath, trackedPath, trackedContent, idx)
	saveIndex(t, repoPath, idx)

	if err := os.Remove(trackedFilePath); err != nil {
		t.Fatalf("failed to replace tracked file with directory: %v", err)
	}
	untrackedPath := testutils.CreateTestFileWithDirs(
		t,
		repoPath,
		filepath.Join(trackedPath, testutils.RandomString(11)),
		testutils.RandomBytes(21),
	)

	original := objects.TreeSnapshot{
		trackedPath: {Hash: trackedBlob.Hash(), Mode: objects.ModeRegularFile},
	}

	// Act
	err := ApplySnapshot(repoPath, store, idx, original, objects.TreeSnapshot{})

	// Assert
	if err == nil {
		t.Fatal("expected snapshot application to fail when removing a non-empty directory")
	}

	expectedErrorMessage := "failed to remove obsolete worktree paths"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error to contain [%s], got [%v]", expectedErrorMessage, err)
	}
	testutils.AssertDirExists(t, trackedFilePath)
	testutils.AssertFileExists(t, untrackedPath)

	loadedIndex, loadErr := index.NewManager(repoPath).Load()
	if loadErr != nil {
		t.Fatalf("failed to load index after removal failure: %v", loadErr)
	}
	assertIndexEntries(t, loadedIndex, map[string]testIdxEntry{
		trackedPath: {hash: trackedBlob.Hash(), mode: index.ModeRegularFile},
	})
}

// TestApplySnapshot_EmptyTargetRemovesTrackedFilesAndPersistsEmptyIndex
// verifies that applying an empty target removes all tracked files and their
// empty parent directories while preserving unrelated untracked content.
func TestApplySnapshot_EmptyTargetRemovesTrackedFilesAndPersistsEmptyIndex(t *testing.T) {
	// Arrange
	repoPath := testutils.SetupTestRepoWithInit(t)
	idx := index.NewIndex()
	store := objects.NewObjectStore(repoPath)

	rootPath := testutils.RandomString(10)
	rootContent := testutils.RandomBytes(20)
	rootBlob := objectstest.CreateAndStoreBlob(t, store, rootContent)
	rootFilePath := indextest.CreateTrackedFileContent(t, repoPath, repoPath, rootPath, rootContent, idx)

	nestedDir := testutils.RandomString(11)
	nestedName := testutils.RandomString(12)
	nestedLogicalPath := path.Join(nestedDir, nestedName)
	nestedContent := testutils.RandomBytes(21)
	nestedBlob := objectstest.CreateAndStoreBlob(t, store, nestedContent)
	nestedFilePath := indextest.CreateTrackedFileContent(t, repoPath, filepath.Join(repoPath, nestedDir), nestedName, nestedContent, idx)

	untrackedContent := testutils.RandomBytes(22)
	untrackedFilePath := testutils.CreateTestFile(t, repoPath, testutils.RandomString(13), untrackedContent)
	saveIndex(t, repoPath, idx)

	original := objects.TreeSnapshot{
		rootPath:          {Hash: rootBlob.Hash(), Mode: objects.ModeRegularFile},
		nestedLogicalPath: {Hash: nestedBlob.Hash(), Mode: objects.ModeRegularFile},
	}

	// Act
	if err := ApplySnapshot(repoPath, store, idx, original, objects.TreeSnapshot{}); err != nil {
		t.Fatalf("failed to apply empty target snapshot: %v", err)
	}

	// Assert
	testutils.AssertFileNotExists(t, rootFilePath)
	testutils.AssertFileNotExists(t, nestedFilePath)
	testutils.AssertDirNotExists(t, filepath.Join(repoPath, nestedDir))
	testutils.AssertFileExists(t, untrackedFilePath)
	testutils.AssertFileContent(t, untrackedFilePath, untrackedContent)

	loadedIndex, err := index.NewManager(repoPath).Load()
	if err != nil {
		t.Fatalf("failed to load index after applying empty target: %v", err)
	}
	assertIndexEntries(t, loadedIndex, map[string]testIdxEntry{})
}

// TestApplySnapshot_EmptyDirectoryAtTargetPathIsReplaced verifies that an
// empty directory occupying a target file path is removed and replaced by the
// target file, with the replacement recorded in the persisted index.
func TestApplySnapshot_EmptyDirectoryAtTargetPathIsReplaced(t *testing.T) {
	// Arrange
	repoPath := testutils.SetupTestRepoWithInit(t)
	idx := index.NewIndex()
	store := objects.NewObjectStore(repoPath)

	targetPath := testutils.RandomString(10)
	targetAbsPath := filepath.Join(repoPath, targetPath)
	if err := os.Mkdir(targetAbsPath, constants.DirPerms); err != nil {
		t.Fatalf("failed to create empty directory at target path: %v", err)
	}

	targetContent := testutils.RandomBytes(20)
	targetBlob := objectstest.CreateAndStoreBlob(t, store, targetContent)
	target := objects.TreeSnapshot{
		targetPath: {Hash: targetBlob.Hash(), Mode: objects.ModeRegularFile},
	}
	saveIndex(t, repoPath, idx)

	// Act
	if err := ApplySnapshot(repoPath, store, idx, objects.TreeSnapshot{}, target); err != nil {
		t.Fatalf("failed to replace empty directory with target file: %v", err)
	}

	// Assert
	fileInfo, err := os.Stat(targetAbsPath)
	if err != nil {
		t.Fatalf("failed to stat materialized target file: %v", err)
	}
	if !fileInfo.Mode().IsRegular() {
		t.Fatalf("expected target path to be a regular file, got mode [%s]", fileInfo.Mode())
	}
	testutils.AssertFileContent(t, targetAbsPath, targetContent)

	loadedIndex, err := index.NewManager(repoPath).Load()
	if err != nil {
		t.Fatalf("failed to load index after replacing empty directory: %v", err)
	}
	assertIndexEntries(t, loadedIndex, map[string]testIdxEntry{
		targetPath: {hash: targetBlob.Hash(), mode: index.ModeRegularFile},
	})
}
