package commits

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/index/indextest"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
	"github.com/KostasZigo/gogit/internal/worktree"
)

// TestCheckout_Orchestrate_SameCommitBranchSwitchPreservesLocalState verifies
// that switching branch names at the same commit updates only HEAD, preserving
// staged changes, worktree changes, branch refs, and exact index bytes.
func TestCheckout_Orchestrate_SameCommitBranchSwitchPreservesLocalState(t *testing.T) {
	trackedPath := testutils.RandomString(10)
	trackedContent := testutils.RandomBytes(20)
	fixture := newCheckoutFixture(t, map[string]checkoutFile{trackedPath: {content: trackedContent, mode: objects.ModeRegularFile}}, nil)
	testutils.WriteRefFile(t, fixture.repoPath, fixture.targetBranch, fixture.headCommit.Hash())

	dirtyContent := testutils.RandomBytes(21)
	trackedFilePath := testutils.CreateTestFile(t, fixture.repoPath, trackedPath, dirtyContent)
	idxManager := index.NewManager(fixture.repoPath)
	idx, err := idxManager.Load()
	if err != nil {
		t.Fatalf("failed to load index before branch switch: %v", err)
	}
	stagedPath := testutils.RandomString(11)
	stagedContent := testutils.RandomBytes(22)
	stagedFilePath := indextest.CreateTrackedFileContent(t, fixture.repoPath, fixture.repoPath, stagedPath, stagedContent, idx)
	if err := idxManager.Save(idx); err != nil {
		t.Fatalf("failed to persist staged addition: %v", err)
	}
	metadataBefore := captureCheckoutMetadata(t, fixture.repoPath, constants.DefaultBranch, fixture.targetBranch)

	if err := OrchestrateCheckoutExecution(fixture.repoPath, fixture.targetBranch, false); err != nil {
		t.Fatalf("same-commit branch switch failed: %v", err)
	}

	testutils.AssertHEADContent(t, fixture.repoPath, constants.DefaultRefPrefix+fixture.targetBranch+"\n")
	assertCheckoutIndexAndRefsUnchanged(t, fixture.repoPath, metadataBefore)
	testutils.AssertFileContent(t, trackedFilePath, dirtyContent)
	testutils.AssertFileContent(t, stagedFilePath, stagedContent)
}

// TestCheckout_Orchestrate_StagedChangeRejectsWithoutForce verifies that a
// staged addition returns structured preflight state and leaves HEAD, refs,
// index bytes, and worktree files unchanged.
func TestCheckout_Orchestrate_StagedChangeRejectsWithoutForce(t *testing.T) {
	trackedPath := testutils.RandomString(10)
	trackedContent := testutils.RandomBytes(20)
	targetContent := testutils.RandomBytes(21)
	fixture := newCheckoutFixture(t, map[string]checkoutFile{trackedPath: {content: trackedContent, mode: objects.ModeRegularFile}}, map[string]checkoutFile{trackedPath: {content: targetContent, mode: objects.ModeRegularFile}})

	idxManager := index.NewManager(fixture.repoPath)
	idx, err := idxManager.Load()
	if err != nil {
		t.Fatalf("failed to load index before staging addition: %v", err)
	}
	stagedPath := testutils.RandomString(11)
	stagedContent := testutils.RandomBytes(22)
	stagedFilePath := indextest.CreateTrackedFileContent(t, fixture.repoPath, fixture.repoPath, stagedPath, stagedContent, idx)
	if err := idxManager.Save(idx); err != nil {
		t.Fatalf("failed to persist staged addition: %v", err)
	}
	metadataBefore := captureCheckoutMetadata(t, fixture.repoPath, constants.DefaultBranch, fixture.targetBranch)

	state := requirePreflightError(t, OrchestrateCheckoutExecution(fixture.repoPath, fixture.targetBranch, false))

	expectedChanges := []worktree.Change{{Path: stagedPath, Kind: worktree.ChangeAdded}}
	if !slices.Equal(state.StagedChanges, expectedChanges) {
		t.Fatalf("expected staged changes [%#v], got [%#v]", expectedChanges, state.StagedChanges)
	}
	if len(state.WorktreeChanges) != 0 || len(state.Collisions) != 0 {
		t.Fatalf("expected only a staged change, got state [%#v]", state)
	}
	assertCheckoutMetadataUnchanged(t, fixture.repoPath, metadataBefore)
	testutils.AssertFileContent(t, filepath.Join(fixture.repoPath, trackedPath), trackedContent)
	testutils.AssertFileContent(t, stagedFilePath, stagedContent)
}

