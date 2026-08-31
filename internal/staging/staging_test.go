package staging

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
	"github.com/agiledragon/gomonkey/v2"
)

// TestOrchestrateAddExecution_SingleFile verifies that staging a single file
// creates a blob object and adds an entry to the index.
func TestOrchestrateAddExecution_SingleFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	testFileName := testutils.RandomString(10)
	testutils.CreateTestFile(t, repoPath, testFileName, []byte(testutils.RandomString(500)))

	addedFiles, _, err := OrchestrateAddExecution(repoPath, []string{testFileName})
	if err != nil {
		t.Fatalf("OrchestrateAddExecution failed: %v", err)
	}

	if len(addedFiles) != 1 {
		t.Fatalf("expected 1 added file, got %d", len(addedFiles))
	}
	if addedFiles[0] != testFileName {
		t.Errorf("expected added file %q, got %q", testFileName, addedFiles[0])
	}

	idx := loadIndex(t, repoPath)
	entryList := idx.GetEntryList()
	if len(entryList) != 1 {
		t.Fatalf("expected 1 index entry, got %d", len(entryList))
	}
	if entryList[0].Path() != testFileName {
		t.Errorf("expected index entry path %q, got %q", testFileName, entryList[0].Path())
	}
}

// TestOrchestrateAddExecution_MultipleFiles verifies staging multiple files
// produces sorted index entries and returns all staged paths.
func TestOrchestrateAddExecution_MultipleFiles(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	testFileNames := []string{
		testutils.RandomString(10),
		testutils.RandomString(10),
		testutils.RandomString(10),
	}
	for _, name := range testFileNames {
		testutils.CreateTestFile(t, repoPath, name, []byte(testutils.RandomString(100)))
	}

	addedFiles, _, err := OrchestrateAddExecution(repoPath, testFileNames)
	if err != nil {
		t.Fatalf("OrchestrateAddExecution failed: %v", err)
	}

	expectedSorted := slices.Clone(testFileNames)
	slices.Sort(expectedSorted)

	if len(addedFiles) != len(expectedSorted) {
		t.Fatalf("expected %d added files, got %d", len(expectedSorted), len(addedFiles))
	}
	for i, expected := range expectedSorted {
		if addedFiles[i] != expected {
			t.Errorf("addedFiles[%d] = %q, want %q", i, addedFiles[i], expected)
		}
	}

	idx := loadIndex(t, repoPath)
	entryList := idx.GetEntryList()
	if len(entryList) != len(expectedSorted) {
		t.Fatalf("expected %d index entries, got %d", len(expectedSorted), len(entryList))
	}
	for i, entry := range entryList {
		if entry.Path() != expectedSorted[i] {
			t.Errorf("index entry[%d] path = %q, want %q", i, entry.Path(), expectedSorted[i])
		}
	}
}

// TestOrchestrateAddExecution_FileNotFound verifies that a missing file
// produces an appropriate error.
func TestOrchestrateAddExecution_FileNotFound(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	nonExistentFile := testutils.RandomString(10)

	_, _, err := OrchestrateAddExecution(repoPath, []string{nonExistentFile})
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

// TestOrchestrateAddExecution_UpdateExistingFile verifies that re-staging a
// modified file updates the index entry hash and file size.
func TestOrchestrateAddExecution_UpdateExistingFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	testFileName := testutils.RandomString(10)
	testutils.CreateTestFile(t, repoPath, testFileName, []byte(testutils.RandomString(500)))

	// Stage original
	if _, _, err := OrchestrateAddExecution(repoPath, []string{testFileName}); err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	idx := loadIndex(t, repoPath)
	originalHash := idx.GetEntryList()[0].Hash()
	originalSize := idx.GetEntryList()[0].FileSize()

	// Modify file with different content length
	testutils.CreateTestFile(t, repoPath, testFileName, []byte(testutils.RandomString(1000)))

	// Stage modified
	addedFiles, _, err := OrchestrateAddExecution(repoPath, []string{testFileName})
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}
	if len(addedFiles) != 1 {
		t.Fatalf("expected modified file to be re-staged, got %d added files", len(addedFiles))
	}

	idx = loadIndex(t, repoPath)
	modifiedHash := idx.GetEntryList()[0].Hash()
	modifiedSize := idx.GetEntryList()[0].FileSize()

	if originalHash == modifiedHash {
		t.Error("expected hash to change after file modification")
	}
	if originalSize == modifiedSize {
		t.Error("expected file size to change after file modification")
	}
}

