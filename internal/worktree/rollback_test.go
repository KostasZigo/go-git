package worktree

import (
	"bytes"
	"errors"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/objects/objectstest"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestRestoreIndexedFiles_RestoresContentDirectoriesAndModes verifies that
// recovery recreates missing files and parents, overwrites changed content,
// and reapplies regular and executable modes from the index.
func TestRestoreIndexedFiles_RestoresContentDirectoriesAndModes(t *testing.T) {
	// Arrange
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)
	idx := index.NewIndex()

	regularPath := testutils.RandomString(10)
	regularContent := testutils.RandomBytes(20)
	addRollbackIndexEntry(t, store, idx, regularPath, regularContent, objects.ModeRegularFile)

	nestedDir := testutils.RandomString(11)
	executablePath := path.Join(nestedDir, testutils.RandomString(12))
	executableContent := testutils.RandomBytes(21)
	addRollbackIndexEntry(t, store, idx, executablePath, executableContent, objects.ModeExecutable)

	overwrittenPath := testutils.RandomString(13)
	overwrittenContent := testutils.RandomBytes(22)
	addRollbackIndexEntry(t, store, idx, overwrittenPath, overwrittenContent, objects.ModeRegularFile)
	testutils.CreateTestFile(t, repoPath, overwrittenPath, testutils.RandomBytes(23))

	// Act
	if err := restoreIndexedFiles(repoPath, store, idx); err != nil {
		t.Fatalf("failed to restore indexed files: %v", err)
	}

	// Assert
	regularAbsPath := filepath.Join(repoPath, regularPath)
	executableAbsPath := filepath.Join(repoPath, filepath.FromSlash(executablePath))
	testutils.AssertFileContent(t, regularAbsPath, regularContent)
	testutils.AssertFileContent(t, executableAbsPath, executableContent)
	testutils.AssertFileContent(t, filepath.Join(repoPath, overwrittenPath), overwrittenContent)
	testutils.AssertDirExists(t, filepath.Join(repoPath, nestedDir))

	if runtime.GOOS != "windows" {
		regularInfo, err := os.Stat(regularAbsPath)
		if err != nil {
			t.Fatalf("failed to stat restored regular file: %v", err)
		}
		executableInfo, err := os.Stat(executableAbsPath)
		if err != nil {
			t.Fatalf("failed to stat restored executable file: %v", err)
		}
		regularPermissions, _ := objects.ModeRegularFile.ToOsFileMOde()
		executablePermissions, _ := objects.ModeExecutable.ToOsFileMOde()
		if regularInfo.Mode().Perm() != regularPermissions.Perm() {
			t.Fatalf("expected regular permissions [%o], got [%o]", regularPermissions.Perm(), regularInfo.Mode().Perm())
		}
		if executableInfo.Mode().Perm() != executablePermissions.Perm() {
			t.Fatalf("expected executable permissions [%o], got [%o]", executablePermissions.Perm(), executableInfo.Mode().Perm())
		}
	}
}

// TestRestoreIndexedFiles_ContinuesAfterMultipleFailures verifies that one
// restoration failure does not prevent later indexed entries from being
// attempted and that all failures retain their path context.
func TestRestoreIndexedFiles_ContinuesAfterMultipleFailures(t *testing.T) {
	// Arrange
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)
	idx := index.NewIndex()

	missingBlobPath := "a-" + testutils.RandomString(10)
	addIndexEntryWithContent(t, idx, objects.ModeRegularFile, testutils.RandomHash(), missingBlobPath, testutils.RandomBytes(20), time.Now())

	restoredPath := "b-" + testutils.RandomString(10)
	restoredContent := testutils.RandomBytes(21)
	addRollbackIndexEntry(t, store, idx, restoredPath, restoredContent, objects.ModeRegularFile)

	blockedParent := "c-" + testutils.RandomString(10)
	blockedPath := path.Join(blockedParent, testutils.RandomString(11))
	addRollbackIndexEntry(t, store, idx, blockedPath, testutils.RandomBytes(22), objects.ModeRegularFile)

	// create directory where the file would be restored
	testutils.CreateTestFile(t, repoPath, blockedParent, testutils.RandomBytes(23))

	// Act
	err := restoreIndexedFiles(repoPath, store, idx)

	// Assert
	if err == nil {
		t.Fatal("expected multiple indexed-file restoration failures")
	}
	if !strings.Contains(err.Error(), missingBlobPath) || !strings.Contains(err.Error(), blockedPath) {
		t.Fatalf("expected both failing paths in error, got [%v]", err)
	}
	testutils.AssertFileContent(t, filepath.Join(repoPath, restoredPath), restoredContent)
}

