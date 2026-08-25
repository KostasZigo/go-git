// Package commits orchestrates the high-level version control operations that
// act on or produce commit objects. It bridges the cmd layer and the lower-level
// internal packages (objects, index, utils) to implement commit creation,
// history traversal and working-tree checkout.
package commits

import (
	"fmt"

	"github.com/KostasZigo/gogit/internal/branches"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
)

// LoadIndexEntries returns all entries of staged files for repository's index
func loadIndexEntries(repoPath string) ([]*index.Entry, *index.Index, error) {
	indexManager := index.NewManager(repoPath)
	idx, err := indexManager.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load index from path [%s]: %w", repoPath, err)
	}

	return idx.GetEntryList(), idx, nil
}

// createAndStoreCommit creates and stores commit in the file system and returns the commit hash
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

// OrchestrateCommitExecution loads staged index entries, converts them to a
// tree snapshot, resolves the parent commit, creates and stores the commit,
// and advances the current branch ref.
func OrchestrateCommitExecution(repoPath string, message string, author objects.Author) (string, error) {
	// 1. load staged files entries from index
	entries, idx, err := loadIndexEntries(repoPath)
	if err != nil {
		return "", err
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("nothing to commit")
	}

	// 2. resolve the current branch and commit parent before creating objects
	currentBranch, err := branches.ResolveCurrent(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve current branch: %w", err)
	}
	parentHash := currentBranch.Hash

	// 3. convert staged index entries into the shared tree snapshot representation
	snapshot, err := idx.ToTreeSnapshot()
	if err != nil {
		return "", err
	}

	// 4. Create and store recursively and bottom up all trees from snapshot
	//  and return root tree hash
	store := objects.NewObjectStore(repoPath)
	rootTreeHash, err := store.StoreTreeSnapshot(snapshot)
	if err != nil {
		return "", fmt.Errorf("failed to create commit tree directory: %w", err)
	}

	// 5. reject commit if tree is unchanged from parent
	if parentHash != "" {
		parentCommit, err := store.ReadCommit(parentHash)
		if err != nil {
			return "", fmt.Errorf("failed to read parent commit %q: %w", parentHash, err)
		}
		if parentCommit.TreeHash() == rootTreeHash {
			return "", fmt.Errorf("nothing to commit: working tree clean")
		}
	}

	// 6. create and store commit in the filesystem
	commitHash, err := createAndStoreCommit(rootTreeHash, parentHash, message, author, store)
	if err != nil {
		return "", err
	}

	// 7. advance the current branch only if it still points to the parent commit
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