// TestOrchestrateAddExecution_AddAll verifies that passing "." stages all
// trackable files in the repository recursively.
func TestOrchestrateAddExecution_AddAll(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	// Create files at root and nested directory
	subDir := testutils.RandomString(5)
	if err := os.MkdirAll(filepath.Join(repoPath, subDir), constants.DirPerms); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	files := map[string][]byte{
		testutils.RandomString(10):                        []byte(testutils.RandomString(100)),
		testutils.RandomString(10):                        []byte(testutils.RandomString(100)),
		filepath.Join(subDir, testutils.RandomString(10)): []byte(testutils.RandomString(100)),
	}

	for name, content := range files {
		testutils.CreateTestFile(t, repoPath, name, content)
	}

	addedFiles, _, err := OrchestrateAddExecution(repoPath, []string{"."})
	if err != nil {
		t.Fatalf("OrchestrateAddExecution with '.' failed: %v", err)
	}

	if len(addedFiles) != len(files) {
		t.Fatalf("expected %d added files, got %d", len(files), len(addedFiles))
	}

	// Verify index contains all files
	idx := loadIndex(t, repoPath)
	if idx.CountEntries() != len(files) {
		t.Fatalf("expected %d index entries, got %d", len(files), idx.CountEntries())
	}
}

// TestOrchestrateAddExecution_ExplicitFilePreservesUnselectedDeletion verifies
// that staging one explicit file does not remove another missing tracked path
// from the index.
func TestOrchestrateAddExecution_ExplicitFilePreservesUnselectedDeletion(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	selectedPath := testutils.RandomString(10)
	deletedPath := testutils.RandomString(11)
	testutils.CreateTestFile(t, repoPath, selectedPath, testutils.RandomBytes(20))
	deletedFilePath := testutils.CreateTestFile(t, repoPath, deletedPath, testutils.RandomBytes(21))
	if _, _, err := OrchestrateAddExecution(repoPath, []string{selectedPath, deletedPath}); err != nil {
		t.Fatalf("failed to stage initial files: %v", err)
	}
	if err := os.Remove(deletedFilePath); err != nil {
		t.Fatalf("failed to remove unselected tracked file: %v", err)
	}
	selectedContent := testutils.RandomBytes(22)
	testutils.CreateTestFile(t, repoPath, selectedPath, selectedContent)

	addedFiles, deletedFiles, err := OrchestrateAddExecution(repoPath, []string{selectedPath})
	if err != nil {
		t.Fatalf("failed to stage selected file: %v", err)
	}
	if !slices.Equal(addedFiles, []string{selectedPath}) {
		t.Fatalf("expected added files [%s], got [%v]", selectedPath, addedFiles)
	}
	if len(deletedFiles) != 0 {
		t.Fatalf("expected no unselected deletions, got [%v]", deletedFiles)
	}

	idx := loadIndex(t, repoPath)
	if idx.GetEntry(deletedPath) == nil {
		t.Fatalf("expected unselected deleted path [%s] to remain in the index", deletedPath)
	}
	selectedEntry := idx.GetEntry(selectedPath)
	if selectedEntry == nil {
		t.Fatalf("expected selected path [%s] in the index", selectedPath)
	}
	if selectedEntry.FileSize() != int64(len(selectedContent)) {
		t.Fatalf("expected selected file size [%d], got [%d]", len(selectedContent), selectedEntry.FileSize())
	}
}

