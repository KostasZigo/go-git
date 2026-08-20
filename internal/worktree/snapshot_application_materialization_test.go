package worktree

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestWorktreeApplication_RemovePaths_DepthFirst verifies that the application
// plan removes a nested file before its explicitly listed parent directory,
// even when the parent appears first in the removal plan.
func TestWorktreeApplication_RemovePaths_DepthFirst(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	dirLogicalPath := testutils.RandomString(10)

	fileName := testutils.RandomString(10)
	fileLogicalPath := path.Join(dirLogicalPath, fileName)
	filePath := testutils.CreateTestFileWithDirs(t, repoPath, filepath.Join(dirLogicalPath, fileName), testutils.RandomBytes(20))

	// Intentionally add least depth first - testing the expecting ordering
	appPlan := &applicationPlan{pathsToRemove: []string{dirLogicalPath, fileLogicalPath}}
	if err := appPlan.removeWorkTreePaths(repoPath); err != nil {
		t.Fatalf("failed to remove paths from file system: %v", err)
	}

	testutils.AssertFileNotExists(t, filePath)
	testutils.AssertDirNotExists(t, filepath.Join(repoPath, dirLogicalPath))
}

// TestWorktreeApplication_RemovePathsError_NonEmptyDirectoryPreservesUntrackedContent
// verifies that removal stops with an error when an explicitly removed directory
// still contains an untracked file, preserving that file after tracked descendants
// have already been removed.
func TestWorktreeApplication_RemovePathsError_NonEmptyDirectoryPreservesUntrackedContent(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	dirLogicalPath := testutils.RandomString(10)

	trackedFile := testutils.RandomString(10)
	trackedLogicalPath := path.Join(dirLogicalPath, trackedFile)
	trackedAbsPath := testutils.CreateTestFileWithDirs(t, repoPath, filepath.Join(dirLogicalPath, trackedFile), testutils.RandomBytes(20))

	untrackedFilePath := testutils.CreateTestFile(t, repoPath, filepath.Join(dirLogicalPath, testutils.RandomString(20)), testutils.RandomBytes(21))

	appPlan := &applicationPlan{pathsToRemove: []string{dirLogicalPath, trackedLogicalPath}}
	err := appPlan.removeWorkTreePaths(repoPath)
	if err == nil {
		t.Fatal("expexted an error when trying to remove directory with untracked files")
	}
	expectedErrorMessage := fmt.Sprintf("failed to remove [%s]:", dirLogicalPath)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}

	testutils.AssertFileNotExists(t, trackedAbsPath)
	testutils.AssertDirExists(t, filepath.Join(repoPath, dirLogicalPath))
	testutils.AssertFileExists(t, untrackedFilePath)
}

// TestWorktreeApplication_RemovePaths_EmptyPathsDoNotAffectFileSystem verifies
// that applying an empty removal plan succeeds without changing existing
// worktree content.
func TestWorktreeApplication_RemovePaths_EmptyPathsDoNotAffectFileSystem(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	untrackedFilePath := testutils.CreateTestFile(t, repoPath, testutils.RandomString(20), testutils.RandomBytes(21))
	appPlan := &applicationPlan{}
	if err := appPlan.removeWorkTreePaths(repoPath); err != nil {
		t.Fatalf("failed to remove paths from file system: %v", err)
	}

	testutils.AssertFileExists(t, untrackedFilePath)
}