// TestRollbackSnapshotApplication_RestoresWorktreeAndPreservesIndex verifies
// complete recovery of changed and deleted indexed files, cleanup of target-only
// files, preservation of untracked content, and byte-for-byte index stability.
func TestRollbackSnapshotApplication_RestoresWorktreeAndPreservesIndex(t *testing.T) {
	// Arrange
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)
	idx := index.NewIndex()

	changedPath := testutils.RandomString(10)
	changedContent := testutils.RandomBytes(20)
	changedHash := addRollbackIndexEntry(t, store, idx, changedPath, changedContent, objects.ModeRegularFile)
	changedAbsPath := testutils.CreateTestFile(t, repoPath, changedPath, testutils.RandomBytes(21))

	deletedPath := testutils.RandomString(11)
	deletedContent := testutils.RandomBytes(22)
	deletedHash := addRollbackIndexEntry(t, store, idx, deletedPath, deletedContent, objects.ModeRegularFile)
	deletedAbsPath := filepath.Join(repoPath, deletedPath)

	targetOnlyPath := testutils.RandomString(12)
	targetOnlyAbsPath := testutils.CreateTestFile(t, repoPath, targetOnlyPath, testutils.RandomBytes(23))
	targetDir := testutils.RandomString(13)
	nestedTargetPath := path.Join(targetDir, testutils.RandomString(14))
	nestedTargetAbsPath := testutils.CreateTestFileWithDirs(t, repoPath, filepath.FromSlash(nestedTargetPath), testutils.RandomBytes(24))

	untrackedPath := testutils.RandomString(15)
	untrackedContent := testutils.RandomBytes(25)
	untrackedAbsPath := testutils.CreateTestFile(t, repoPath, untrackedPath, untrackedContent)

	saveIndex(t, repoPath, idx)
	persistedIndexBefore := readPersistedIndex(t, repoPath)
	target := objects.TreeSnapshot{
		changedPath:      {Hash: testutils.RandomHash(), Mode: objects.ModeRegularFile},
		targetOnlyPath:   {Hash: testutils.RandomHash(), Mode: objects.ModeRegularFile},
		nestedTargetPath: {Hash: testutils.RandomHash(), Mode: objects.ModeRegularFile},
	}

	// Act
	if err := rollbackSnapshotApplication(repoPath, store, idx, target); err != nil {
		t.Fatalf("failed to roll back snapshot application: %v", err)
	}

	// Assert
	testutils.AssertFileContent(t, changedAbsPath, changedContent)
	testutils.AssertFileContent(t, deletedAbsPath, deletedContent)
	testutils.AssertFileNotExists(t, targetOnlyAbsPath)
	testutils.AssertFileNotExists(t, nestedTargetAbsPath)
	testutils.AssertDirNotExists(t, filepath.Join(repoPath, targetDir))
	testutils.AssertFileContent(t, untrackedAbsPath, untrackedContent)
	if !bytes.Equal(persistedIndexBefore, readPersistedIndex(t, repoPath)) {
		t.Fatal("expected rollback to leave persisted index bytes unchanged")
	}
	assertIndexEntries(t, idx, map[string]testIdxEntry{
		changedPath: {hash: changedHash, mode: index.ModeRegularFile},
		deletedPath: {hash: deletedHash, mode: index.ModeRegularFile},
	})
}

