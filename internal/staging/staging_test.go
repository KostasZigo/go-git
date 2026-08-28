package staging

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestOrchestrateAddExecution_SingleFile verifies that staging a single file
// creates a blob object and adds an entry to the index.
func TestOrchestrateAddExecution_SingleFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	testFileName := testutils.RandomString(10)
	testutils.CreateTestFile(t, repoPath, testFileName, []byte(testutils.RandomString(500)))

	addedFiles, err := OrchestrateAddExecution(repoPath, []string{testFileName})
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

	addedFiles, err := OrchestrateAddExecution(repoPath, testFileNames)
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

	_, err := OrchestrateAddExecution(repoPath, []string{nonExistentFile})
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
	if _, err := OrchestrateAddExecution(repoPath, []string{testFileName}); err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	idx := loadIndex(t, repoPath)
	originalHash := idx.GetEntryList()[0].Hash()
	originalSize := idx.GetEntryList()[0].FileSize()

	// Modify file with different content length
	testutils.CreateTestFile(t, repoPath, testFileName, []byte(testutils.RandomString(1000)))

	// Stage modified
	addedFiles, err := OrchestrateAddExecution(repoPath, []string{testFileName})
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

	addedFiles, err := OrchestrateAddExecution(repoPath, []string{"."})
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

// TestOrchestrateAddExecution_AddAll_EmptyRepository verifies that "." on an
// empty repository succeeds with zero staged files.
func TestOrchestrateAddExecution_AddAll_EmptyRepository(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	addedFiles, err := OrchestrateAddExecution(repoPath, []string{"."})
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
	if _, err := OrchestrateAddExecution(repoPath, []string{testFileName}); err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	// Stage again without modification
	addedFiles, err := OrchestrateAddExecution(repoPath, []string{testFileName})
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

	if _, err := OrchestrateAddExecution(repoPath, []string{testFileName}); err != nil {
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

	addedFiles, err := OrchestrateAddExecution(repoPath, []string{testFileName})
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