// TestOrchestrateAddExecution_ExplicitDeletion verifies that selecting a
// missing tracked file removes only that file from the index and reports it as
// deleted.
func TestOrchestrateAddExecution_ExplicitDeletion(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	deletedPath := testutils.RandomString(10)
	retainedPath := testutils.RandomString(11)
	deletedFilePath := testutils.CreateTestFile(t, repoPath, deletedPath, testutils.RandomBytes(20))
	testutils.CreateTestFile(t, repoPath, retainedPath, testutils.RandomBytes(21))
	if _, _, err := OrchestrateAddExecution(repoPath, []string{deletedPath, retainedPath}); err != nil {
		t.Fatalf("failed to stage initial files: %v", err)
	}
	if err := os.Remove(deletedFilePath); err != nil {
		t.Fatalf("failed to remove tracked file: %v", err)
	}

	addedFiles, deletedFiles, err := OrchestrateAddExecution(repoPath, []string{deletedPath, deletedPath})
	if err != nil {
		t.Fatalf("failed to stage explicit deletion: %v", err)
	}
	if len(addedFiles) != 0 {
		t.Fatalf("expected no added files, got [%v]", addedFiles)
	}
	if !slices.Equal(deletedFiles, []string{deletedPath}) {
		t.Fatalf("expected deleted files [%s], got [%v]", deletedPath, deletedFiles)
	}

	idx := loadIndex(t, repoPath)
	if idx.GetEntry(deletedPath) != nil {
		t.Fatalf("expected deleted path [%s] to be removed from the index", deletedPath)
	}
	if idx.GetEntry(retainedPath) == nil {
		t.Fatalf("expected unrelated path [%s] to remain in the index", retainedPath)
	}
}

// TestOrchestrateAddExecution_ENOTDIRStagesTrackedDeletion verifies that an
// indexed descendant blocked by a regular-file ancestor is treated as absent
// when the filesystem reports ENOTDIR.
func TestOrchestrateAddExecution_ENOTDIRStagesTrackedDeletion(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	parentPath := testutils.RandomString(10)
	childPath := filepath.ToSlash(filepath.Join(parentPath, testutils.RandomString(11)))
	testutils.CreateTestFileWithDirs(t, repoPath, filepath.FromSlash(childPath), testutils.RandomBytes(20))
	if _, _, err := OrchestrateAddExecution(repoPath, []string{childPath}); err != nil {
		t.Fatalf("failed to stage initial descendant: %v", err)
	}

	patches := gomonkey.ApplyFunc(os.Lstat, func(filePath string) (os.FileInfo, error) {
		return nil, &os.PathError{Op: "lstat", Path: filePath, Err: syscall.ENOTDIR}
	})
	defer patches.Reset()

	addedFiles, deletedFiles, err := OrchestrateAddExecution(repoPath, []string{childPath})
	if err != nil {
		t.Fatalf("failed to stage ENOTDIR descendant as deleted: %v", err)
	}
	if len(addedFiles) != 0 {
		t.Fatalf("expected no added files, got [%v]", addedFiles)
	}
	if !slices.Equal(deletedFiles, []string{childPath}) {
		t.Fatalf("expected deleted path [%s], got [%v]", childPath, deletedFiles)
	}
	if idx := loadIndex(t, repoPath); idx.GetEntry(childPath) != nil {
		t.Fatalf("expected ENOTDIR path [%s] to be removed from the index", childPath)
	}
}

