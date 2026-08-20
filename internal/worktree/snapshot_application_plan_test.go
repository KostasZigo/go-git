package worktree

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/objects/objectstest"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestBuildApplicationPlan verifies that target blobs, modes, permissions,
// and paths are fully resolved in deterministic order before application.
func TestBuildApplicationPlan(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)
	idx := index.NewIndex()

	original := objects.TreeSnapshot{
		"config/app.yaml": {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
		"readme.md": {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeExecutable,
		},
	}

	content := testutils.RandomBytes(20)
	helloBlob := objects.NewBlob(content)
	if err := store.Store(helloBlob); err != nil {
		t.Fatalf("failed to store hello blob: %v", err)
	}

	configHelloBlob := objects.NewBlob(content)
	if err := store.Store(configHelloBlob); err != nil {
		t.Fatalf("failed to store config blob: %v", err)
	}

	target := objects.TreeSnapshot{
		"hello.txt": {
			Hash: helloBlob.Hash(),
			Mode: objects.ModeRegularFile,
		},
		"config/hello.txt": {
			Hash: configHelloBlob.Hash(),
			Mode: objects.ModeExecutable,
		},
	}

	applicationPlan, err := buildApplicationPlan(repoPath, store, idx, original, target)
	if err != nil {
		t.Fatalf("failed to build application plan: %v", err)
	}

	if len(applicationPlan.pathsToRemove) != len(original) {
		t.Fatalf("expected [%d] removal paths, got [%d]", len(original), len(applicationPlan.pathsToRemove))
	}
	if len(applicationPlan.targetPlannedFiles) != len(target) {
		t.Fatalf("expected [%d] original paths, got [%d]", len(target), len(applicationPlan.targetPlannedFiles))
	}

	expectedOriginalPaths := []string{
		"config/app.yaml",
		"readme.md",
	}
	expectedPlannedFiles := []plannedFile{
		{
			hash:          configHelloBlob.Hash(),
			path:          "config/hello.txt",
			mode:          objects.ModeExecutable,
			content:       content,
			permissions:   constants.ExecutableFilePerms,
			writeRequired: true,
		},
		{
			hash:          helloBlob.Hash(),
			path:          "hello.txt",
			mode:          objects.ModeRegularFile,
			content:       content,
			permissions:   constants.FilePerms,
			writeRequired: true,
		},
	}

	if !slices.Equal(expectedOriginalPaths, applicationPlan.pathsToRemove) {
		t.Fatalf("expected removal paths [%#v], got [%#v]", expectedOriginalPaths, applicationPlan.pathsToRemove)
	}

	assertPlannedFileLists(t, expectedPlannedFiles, applicationPlan.targetPlannedFiles)
}

