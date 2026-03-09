package commits

import (
	"fmt"
	"strings"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
)

// CommitLogEntry acts as a DTO holding the metadata of a single commit used for
// rendering the commit history log output.
type CommitLogEntry struct {
	Hash    string
	Message string
	Author  objects.Author
}

// newCommitLogEntry constructs a CommitLogEntry from the given
// commit hash, message, and author metadata.
func newCommitLogEntry(hash, message string, author objects.Author) CommitLogEntry {
	return CommitLogEntry{
		Hash:    hash,
		Message: message,
		Author:  author,
	}
}

// collectCommitHistory walks the commit chain starting from commitHash,
// following parent references until the root commit is reached. Returns
// entries in reverse chronological order (most recent first). Returns an
// error if the hash is empty, has invalid length, or any commit in the
// chain cannot be read from the object store.
func collectCommitHistory(store *objects.ObjectStore, commitHash string) ([]CommitLogEntry, error) {
	if commitHash == "" {
		return nil, fmt.Errorf("commit hash cannot be empty")
	}

	if len(commitHash) != constants.HashStringLength {
		return nil, fmt.Errorf("commit hash length is invalid")
	}

	var commitLogEntries []CommitLogEntry

	commit, err := store.ReadCommit(commitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to read commit with hash [%s]: %w", commitHash, err)
	}

	commitLogEntries = append(commitLogEntries, newCommitLogEntry(commit.Hash(), commit.Message(), commit.Author()))

	for commit.ParentHash() != "" {
		commit, err = store.ReadCommit(commit.ParentHash())
		if err != nil {
			return nil, fmt.Errorf("failed to read commit with hash [%s]: %w", commit.ParentHash(), err)
		}
		commitLogEntries = append(commitLogEntries, newCommitLogEntry(commit.Hash(), commit.Message(), commit.Author()))
	}

	return commitLogEntries, nil
}

// formatCommitHistory renders a slice of CommitLogEntry values into a
// human-readable string. Each entry is formatted as a single line
// containing the hash, message, author, and date. Returns an error
// if the slice is empty.
func formatCommitHistory(commitLogEntries []CommitLogEntry) (string, error) {
	if len(commitLogEntries) == 0 {
		return "", fmt.Errorf("no commits found")
	}

	var outputCommitHistory strings.Builder
	for _, commit := range commitLogEntries {
		outputCommitHistory.WriteString(fmt.Sprintf("%s %s Author: %s, Date: %s\n", commit.Hash,
			commit.Message,
			commit.Author.String(),
			commit.Author.Time().Format(constants.CommitDateFormat)))
	}

	return outputCommitHistory.String(), nil
}

// OrchestrateLogExecution resolves the current HEAD reference, reads the
// commit hash from the ref file, collects the full commit history chain,
// and returns the formatted log output string. This is the top-level
// entry point invoked by the CLI log command.
func OrchestrateLogExecution(repoPath string) (string, error) {
	refPath, err := resolveHEADRef(repoPath)
	if err != nil {
		return "", err
	}

	refHash, err := getRefCommitHash(refPath)
	if err != nil {
		return "", err
	}
	if refHash == "" {
		return "", fmt.Errorf("failed to read commit hash from [%s]", refPath)
	}

	store := objects.NewObjectStore(repoPath)
	commitLogEntries, err := collectCommitHistory(store, refHash)
	if err != nil {
		return "", fmt.Errorf("failed to collect commit history: %w", err)
	}

	commitHistoryOutput, err := formatCommitHistory(commitLogEntries)
	if err != nil {
		return "", fmt.Errorf("failed to format commit history: %w", err)
	}

	return commitHistoryOutput, nil
}