// TestOrchestrateAddExecution_InspectionErrorPreservesPersistedIndex verifies
// that an inspection failure after an in-memory deletion does not persist any
// partial index mutation.
func TestOrchestrateAddExecution_InspectionErrorPreservesPersistedIndex(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	deletedPath := "a-" + testutils.RandomString(10)
	deniedPath := "z-" + testutils.RandomString(10)
	deletedFilePath := testutils.CreateTestFile(t, repoPath, deletedPath, testutils.RandomBytes(20))
	testutils.CreateTestFile(t, repoPath, deniedPath, testutils.RandomBytes(21))
	if _, _, err := OrchestrateAddExecution(repoPath, []string{deletedPath, deniedPath}); err != nil {
		t.Fatalf("failed to stage initial files: %v", err)
	}
	if err := os.Remove(deletedFilePath); err != nil {
		t.Fatalf("failed to remove tracked file: %v", err)
	}

	indexPath := filepath.Join(repoPath, constants.Gogit, constants.Index)
	indexBefore, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index before staging failure: %v", err)
	}
	deniedAbsolutePath := filepath.Join(repoPath, deniedPath)
	patches := gomonkey.ApplyFunc(os.Lstat, func(filePath string) (os.FileInfo, error) {
		if filePath == deniedAbsolutePath {
			return nil, &os.PathError{Op: "lstat", Path: filePath, Err: syscall.EACCES}
		}
		return nil, &os.PathError{Op: "lstat", Path: filePath, Err: fs.ErrNotExist}
	})
	defer patches.Reset()

	_, _, err = OrchestrateAddExecution(repoPath, []string{deletedPath, deniedPath})
	if err == nil {
		t.Fatal("expected inspection error")
	}
	if !errors.Is(err, syscall.EACCES) {
		t.Fatalf("expected permission error, got [%v]", err)
	}

	indexAfter, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index after staging failure: %v", err)
	}
	if !slices.Equal(indexAfter, indexBefore) {
		t.Fatal("expected inspection failure to preserve persisted index bytes")
	}
}

// TestOrchestrateAddExecution_TrackedFileBecomesDirectory verifies that a
// tracked file entry is removed before staging a descendant created beneath
// the replacement directory.
func TestOrchestrateAddExecution_TrackedFileBecomesDirectory(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	parentPath := testutils.RandomString(10)
	parentFilePath := testutils.CreateTestFile(t, repoPath, parentPath, testutils.RandomBytes(20))
	if _, _, err := OrchestrateAddExecution(repoPath, []string{parentPath}); err != nil {
		t.Fatalf("failed to stage parent file: %v", err)
	}
	if err := os.Remove(parentFilePath); err != nil {
		t.Fatalf("failed to remove parent file: %v", err)
	}
	childName := testutils.RandomString(11)
	childPath := filepath.ToSlash(filepath.Join(parentPath, childName))
	testutils.CreateTestFileWithDirs(t, repoPath, filepath.FromSlash(childPath), testutils.RandomBytes(21))

	addedFiles, deletedFiles, err := OrchestrateAddExecution(repoPath, []string{childPath})
	if err != nil {
		t.Fatalf("failed to stage file-to-directory transition: %v", err)
	}
	if !slices.Equal(addedFiles, []string{childPath}) {
		t.Fatalf("expected added child [%s], got [%v]", childPath, addedFiles)
	}
	if !slices.Equal(deletedFiles, []string{parentPath}) {
		t.Fatalf("expected deleted parent [%s], got [%v]", parentPath, deletedFiles)
	}

	idx := loadIndex(t, repoPath)
	if idx.CountEntries() != 1 || idx.GetEntry(parentPath) != nil || idx.GetEntry(childPath) == nil {
		t.Fatalf("expected index to contain only descendant [%s]", childPath)
	}
}

