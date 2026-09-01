package commits

import (
	"fmt"
	"sort"

	"github.com/KostasZigo/gogit/internal/hasher"
	"github.com/KostasZigo/gogit/internal/objects"
)

// Graph provides commit-history queries backed by an object store.
type Graph struct {
	store *objects.ObjectStore
}

// NewGraph creates a commit graph that reads history from store.
func NewGraph(store *objects.ObjectStore) *Graph {
	return &Graph{store: store}
}

// readCommit validates a commit hash before delegating to the object store.
// Validation keeps malformed hashes away from the object store's hash-based
// path construction and gives every graph query the same failure behavior.
func (graph *Graph) readCommit(commitHash string) (*objects.Commit, error) {
	if graph == nil || graph.store == nil {
		return nil, fmt.Errorf("commit graph requires an object store")
	}
	if !hasher.IsValidSHA1(commitHash) {
		return nil, fmt.Errorf("invalid commit hash [%s]", commitHash)
	}

	commit, err := graph.store.ReadCommit(commitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to read commit [%s]: %w", commitHash, err)
	}
	return commit, nil
}

// collectAncestors reads startHash and every commit reachable through any
// parent. The returned map includes the starting commit and doubles as the
// visited set, so shared ancestors and malformed cycles cannot be processed
// indefinitely.
func (graph *Graph) collectAncestors(startHash string) (map[string]*objects.Commit, error) {
	commitsByHash := make(map[string]*objects.Commit)
	pendingHashes := []string{startHash}

	// Process an explicit stack instead of recursing so graph depth is bounded
	// by available memory rather than the goroutine call stack. Parents are
	// pushed in reverse order to preserve serialized parent order when read.
	for len(pendingHashes) > 0 {
		lastIndex := len(pendingHashes) - 1
		commitHash := pendingHashes[lastIndex]
		pendingHashes = pendingHashes[:lastIndex]

		if _, visited := commitsByHash[commitHash]; visited {
			continue
		}

		commit, err := graph.readCommit(commitHash)
		if err != nil {
			return nil, err
		}
		commitsByHash[commitHash] = commit

		parentHashes := commit.ParentHashes()
		for parentIndex := len(parentHashes) - 1; parentIndex >= 0; parentIndex-- {
			parentHash := parentHashes[parentIndex]
			if _, visited := commitsByHash[parentHash]; !visited {
				pendingHashes = append(pendingHashes, parentHash)
			}
		}
	}

	return commitsByHash, nil
}

// IsAncestor reports whether ancestorHash is the same commit as descendantHash
// or is reachable through any of the descendant's parent chains. Both endpoint
// commits and the descendant's complete reachable history must be readable.
func (graph *Graph) IsAncestor(ancestorHash, descendantHash string) (bool, error) {
	if _, err := graph.readCommit(ancestorHash); err != nil {
		return false, fmt.Errorf("failed to resolve ancestor commit: %w", err)
	}

	descendantAncestors, err := graph.collectAncestors(descendantHash)
	if err != nil {
		return false, fmt.Errorf("failed to traverse descendant history: %w", err)
	}

	_, isAncestor := descendantAncestors[ancestorHash]
	return isAncestor, nil
}

// findCommonAncestors returns hashes reachable from both commit histories.
func findCommonAncestors(oursAncestors, theirsAncestors map[string]*objects.Commit) map[string]struct{} {
	commonHashes := make(map[string]struct{})
	for commitHash := range oursAncestors {
		if _, exists := theirsAncestors[commitHash]; exists {
			commonHashes[commitHash] = struct{}{}
		}
	}
	return commonHashes
}

// findBestCommonAncestors removes every common commit that is an ancestor of
// another common commit. The remaining hashes are the graph's best merge-base
// candidates and are sorted to keep diagnostics deterministic.
func findBestCommonAncestors(commonHashes map[string]struct{}, commitsByHash map[string]*objects.Commit) []string {
	dominatedHashes := make(map[string]struct{})

	// Walk backward from every common commit. Any different common commit
	// reached from it is older and therefore cannot be a best merge base.
	// This uses only the already validated ancestry map, avoiding object-store
	// reads and preserving the iterative traversal guarantee.
	for commonHash := range commonHashes {
		visitedHashes := map[string]struct{}{commonHash: {}}
		pendingHashes := commitsByHash[commonHash].ParentHashes()

		for len(pendingHashes) > 0 {
			lastIndex := len(pendingHashes) - 1
			commitHash := pendingHashes[lastIndex]
			pendingHashes = pendingHashes[:lastIndex]

			if _, visited := visitedHashes[commitHash]; visited {
				continue
			}
			visitedHashes[commitHash] = struct{}{}

			if _, common := commonHashes[commitHash]; common {
				dominatedHashes[commitHash] = struct{}{}
			}

			commit, exists := commitsByHash[commitHash]
			if exists {
				pendingHashes = append(pendingHashes, commit.ParentHashes()...)
			}
		}
	}

	bestHashes := make([]string, 0, len(commonHashes)-len(dominatedHashes))
	for commonHash := range commonHashes {
		if _, dominated := dominatedHashes[commonHash]; !dominated {
			bestHashes = append(bestHashes, commonHash)
		}
	}
	sort.Strings(bestHashes)
	return bestHashes
}

// FindMergeBase returns the unique best common ancestor of oursHash and
// theirsHash. It rejects unrelated histories and histories with several
// incomparable best bases because choosing one arbitrarily would produce an
// incorrect three-way merge base.
func (graph *Graph) FindMergeBase(oursHash, theirsHash string) (string, error) {
	oursAncestors, err := graph.collectAncestors(oursHash)
	if err != nil {
		return "", fmt.Errorf("failed to traverse ours history: %w", err)
	}
	theirsAncestors, err := graph.collectAncestors(theirsHash)
	if err != nil {
		return "", fmt.Errorf("failed to traverse theirs history: %w", err)
	}

	commonHashes := findCommonAncestors(oursAncestors, theirsAncestors)
	if len(commonHashes) == 0 {
		return "", fmt.Errorf("commit histories are unrelated: commits [%s] and [%s]", oursHash, theirsHash)
	}

	bestHashes := findBestCommonAncestors(commonHashes, oursAncestors)
	if len(bestHashes) == 0 {
		return "", fmt.Errorf("failed to determine a best merge base for commits [%s] and [%s]", oursHash, theirsHash)
	}
	if len(bestHashes) > 1 {
		return "", fmt.Errorf("multiple best merge bases are unsupported: %v", bestHashes)
	}

	return bestHashes[0], nil
}
