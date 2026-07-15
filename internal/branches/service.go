// Package branches provides branch management operations for the gogit repository.
// It handles creation of new branch references by resolving the current HEAD
// and writing the corresponding commit hash under refs/heads/.
package branches

import (
	"bytes"
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

	// 1. Compute refs path
	refsPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads)

	// 2. retrieve commit hash from HEAD.
	// Create the commit Hash for the new branch from the Head content (detached head state).
	// If the HEAD content contains a ref then retrieve the hash from that branch's file.
	headContent, err := os.ReadFile(filepath.Join(repoPath, constants.Gogit, constants.Head))
	if err != nil {
		return fmt.Errorf("failed to read HEAD file: %w", err)
	}

	trimmedHeadContent := bytes.TrimSpace(headContent)
	commitHash := append([]byte(nil), trimmedHeadContent...)
	commitHash = append(commitHash, '\n')

	refPrefix := []byte(constants.DefaultRefPrefix)
	headRefPath, hasPrefix := bytes.CutPrefix(trimmedHeadContent, refPrefix)
	if hasPrefix {
		relRefPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, filepath.FromSlash(string(headRefPath)))
		commitHash, err = os.ReadFile(relRefPath)
		if err != nil {
			return fmt.Errorf("failed to read current ref: %w", err)
		}
	}

	// 3. Create file and write content (exclusive create)
	newRefPath := filepath.Join(refsPath, branchName)
	if err := os.MkdirAll(filepath.Dir(newRefPath), constants.DirPerms); err != nil {
		return fmt.Errorf("failed to create refs directory for branch [%s]: %w", branchName, err)
	}

	if err := writeRefFileExclusive(newRefPath, commitHash); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("branch [%s] already exists", branchName)
		}
		return fmt.Errorf("failed to create new ref/branch [%s]: %w", branchName, err)
	}

	return nil
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

// writeRefFileExclusive creates a new ref file and writes content with durability safeguards.
// It fails with os.ErrExist if the path already exists.
func writeRefFileExclusive(filePath string, content []byte) error {
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, constants.FilePerms)
	if err != nil {
		return err
	}

	if _, err := file.Write(content); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}

	if err := file.Sync(); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	return nil
}
