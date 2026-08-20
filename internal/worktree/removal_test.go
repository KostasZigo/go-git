package worktree

import (
	"path/filepath"
	"testing"

	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestRemovePaths_SingleFile verifies that RemovePaths
// removes a single tracked file from the repository root.
func TestRemovePaths_SingleFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	fileName := testutils.RandomString(10)
	filePath := testutils.CreateTestFile(t, repoPath, fileName, testutils.RandomBytes(20))

	if err := RemovePaths(repoPath, []string{fileName}); err != nil {
		t.Fatalf("failed to clean working directory: %v", err)
	}

	testutils.AssertFileNotExists(t, filePath)
}

// TestRemovePaths_NestedFiles verifies that RemovePaths
// removes files inside a subdirectory and prunes the now-empty parent directory.
func TestRemovePaths_NestedFiles(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	dir := filepath.Join(repoPath, testutils.RandomString(10))
	filePaths := make([]string, 2)
	for i := range filePaths {
		filePaths[i] = testutils.CreateTestFileWithDirs(t, dir, testutils.RandomString(10), testutils.RandomBytes(30))
	}

	if err := RemovePaths(repoPath, extractLogicalPaths(t, repoPath, filePaths)); err != nil {
		t.Fatalf("failed to clean working directory: %v", err)
	}

	for _, filePath := range filePaths {
		testutils.AssertFileNotExists(t, filePath)
	}
	testutils.AssertDirNotExists(t, dir)
}

// TestRemovePaths_UnspecifiedFilesRemain verifies that removePaths
// only removes paths that were in the input list, leaving unspecified files and their parent
// directories intact — even when both share the same directory.
func TestRemovePaths_UnspecifiedFilesRemain(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	dir := filepath.Join(repoPath, testutils.RandomString(10))
	// Specified files: registered in the index, expected to be deleted
	specifiedPaths := []string{
		testutils.CreateTestFileWithDirs(t, dir, testutils.RandomString(10), testutils.RandomBytes(20)),
		testutils.CreateTestFileWithDirs(t, dir, testutils.RandomString(11), testutils.RandomBytes(21)),
	}

	// Unspecified files expected to survive cleanup
	unspecifiedPaths := []string{
		testutils.CreateTestFile(t, dir, testutils.RandomString(12), []byte(testutils.RandomString(11))),
		testutils.CreateTestFile(t, repoPath, testutils.RandomString(13), []byte(testutils.RandomString(12))),
	}

	if err := RemovePaths(repoPath, extractLogicalPaths(t, repoPath, specifiedPaths)); err != nil {
		t.Fatalf("failed to clean working directory: %v", err)
	}

	for _, filePath := range specifiedPaths {
		testutils.AssertFileNotExists(t, filePath)
	}
	for _, filePath := range unspecifiedPaths {
		testutils.AssertFileExists(t, filePath)
	}
	testutils.AssertDirExists(t, dir)
}

// TestRemovePaths_EmptyPathList verifies that RemovePaths
// returns no error and leaves existing files untouched when the path list is empty.
func TestRemovePaths_EmptyPathList(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	filePath := testutils.CreateTestFile(t, repoPath, testutils.RandomString(10), []byte(testutils.RandomString(10)))

	if err := RemovePaths(repoPath, []string{}); err != nil {
		t.Fatalf("failed to clean working directory: %v", err)
	}
	testutils.AssertFileExists(t, filePath)
}

// TestRemoval_DeleteIndexFiles_FileAlreadyMissing verifies that CleanWorkingTree
// does not error when an index entry references a file that no longer exists on disk.
func TestRemoval_DeleteIndexFiles_FileAlreadyMissing(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	fileName := testutils.RandomString(10)
	if err := RemovePaths(repoPath, []string{fileName}); err != nil {
		t.Fatalf("failed to clean working directory: %v", err)
	}

	testutils.AssertFileNotExists(t, filepath.Join(repoPath, fileName))
}

// extractLogicalPaths converts the absolut path list to a repo relative path list
func extractLogicalPaths(t *testing.T, repoPath string, paths []string) []string {
	t.Helper()

	logicalPaths := make([]string, len(paths))
	for i, fp := range paths {
		logicalPath, err := filepath.Rel(repoPath, fp)
		if err != nil {
			t.Fatalf("failed to extract relative path from [%s]: %v", fp, err)
		}
		logicalPaths[i] = filepath.ToSlash(logicalPath)
	}
	return logicalPaths
}