// TestOrchestrateAddExecution_TrackedDirectoryBecomesFile verifies that all
// tracked descendants are removed before staging a file at their parent path.
func TestOrchestrateAddExecution_TrackedDirectoryBecomesFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	parentPath := testutils.RandomString(10)
	childPaths := []string{
		filepath.ToSlash(filepath.Join(parentPath, testutils.RandomString(11))),
		filepath.ToSlash(filepath.Join(parentPath, testutils.RandomString(12), testutils.RandomString(13))),
	}
	for _, childPath := range childPaths {
		testutils.CreateTestFileWithDirs(t, repoPath, filepath.FromSlash(childPath), testutils.RandomBytes(20))
	}
	if _, _, err := OrchestrateAddExecution(repoPath, childPaths); err != nil {
		t.Fatalf("failed to stage child files: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(repoPath, parentPath)); err != nil {
		t.Fatalf("failed to remove parent directory: %v", err)
	}
	testutils.CreateTestFile(t, repoPath, parentPath, testutils.RandomBytes(21))

	addedFiles, deletedFiles, err := OrchestrateAddExecution(repoPath, []string{"."})
	if err != nil {
		t.Fatalf("failed to stage directory-to-file transition: %v", err)
	}
	if !slices.Equal(addedFiles, []string{parentPath}) {
		t.Fatalf("expected added parent [%s], got [%v]", parentPath, addedFiles)
	}
	slices.Sort(childPaths)
	if !slices.Equal(deletedFiles, childPaths) {
		t.Fatalf("expected deleted children [%v], got [%v]", childPaths, deletedFiles)
	}

	idx := loadIndex(t, repoPath)
	if idx.CountEntries() != 1 || idx.GetEntry(parentPath) == nil {
		t.Fatalf("expected index to contain only parent file [%s]", parentPath)
	}
	for _, childPath := range childPaths {
		if idx.GetEntry(childPath) != nil {
			t.Fatalf("expected descendant [%s] to be removed from the index", childPath)
		}
	}
}

// TestOrchestrateAddExecution_RejectsUnsupportedSymlink verifies that staging
// rejects symbolic links rather than following them and recording the target
// as a regular file.
func TestOrchestrateAddExecution_RejectsUnsupportedSymlink(t *testing.T) {
	testCases := []struct {
		name   string
		addAll bool
	}{
		{name: "explicit path"},
		{name: "repository-wide", addAll: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repoPath := testutils.SetupTestRepoWithInit(t)
			testutils.ChangeToDir(t, repoPath)

			targetName := testutils.RandomString(10)
			testutils.CreateTestFile(t, repoPath, targetName, testutils.RandomBytes(20))
			linkName := testutils.RandomString(11)
			if err := os.Symlink(targetName, filepath.Join(repoPath, linkName)); err != nil {
				t.Skipf("symbolic links are unavailable: %v", err)
			}

			args := []string{linkName}
			if testCase.addAll {
				args = []string{"."}
			}
			_, _, err := OrchestrateAddExecution(repoPath, args)
			if err == nil {
				t.Fatal("expected unsupported symbolic link to be rejected")
			}
			if !strings.Contains(err.Error(), "unsupported filesystem object") {
				t.Fatalf("expected unsupported object error, got [%v]", err)
			}
			if idx := loadIndex(t, repoPath); idx.CountEntries() != 0 {
				t.Fatalf("expected rejected staging operation to preserve an empty index, got [%d] entries", idx.CountEntries())
			}
		})
	}
}

// TestOrchestrateAddExecution_ExplicitNestedFileUsesRepositoryPath verifies
// that an explicit path resolved from a nested working directory is stored as
// one canonical path relative to the repository root.
func TestOrchestrateAddExecution_ExplicitNestedFileUsesRepositoryPath(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	nestedDirectory := testutils.RandomString(10)
	nestedDirectoryPath := filepath.Join(repoPath, nestedDirectory)
	if err := os.MkdirAll(nestedDirectoryPath, constants.DirPerms); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}
	fileName := testutils.RandomString(11)
	testutils.CreateTestFile(t, nestedDirectoryPath, fileName, testutils.RandomBytes(20))
	testutils.ChangeToDir(t, nestedDirectoryPath)
	expectedPath := filepath.ToSlash(filepath.Join(nestedDirectory, fileName))

	addedFiles, deletedFiles, err := OrchestrateAddExecution(repoPath, []string{fileName})
	if err != nil {
		t.Fatalf("failed to stage nested explicit file: %v", err)
	}
	if !slices.Equal(addedFiles, []string{expectedPath}) {
		t.Fatalf("expected added files [%s], got [%v]", expectedPath, addedFiles)
	}
	if len(deletedFiles) != 0 {
		t.Fatalf("expected no deleted files, got [%v]", deletedFiles)
	}
	if idxEntry := loadIndex(t, repoPath).GetEntry(expectedPath); idxEntry == nil {
		t.Fatalf("expected canonical nested index path [%s]", expectedPath)
	}
}