// TestCheckout_Orchestrate_ModifiedTrackedFileRejectsWithoutForce verifies that
// modified tracked content returns structured worktree state without changing
// HEAD, branch refs, index bytes, or the local modification.
func TestCheckout_Orchestrate_ModifiedTrackedFileRejectsWithoutForce(t *testing.T) {
	trackedPath := testutils.RandomString(10)
	trackedContent := testutils.RandomBytes(20)
	fixture := newCheckoutFixture(t, map[string]checkoutFile{trackedPath: {content: trackedContent, mode: objects.ModeRegularFile}}, map[string]checkoutFile{trackedPath: {content: testutils.RandomBytes(21), mode: objects.ModeRegularFile}})

	dirtyContent := testutils.RandomBytes(22)
	trackedFilePath := testutils.CreateTestFile(t, fixture.repoPath, trackedPath, dirtyContent)
	metadataBefore := captureCheckoutMetadata(t, fixture.repoPath, constants.DefaultBranch, fixture.targetBranch)

	state := requirePreflightError(t, OrchestrateCheckoutExecution(fixture.repoPath, fixture.targetBranch, false))

	expectedChanges := []worktree.Change{{Path: trackedPath, Kind: worktree.ChangeContentModified}}
	if !slices.Equal(state.WorktreeChanges, expectedChanges) {
		t.Fatalf("expected worktree changes [%#v], got [%#v]", expectedChanges, state.WorktreeChanges)
	}
	if len(state.StagedChanges) != 0 || len(state.Collisions) != 0 {
		t.Fatalf("expected only a worktree change, got state [%#v]", state)
	}
	assertCheckoutMetadataUnchanged(t, fixture.repoPath, metadataBefore)
	testutils.AssertFileContent(t, trackedFilePath, dirtyContent)
}

// TestCheckout_Orchestrate_DeletedTrackedFileRejectsWithoutForce verifies that
// deleting an indexed file returns a structured deletion and leaves HEAD,
// branch refs, index bytes, and the missing worktree path unchanged.
func TestCheckout_Orchestrate_DeletedTrackedFileRejectsWithoutForce(t *testing.T) {
	trackedPath := testutils.RandomString(10)
	fixture := newCheckoutFixture(t, map[string]checkoutFile{trackedPath: {content: testutils.RandomBytes(20), mode: objects.ModeRegularFile}}, map[string]checkoutFile{testutils.RandomString(11): {content: testutils.RandomBytes(21), mode: objects.ModeRegularFile}})

	trackedFilePath := filepath.Join(fixture.repoPath, trackedPath)
	if err := os.Remove(trackedFilePath); err != nil {
		t.Fatalf("failed to delete tracked file: %v", err)
	}
	metadataBefore := captureCheckoutMetadata(t, fixture.repoPath, constants.DefaultBranch, fixture.targetBranch)

	state := requirePreflightError(t, OrchestrateCheckoutExecution(fixture.repoPath, fixture.targetBranch, false))

	expectedChanges := []worktree.Change{{Path: trackedPath, Kind: worktree.ChangeDeleted}}
	if !slices.Equal(state.WorktreeChanges, expectedChanges) {
		t.Fatalf("expected worktree changes [%#v], got [%#v]", expectedChanges, state.WorktreeChanges)
	}
	if len(state.StagedChanges) != 0 || len(state.Collisions) != 0 {
		t.Fatalf("expected only a worktree deletion, got state [%#v]", state)
	}
	assertCheckoutMetadataUnchanged(t, fixture.repoPath, metadataBefore)
	testutils.AssertFileNotExists(t, trackedFilePath)
}