// TestRollbackSnapshotApplication_CleanupFailureStillRestoresIndexedFiles
// verifies that indexed-file restoration proceeds after target cleanup fails.
func TestRollbackSnapshotApplication_CleanupFailureStillRestoresIndexedFiles(t *testing.T) {
	// Arrange
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)
	idx := index.NewIndex()

	restoredPath := testutils.RandomString(10)
	restoredContent := testutils.RandomBytes(20)
	addRollbackIndexEntry(t, store, idx, restoredPath, restoredContent, objects.ModeRegularFile)

	blockedTargetPath := testutils.RandomString(11)
	untrackedContent := testutils.RandomBytes(21)
	untrackedPath := testutils.CreateTestFileWithDirs(t, repoPath, filepath.Join(blockedTargetPath, testutils.RandomString(12)), untrackedContent)
	target := objects.TreeSnapshot{blockedTargetPath: {Hash: testutils.RandomHash(), Mode: objects.ModeRegularFile}}

	// Act
	err := rollbackSnapshotApplication(repoPath, store, idx, target)

	// Assert
	if !errors.Is(err, ErrRollback) {
		t.Fatalf("expected ErrRollback, got [%v]", err)
	}
	if !strings.Contains(err.Error(), "failed to remove target snapshot paths") {
		t.Fatalf("expected target cleanup context, got [%v]", err)
	}
	testutils.AssertDirExists(t, filepath.Join(repoPath, blockedTargetPath))
	testutils.AssertFileContent(t, untrackedPath, untrackedContent)
	testutils.AssertFileContent(t, filepath.Join(repoPath, restoredPath), restoredContent)
}

// TestRollbackSnapshotApplication_JoinsCleanupAndRestorationFailures verifies
// that failures from both rollback phases are retained under ErrRollback.
func TestRollbackSnapshotApplication_JoinsCleanupAndRestorationFailures(t *testing.T) {
	// Arrange
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)
	idx := index.NewIndex()

	missingBlobPath := testutils.RandomString(10)
	addIndexEntryWithContent(t, idx, objects.ModeRegularFile, testutils.RandomHash(), missingBlobPath, testutils.RandomBytes(20), time.Now())

	blockedTargetPath := testutils.RandomString(11)
	untrackedPath := testutils.CreateTestFileWithDirs(t, repoPath, filepath.Join(blockedTargetPath, testutils.RandomString(12)), testutils.RandomBytes(21))
	target := objects.TreeSnapshot{blockedTargetPath: {Hash: testutils.RandomHash(), Mode: objects.ModeRegularFile}}

	// Act
	err := rollbackSnapshotApplication(repoPath, store, idx, target)

	// Assert
	if !errors.Is(err, ErrRollback) {
		t.Fatalf("expected ErrRollback, got [%v]", err)
	}
	for _, expected := range []string{"failed to remove target snapshot paths", blockedTargetPath, "failed to restore indexed files", missingBlobPath} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("expected error to contain [%s], got [%v]", expected, err)
		}
	}
	testutils.AssertFileExists(t, untrackedPath)
}

