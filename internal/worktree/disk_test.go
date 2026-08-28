package worktree

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/hasher"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestServiceInspectWorktreeChanges verifies every supported difference between
// the index baseline and one tracked filesystem path.
func TestServiceInspectWorktreeChanges(t *testing.T) {
	fileContent := testutils.RandomBytes(20)
	updatedFileContent := testutils.RandomBytes(30)
	fileName := testutils.RandomString(10)

	tests := []struct {
		name            string
		fileName        string
		diskContent     []byte
		indexContent    []byte
		indexMode       objects.FileMode
		deleteDiskFile  bool
		replaceWithDir  bool
		expectedChanges []Change
		expectedError   string
	}{
		{
			name:         "unchanged regular file",
			fileName:     fileName,
			diskContent:  fileContent,
			indexContent: fileContent,
			indexMode:    objects.ModeRegularFile,
		},
		{
			name:         "unchanged nested file",
			fileName:     "docs/readme.md",
			diskContent:  fileContent,
			indexContent: fileContent,
			indexMode:    objects.ModeRegularFile,
		},
		{
			name:         "content modified",
			fileName:     fileName,
			diskContent:  fileContent,
			indexContent: updatedFileContent,
			indexMode:    objects.ModeRegularFile,
			expectedChanges: []Change{
				{Path: fileName, Kind: ChangeContentModified},
			},
		},
		{
			name:         "mode modified",
			fileName:     fileName,
			diskContent:  fileContent,
			indexContent: fileContent,
			indexMode:    objects.ModeExecutable,
			expectedChanges: []Change{
				{Path: fileName, Kind: ChangeModeModified},
			},
		},
		{
			name:         "content and mode modified",
			fileName:     fileName,
			diskContent:  fileContent,
			indexContent: updatedFileContent,
			indexMode:    objects.ModeExecutable,
			expectedChanges: []Change{
				{Path: fileName, Kind: ChangeContentModified},
				{Path: fileName, Kind: ChangeModeModified},
			},
		},
		{
			name:           "tracked file deleted",
			fileName:       fileName,
			diskContent:    fileContent,
			indexContent:   fileContent,
			indexMode:      objects.ModeRegularFile,
			deleteDiskFile: true,
			expectedChanges: []Change{
				{Path: fileName, Kind: ChangeDeleted},
			},
		},
		{
			name:           "tracked file replaced by directory",
			fileName:       fileName,
			diskContent:    fileContent,
			indexContent:   fileContent,
			indexMode:      objects.ModeRegularFile,
			replaceWithDir: true,
			expectedChanges: []Change{
				{Path: fileName, Kind: ChangeTypeModified},
			},
		},
		{
			name:          "unsupported symlink index mode",
			fileName:      fileName,
			diskContent:   fileContent,
			indexContent:  fileContent,
			indexMode:     objects.ModeSymlink,
			expectedError: "unsupported index file mode",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repoPath := testutils.SetupTestRepoWithInit(t)
			filePath := filepath.Join(repoPath, test.fileName)
			if err := os.MkdirAll(filepath.Dir(filePath), constants.DirPerms); err != nil {
				t.Fatalf("failed to create parent directory for tracked file: %v", err)
			}
			filePath = testutils.CreateTestFile(t, repoPath, test.fileName, test.diskContent)

			fileInfo, err := os.Stat(filePath)
			if err != nil {
				t.Fatalf("failed to stat tracked file: %v", err)
			}

			indexHash, err := hasher.ComputeHash(test.indexContent, hasher.Blob)
			if err != nil {
				t.Fatalf("failed to compute index content hash: %v", err)
			}

			stagingIndex := index.NewIndex()
			addIndexEntryWithContent(
				t,
				stagingIndex,
				test.indexMode,
				indexHash,
				test.fileName,
				test.indexContent,
				fileInfo.ModTime().Truncate(time.Second),
			)
			saveIndex(t, repoPath, stagingIndex)

			if test.deleteDiskFile || test.replaceWithDir {
				if err := os.Remove(filePath); err != nil {
					t.Fatalf("failed to remove tracked file: %v", err)
				}
			}

			if test.replaceWithDir {
				if err := os.Mkdir(filePath, constants.DirPerms); err != nil {
					t.Fatalf("failed to replace tracked file with directory: %v", err)
				}
			}

			service, err := NewService(repoPath)
			if err != nil {
				t.Fatalf("failed to create worktree service: %v", err)
			}
			changes, err := service.InspectWorktreeChanges()

			if test.expectedError != "" {
				if err == nil {
					t.Fatalf("expected error containing [%s]", test.expectedError)
				}

				if !strings.Contains(err.Error(), test.expectedError) {
					t.Fatalf("expected error containing [%s], got [%s]", test.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("failed to inspect working-tree changes: %v", err)
			}

			if !slices.Equal(changes, test.expectedChanges) {
				t.Fatalf("expected changes [%#v], got [%#v]", test.expectedChanges, changes)
			}
		})
	}
}

// TestServiceInspectWorktreeChanges_ChangesAreSorted verifies that changes are
// returned in path order even when index entries are added in another order.
func TestServiceInspectWorktreeChanges_ChangesAreSorted(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	stagingIndex := index.NewIndex()

	files := []struct {
		name    string
		content []byte
	}{
		{name: "c-deleted.txt", content: []byte("deleted")},
		{name: "b-content.txt", content: []byte("original")},
		{name: "a-type.txt", content: []byte("original")},
	}

	for _, file := range files {
		testutils.CreateTestFile(t, repoPath, file.name, file.content)

		hash, err := hasher.ComputeHash(file.content, hasher.Blob)
		if err != nil {
			t.Fatalf("failed to compute hash for %q: %v", file.name, err)
		}

		addIndexEntryWithContent(
			t,
			stagingIndex,
			objects.ModeRegularFile,
			hash,
			file.name,
			file.content,
			time.Now().UTC(),
		)
	}
	saveIndex(t, repoPath, stagingIndex)

	// change type file to dir
	typePath := filepath.Join(repoPath, "a-type.txt")
	if err := os.Remove(typePath); err != nil {
		t.Fatalf("failed to remove tracked file: %v", err)
	}
	if err := os.Mkdir(typePath, constants.DirPerms); err != nil {
		t.Fatalf("failed to replace tracked file with directory: %v", err)
	}

	// Update file's content
	if err := os.WriteFile(filepath.Join(repoPath, "b-content.txt"), []byte("changed"), constants.FilePerms); err != nil {
		t.Fatalf("failed to modify tracked file: %v", err)
	}

	// Delete file
	if err := os.Remove(filepath.Join(repoPath, "c-deleted.txt")); err != nil {
		t.Fatalf("failed to delete tracked file: %v", err)
	}

	service, err := NewService(repoPath)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}
	changes, err := service.InspectWorktreeChanges()
	if err != nil {
		t.Fatalf("failed to inspect working-tree changes: %v", err)
	}

	expectedChanges := []Change{
		{Path: "a-type.txt", Kind: ChangeTypeModified},
		{Path: "b-content.txt", Kind: ChangeContentModified},
		{Path: "c-deleted.txt", Kind: ChangeDeleted},
	}

	if !slices.Equal(changes, expectedChanges) {
		t.Fatalf(
			"expected sorted changes %#v, got %#v",
			expectedChanges,
			changes,
		)
	}
}
