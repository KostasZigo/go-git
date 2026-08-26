package worktree

import (
	"slices"
	"testing"

	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/index/indextest"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestService_UsesConstructorLoadedIndexAcrossInspectionAndApplication verifies
// that replacing the persisted index after service construction does not
// change the index observed by either state inspection or snapshot application.
func TestService_UsesConstructorLoadedIndexAcrossInspectionAndApplication(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	loadedIndex := index.NewIndex()
	indexedPath := testutils.RandomString(10)
	indexedFilePath := indextest.CreateTrackedFileContent(t, repoPath, repoPath, indexedPath, testutils.RandomBytes(20), loadedIndex)
	saveIndex(t, repoPath, loadedIndex)
	service := newWorktreeService(t, repoPath)
	saveIndex(t, repoPath, index.NewIndex())
	emptySnapshot := objects.TreeSnapshot{}

	state, err := service.ResolveRepositoryState(emptySnapshot, emptySnapshot)
	if err != nil {
		t.Fatalf("failed to resolve repository state: %v", err)
	}
	expectedStagedChanges := []Change{{Path: indexedPath, Kind: ChangeAdded}}
	if !slices.Equal(state.StagedChanges, expectedStagedChanges) {
		t.Fatalf("expected staged changes [%#v], got [%#v]", expectedStagedChanges, state.StagedChanges)
	}
	if len(state.WorktreeChanges) != 0 || len(state.Collisions) != 0 {
		t.Fatalf("expected only the constructor-loaded staged addition, got state [%#v]", state)
	}

	if err := service.ApplySnapshot(objects.NewObjectStore(repoPath), emptySnapshot, emptySnapshot); err != nil {
		t.Fatalf("failed to apply snapshot with constructor-loaded index: %v", err)
	}
	testutils.AssertFileNotExists(t, indexedFilePath)
	persistedIndex, err := index.NewManager(repoPath).Load()
	if err != nil {
		t.Fatalf("failed to load replacement index: %v", err)
	}
	if persistedIndex.CountEntries() != 0 {
		t.Fatalf("expected empty replacement index, got [%d] entries", persistedIndex.CountEntries())
	}
}