// TestOrchestrateAddExecution_AddAllFromNestedDirectoryReconcilesRepository
// verifies that add . retains repository-wide staging semantics when invoked
// from a nested working directory.
func TestOrchestrateAddExecution_AddAllFromNestedDirectoryReconcilesRepository(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	rootPath := testutils.RandomString(10)
	rootFilePath := testutils.CreateTestFile(t, repoPath, rootPath, testutils.RandomBytes(20))
	nestedDirectory := testutils.RandomString(11)
	nestedDirectoryPath := filepath.Join(repoPath, nestedDirectory)
	if err := os.MkdirAll(nestedDirectoryPath, constants.DirPerms); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}
	nestedRetainedName := testutils.RandomString(12)
	nestedDeletedName := testutils.RandomString(13)
	testutils.CreateTestFile(t, nestedDirectoryPath, nestedRetainedName, testutils.RandomBytes(21))
	nestedDeletedFilePath := testutils.CreateTestFile(t, nestedDirectoryPath, nestedDeletedName, testutils.RandomBytes(22))
	if _, _, err := OrchestrateAddExecution(repoPath, []string{"."}); err != nil {
		t.Fatalf("failed to stage initial repository: %v", err)
	}
	if err := os.Remove(rootFilePath); err != nil {
		t.Fatalf("failed to remove root tracked file: %v", err)
	}
	if err := os.Remove(nestedDeletedFilePath); err != nil {
		t.Fatalf("failed to remove nested tracked file: %v", err)
	}
	testutils.ChangeToDir(t, nestedDirectoryPath)
	nestedDeletedPath := filepath.ToSlash(filepath.Join(nestedDirectory, nestedDeletedName))

	addedFiles, deletedFiles, err := OrchestrateAddExecution(repoPath, []string{"."})
	if err != nil {
		t.Fatalf("failed to stage repository from nested directory: %v", err)
	}
	if len(addedFiles) != 0 {
		t.Fatalf("expected no changed files in nested subtree, got [%v]", addedFiles)
	}
	expectedDeletedFiles := []string{rootPath, nestedDeletedPath}
	slices.Sort(expectedDeletedFiles)
	if !slices.Equal(deletedFiles, expectedDeletedFiles) {
		t.Fatalf("expected deleted files [%v], got [%v]", expectedDeletedFiles, deletedFiles)
	}

	idx := loadIndex(t, repoPath)
	for _, deletedPath := range expectedDeletedFiles {
		if idx.GetEntry(deletedPath) != nil {
			t.Fatalf("expected deleted path [%s] to be removed from the index", deletedPath)
		}
	}
}

// TestOrchestrateAddExecution_AddAll_EmptyRepository verifies that "." on an
// empty repository succeeds with zero staged files.
func TestOrchestrateAddExecution_AddAll_EmptyRepository(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	addedFiles, _, err := OrchestrateAddExecution(repoPath, []string{"."})
	if err != nil {
		t.Fatalf("OrchestrateAddExecution should succeed on empty repo: %v", err)
	}

	if len(addedFiles) != 0 {
		t.Errorf("expected 0 added files for empty repo, got %d", len(addedFiles))
	}

	idx := loadIndex(t, repoPath)
	if idx.CountEntries() != 0 {
		t.Errorf("expected 0 index entries for empty repo, got %d", idx.CountEntries())
	}
}

