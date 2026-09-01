package commits

import (
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestGraph_IsAncestor_LinearHistory verifies identity, transitive ancestry,
// direct ancestry, and the reverse non-ancestor case on one linear chain.
func TestGraph_IsAncestor_LinearHistory(t *testing.T) {
	fixture := newGraphTestFixture(t)
	rootCommit := fixture.storeCommit(t)
	middleCommit := fixture.storeCommit(t, rootCommit.Hash())
	tipCommit := fixture.storeCommit(t, middleCommit.Hash())

	testCases := []struct {
		name           string
		ancestorHash   string
		descendantHash string
		expected       bool
	}{
		{name: "same commit", ancestorHash: middleCommit.Hash(), descendantHash: middleCommit.Hash(), expected: true},
		{name: "root ancestor of tip", ancestorHash: rootCommit.Hash(), descendantHash: tipCommit.Hash(), expected: true},
		{name: "direct parent", ancestorHash: middleCommit.Hash(), descendantHash: tipCommit.Hash(), expected: true},
		{name: "descendant is not ancestor", ancestorHash: tipCommit.Hash(), descendantHash: rootCommit.Hash(), expected: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := fixture.graph.IsAncestor(testCase.ancestorHash, testCase.descendantHash)
			if err != nil {
				t.Fatalf("failed to query ancestry: %v", err)
			}
			if actual != testCase.expected {
				t.Fatalf("expected ancestry result [%t], got [%t]", testCase.expected, actual)
			}
		})
	}
}

// TestGraph_IsAncestor_DivergedHistory verifies that siblings are not
// ancestors of each other while their shared root remains an ancestor of both.
func TestGraph_IsAncestor_DivergedHistory(t *testing.T) {
	fixture := newGraphTestFixture(t)
	rootCommit := fixture.storeCommit(t)
	oursCommit := fixture.storeCommit(t, rootCommit.Hash())
	theirsCommit := fixture.storeCommit(t, rootCommit.Hash())

	oursIsAncestor, err := fixture.graph.IsAncestor(oursCommit.Hash(), theirsCommit.Hash())
	if err != nil {
		t.Fatalf("failed to query diverged ancestry: %v", err)
	}
	if oursIsAncestor {
		t.Fatalf("expected diverged commit [%s] not to be an ancestor of [%s]", oursCommit.Hash(), theirsCommit.Hash())
	}

	rootIsAncestor, err := fixture.graph.IsAncestor(rootCommit.Hash(), theirsCommit.Hash())
	if err != nil {
		t.Fatalf("failed to query shared-root ancestry: %v", err)
	}
	if !rootIsAncestor {
		t.Fatalf("expected root commit [%s] to be an ancestor of [%s]", rootCommit.Hash(), theirsCommit.Hash())
	}
}

// TestGraph_IsAncestor_PreviouslyMergedHistory verifies that traversal follows
// a merge commit's second parent instead of treating history as a first-parent chain.
func TestGraph_IsAncestor_PreviouslyMergedHistory(t *testing.T) {
	fixture := newGraphTestFixture(t)
	rootCommit := fixture.storeCommit(t)
	oursCommit := fixture.storeCommit(t, rootCommit.Hash())
	theirsCommit := fixture.storeCommit(t, rootCommit.Hash())
	mergeCommit := fixture.storeCommit(t, oursCommit.Hash(), theirsCommit.Hash())

	isAncestor, err := fixture.graph.IsAncestor(theirsCommit.Hash(), mergeCommit.Hash())
	if err != nil {
		t.Fatalf("failed to query second-parent ancestry: %v", err)
	}
	if !isAncestor {
		t.Fatalf("expected second parent [%s] to be an ancestor of merge commit [%s]", theirsCommit.Hash(), mergeCommit.Hash())
	}
}

// TestGraph_IsAncestor_RejectsInvalidInputHash verifies malformed endpoint
// hashes fail before object-store path construction.
func TestGraph_IsAncestor_RejectsInvalidInputHash(t *testing.T) {
	fixture := newGraphTestFixture(t)
	rootCommit := fixture.storeCommit(t)

	_, err := fixture.graph.IsAncestor("not-a-sha1", rootCommit.Hash())
	if err == nil {
		t.Fatal("expected malformed ancestor hash to fail")
	}
	if !strings.Contains(err.Error(), "invalid commit hash") {
		t.Fatalf("expected invalid-hash error, got [%v]", err)
	}
}