// TestWorktreeApplication_MaterializePlannedFiles_WritesFileSystemAndBuildsReplacementIndex
// verifies that materialization retains unchanged files, rewrites changed files,
// creates missing parent directories, and indexes the complete target snapshot.
func TestWorktreeApplication_MaterializePlannedFiles_WritesFileSystemAndBuildsReplacementIndex(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	fileMode := objects.ModeRegularFile

	retainedFileContent := testutils.RandomBytes(20)
	retainedFileLogicalPath := testutils.RandomString(10)
	retainedFileHash := testutils.RandomHash()
	retainedFilePath := testutils.CreateTestFile(t, repoPath, retainedFileLogicalPath, retainedFileContent)
	retainedFileInfoBefore, err := os.Stat(retainedFilePath)
	if err != nil {
		t.Fatalf("failed to stat retained file before materialization: %v", err)
	}
	retainedPlannedFile := plannedFile{
		path: retainedFileLogicalPath, hash: retainedFileHash, mode: fileMode,
		content: retainedFileContent, permissions: constants.FilePerms, writeRequired: false,
	}

	existingFileLogicalPath := testutils.RandomString(11)
	existingFileContent := testutils.RandomBytes(20)
	changedFileContent := testutils.RandomBytes(21)
	existingFileHash := testutils.RandomHash()
	existingFilePath := testutils.CreateTestFile(t, repoPath, existingFileLogicalPath, existingFileContent)
	existingPlannedFile := plannedFile{
		path: existingFileLogicalPath, hash: existingFileHash, mode: fileMode,
		content: changedFileContent, permissions: constants.FilePerms, writeRequired: true,
	}

	dir := testutils.RandomString(12)
	nestedFileLogicalPath := path.Join(dir, testutils.RandomString(13))
	nestedFileContent := testutils.RandomBytes(20)
	nestedFileHash := testutils.RandomHash()
	nestedFilePath := filepath.Join(repoPath, filepath.FromSlash(nestedFileLogicalPath))
	nestedFilePlannedFile := plannedFile{
		path: nestedFileLogicalPath, hash: nestedFileHash, mode: fileMode,
		content: nestedFileContent, permissions: constants.FilePerms, writeRequired: true,
	}

	appPlan := &applicationPlan{
		targetPlannedFiles: []plannedFile{retainedPlannedFile, nestedFilePlannedFile, existingPlannedFile},
	}
	idx, err := appPlan.materializePlannedFiles(repoPath)
	if err != nil {
		t.Fatalf("failed to write on the file system and create replacement index: %v", err)
	}

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

	testutils.AssertDirExists(t, filepath.Join(repoPath, dir))
	testutils.AssertFileExists(t, nestedFilePath)
	testutils.AssertFileContent(t, nestedFilePath, nestedFileContent)

	indexMode, _ := index.FromObjectFileMode(fileMode)
	expectedIndexEntries := map[string]testIdxEntry{
		retainedFileLogicalPath: {hash: retainedFileHash, mode: indexMode},
		nestedFileLogicalPath:   {hash: nestedFileHash, mode: indexMode},
		existingFileLogicalPath: {hash: existingFileHash, mode: indexMode},
	}
	assertIndexEntries(t, idx, expectedIndexEntries)
}

