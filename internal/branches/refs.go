package branches

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/hasher"
)

// ErrDetachedHEAD indicates that HEAD contains a commit hash instead of a
// symbolic reference to a local branch.
var ErrDetachedHEAD = errors.New("HEAD is detached")

// ErrBranchNotFound indicates that a named local branch ref does not exist.
var ErrBranchNotFound = errors.New("branch not found")

// ErrReferenceChanged indicates that a branch no longer contains the expected hash.
var ErrReferenceChanged = errors.New("branch reference changed")

// ErrReferenceLocked indicates that another operation owns the branch ref lock.
var ErrReferenceLocked = errors.New("branch reference is locked")

// Reference identifies a local branch, its filesystem ref path, and the
// commit hash it currently stores. Hash is empty for an unborn branch.
type Reference struct {
	Name     string
	Hash     string
	FilePath string
}

// ResolveCurrent resolves symbolic HEAD to the current local branch and its
// commit hash. A missing branch ref is returned as an unborn branch.
func ResolveCurrent(repoPath string) (Reference, error) {
	headPath := filepath.Join(repoPath, constants.Gogit, constants.Head)
	headContent, err := os.ReadFile(headPath)
	if err != nil {
		return Reference{}, fmt.Errorf("failed to read HEAD file: %w", err)
	}

	trimmedHeadContent := strings.TrimSpace(string(headContent))
	if !strings.HasPrefix(trimmedHeadContent, constants.DefaultRefPrefix) {
		return Reference{}, fmt.Errorf("%w at [%s]", ErrDetachedHEAD, trimmedHeadContent)
	}

	branchName := strings.TrimPrefix(trimmedHeadContent, constants.DefaultRefPrefix)
	refPath, err := branchRefPath(repoPath, branchName)
	if err != nil {
		return Reference{}, fmt.Errorf("failed to resolve current branch [%s]: %w", branchName, err)
	}
	hash, _, err := readBranchRef(repoPath, branchName)
	if err != nil {
		return Reference{}, fmt.Errorf("failed to resolve current branch [%s]: %w", branchName, err)
	}

	return Reference{Name: branchName, Hash: hash, FilePath: refPath}, nil
}

// Resolve resolves a named local branch independently of HEAD.
func Resolve(repoPath, branchName string) (Reference, error) {
	refPath, err := branchRefPath(repoPath, branchName)
	if err != nil {
		return Reference{}, fmt.Errorf("failed to resolve branch [%s]: %w", branchName, err)
	}
	hash, exists, err := readBranchRef(repoPath, branchName)
	if err != nil {
		return Reference{}, fmt.Errorf("failed to resolve branch [%s]: %w", branchName, err)
	}
	if !exists {
		return Reference{}, fmt.Errorf("branch [%s] not found: %w", branchName, ErrBranchNotFound)
	}

	return Reference{Name: branchName, Hash: hash, FilePath: refPath}, nil
}

// CompareAndSwap updates a local branch only when it still contains the
// expected hash. An empty expected hash represents a missing ref.
func CompareAndSwap(repoPath, branchName, expectedHash, newHash string) (returnErr error) {
	refPath, err := branchRefPath(repoPath, branchName)
	if err != nil {
		return err
	}
	if expectedHash != "" {
		if err := validateRefHash(expectedHash); err != nil {
			return fmt.Errorf("invalid expected hash for branch [%s]: %w", branchName, err)
		}
	}
	if err := validateRefHash(newHash); err != nil {
		return fmt.Errorf("invalid new hash for branch [%s]: %w", branchName, err)
	}

	if err := os.MkdirAll(filepath.Dir(refPath), constants.DirPerms); err != nil {
		return fmt.Errorf("failed to create ref directory for branch [%s]: %w", branchName, err)
	}

	lockPath := refPath + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, constants.FilePerms)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: branch [%s]", ErrReferenceLocked, branchName)
		}
		return fmt.Errorf("failed to create ref lock for branch [%s]: %w", branchName, err)
	}

	succeeded := false
	lockClosed := false
	defer func() {
		if !succeeded {
			returnErr = errors.Join(
				returnErr,
				cleanupRefLock(lockFile, lockPath, branchName, lockClosed),
			)
		}
	}()

	actualHash, exists, err := readBranchRef(repoPath, branchName)
	if err != nil {
		return fmt.Errorf("failed to read current ref for branch [%s]: %w", branchName, err)
	}
	if !exists {
		actualHash = ""
	}
	if actualHash != expectedHash {
		return fmt.Errorf(
			"%w: branch [%s] expected [%s] but found [%s]",
			ErrReferenceChanged,
			branchName,
			expectedHash,
			actualHash,
		)
	}

	if _, err := lockFile.WriteString(newHash + "\n"); err != nil {
		return fmt.Errorf("failed to write ref lock for branch [%s]: %w", branchName, err)
	}
	if err := lockFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync ref lock for branch [%s]: %w", branchName, err)
	}
	closeErr := lockFile.Close()
	lockClosed = true
	if closeErr != nil {
		return fmt.Errorf("failed to close ref lock for branch [%s]: %w", branchName, closeErr)
	}
	if err := os.Rename(lockPath, refPath); err != nil {
		return fmt.Errorf("failed to replace ref for branch [%s]: %w", branchName, err)
	}

	succeeded = true
	return nil
}

// cleanupRefLock closes and removes a lock owned by the current operation.
func cleanupRefLock(lockFile *os.File, lockPath, branchName string, lockClosed bool) error {
	var cleanupErr error
	if !lockClosed {
		if err := lockFile.Close(); err != nil {
			cleanupErr = errors.Join(
				cleanupErr,
				fmt.Errorf("failed to close ref lock for branch [%s]: %w", branchName, err),
			)
		}
	}

	if err := os.Remove(lockPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		cleanupErr = errors.Join(
			cleanupErr,
			fmt.Errorf("failed to remove ref lock for branch [%s]: %w", branchName, err),
		)
	}

	return cleanupErr
}

// branchRefPath validates a logical branch name and converts it to its
// operating-system path under .gogit/refs/heads.
func branchRefPath(repoPath, branchName string) (string, error) {
	if err := validateBranchName(branchName); err != nil {
		return "", err
	}

	return filepath.Join(
		repoPath,
		constants.Gogit,
		constants.Refs,
		constants.Heads,
		filepath.FromSlash(branchName),
	), nil
}

// readBranchRef reads and validates the commit hash stored for a logical
// branch name. A missing ref returns an empty hash, false, and no error.
func readBranchRef(repoPath, branchName string) (hash string, exists bool, err error) {
	refPath, err := branchRefPath(repoPath, branchName)
	if err != nil {
		return "", false, err
	}

	content, err := os.ReadFile(refPath)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("failed to read branch [%s] ref: %w", branchName, err)
	}

	hash = strings.TrimSpace(string(content))
	if err := validateRefHash(hash); err != nil {
		return "", true, fmt.Errorf("branch [%s] contains invalid commit hash: %w", branchName, err)
	}

	return hash, true, nil
}

// validateRefHash verifies that a branch ref value is a well-formed SHA-1 hash.
func validateRefHash(hash string) error {
	if !hasher.IsValidSHA1(hash) {
		return fmt.Errorf("invalid SHA-1 hash [%s]", hash)
	}

	return nil
}