// TestAddFile_RejectsRepositoryMetadataPath verifies that explicitly adding a
// file below .gogit fails before the index or object store is mutated.
func TestAddFile_RejectsRepositoryMetadataPath(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	idx := index.NewIndex()
	store := objects.NewObjectStore(repoPath)
	metadataPath := filepath.ToSlash(filepath.Join(constants.Gogit, constants.Head))
	metadataContent, err := os.ReadFile(filepath.Join(repoPath, constants.Gogit, constants.Head))
	if err != nil {
		t.Fatalf("failed to read repository HEAD: %v", err)
	}
	metadataBlob := objects.NewBlob(metadataContent)

	addedPath, err := addFile(repoPath, metadataPath, idx, store)

	if err == nil {
		t.Fatal("expected explicit repository metadata path to be rejected")
	}
	if addedPath != "" {
		t.Fatalf("expected no staged path, got [%s]", addedPath)
	}
	if idx.CountEntries() != 0 {
		t.Fatalf("expected index to remain empty, got [%d] entries", idx.CountEntries())
	}
	if _, readErr := store.ReadBlob(metadataBlob.Hash()); readErr == nil {
		t.Fatal("expected rejected metadata blob not to be stored")
	}
}

// TestOrchestrateAddExecution_SkipUnchangedFile verifies that re-staging an
// unmodified file returns an empty added list.
func TestOrchestrateAddExecution_SkipUnchangedFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	testFileName := testutils.RandomString(10)
	testutils.CreateTestFile(t, repoPath, testFileName, []byte(testutils.RandomString(100)))

	// Stage once
	if _, _, err := OrchestrateAddExecution(repoPath, []string{testFileName}); err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	// Stage again without modification
	addedFiles, _, err := OrchestrateAddExecution(repoPath, []string{testFileName})
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}

	if len(addedFiles) != 0 {
		t.Errorf("expected 0 added files for unchanged file, got %d", len(addedFiles))
	}
}

// TestOrchestrateAddExecution_RestagesModeOnlyChange verifies that an
// executable-bit change updates the existing index entry without changing its blob.
func TestOrchestrateAddExecution_RestagesModeOnlyChange(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable file permissions are not reliable on Windows")
	}

	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	testFileName := testutils.RandomString(10)
	testutils.CreateTestFile(t, repoPath, testFileName, []byte(testutils.RandomString(100)))

	if _, _, err := OrchestrateAddExecution(repoPath, []string{testFileName}); err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	originalEntry := loadIndex(t, repoPath).GetEntry(testFileName)
	if originalEntry == nil {
		t.Fatal("expected staged entry after first add")
	}
	originalHash := originalEntry.Hash()

	filePath := filepath.Join(repoPath, testFileName)
	if err := os.Chmod(filePath, constants.ExecutableFilePerms); err != nil {
		t.Fatalf("failed to make file executable: %v", err)
	}

	addedFiles, _, err := OrchestrateAddExecution(repoPath, []string{testFileName})
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}
	if len(addedFiles) != 1 || addedFiles[0] != testFileName {
		t.Fatalf("expected mode-only change to re-stage [%q], got [%v]", testFileName, addedFiles)
	}

	updatedEntry := loadIndex(t, repoPath).GetEntry(testFileName)
	if updatedEntry == nil {
		t.Fatal("expected staged entry after mode-only change")
	}
	if updatedEntry.Hash() != originalHash {
		t.Errorf("expected mode-only change to retain blob hash [%s], got [%s]", originalHash, updatedEntry.Hash())
	}
	if updatedEntry.Mode() != index.ModeExecutable {
		t.Errorf("expected executable index mode, got [%v]", updatedEntry.Mode())
	}
}

// Helpers

// loadIndex is a test helper that loads and returns the repository index.
func loadIndex(t *testing.T, repoPath string) *index.Index {
	t.Helper()

	indexManager := index.NewManager(repoPath)
	idx, err := indexManager.Load()
	if err != nil {
		t.Fatalf("failed to load index: %v", err)
	}
	return idx
}
