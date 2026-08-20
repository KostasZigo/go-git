package worktree

import (
	"bytes"
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestServiceInspectCollisions_UntrackedFile verifies that an untracked file
// at a target file path is reported as an overwrite collision.
func TestServiceInspectCollisions_UntrackedFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	targetFilePath := testutils.RandomString(10)
	targetFileContent := testutils.RandomBytes(20)

	// write file in disk
	_ = testutils.CreateTestFile(t, repoPath, targetFilePath, targetFileContent)

	targetSnapshot := objects.TreeSnapshot{
		targetFilePath: {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	collisions, err := NewService(repoPath).InspectCollisions(targetSnapshot)
	if err != nil {
		t.Fatalf("failed to inspect collisions: %v", err)
	}

	expectedCollisions := []Collision{{Path: targetFilePath, Kind: CollisionUntrackedFile}}
	if !slices.Equal(collisions, expectedCollisions) {
		t.Fatalf("expected collisions to be [%#v], got [%#v]", expectedCollisions, collisions)
	}
}

// TestServiceInspectCollisions_UntrackedDirectory verifies that an untracked
// directory at a target file path is reported as an overwrite collision.
func TestServiceInspectCollisions_UntrackedDirectory(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	targetFilePath := testutils.RandomString(10)
	if err := os.MkdirAll(filepath.Join(repoPath, targetFilePath), constants.DirPerms); err != nil {
		t.Fatalf("failed to create directory [%s]: %v", targetFilePath, err)
	}

	targetSnapshot := objects.TreeSnapshot{
		targetFilePath: {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	collisions, err := NewService(repoPath).InspectCollisions(targetSnapshot)
	if err != nil {
		t.Fatalf("failed to inspect collisions: %v", err)
	}

	expectedCollisions := []Collision{{Path: targetFilePath, Kind: CollisionUntrackedDirectory}}
	if !slices.Equal(collisions, expectedCollisions) {
		t.Fatalf("expected collisions to be [%#v], got [%#v]", expectedCollisions, collisions)
	}
}

// TestServiceInspectCollisions_UntrackedParentFile verifies that an untracked
// file blocking creation of a target descendant is reported as a collision.
func TestServiceInspectCollisions_UntrackedParentFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	targetParentDir := testutils.RandomString(10)
	targetFilePath := path.Join(targetParentDir, testutils.RandomString(10))

	_ = testutils.CreateTestFile(t, repoPath, targetParentDir, testutils.RandomBytes(10))

	targetSnapshot := objects.TreeSnapshot{
		targetFilePath: {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	collisions, err := NewService(repoPath).InspectCollisions(targetSnapshot)
	if err != nil {
		t.Fatalf("failed to inspect collisions: %v", err)
	}

	expectedCollisions := []Collision{{Path: targetParentDir, Kind: CollisionParentFile}}
	if !slices.Equal(collisions, expectedCollisions) {
		t.Fatalf("expected collisions to be [%#v], got [%#v]", expectedCollisions, collisions)
	}
}

// TestServiceInspectCollisions_UntrackedDescendant verifies that an untracked
// file inside a tracked directory prevents the target from replacing that
// directory with a file.
func TestServiceInspectCollisions_UntrackedDescendant(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	targetFilePath := testutils.RandomString(10)
	diskFilePath := filepath.Join(targetFilePath, testutils.RandomString(20))
	indexFilePath := filepath.Join(targetFilePath, testutils.RandomString(20))

	// Write disk file
	if err := os.MkdirAll(filepath.Join(repoPath, targetFilePath), constants.DirPerms); err != nil {
		t.Fatalf("failed to create directory in disk [%s]: %v", targetFilePath, err)
	}
	_ = testutils.CreateTestFile(t, repoPath, diskFilePath, testutils.RandomBytes(100))

	// Write index
	idx := index.NewIndex()
	addIndexEntry(t, idx, objects.ModeRegularFile, testutils.RandomHash(), filepath.ToSlash(indexFilePath), time.Now().UTC())
	saveIndex(t, repoPath, idx)

	// Write target snapshot
	targetSnapshot := objects.TreeSnapshot{
		targetFilePath: {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	collisions, err := NewService(repoPath).InspectCollisions(targetSnapshot)
	if err != nil {
		t.Fatalf("failed to inspect target snapshot for collisions: %v", err)
	}

	expectedCollisions := []Collision{{Path: filepath.ToSlash(diskFilePath), Kind: CollisionUntrackedDescendant}}
	if !slices.Equal(collisions, expectedCollisions) {
		t.Fatalf("expected collisions to be [%#v], got [%#v]", expectedCollisions, collisions)
	}
}

// TestServiceInspectCollisions_TrackedFileBecomesDirectory verifies that a
// tracked file may be replaced by the directory required by a target child.
func TestServiceInspectCollisions_TrackedFileBecomesDirectory(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	diskFileName := testutils.RandomString(10)
	_ = testutils.CreateTestFile(t, repoPath, diskFileName, testutils.RandomBytes(10))

	idx := index.NewIndex()
	addIndexEntry(t, idx, objects.ModeRegularFile, testutils.RandomHash(), diskFileName, time.Now().UTC())
	saveIndex(t, repoPath, idx)

	targetSnapshot := objects.TreeSnapshot{
		path.Join(diskFileName, testutils.RandomString(10)): {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	collisions, err := NewService(repoPath).InspectCollisions(targetSnapshot)
	if err != nil {
		t.Fatalf("failed to inspect target snapshot for collisions: %v", err)
	}
	if len(collisions) != 0 {
		t.Fatalf("expected no collisions, got [%d]: [%#v]", len(collisions), collisions)
	}
}

// TestServiceInspectCollisions_TrackedDirectoryBecomesFile verifies that an
// implicit tracked directory may become a file when it has no untracked files.
func TestServiceInspectCollisions_TrackedDirectoryBecomesFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	targetFileName := testutils.RandomString(10)

	if err := os.MkdirAll(filepath.Join(repoPath, targetFileName), constants.DirPerms); err != nil {
		t.Fatalf("failed to create directory [%s]: %v", targetFileName, err)
	}

	// create file with targetFileName as parent dir
	diskFileName := testutils.RandomString(10)
	_ = testutils.CreateTestFile(t, repoPath, filepath.Join(targetFileName, diskFileName), testutils.RandomBytes(10))

	idx := index.NewIndex()
	addIndexEntry(t, idx, objects.ModeRegularFile, testutils.RandomHash(), path.Join(targetFileName, diskFileName), time.Now().UTC())
	saveIndex(t, repoPath, idx)

	targetSnapshot := objects.TreeSnapshot{
		targetFileName: {
			Mode: objects.ModeRegularFile,
			Hash: testutils.RandomHash(),
		},
	}

	collisions, err := NewService(repoPath).InspectCollisions(targetSnapshot)
	if err != nil {
		t.Fatalf("failed to inspect collisions: %v", err)
	}
	if len(collisions) != 0 {
		t.Fatalf("expected no collisions, got [%d]: [%#v]", len(collisions), collisions)
	}
}

// TestServiceInspectCollisions_AllowsUnrelatedUntrackedContent verifies that
// unrelated untracked files and directories do not block the target.
func TestServiceInspectCollisions_AllowsUnrelatedUntrackedContent(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	targetPath := testutils.RandomString(10)
	unrelatedFilePath := testutils.RandomString(11)
	unrelatedNestedPath := filepath.Join(testutils.RandomString(12), testutils.RandomString(13))

	if err := os.MkdirAll(filepath.Join(repoPath, filepath.Dir(unrelatedNestedPath)), constants.DirPerms); err != nil {
		t.Fatalf("failed to created directory [%s]: %v", unrelatedNestedPath, err)
	}

	unrelatedFileContent := testutils.RandomBytes(20)
	unrelatedNestedContent := testutils.RandomBytes(20)
	_ = testutils.CreateTestFile(t, repoPath, unrelatedFilePath, unrelatedFileContent)
	_ = testutils.CreateTestFile(t, repoPath, unrelatedNestedPath, unrelatedNestedContent)
	_ = testutils.CreateTestFile(t, repoPath, targetPath, testutils.RandomBytes(20))

	idx := index.NewIndex()
	addIndexEntry(t, idx, objects.ModeRegularFile, testutils.RandomHash(), targetPath, time.Now().UTC())
	saveIndex(t, repoPath, idx)

	targetSnapshot := objects.TreeSnapshot{
		targetPath: {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	collisions, err := NewService(repoPath).InspectCollisions(targetSnapshot)
	if err != nil {
		t.Fatalf("failed to inspect collisions: %v", err)
	}
	if len(collisions) != 0 {
		t.Fatalf("expected no collisions, got %#v", collisions)
	}

	actualFileContent, err := os.ReadFile(filepath.Join(repoPath, unrelatedFilePath))
	if err != nil {
		t.Fatalf("failed to read unrelated file: %v", err)
	}
	if !bytes.Equal(actualFileContent, unrelatedFileContent) {
		t.Fatal("unrelated file content changed during inspection")
	}

	actualNestedContent, err := os.ReadFile(filepath.Join(repoPath, unrelatedNestedPath))
	if err != nil {
		t.Fatalf("failed to read unrelated nested file: %v", err)
	}
	if !bytes.Equal(actualNestedContent, unrelatedNestedContent) {
		t.Fatal("unrelated nested content changed during inspection")
	}
}

// TestServiceInspectCollisions_RepositoryMetadataTargetIsRejected verifies
// that internal repository metadata can never be a target snapshot path.
func TestServiceInspectCollisions_RepositoryMetadataTargetIsRejected(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	targetSnapshot := objects.TreeSnapshot{
		path.Join(constants.Gogit, "objects", testutils.RandomString(10)): {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	_, err := NewService(repoPath).InspectCollisions(targetSnapshot)
	if !errors.Is(err, ErrRepositoryMetadataTarget) {
		t.Fatalf("expected repository metadata target error, got %v", err)
	}
}

// TestServiceInspectCollisions_DeduplicatesAndSorts verifies that repeated
// structural findings are returned once in stable path order.
func TestServiceInspectCollisions_DeduplicatesAndSorts(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	parentPath := "parent"
	_ = testutils.CreateTestFile(t, repoPath, parentPath, testutils.RandomBytes(20))

	targetSnapshot := objects.TreeSnapshot{
		"z.txt": {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
		path.Join(parentPath, "second.txt"): {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
		path.Join(parentPath, "first.txt"): {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	_ = testutils.CreateTestFile(t, repoPath, "z.txt", testutils.RandomBytes(20))

	collisions, err := NewService(repoPath).InspectCollisions(targetSnapshot)
	if err != nil {
		t.Fatalf("failed to inspect collisions: %v", err)
	}

	expectedCollisions := []Collision{
		{Path: parentPath, Kind: CollisionParentFile},
		{Path: "z.txt", Kind: CollisionUntrackedFile},
	}
	if !slices.Equal(collisions, expectedCollisions) {
		t.Fatalf(
			"expected collisions %#v, got %#v",
			expectedCollisions,
			collisions,
		)
	}
}

// TestServiceInspectCollisions_RejectedPreflightDoesNotMutateState verifies
// that collision detection leaves the working tree and index unchanged.
func TestServiceInspectCollisions_RejectedPreflightDoesNotMutateState(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	trackedPath := testutils.RandomString(10)
	untrackedPath := testutils.RandomString(10)
	trackedContent := testutils.RandomBytes(20)
	untrackedContent := testutils.RandomBytes(20)

	_ = testutils.CreateTestFile(t, repoPath, trackedPath, trackedContent)
	_ = testutils.CreateTestFile(t, repoPath, untrackedPath, untrackedContent)

	idx := index.NewIndex()
	addIndexEntry(t, idx, objects.ModeRegularFile, testutils.RandomHash(), trackedPath, time.Now().UTC())
	saveIndex(t, repoPath, idx)

	indexPath := filepath.Join(repoPath, constants.Gogit, constants.Index)
	indexBefore, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index before inspection: %v", err)
	}

	targetSnapshot := objects.TreeSnapshot{
		untrackedPath: {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	collisions, err := NewService(repoPath).InspectCollisions(targetSnapshot)
	if err != nil {
		t.Fatalf("failed to inspect collisions: %v", err)
	}

	expectedCollisions := []Collision{
		{
			Path: untrackedPath,
			Kind: CollisionUntrackedFile,
		},
	}
	if !slices.Equal(collisions, expectedCollisions) {
		t.Fatalf("expected collisions %#v, got %#v", expectedCollisions, collisions)
	}

	actualTrackedContent, err := os.ReadFile(filepath.Join(repoPath, trackedPath))
	if err != nil {
		t.Fatalf("failed to read tracked file after inspection: %v", err)
	}
	if !bytes.Equal(actualTrackedContent, trackedContent) {
		t.Fatal("tracked file content changed during rejected preflight")
	}

	actualUntrackedContent, err := os.ReadFile(filepath.Join(repoPath, untrackedPath))
	if err != nil {
		t.Fatalf("failed to read untracked file after inspection: %v", err)
	}
	if !bytes.Equal(actualUntrackedContent, untrackedContent) {
		t.Fatal("untracked file content changed during rejected preflight")
	}

	indexAfter, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index after inspection: %v", err)
	}
	if !bytes.Equal(indexAfter, indexBefore) {
		t.Fatal("index bytes changed during rejected preflight")
	}
}

// TestServiceInspectCollisions_TrackedFileReplacedByDirectoryWithUntrackedChild
// verifies that force cannot discard untracked content inside a directory that
// replaced a directly tracked file.
func TestServiceInspectCollisions_TrackedFileReplacedByDirectoryWithUntrackedChild(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	trackedPath := testutils.RandomString(10)
	untrackedChildPath := filepath.Join(trackedPath, testutils.RandomString(10))

	idx := index.NewIndex()
	addIndexEntry(t, idx, objects.ModeRegularFile, testutils.RandomHash(), trackedPath, time.Now().UTC())
	saveIndex(t, repoPath, idx)

	if err := os.MkdirAll(filepath.Join(repoPath, filepath.Dir(untrackedChildPath)), constants.DirPerms); err != nil {
		t.Fatalf("failed to create directory [%s], %v", filepath.Dir(untrackedChildPath), err)
	}
	_ = testutils.CreateTestFile(t, repoPath, untrackedChildPath, testutils.RandomBytes(20))

	targetSnapshot := objects.TreeSnapshot{
		trackedPath: {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	collisions, err := NewService(repoPath).InspectCollisions(targetSnapshot)
	if err != nil {
		t.Fatalf("failed to inspect collisions: %v", err)
	}

	expectedCollisions := []Collision{
		{
			Path: filepath.ToSlash(untrackedChildPath),
			Kind: CollisionUntrackedDescendant,
		},
	}
	if !slices.Equal(collisions, expectedCollisions) {
		t.Fatalf("expected collisions [%#v], got [%#v]", expectedCollisions, collisions)
	}
}

// TestServiceInspectCollisions_RemovedTrackedPathWithUntrackedDescendant
// verifies that removing a tracked path cannot discard an untracked file
// contained in a directory that replaced it on disk.
func TestServiceInspectCollisions_RemovedTrackedPathWithUntrackedDescendant(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	trackedPath := testutils.RandomString(10)
	untrackedChildPath := filepath.Join(trackedPath, testutils.RandomString(10))

	idx := index.NewIndex()
	addIndexEntry(t, idx, objects.ModeRegularFile, testutils.RandomHash(), trackedPath, time.Now().UTC())
	saveIndex(t, repoPath, idx)

	if err := os.MkdirAll(filepath.Join(repoPath, filepath.Dir(untrackedChildPath)), constants.DirPerms); err != nil {
		t.Fatalf("failed to create directory [%s], %v", filepath.Dir(untrackedChildPath), err)
	}
	_ = testutils.CreateTestFile(t, repoPath, untrackedChildPath, testutils.RandomBytes(20))

	targetSnapshot := objects.TreeSnapshot{}
	collisions, err := NewService(repoPath).InspectCollisions(targetSnapshot)
	if err != nil {
		t.Fatalf("failed to inspect collisions: %v", err)
	}

	expectedCollisions := []Collision{
		{
			Path: filepath.ToSlash(untrackedChildPath),
			Kind: CollisionUntrackedDescendant,
		},
	}
	if !slices.Equal(collisions, expectedCollisions) {
		t.Fatalf("expected collisions [%#v], got [%#v]", expectedCollisions, collisions)
	}
}

// TestServiceInspectCollisions_RemovedTrackedRegularFile verifies that a
// regular tracked file absent from the target produces no collision.
func TestServiceInspectCollisions_RemovedTrackedRegularFile(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	trackedPath := testutils.RandomString(10)
	_ = testutils.CreateTestFile(t, repoPath, trackedPath, testutils.RandomBytes(20))

	idx := index.NewIndex()
	addIndexEntry(t, idx, objects.ModeRegularFile, testutils.RandomHash(), trackedPath, time.Now().UTC())
	saveIndex(t, repoPath, idx)

	collisions, err := NewService(repoPath).InspectCollisions(
		objects.TreeSnapshot{},
	)
	if err != nil {
		t.Fatalf("failed to inspect collisions: %v", err)
	}
	if len(collisions) != 0 {
		t.Fatalf("expected no collisions, got [%#v]", collisions)
	}
}