// TestGraph_IsAncestor_RejectsMalformedReachableHistory verifies the complete
// graph is validated even when a valid ancestor is found through the first parent.
func TestGraph_IsAncestor_RejectsMalformedReachableHistory(t *testing.T) {
	fixture := newGraphTestFixture(t)
	rootCommit := fixture.storeCommit(t)
	missingParentHash := testutils.RandomHash()
	malformedMergeCommit := fixture.storeCommit(t, rootCommit.Hash(), missingParentHash)

	_, err := fixture.graph.IsAncestor(rootCommit.Hash(), malformedMergeCommit.Hash())
	if err == nil {
		t.Fatal("expected dangling merge parent to fail ancestry traversal")
	}
	if !strings.Contains(err.Error(), missingParentHash) {
		t.Fatalf("expected error to identify missing parent [%s], got [%v]", missingParentHash, err)
	}
}

// TestGraph_FindMergeBase_LinearHistory verifies that the newer of two commits
// on one chain is selected when it is itself an ancestor of the other tip.
func TestGraph_FindMergeBase_LinearHistory(t *testing.T) {
	fixture := newGraphTestFixture(t)
	rootCommit := fixture.storeCommit(t)
	middleCommit := fixture.storeCommit(t, rootCommit.Hash())
	tipCommit := fixture.storeCommit(t, middleCommit.Hash())

	assertMergeBase(t, fixture.graph, tipCommit.Hash(), middleCommit.Hash(), middleCommit.Hash())
}

// TestGraph_FindMergeBase_DivergedHistory verifies that the shared root is
// selected after two branches independently advance from it.
func TestGraph_FindMergeBase_DivergedHistory(t *testing.T) {
	fixture := newGraphTestFixture(t)
	rootCommit := fixture.storeCommit(t)
	oursCommit := fixture.storeCommit(t, rootCommit.Hash())
	theirsCommit := fixture.storeCommit(t, rootCommit.Hash())

	assertMergeBase(t, fixture.graph, oursCommit.Hash(), theirsCommit.Hash(), rootCommit.Hash())
}

// TestGraph_FindMergeBase_PreviouslyMergedHistory verifies that a commit
// already incorporated as a merge's second parent is selected as the base.
func TestGraph_FindMergeBase_PreviouslyMergedHistory(t *testing.T) {
	fixture := newGraphTestFixture(t)
	rootCommit := fixture.storeCommit(t)
	oursCommit := fixture.storeCommit(t, rootCommit.Hash())
	theirsCommit := fixture.storeCommit(t, rootCommit.Hash())
	mergeCommit := fixture.storeCommit(t, oursCommit.Hash(), theirsCommit.Hash())

	assertMergeBase(t, fixture.graph, mergeCommit.Hash(), theirsCommit.Hash(), theirsCommit.Hash())
}

// TestGraph_FindMergeBase_RejectsUnrelatedHistories verifies independent root
// commits return an identifiable unrelated-history error.
func TestGraph_FindMergeBase_RejectsUnrelatedHistories(t *testing.T) {
	fixture := newGraphTestFixture(t)
	oursCommit := fixture.storeCommit(t)
	theirsCommit := fixture.storeCommit(t)

	mergeBaseHash, err := fixture.graph.FindMergeBase(oursCommit.Hash(), theirsCommit.Hash())
	if err == nil {
		t.Fatal("expected unrelated histories to fail merge-base selection")
	}
	if !strings.Contains(err.Error(), "commit histories are unrelated") {
		t.Fatalf("expected unrelated-history error, got [%v]", err)
	}
	if mergeBaseHash != "" {
		t.Fatalf("expected no merge base for unrelated histories, got [%s]", mergeBaseHash)
	}
}