// TestWorktreeApplication_MaterializePlannedFiles_ChangeMode verifies that
// materialization applies both regular-to-executable and executable-to-regular
// transitions and records the target mode in the replacement index.
func TestWorktreeApplication_MaterializePlannedFiles_ChangeMode(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	tests := map[string]struct {
		originalMode      objects.FileMode
		udpatedMode       objects.FileMode
		expectedIndexMode index.FileMode
	}{
		"regular to executable": {
			originalMode: objects.ModeRegularFile, udpatedMode: objects.ModeExecutable, expectedIndexMode: index.ModeExecutable,
		},
		"executable to regular": {
			originalMode: objects.ModeExecutable, udpatedMode: objects.ModeRegularFile, expectedIndexMode: index.ModeRegularFile,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			fileName := testutils.RandomString(10)
			fileContent := testutils.RandomBytes(20)
			fileHash := testutils.RandomHash()
			filePath := testutils.CreateTestFile(t, repoPath, fileName, fileContent)

			originalOsMode, err := test.originalMode.ToOsFileMOde()
			if err != nil {
				t.Fatalf("failed to convert objects FileMode to system permissions: %v", err)
			}
			if err := os.Chmod(filepath.Join(repoPath, fileName), originalOsMode); err != nil {
				t.Fatalf("failed to change mode of recently created file [%s]: %v", fileName, err)
			}

			updatedOsMode, _ := test.udpatedMode.ToOsFileMOde()
			appPlan := &applicationPlan{
				targetPlannedFiles: []plannedFile{{
					path: fileName, hash: fileHash, mode: test.udpatedMode,
					content: fileContent, permissions: updatedOsMode, writeRequired: true,
				}},
			}

			idx, err := appPlan.materializePlannedFiles(repoPath)
			if err != nil {
				t.Fatalf("failed to write on the file system and create replacement index: %v", err)
			}

			testutils.AssertFileExists(t, filePath)
			testutils.AssertFileContent(t, filePath, fileContent)

			info, err := os.Stat(filePath)
			if err != nil {
				t.Fatalf("failed to stat file [%s], %v", filePath, err)
			}
			if runtime.GOOS != "windows" && info.Mode() != updatedOsMode {
				t.Fatalf("expected file mode to be updated from [%v] to  [%v], got [%v]", originalOsMode, updatedOsMode, info.Mode())
			}

			expectedIndexEntries := map[string]testIdxEntry{
				fileName: {hash: fileHash, mode: test.expectedIndexMode},
			}
			assertIndexEntries(t, idx, expectedIndexEntries)
		})
	}
}

// TestWorktreeApplication_MaterializePlannedFiles_EmptyPlanNoErrorEmptyIndex
// verifies that an empty target plan leaves the worktree untouched and returns
// a non-nil replacement index with no entries.
func TestWorktreeApplication_MaterializePlannedFiles_EmptyPlanNoErrorEmptyIndex(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	retainedFileContent := testutils.RandomBytes(20)
	retainedFilePath := testutils.CreateTestFile(t, repoPath, testutils.RandomString(10), retainedFileContent)

	appPlan := &applicationPlan{}
	idx, err := appPlan.materializePlannedFiles(repoPath)
	if err != nil {
		t.Fatalf("unexpected error for empty planned files list of application plan: %v", err)
	}

	if idx == nil {
		t.Fatalf("expected non nill index when application plan is empty, got [%#v]", idx)
	}
	if idx.CountEntries() != 0 {
		t.Fatalf("expected index to be empty for empty application plan, got [%d] index entries", idx.CountEntries())
	}

	testutils.AssertFileExists(t, retainedFilePath)
	testutils.AssertFileContent(t, retainedFilePath, retainedFileContent)
}

// TestWorktreeApplication_MaterializePlannedFiles_WriteFailureReturnsNoIndex
// verifies that a filesystem write failure returns an error and no partial
// replacement index.
func TestWorktreeApplication_MaterializePlannedFiles_WriteFailureReturnsNoIndex(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	fileName := testutils.RandomString(10)

	// Create a directory at the exact target path to cause write failure
	if err := os.MkdirAll(filepath.Join(repoPath, fileName), constants.DirPerms); err != nil {
		t.Fatalf("failed to create directory [%s]: %v", fileName, err)
	}

	file := plannedFile{
		path: fileName, hash: testutils.RandomHash(), mode: objects.ModeRegularFile,
		content: testutils.RandomBytes(20), permissions: constants.FilePerms, writeRequired: true,
	}
	appPlan := &applicationPlan{targetPlannedFiles: []plannedFile{file}}
	idx, err := appPlan.materializePlannedFiles(repoPath)
	if err == nil {
		t.Fatalf("expected an error when there is write failure due to a directory occupying the target path in the file system.")
	}

	expectedErrorMessage := fmt.Sprintf("failed to write file [%s] with permissions [%o] in the disk:", filepath.Join(repoPath, fileName), file.permissions)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}

	if idx != nil {
		t.Fatal("expected index to be nil when there is a write failure")
	}
}