// TestApplySnapshot_IndexSaveFailureRollsBackWorktree verifies that a failure
// after materialization restores the complete pre-application worktree while
// preserving untracked content and the persisted index.
func TestApplySnapshot_IndexSaveFailureRollsBackWorktree(t *testing.T) {
	// Arrange
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)
	idx := index.NewIndex()

	changedPath := testutils.RandomString(10)
	changedContent := testutils.RandomBytes(20)
	changedHash := addRollbackIndexEntry(t, store, idx, changedPath, changedContent, objects.ModeRegularFile)
	changedAbsPath := testutils.CreateTestFile(t, repoPath, changedPath, changedContent)

	deletedPath := testutils.RandomString(11)
	deletedContent := testutils.RandomBytes(21)
	deletedHash := addRollbackIndexEntry(t, store, idx, deletedPath, deletedContent, objects.ModeRegularFile)
	deletedAbsPath := testutils.CreateTestFile(t, repoPath, deletedPath, deletedContent)

	targetChangedContent := testutils.RandomBytes(22)
	targetChangedBlob := objectstest.CreateAndStoreBlob(t, store, targetChangedContent)
	targetDir := testutils.RandomString(12)
	targetNestedPath := path.Join(targetDir, testutils.RandomString(13))
	targetNestedContent := testutils.RandomBytes(23)
	targetNestedBlob := objectstest.CreateAndStoreBlob(t, store, targetNestedContent)
	targetNestedAbsPath := filepath.Join(repoPath, filepath.FromSlash(targetNestedPath))

	untrackedPath := testutils.RandomString(14)
	untrackedContent := testutils.RandomBytes(24)
	untrackedAbsPath := testutils.CreateTestFile(t, repoPath, untrackedPath, untrackedContent)

	saveIndex(t, repoPath, idx)
	persistedIndexBefore := readPersistedIndex(t, repoPath)
	original := objects.TreeSnapshot{
		changedPath: {Hash: changedHash, Mode: objects.ModeRegularFile},
		deletedPath: {Hash: deletedHash, Mode: objects.ModeRegularFile},
	}
	target := objects.TreeSnapshot{
		changedPath:      {Hash: targetChangedBlob.Hash(), Mode: objects.ModeRegularFile},
		targetNestedPath: {Hash: targetNestedBlob.Hash(), Mode: objects.ModeRegularFile},
	}

	// Act
	saveErr := errors.New("injected index save failure")
	err := applySnapshot(repoPath, store, idx, original, target, failingIndexSave(saveErr))

	// Assert
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected injected index save error, got [%v]", err)
	}
	if errors.Is(err, ErrRollback) {
		t.Fatalf("expected rollback to succeed, got [%v]", err)
	}
	testutils.AssertFileContent(t, changedAbsPath, changedContent)
	testutils.AssertFileContent(t, deletedAbsPath, deletedContent)
	testutils.AssertFileNotExists(t, targetNestedAbsPath)
	testutils.AssertDirNotExists(t, filepath.Join(repoPath, targetDir))
	testutils.AssertFileContent(t, untrackedAbsPath, untrackedContent)
	if !bytes.Equal(persistedIndexBefore, readPersistedIndex(t, repoPath)) {
		t.Fatal("expected failed application and rollback to preserve persisted index bytes")
	}
}

// TestApplySnapshot_IndexSaveAndRollbackFailuresAreJoined verifies that the
// returned error preserves both the application failure and ErrRollback when
// an indexed blob cannot be restored.
func TestApplySnapshot_IndexSaveAndRollbackFailuresAreJoined(t *testing.T) {
	// Arrange
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)
	idx := index.NewIndex()

	missingOriginalPath := testutils.RandomString(10)
	missingOriginalHash := testutils.RandomHash()
	addIndexEntryWithContent(t, idx, objects.ModeRegularFile, missingOriginalHash, missingOriginalPath, testutils.RandomBytes(20), time.Now())
	saveIndex(t, repoPath, idx)

	targetPath := testutils.RandomString(11)
	targetBlob := objectstest.CreateAndStoreBlob(t, store, testutils.RandomBytes(21))
	targetAbsPath := filepath.Join(repoPath, targetPath)
	original := objects.TreeSnapshot{missingOriginalPath: {Hash: missingOriginalHash, Mode: objects.ModeRegularFile}}
	target := objects.TreeSnapshot{targetPath: {Hash: targetBlob.Hash(), Mode: objects.ModeRegularFile}}

	// Act
	saveErr := errors.New("injected index save failure")
	err := applySnapshot(repoPath, store, idx, original, target, failingIndexSave(saveErr))

	// Assert
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected injected index save error, got [%v]", err)
	}
	if !errors.Is(err, ErrRollback) {
		t.Fatalf("expected joined ErrRollback, got [%v]", err)
	}
	if !strings.Contains(err.Error(), missingOriginalPath) {
		t.Fatalf("expected failed restoration path in error, got [%v]", err)
	}
	testutils.AssertFileNotExists(t, targetAbsPath)
}
