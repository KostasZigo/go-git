// Package branches provides branch management operations for the gogit repository.
// It handles creation of new branch references by resolving the current HEAD
// and writing the corresponding commit hash under refs/heads/.
package branches

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KostasZigo/gogit/internal/constants"
)

// OrchestrateBranchCreation creates a new branch reference pointing to the commit
// currently referenced by HEAD.
// If branch already exists it returns an error.
func OrchestrateBranchCreation(repoPath, branchName string) error {
	if err := validateBranchName(branchName); err != nil {
		return err
	}

	commitHash, err := resolveHEADCommitHash(repoPath)
	if err != nil {
		return err
	}

	if err := CompareAndSwap(repoPath, branchName, "", commitHash); err != nil {
		if errors.Is(err, ErrReferenceChanged) {
			return fmt.Errorf("branch [%s] already exists: %w", branchName, err)
		}
		return fmt.Errorf("failed to create new ref/branch [%s]: %w", branchName, err)
	}

	return nil
}

// resolveHEADCommitHash returns the commit referenced by symbolic or detached HEAD.
func resolveHEADCommitHash(repoPath string) (string, error) {
	headContent, err := os.ReadFile(filepath.Join(repoPath, constants.Gogit, constants.Head))
	if err != nil {
		return "", fmt.Errorf("failed to read HEAD file: %w", err)
	}

	trimmedHeadContent := strings.TrimSpace(string(headContent))
	branchName, isSymbolic := strings.CutPrefix(trimmedHeadContent, constants.DefaultRefPrefix)
	if !isSymbolic {
		if err := validateRefHash(trimmedHeadContent); err != nil {
			return "", fmt.Errorf("HEAD contains invalid commit hash: %w", err)
		}
		return trimmedHeadContent, nil
	}

	commitHash, exists, err := readBranchRef(repoPath, branchName)
	if err != nil {
		return "", fmt.Errorf("failed to read current ref: %w", err)
	}
	if !exists {
		return "", fmt.Errorf("failed to read current ref: branch [%s] not found", branchName)
	}

	return commitHash, nil
}

// validateBranchName verifies branch names are acceptable before creating refs/heads/<name>.
func validateBranchName(branchName string) error {
	if branchName == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	if branchName == "." ||
		strings.Contains(branchName, "..") ||
		strings.ContainsAny(branchName, `\\`) ||
		strings.HasSuffix(branchName, ".lock") {
		return fmt.Errorf("invalid branch name [%s]", branchName)
	}

	for _, r := range branchName {
		if r < 0x20 || r == 0x7f { // this check catches ASCII control chars and DEL
			return fmt.Errorf("invalid branch name [%s]", branchName)
		}
	}

	return nil
}
