package commits

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/utils"
)

// ResolvedTarget holds the result of resolving a checkout target string.
// IsBranch indicates whether the target was resolved via a branch ref file or direct commit.
// Hash contains the commit hash the target points to.
type ResolvedTarget struct {
	IsBranch bool
	Hash     string
}

// ResolveTarget resolves a checkout target string to a commit hash.
// Resolution order follows convention: branch refs are checked first,
// then the object store for direct commit hashes.
// Returns an error if the target is empty, not found, or unreadable.
func ResolveTarget(repoPath, target string) (resolvedTarget *ResolvedTarget, err error) {
	if target == "" {
		return nil, fmt.Errorf("checkout target cannot be empty")
	}

	resolvedTarget, err = searchForTargetInRefs(repoPath, target)
	if err != nil {
		return nil, fmt.Errorf("failed search for checkout target [%s] in refs: %w", target, err)
	}
	if resolvedTarget != nil {
		return resolvedTarget, nil
	}

	resolvedTarget, err = searchForTargetInCommitObjects(repoPath, target)
	if err != nil {
		return nil, fmt.Errorf("failed search for checkout target [%s] in commit objects: %w", target, err)
	}
	if resolvedTarget == nil {
		return nil, fmt.Errorf("checkout target [%s] not found as branch or commit", target)
	}
	return resolvedTarget, nil
}

// searchForTargetInRefs checks if target matches a branch name under refs/heads/.
// Returns nil, nil if no matching branch exists (not an error).
// Returns an error only for filesystem failures.
func searchForTargetInRefs(repoPath, target string) (*ResolvedTarget, error) {
	branchPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, target)
	content, err := os.ReadFile(branchPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read branch file: %w", err)
	}

	hash := strings.TrimSpace(string(content))
	return &ResolvedTarget{
		IsBranch: true,
		Hash:     hash,
	}, nil
}

// searchForTargetInCommitObjects checks if target is a valid SHA-1 hash
// pointing to an existing commit object in the store.
// Returns nil, nil if the target is not a valid hash or the object does not exist.
// Returns an error only if the object exists but cannot be read as a commit.
func searchForTargetInCommitObjects(repoPath, target string) (*ResolvedTarget, error) {
	if !utils.IsValidSHA1Hash(target) {
		return nil, nil
	}

	store := objects.NewObjectStore(repoPath)
	if !store.Exists(target) {
		return nil, nil
	}

	commit, err := store.ReadCommit(target)
	if err != nil {
		return nil, fmt.Errorf("failed to read commit for checkout target [%s]: %w", target, err)
	}
	return &ResolvedTarget{
		IsBranch: false,
		Hash:     commit.Hash(),
	}, nil
}

// RestoreTree reads a stored tree object and reconstructs its contents on the
// filesystem under repoPath. Constructs the ObjectStore internally and delegates
// to the recursive walker.
func RestoreTree(repoPath, treeHash string) error {
	store := objects.NewObjectStore(repoPath)
	return restoreTreeRecursive(repoPath, treeHash, store)
}

// restoreTreeRecursive walks a tree object and writes its entries to dirPath.
// Subtree entries are created as directories and descended into recursively.
// Blob entries are written as files via the object store.
func restoreTreeRecursive(dirPath, treeHash string, store *objects.ObjectStore) error {
	tree, err := store.ReadTree(treeHash)
	if err != nil {
		return fmt.Errorf("failed to read tree [%s]: %w", treeHash, err)
	}

	for _, treeEntry := range tree.Entries() {
		entryPath := filepath.Join(dirPath, treeEntry.Name())

		if !treeEntry.IsDirectory() {
			if err := createFileFromBlob(store, treeEntry.Hash(), entryPath); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(entryPath, constants.DirPerms); err != nil {
			return fmt.Errorf("failed to create directory [%s]: %w", entryPath, err)
		}
		if err := restoreTreeRecursive(entryPath, treeEntry.Hash(), store); err != nil {
			return err
		}
	}

	return nil
}

// createFileFromBlob reads a blob from the object store and writes its content
// to the specified file path.
func createFileFromBlob(store *objects.ObjectStore, blobHash, filePath string) error {
	blob, err := store.ReadBlob(blobHash)
	if err != nil {
		return fmt.Errorf("failed to create file [%s] from blob [%s]: %w", filePath, blobHash, err)
	}

	if err := os.WriteFile(filePath, blob.Content(), constants.FilePerms); err != nil {
		return fmt.Errorf("failed to write [%s] file: %w", filePath, err)
	}

	return nil
}