// TestGraph_FindMergeBase_RejectsMultipleBestBases verifies a criss-cross
// history is rejected instead of choosing either incomparable shared parent.
func TestGraph_FindMergeBase_RejectsMultipleBestBases(t *testing.T) {
	fixture := newGraphTestFixture(t)
	rootCommit := fixture.storeCommit(t)
	oursCommit := fixture.storeCommit(t, rootCommit.Hash())
	theirsCommit := fixture.storeCommit(t, rootCommit.Hash())
	oursMergeCommit := fixture.storeCommit(t, oursCommit.Hash(), theirsCommit.Hash())
	theirsMergeCommit := fixture.storeCommit(t, theirsCommit.Hash(), oursCommit.Hash())

	mergeBaseHash, err := fixture.graph.FindMergeBase(oursMergeCommit.Hash(), theirsMergeCommit.Hash())
	if err == nil {
		t.Fatal("expected multiple best merge bases to fail merge-base selection")
	}
	if !strings.Contains(err.Error(), "multiple best merge bases are unsupported") {
		t.Fatalf("expected multiple-merge-base error, got [%v]", err)
	}
	if mergeBaseHash != "" {
		t.Fatalf("expected no arbitrary merge base, got [%s]", mergeBaseHash)
	}
	if !strings.Contains(err.Error(), oursCommit.Hash()) || !strings.Contains(err.Error(), theirsCommit.Hash()) {
		t.Fatalf("expected error to identify both best bases, got [%v]", err)
	}
}

// TestGraph_FindMergeBase_RejectsMalformedHistory verifies a dangling parent
// aborts base selection even when another valid parent provides a common root.
func TestGraph_FindMergeBase_RejectsMalformedHistory(t *testing.T) {
	fixture := newGraphTestFixture(t)
	rootCommit := fixture.storeCommit(t)
	missingParentHash := testutils.RandomHash()
	malformedMergeCommit := fixture.storeCommit(t, rootCommit.Hash(), missingParentHash)

	_, err := fixture.graph.FindMergeBase(rootCommit.Hash(), malformedMergeCommit.Hash())
	if err == nil {
		t.Fatal("expected dangling parent to fail merge-base selection")
	}
	if !strings.Contains(err.Error(), missingParentHash) {
		t.Fatalf("expected error to identify missing parent [%s], got [%v]", missingParentHash, err)
	}
}

// graphTestFixture stores commits against one valid tree and exposes the graph
// under test without involving refs, the index, or the worktree.
type graphTestFixture struct {
	graph    *Graph
	store    *objects.ObjectStore
	treeHash string
}

// newGraphTestFixture creates an initialized repository, stores an empty tree,
// and returns an isolated commit graph fixture.
func newGraphTestFixture(t *testing.T) *graphTestFixture {
	t.Helper()

	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)
	treeHash, err := store.StoreTreeSnapshot(objects.TreeSnapshot{})
	if err != nil {
		t.Fatalf("failed to store graph fixture tree: %v", err)
	}

	return &graphTestFixture{
		graph:    NewGraph(store),
		store:    store,
		treeHash: treeHash,
	}
}

// storeCommit creates and stores a root, ordinary, or two-parent merge commit
// according to the number of supplied parent hashes.
func (fixture *graphTestFixture) storeCommit(t *testing.T, parentHashes ...string) *objects.Commit {
	t.Helper()

	author := objects.DefaultAuthor()
	message := testutils.RandomString(20)
	var commit *objects.Commit
	var err error

	switch len(parentHashes) {
	case 0:
		commit, err = objects.NewInitialCommit(fixture.treeHash, message, author)
	case 1:
		commit, err = objects.NewCommit(fixture.treeHash, parentHashes[0], message, author)
	case 2:
		commit, err = objects.NewMergeCommit(fixture.treeHash, parentHashes[0], parentHashes[1], message, author)
	default:
		t.Fatalf("graph fixture supports at most two parents, got [%d]", len(parentHashes))
		return nil
	}
	if err != nil {
		t.Fatalf("failed to create graph fixture commit: %v", err)
	}
	if err := fixture.store.Store(commit); err != nil {
		t.Fatalf("failed to store graph fixture commit: %v", err)
	}

	return commit
}

// assertMergeBase verifies that a graph query succeeds with the expected
// unique best common ancestor.
func assertMergeBase(t *testing.T, graph *Graph, oursHash, theirsHash, expectedHash string) {
	t.Helper()

	mergeBaseHash, err := graph.FindMergeBase(oursHash, theirsHash)
	if err != nil {
		t.Fatalf("failed to find merge base: %v", err)
	}
	if mergeBaseHash != expectedHash {
		t.Fatalf("expected merge base [%s], got [%s]", expectedHash, mergeBaseHash)
	}
}
