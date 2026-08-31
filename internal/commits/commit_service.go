// Package commits orchestrates commit creation, history traversal, and
// checkout across branch references, the index, object storage, and worktree.
package commits

import (
	"fmt"

	"github.com/KostasZigo/gogit/internal/branches"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
)

// loadCommitIndex loads the repository's current staging index.
func loadCommitIndex(repoPath string) (*index.Index, error) {
	indexManager := index.NewManager(repoPath)
	idx, err := indexManager.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load index from path [%s]: %w", repoPath, err)
	}

	return idx, nil
}

// createAndStoreCommit creates an initial commit when parentHash is empty or a
// normal single-parent commit otherwise, stores it, and returns its hash.
func createAndStoreCommit(treeHash, parentHash, message string, author objects.Author, store *objects.ObjectStore) (string, error) {
	var commit *objects.Commit
	var err error
	if parentHash == "" {
		commit, err = objects.NewInitialCommit(treeHash, message, author)
	} else {
		commit, err = objects.NewCommit(treeHash, parentHash, message, author)
	}
	if err != nil {
		return "", fmt.Errorf("failed to create commit: %w", err)
	}

	if err := store.Store(commit); err != nil {
		return "", fmt.Errorf("failed to store commit: %w", err)
	}

	return commit.Hash(), nil
}

// OrchestrateCommitExecution records the current index as a commit and advances
// the active branch with a compare-and-swap update.
//
// Commit eligibility is based on tree identity rather than index entry count.
// The index is converted to a snapshot and stored first, including the
// canonical empty tree. An unborn branch rejects that empty tree, while an
// established branch rejects only a tree equal to its parent's tree. This
// allows an empty index to commit deletion of every path from a non-empty
// parent without permitting empty initial or duplicate commits.
func OrchestrateCommitExecution(repoPath string, message string, author objects.Author) (string, error) {
	idx, err := loadCommitIndex(repoPath)
	if err != nil {
		return "", err
	}

	// Resolve the branch before creating the commit so its current hash defines
	// both the parent and the expected value for the final atomic ref update.
	currentBranch, err := branches.ResolveCurrent(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve current branch: %w", err)
	}
	parentHash := currentBranch.Hash

	snapshot, err := idx.ToTreeSnapshot()
	if err != nil {
		return "", err
	}

	// Store child trees before their parents and return the root tree hash. This
	// deliberately happens before eligibility checks so empty and non-empty
	// indexes follow the same tree-building path.
	store := objects.NewObjectStore(repoPath)
	rootTreeHash, err := store.StoreTreeSnapshot(snapshot)
	if err != nil {
		return "", fmt.Errorf("failed to create commit tree directory: %w", err)
	}

	// An empty initial snapshot has no meaningful repository state to record.
	// Once a parent exists, equality with its tree is the only no-op condition.
	if parentHash == "" && len(snapshot) == 0 {
		return "", fmt.Errorf("nothing to commit")
	}
	if parentHash != "" {
		parentCommit, err := store.ReadCommit(parentHash)
		if err != nil {
			return "", fmt.Errorf("failed to read parent commit %q: %w", parentHash, err)
		}
		if parentCommit.TreeHash() == rootTreeHash {
			return "", fmt.Errorf("nothing to commit: working tree clean")
		}
	}

	commitHash, err := createAndStoreCommit(rootTreeHash, parentHash, message, author, store)
	if err != nil {
		return "", err
	}

	// Advance only if the branch still points to the resolved parent; a
	// concurrent ref change must not be overwritten by this commit.
	if err := branches.CompareAndSwap(
		repoPath,
		currentBranch.Name,
		parentHash,
		commitHash,
	); err != nil {
		return "", fmt.Errorf("failed to update branch [%s]: %w", currentBranch.Name, err)
	}

	return commitHash, nil
}