// TestBuildApplicationPlan_MissingBlobDoesNotMutate verifies that all target
// objects are resolved before any worktree or index mutation can begin.
func TestBuildApplicationPlan_MissingBlobDoesNotMutate(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	existingFileName := testutils.RandomString(10)
	existingContent := testutils.RandomBytes(20)
	_ = testutils.CreateTestFile(t, repoPath, existingFileName, existingContent)

	original := objects.TreeSnapshot{
		existingFileName: {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	missingFileName := testutils.RandomString(11)
	target := objects.TreeSnapshot{
		missingFileName: {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	_, err := buildApplicationPlan(repoPath, store, index.NewIndex(), original, target)
	if err == nil {
		t.Fatalf("expected an error when the blob for the file menioned in target is missing")
	}
	expectedErrorMessage := fmt.Sprintf("failed to read blob object for target snapshot path [%s]", missingFileName)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expecter error to contain message [%s], got [%s]", expectedErrorMessage, err.Error())
	}

	actualContent, err := os.ReadFile(filepath.Join(repoPath, existingFileName))
	if err != nil {
		t.Fatalf("failed to read existing file after plan failure: %v", err)
	}
	if !bytes.Equal(actualContent, existingContent) {
		t.Fatal("existing file changed during application planning")
	}
}

// TestInspectTargetMaterialization verifies that matching regular files are
// retained while changed, dirty, missing, mode-mismatched, and obstructed
// target paths are scheduled for the required filesystem operations.
func TestInspectTargetMaterialization(t *testing.T) {
	targetPath := "target.txt"
	targetContent := testutils.RandomBytes(40)
	targetBlob := objects.NewBlob(targetContent)

	testCases := []struct {
		name              string
		targetMode        objects.FileMode
		setup             func(t *testing.T, repoPath string) *index.Entry
		expectedWrite     bool
		expectedRemove    bool
		requiresUnixModes bool
	}{
		{
			name:       "unchanged clean target file is retained",
			targetMode: objects.ModeRegularFile,
			setup: func(t *testing.T, repoPath string) *index.Entry {
				t.Helper()
				return createMaterializationIndexEntry(
					t,
					repoPath,
					targetPath,
					targetContent,
					targetBlob.Hash(),
					index.ModeRegularFile,
					constants.FilePerms,
				)
			},
		},
		{
			name:       "changed target file is written",
			targetMode: objects.ModeRegularFile,
			setup: func(t *testing.T, repoPath string) *index.Entry {
				t.Helper()
				oldContent := testutils.RandomBytes(41)
				return createMaterializationIndexEntry(
					t,
					repoPath,
					targetPath,
					oldContent,
					objects.NewBlob(oldContent).Hash(),
					index.ModeRegularFile,
					constants.FilePerms,
				)
			},
			expectedWrite: true,
		},
		{
			name:       "unchanged but dirty target file is written",
			targetMode: objects.ModeRegularFile,
			setup: func(t *testing.T, repoPath string) *index.Entry {
				t.Helper()
				entry := createMaterializationIndexEntry(
					t,
					repoPath,
					targetPath,
					targetContent,
					targetBlob.Hash(),
					index.ModeRegularFile,
					constants.FilePerms,
				)
				if err := os.WriteFile(
					filepath.Join(repoPath, targetPath),
					testutils.RandomBytes(42),
					constants.FilePerms,
				); err != nil {
					t.Fatalf("failed to dirty target file: %v", err)
				}
				return entry
			},
			expectedWrite: true,
		},
		{
			name:       "missing target file is written",
			targetMode: objects.ModeRegularFile,
			setup: func(t *testing.T, _ string) *index.Entry {
				t.Helper()
				return nil
			},
			expectedWrite: true,
		},
		{
			name:       "mode mismatch is written",
			targetMode: objects.ModeExecutable,
			setup: func(t *testing.T, repoPath string) *index.Entry {
				t.Helper()
				return createMaterializationIndexEntry(
					t,
					repoPath,
					targetPath,
					targetContent,
					targetBlob.Hash(),
					index.ModeExecutable,
					constants.FilePerms,
				)
			},
			expectedWrite:     true,
			requiresUnixModes: true,
		},
		{
			name:       "disk directory obstruction is removed and written",
			targetMode: objects.ModeRegularFile,
			setup: func(t *testing.T, repoPath string) *index.Entry {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(repoPath, targetPath), constants.DirPerms); err != nil {
					t.Fatalf("failed to create target directory: %v", err)
				}
				return nil
			},
			expectedWrite:  true,
			expectedRemove: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.requiresUnixModes && runtime.GOOS == "windows" {
				t.Skip("executable permission bits are not reliable on Windows")
			}

			repoPath := testutils.SetupTestRepoWithInit(t)
			indexEntry := testCase.setup(t, repoPath)

			writeRequired, removeTargetPath, err := inspectTargetMaterialization(
				repoPath,
				targetPath,
				objects.SnapshotEntry{
					Hash: targetBlob.Hash(),
					Mode: testCase.targetMode,
				},
				indexEntry,
			)
			if err != nil {
				t.Fatalf("failed to inspect target materialization: %v", err)
			}
			if writeRequired != testCase.expectedWrite {
				t.Fatalf("expected writeRequired [%t], got [%t]", testCase.expectedWrite, writeRequired)
			}
			if removeTargetPath != testCase.expectedRemove {
				t.Fatalf("expected removeTargetPath [%t], got [%t]", testCase.expectedRemove, removeTargetPath)
			}
		})
	}
}

// TestBuildApplicationPlan_DeletedOriginalPathIsScheduled verifies that a
// tracked path absent from the target is included in the removal plan.
func TestBuildApplicationPlan_DeletedOriginalPathIsScheduled(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	deletedPath := testutils.RandomString(10)

	originalSnapshot := objects.TreeSnapshot{
		deletedPath: {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	applicationPlan, err := buildApplicationPlan(repoPath, objects.NewObjectStore(repoPath), index.NewIndex(), originalSnapshot, objects.TreeSnapshot{})
	if err != nil {
		t.Fatalf("failed to build application plan: %v", err)
	}
	if !slices.Equal(applicationPlan.pathsToRemove, []string{deletedPath}) {
		t.Fatalf("expected removal path [%s], got [%#v]", deletedPath, applicationPlan.pathsToRemove)
	}
}

// TestBuildApplicationPlan_TrackedFileBecomesDirectory verifies that a tracked
// file blocking a target descendant is removed and the descendant is planned
// for writing.
func TestBuildApplicationPlan_TrackedFileBecomesDirectory(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	parentPath := testutils.RandomString(10)
	targetPath := path.Join(parentPath, testutils.RandomString(10))
	targetContent := testutils.RandomBytes(20)
	targetBlob := objectstest.CreateAndStoreBlob(t, store, targetContent)
	_ = testutils.CreateTestFile(t, repoPath, parentPath, testutils.RandomBytes(21))

	original := objects.TreeSnapshot{parentPath: {Hash: testutils.RandomHash(), Mode: objects.ModeRegularFile}}
	target := objects.TreeSnapshot{targetPath: {Hash: targetBlob.Hash(), Mode: objects.ModeRegularFile}}

	applicationPlan, err := buildApplicationPlan(repoPath, store, index.NewIndex(), original, target)
	if err != nil {
		t.Fatalf("failed to plan tracked file to directory transition: %v", err)
	}
	if !slices.Equal(applicationPlan.pathsToRemove, []string{parentPath}) {
		t.Fatalf("expected removal paths [%v], got [%v]", []string{parentPath}, applicationPlan.pathsToRemove)
	}

	expectedFiles := []plannedFile{
		{
			path:          targetPath,
			hash:          targetBlob.Hash(),
			mode:          objects.ModeRegularFile,
			content:       targetContent,
			permissions:   constants.FilePerms,
			writeRequired: true,
		},
	}
	assertPlannedFileLists(t, expectedFiles, applicationPlan.targetPlannedFiles)
}

// TestBuildApplicationPlan_MultipleFiles verifies that one plan can retain,
// rewrite, create, replace, and delete files while preserving sorted output.
func TestBuildApplicationPlan_MultipleFiles(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)
	idx := index.NewIndex()

	retainedContent := testutils.RandomBytes(20)
	changedContent := testutils.RandomBytes(21)
	createdContent := testutils.RandomBytes(22)
	obstructedContent := testutils.RandomBytes(23)
	retainedBlob := objectstest.CreateAndStoreBlob(t, store, retainedContent)
	changedBlob := objectstest.CreateAndStoreBlob(t, store, changedContent)
	createdBlob := objectstest.CreateAndStoreBlob(t, store, createdContent)
	obstructedBlob := objectstest.CreateAndStoreBlob(t, store, obstructedContent)

	retainedPath := "a-retained.txt"
	changedPath := "b-changed.txt"
	createdPath := "c-created.txt"
	obstructedPath := "d-obstructed.txt"
	deletedPath := "e-deleted.txt"
	retainedEntry := createMaterializationIndexEntry(t, repoPath, retainedPath, retainedContent, retainedBlob.Hash(), index.ModeRegularFile, constants.FilePerms)
	if err := idx.AddEntry(retainedEntry); err != nil {
		t.Fatalf("failed to add retained index entry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, changedPath), testutils.RandomBytes(24), constants.FilePerms); err != nil {
		t.Fatalf("failed to write changed file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoPath, obstructedPath), constants.DirPerms); err != nil {
		t.Fatalf("failed to create obstructing directory: %v", err)
	}

	original := objects.TreeSnapshot{
		retainedPath:   {Hash: retainedBlob.Hash(), Mode: objects.ModeRegularFile},
		changedPath:    {Hash: testutils.RandomHash(), Mode: objects.ModeRegularFile},
		obstructedPath: {Hash: testutils.RandomHash(), Mode: objects.ModeRegularFile},
		deletedPath:    {Hash: testutils.RandomHash(), Mode: objects.ModeRegularFile},
	}
	target := objects.TreeSnapshot{
		retainedPath:   {Hash: retainedBlob.Hash(), Mode: objects.ModeRegularFile},
		changedPath:    {Hash: changedBlob.Hash(), Mode: objects.ModeRegularFile},
		createdPath:    {Hash: createdBlob.Hash(), Mode: objects.ModeRegularFile},
		obstructedPath: {Hash: obstructedBlob.Hash(), Mode: objects.ModeRegularFile},
	}

	applicationPlan, err := buildApplicationPlan(repoPath, store, idx, original, target)
	if err != nil {
		t.Fatalf("failed to build multi-file application plan: %v", err)
	}
	if !slices.Equal(applicationPlan.pathsToRemove, []string{obstructedPath, deletedPath}) {
		t.Fatalf("expected removal paths [%v], got [%v]", []string{obstructedPath, deletedPath}, applicationPlan.pathsToRemove)
	}

	expectedFiles := []plannedFile{
		{path: retainedPath, hash: retainedBlob.Hash(), mode: objects.ModeRegularFile, content: retainedContent, permissions: constants.FilePerms, writeRequired: false},
		{path: changedPath, hash: changedBlob.Hash(), mode: objects.ModeRegularFile, content: changedContent, permissions: constants.FilePerms, writeRequired: true},
		{path: createdPath, hash: createdBlob.Hash(), mode: objects.ModeRegularFile, content: createdContent, permissions: constants.FilePerms, writeRequired: true},
		{path: obstructedPath, hash: obstructedBlob.Hash(), mode: objects.ModeRegularFile, content: obstructedContent, permissions: constants.FilePerms, writeRequired: true},
	}
	assertPlannedFileLists(t, expectedFiles, applicationPlan.targetPlannedFiles)
	for _, plannedTargetFile := range applicationPlan.targetPlannedFiles {
		if plannedTargetFile.path == deletedPath {
			t.Fatalf("expected removed path [%s] to be absent from planned target files", deletedPath)
		}
	}
}

// TestBuildApplicationPlan_RetainsMatchingFileWithoutIndexMetadata verifies
// that disk hashing retains correct content when the index fast path is unavailable.
func TestBuildApplicationPlan_RetainsMatchingFileWithoutIndexMetadata(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	targetPath := "matching.txt"
	targetContent := testutils.RandomBytes(20)
	targetBlob := objectstest.CreateAndStoreBlob(t, store, targetContent)
	if err := os.WriteFile(filepath.Join(repoPath, targetPath), targetContent, constants.FilePerms); err != nil {
		t.Fatalf("failed to write matching target file: %v", err)
	}

	targetSnapshot := objects.TreeSnapshot{
		targetPath: {
			Hash: targetBlob.Hash(),
			Mode: objects.ModeRegularFile,
		},
	}

	applicationPlan, err := buildApplicationPlan(repoPath, store, index.NewIndex(), objects.TreeSnapshot{}, targetSnapshot)
	if err != nil {
		t.Fatalf("failed to build application plan: %v", err)
	}
	if len(applicationPlan.targetPlannedFiles) != 1 {
		t.Fatalf("expected one planned file, got [%d]", len(applicationPlan.targetPlannedFiles))
	}
	if applicationPlan.targetPlannedFiles[0].writeRequired {
		t.Fatal("expected matching target file to be retained")
	}
	if len(applicationPlan.pathsToRemove) != 0 {
		t.Fatalf("expected no removal paths, got [%v]", applicationPlan.pathsToRemove)
	}
}

// TestBuildApplicationPlan_UnsupportedTargetModeDoesNotMutate verifies that
// modes unsupported by worktree materialization fail during preflight.
func TestBuildApplicationPlan_UnsupportedTargetModeDoesNotMutate(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	existingPath := "existing.txt"
	existingContent := testutils.RandomBytes(20)
	targetBlob := objectstest.CreateAndStoreBlob(t, store, testutils.RandomBytes(21))
	if err := os.WriteFile(filepath.Join(repoPath, existingPath), existingContent, constants.FilePerms); err != nil {
		t.Fatalf("failed to write existing file: %v", err)
	}

	original := objects.TreeSnapshot{
		existingPath: {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}
	target := objects.TreeSnapshot{
		"link": {
			Hash: targetBlob.Hash(),
			Mode: objects.ModeSymlink,
		},
	}

	_, err := buildApplicationPlan(repoPath, store, index.NewIndex(), original, target)
	if err == nil {
		t.Fatal("expected unsupported target mode to fail application planning")
	}

	expectedErrorMessage := fmt.Sprintf("failed to convert mode [%s]", objects.ModeSymlink)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
	actualContent, readErr := os.ReadFile(filepath.Join(repoPath, existingPath))
	if readErr != nil {
		t.Fatalf("failed to read existing file after planning failure: %v", readErr)
	}
	if !bytes.Equal(actualContent, existingContent) {
		t.Fatal("existing file changed during application planning")
	}
}

// TestBuildApplicationPlan_CorruptBlobDoesNotMutate verifies that corrupt
// target objects fail during preflight before existing files are changed.
func TestBuildApplicationPlan_CorruptBlobDoesNotMutate(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	existingPath := "existing.txt"
	existingContent := testutils.RandomBytes(20)
	corruptHash := testutils.RandomHash()
	objectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, corruptHash[:constants.HashDirPrefixLength], corruptHash[constants.HashDirPrefixLength:])
	if err := os.WriteFile(filepath.Join(repoPath, existingPath), existingContent, constants.FilePerms); err != nil {
		t.Fatalf("failed to write existing file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(objectPath), constants.DirPerms); err != nil {
		t.Fatalf("failed to create corrupt object directory: %v", err)
	}
	if err := os.WriteFile(objectPath, testutils.RandomBytes(20), constants.FilePerms); err != nil {
		t.Fatalf("failed to write corrupt object: %v", err)
	}

	original := objects.TreeSnapshot{
		existingPath: {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	targetFileName := "target.txt"
	target := objects.TreeSnapshot{
		targetFileName: {
			Hash: corruptHash,
			Mode: objects.ModeRegularFile,
		},
	}
	_, err := buildApplicationPlan(repoPath, objects.NewObjectStore(repoPath), index.NewIndex(), original, target)
	if err == nil {
		t.Fatal("expected corrupt target blob to fail application planning")
	}

	expectedErrorMessage := fmt.Sprintf("failed to read blob object for target snapshot path [%s]", targetFileName)
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected error to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}

	actualContent, readErr := os.ReadFile(filepath.Join(repoPath, existingPath))
	if readErr != nil {
		t.Fatalf("failed to read existing file after planning failure: %v", readErr)
	}
	if !bytes.Equal(actualContent, existingContent) {
		t.Fatal("existing file changed during application planning")
	}
}

// TestBuildApplicationPlan_InvalidSnapshotDoesNotMutate verifies that original
// and target validation failures occur before worktree mutation.
func TestBuildApplicationPlan_InvalidSnapshotDoesNotMutate(t *testing.T) {
	testCases := []struct {
		name                 string
		original             objects.TreeSnapshot
		target               objects.TreeSnapshot
		expectedErrorMessage string
	}{
		{name: "invalid original snapshot", original: objects.TreeSnapshot{"": {Hash: testutils.RandomHash(), Mode: objects.ModeRegularFile}}, target: objects.TreeSnapshot{}, expectedErrorMessage: "invalid original snapshot"},
		{name: "invalid target snapshot", original: objects.TreeSnapshot{}, target: objects.TreeSnapshot{".gogit/config": {Hash: testutils.RandomHash(), Mode: objects.ModeRegularFile}}, expectedErrorMessage: "invalid target snapshot"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repoPath := testutils.SetupTestRepoWithInit(t)

			existingPath := "existing.txt"
			existingContent := testutils.RandomBytes(20)
			if err := os.WriteFile(filepath.Join(repoPath, existingPath), existingContent, constants.FilePerms); err != nil {
				t.Fatalf("failed to write existing file: %v", err)
			}

			_, err := buildApplicationPlan(repoPath, objects.NewObjectStore(repoPath), index.NewIndex(), testCase.original, testCase.target)
			if err == nil {
				t.Fatal("expected invalid snapshot to fail application planning")
			}
			if !strings.Contains(err.Error(), testCase.expectedErrorMessage) {
				t.Fatalf("expected error to contain [%s], got [%s]", testCase.expectedErrorMessage, err.Error())
			}
			actualContent, err := os.ReadFile(filepath.Join(repoPath, existingPath))
			if err != nil {
				t.Fatalf("failed to read existing file after planning failure: %v", err)
			}
			if !bytes.Equal(actualContent, existingContent) {
				t.Fatal("existing file changed during application planning")
			}
		})
	}
}
