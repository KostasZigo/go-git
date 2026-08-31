package commits

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/KostasZigo/gogit/internal/branches"
	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/hasher"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/worktree"
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
	branch, err := branches.Resolve(repoPath, target)
	if errors.Is(err, branches.ErrBranchNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to resolve branch: %w", err)
	}

	return &ResolvedTarget{
		IsBranch: true,
		Hash:     branch.Hash,
	}, nil
}

// searchForTargetInCommitObjects checks if target is a valid SHA-1 hash
// pointing to an existing commit object in the store.
// Returns nil, nil if the target is not a valid hash or the object does not exist.
// Returns an error only if the object exists but cannot be read as a commit.
func searchForTargetInCommitObjects(repoPath, target string) (*ResolvedTarget, error) {
	if !hasher.IsValidSHA1(target) {
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

// updateHEAD atomically replaces the HEAD file with the given content using
// a temp-file-then-rename strategy identical to index.Manager.Save.
// For branch targets, content should be "ref: refs/heads/<branch>\n".
// For detached commits, content should be "<commit-hash>\n".
func updateHEAD(repoPath string, content string) error {
	headFilePath := filepath.Join(repoPath, constants.Gogit, constants.Head)
	tempFile, err := os.CreateTemp(filepath.Dir(headFilePath), "HEAD-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary HEAD file: %w", err)
	}
	tempPath := tempFile.Name()

	// Track success for cleanup
	succeeded := false
	defer func() {
		if !succeeded {
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.WriteString(content); err != nil {
		return fmt.Errorf("failed to write content to HEAD file: %w", err)
	}

	// Force OS to flush data to physical disk before rename
	// Critical for durability guarantees on power loss
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}

	// Close temp file descriptor before rename
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Replace HEAD file with temporary file
	if err := os.Rename(tempPath, headFilePath); err != nil {
		return fmt.Errorf("failed to rename temporary file to HEAD: %w", err)
	}
	succeeded = true

	return nil
}

// handleHeadUpdate builds the HEAD file content from the resolved target and
// atomically writes it. Branch targets produce a symbolic ref, detached targets
// produce a raw commit hash.
func handleHeadUpdate(repoPath, target string, resolvedTarget ResolvedTarget) error {
	var headFileContent string
	if resolvedTarget.IsBranch {
		headFileContent = constants.DefaultRefPrefix + target + "\n"
	} else {
		headFileContent = resolvedTarget.Hash + "\n"
	}

	if err := updateHEAD(repoPath, headFileContent); err != nil {
		return fmt.Errorf("failure during updating HEAD file: %w", err)
	}

	return nil
}

// handleIdempotency compares the resolved target's commit hash against the
// current HEAD commit. If they match, updates HEAD (to handle branch switches
// at the same commit) and returns true so the caller can skip repository-state
// inspection and snapshot application. Returns false when the hashes differ.
func handleIdempotency(repoPath, headCommitHash, target string, resolvedTarget ResolvedTarget) (bool, error) {
	if headCommitHash != resolvedTarget.Hash {
		return false, nil
	}

	if err := handleHeadUpdate(repoPath, target, resolvedTarget); err != nil {
		return false, err
	}
	return true, nil
}

// resolveHEADCommit resolves symbolic or detached HEAD to its commit object.
// Symbolic HEAD is resolved through the current branch; detached HEAD contains
// the commit hash directly.
func resolveHEADCommit(repoPath string, store *objects.ObjectStore) (*objects.Commit, error) {
	headContent, err := os.ReadFile(filepath.Join(repoPath, constants.Gogit, constants.Head))
	if err != nil {
		return nil, fmt.Errorf("failed to read HEAD file: %w", err)
	}

	commitHash := ""
	trimmed := bytes.TrimSpace(headContent)
	if !bytes.HasPrefix(trimmed, []byte(constants.DefaultRefPrefix)) {
		commitHash = string(trimmed)
	} else {
		currentBranch, err := branches.ResolveCurrent(repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve current branch: %w", err)
		}
		commitHash = currentBranch.Hash
	}

	headCommit, err := store.ReadCommit(commitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to read HEAD commit [%s]: %w", commitHash, err)
	}

	return headCommit, nil
}

// OrchestrateCheckoutExecution orchestrates the full checkout workflow:
// resolves the target and current HEAD commits, inspects staged and worktree
// changes plus target collisions, applies the target snapshot through the
// worktree API, and updates HEAD only after successful application.
//
// When force is true staged and tracked worktree changes do not block the
// checkout, but target collisions always do. Snapshot application performs
// best-effort rollback if mutation or index persistence fails.
func OrchestrateCheckoutExecution(repoPath, target string, force bool) error {
	// 1. Resolve the target reference and load its commit.
	resolvedTarget, err := ResolveTarget(repoPath, target)
	if err != nil {
		return fmt.Errorf("failure while resolving target: %w", err)
	}

	store := objects.NewObjectStore(repoPath)
	targetCommit, err := store.ReadCommit(resolvedTarget.Hash)
	if err != nil {
		return fmt.Errorf("failed to read commit [%s]: %w", resolvedTarget.Hash, err)
	}

	// 2. Resolve and load the commit currently referenced by HEAD.
	headCommit, err := resolveHEADCommit(repoPath, store)
	if err != nil {
		return fmt.Errorf("failure while resolving HEAD: %w", err)
	}

	// 3. Handle a branch or detached-HEAD switch to the current commit.
	isMatching, err := handleIdempotency(repoPath, headCommit.Hash(), target, *resolvedTarget)
	if err != nil {
		return fmt.Errorf("failed during idempotency checks: %w", err)
	}
	if isMatching {
		return nil
	}

	// 4. Load the current and target commit snapshots.
	originalSnapshot, err := store.ReadTreeSnapshot(headCommit.TreeHash())
	if err != nil {
		return fmt.Errorf("failed to convert HEAD commit's tree hash to tree snapshot: %w", err)
	}
	targetSnapshot, err := store.ReadTreeSnapshot(targetCommit.TreeHash())
	if err != nil {
		return fmt.Errorf("failed to convert [%s] target commit's tree hash to tree snapshot: %w", target, err)
	}

	// 5. Inspect staged changes, tracked worktree changes, and target collisions.
	wtService, err := worktree.NewService(repoPath)
	if err != nil {
		return err
	}

	repositoryState, err := wtService.ResolveRepositoryState(originalSnapshot, targetSnapshot)
	if err != nil {
		return err
	}

	// 6. Enforce policy: collisions always block; force permits tracked changes.
	if repositoryState.HasCollisions() || (!force && repositoryState.HasChanges()) {
		return &worktree.PreflightError{State: *repositoryState}
	}

	// 7. Apply with the same index snapshot inspected by the worktree service.
	if err := wtService.ApplySnapshot(store, originalSnapshot, targetSnapshot); err != nil {
		return fmt.Errorf("failure during snapshot application: %w", err)
	}

	// 8. Update HEAD only after the worktree and index transition succeeds.
	return handleHeadUpdate(repoPath, target, *resolvedTarget)
}