// TestCheckout_Orchestrate_ForceDiscardsTrackedChanges verifies that force
// permits a staged deletion, modified tracked file, or deleted tracked file,
// then applies the target snapshot and rebuilds its exact index metadata.
func TestCheckout_Orchestrate_ForceDiscardsTrackedChanges(t *testing.T) {
	tests := map[string]func(*testing.T, *checkoutFixture, string){
		"staged deletion": func(t *testing.T, fixture *checkoutFixture, trackedPath string) {
			t.Helper()
			idxManager := index.NewManager(fixture.repoPath)
			idx, err := idxManager.Load()
			if err != nil {
				t.Fatalf("failed to load index before staged deletion: %v", err)
			}
			idx.RemoveEntry(trackedPath)
			if err := idxManager.Save(idx); err != nil {
				t.Fatalf("failed to persist staged deletion: %v", err)
			}
			if err := os.Remove(filepath.Join(fixture.repoPath, trackedPath)); err != nil {
				t.Fatalf("failed to remove staged path from disk: %v", err)
			}
		},
		"modified tracked file": func(t *testing.T, fixture *checkoutFixture, trackedPath string) {
			t.Helper()
			testutils.CreateTestFile(t, fixture.repoPath, trackedPath, testutils.RandomBytes(31))
		},
		"deleted tracked file": func(t *testing.T, fixture *checkoutFixture, trackedPath string) {
			t.Helper()
			if err := os.Remove(filepath.Join(fixture.repoPath, trackedPath)); err != nil {
				t.Fatalf("failed to delete tracked file: %v", err)
			}
		},
	}

	for name, arrangeChange := range tests {
		t.Run(name, func(t *testing.T) {
			trackedPath := testutils.RandomString(10)
			targetPath := testutils.RandomString(11)
			targetContent := testutils.RandomBytes(21)
			fixture := newCheckoutFixture(t, map[string]checkoutFile{trackedPath: {content: testutils.RandomBytes(20), mode: objects.ModeRegularFile}}, map[string]checkoutFile{targetPath: {content: targetContent, mode: objects.ModeRegularFile}})
			arrangeChange(t, fixture, trackedPath)
			metadataBefore := captureCheckoutMetadata(t, fixture.repoPath, constants.DefaultBranch, fixture.targetBranch)

			if err := OrchestrateCheckoutExecution(fixture.repoPath, fixture.targetBranch, true); err != nil {
				t.Fatalf("forced checkout failed: %v", err)
			}

			testutils.AssertHEADContent(t, fixture.repoPath, constants.DefaultRefPrefix+fixture.targetBranch+"\n")
			assertCheckoutRefsUnchanged(t, fixture.repoPath, metadataBefore)
			testutils.AssertFileNotExists(t, filepath.Join(fixture.repoPath, trackedPath))
			testutils.AssertFileContent(t, filepath.Join(fixture.repoPath, targetPath), targetContent)
			assertIndexMatchesSnapshot(t, fixture.repoPath, fixture.targetSnapshot)
		})
	}
}

// TestCheckout_Orchestrate_ExecutableTargetPreservesMode verifies that checkout
// writes executable content, records its Git mode and filesystem metadata in
// the index, applies executable permissions where supported, and updates HEAD.
func TestCheckout_Orchestrate_ExecutableTargetPreservesMode(t *testing.T) {
	headPath := testutils.RandomString(10)
	targetPath := filepath.ToSlash(filepath.Join(testutils.RandomString(11), testutils.RandomString(12)))
	targetContent := testutils.RandomBytes(21)
	fixture := newCheckoutFixture(t, map[string]checkoutFile{headPath: {content: testutils.RandomBytes(20), mode: objects.ModeRegularFile}}, map[string]checkoutFile{targetPath: {content: targetContent, mode: objects.ModeExecutable}})
	metadataBefore := captureCheckoutMetadata(t, fixture.repoPath, constants.DefaultBranch, fixture.targetBranch)

	if err := OrchestrateCheckoutExecution(fixture.repoPath, fixture.targetBranch, false); err != nil {
		t.Fatalf("checkout to executable target failed: %v", err)
	}

	targetFilePath := filepath.Join(fixture.repoPath, filepath.FromSlash(targetPath))
	testutils.AssertFileContent(t, targetFilePath, targetContent)
	testutils.AssertFileNotExists(t, filepath.Join(fixture.repoPath, headPath))
	testutils.AssertHEADContent(t, fixture.repoPath, constants.DefaultRefPrefix+fixture.targetBranch+"\n")
	assertCheckoutRefsUnchanged(t, fixture.repoPath, metadataBefore)
	assertIndexMatchesSnapshot(t, fixture.repoPath, fixture.targetSnapshot)

	if runtime.GOOS != "windows" {
		fileInfo, err := os.Stat(targetFilePath)
		if err != nil {
			t.Fatalf("failed to stat executable target: %v", err)
		}
		if fileInfo.Mode().Perm() != constants.ExecutableFilePerms.Perm() {
			t.Fatalf("expected executable permissions [%o], got [%o]", constants.ExecutableFilePerms.Perm(), fileInfo.Mode().Perm())
		}
	}
}
