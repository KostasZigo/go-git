package commits

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
	"github.com/KostasZigo/gogit/internal/utils"
	"github.com/fatih/color"
)

// TestMain disables colored output for all tests in this package to
func TestMain(m *testing.M) {
	color.NoColor = true
	os.Exit(m.Run())
}

// TestCollectCommitHistory_SingleCommit creates a single commit in an
// initialized repository and verifies that collectCommitHistory returns
// exactly one entry with the correct hash, message, and author.
func TestCollectCommitHistory_SingleCommit(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	author := objects.Author{
		Name:      testutils.RandomString(10),
		Email:     testutils.RandomString(15),
		Timestamp: time.Now(),
	}
	message := testutils.RandomString(10)
	treeHash := testutils.RandomHash()

	commitHash, err := createAndStoreCommit(treeHash, "", message, author, store)
	if err != nil {
		t.Fatalf("Failed to  create and store commit: %v", err)
	}

	entries, err := collectCommitHistory(store, commitHash)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected a single entry, got %d", len(entries))
	}

	if entries[0].Hash != commitHash {
		t.Errorf("Expected hash %s, got %s", commitHash, entries[0].Hash)
	}

	if entries[0].Message != message {
		t.Errorf("Expected message [%s], got [%s]", message, entries[0].Message)
	}

	if entries[0].Author.String() != author.String() {
		t.Errorf("Expected author [%s], got [%s]", author.String(), entries[0].Author.String())
	}
}

// TestCollectCommitHistory_CommitChain creates three sequential commits
// linked by parent hashes and verifies that collectCommitHistory returns
// them in reverse chronological order (most recent first) with correct
// hashes, messages, and author metadata at each position.
func TestCollectCommitHistory_CommitChain(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	author := objects.Author{
		Name:      testutils.RandomString(10),
		Email:     testutils.RandomString(15),
		Timestamp: time.Now(),
	}
	treeHash := testutils.RandomHash()

	message_first := testutils.RandomString(10)
	commitHash_first, err := createAndStoreCommit(treeHash, "", message_first, author, store)
	if err != nil {
		t.Fatalf("Failed to  create and store commit: %v", err)
	}

	message_second := testutils.RandomString(10)
	commitHash_second, err := createAndStoreCommit(treeHash, commitHash_first, message_second, author, store)
	if err != nil {
		t.Fatalf("Failed to  create and store commit: %v", err)
	}

	message_third := testutils.RandomString(10)
	commitHash_third, err := createAndStoreCommit(treeHash, commitHash_second, message_third, author, store)
	if err != nil {
		t.Fatalf("Failed to  create and store commit: %v", err)
	}

	entries, err := collectCommitHistory(store, commitHash_third)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Most recent commit first
	expectedEntryList := []struct {
		hash    string
		message string
	}{
		{commitHash_third, message_third},
		{commitHash_second, message_second},
		{commitHash_first, message_first},
	}

	if len(entries) != len(expectedEntryList) {
		t.Fatalf("Expected %d entries, got %d", len(expectedEntryList), len(entries))
	}

	for i, exp := range expectedEntryList {
		if entries[i].Hash != exp.hash {
			t.Errorf("Entry %d: expected hash [%s], got [%s]", i, exp.hash, entries[i].Hash)
		}
		if entries[i].Message != exp.message {
			t.Errorf("Entry %d: expected message [%s], got [%s]", i, exp.message, entries[i].Message)
		}
		if entries[0].Author.String() != author.String() {
			t.Errorf("Expected author [%s], got [%s]", author.String(), entries[0].Author.String())
		}
	}
}

// TestCollectCommitHistory_EmptyHash passes an empty string as the commit
// hash and verifies that collectCommitHistory returns an error containing
// "commit hash cannot be empty".
func TestCollectCommitHistory_EmptyHash(t *testing.T) {
	repoDir := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoDir)

	_, err := collectCommitHistory(store, "")
	if err == nil {
		t.Fatal("Expected error for empty hash, got nil")
	}

	expectedErrorMessage := "commit hash cannot be empty"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestCollectCommitHistory_InvalidHash passes a hash string that exceeds
// the valid length and verifies that collectCommitHistory returns an
// error containing "commit hash length is invalid".
func TestCollectCommitHistory_InvalidHash(t *testing.T) {
	repoDir := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoDir)

	_, err := collectCommitHistory(store, testutils.RandomString(100))
	if err == nil {
		t.Fatal("Expected error for nonexistent hash, got nil")
	}

	expectedErrorMessage := "commit hash length is invalid"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestFormatCommitHistory_SingleCommit constructs a single CommitLogEntry
// and verifies that formatCommitHistory produces the expected formatted
// string containing hash, message, author, and date in the defined layout.
func TestFormatCommitHistory_SingleCommit(t *testing.T) {
	author := objects.Author{
		Name:      testutils.RandomString(10),
		Email:     testutils.RandomString(15),
		Timestamp: time.Now(),
	}
	commit := newCommitLogEntry(testutils.RandomHash(), testutils.RandomString(20), author)
	commitLogEntries := []CommitLogEntry{commit}

	historyOutput, err := formatCommitHistory(commitLogEntries)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedHistoryOutput := utils.FormatCommitLogEntry(
		commit.Hash,
		commit.Message,
		commit.Author.String(),
		commit.Author.Time(),
	)

	if historyOutput != expectedHistoryOutput {
		t.Fatalf("Expected commit history output to be [%s], got [%s]", expectedHistoryOutput, historyOutput)
	}
}

// TestFormatCommitHistory_ChainCommits constructs three CommitLogEntry
// values in reverse chronological order and verifies that
// formatCommitHistory concatenates all entries into a single string
// with each entry on its own line in the expected format.
func TestFormatCommitHistory_ChainCommits(t *testing.T) {
	author := objects.Author{
		Name:      testutils.RandomString(10),
		Email:     testutils.RandomString(15),
		Timestamp: time.Now(),
	}
	first_commit := newCommitLogEntry(testutils.RandomHash(), testutils.RandomString(20), author)
	second_commit := newCommitLogEntry(testutils.RandomHash(), testutils.RandomString(20), author)
	third_commit := newCommitLogEntry(testutils.RandomHash(), testutils.RandomString(20), author)
	commitLogEntries := []CommitLogEntry{
		third_commit,
		second_commit,
		first_commit,
	}

	historyOutput, err := formatCommitHistory(commitLogEntries)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var expectedHistoryOutput strings.Builder
	for _, commit := range commitLogEntries {
		expectedHistoryOutput.WriteString(utils.FormatCommitLogEntry(
			commit.Hash,
			commit.Message,
			commit.Author.String(),
			commit.Author.Time(),
		))
	}

	if historyOutput != expectedHistoryOutput.String() {
		t.Fatalf("Expected commit history output to be [%s], got [%s]", expectedHistoryOutput.String(), historyOutput)
	}
}

// TestFormatCommitHistory_NoCommits passes a nil slice to
// formatCommitHistory and verifies that it returns an error
// containing "no commits found".
func TestFormatCommitHistory_NoCommits(t *testing.T) {
	_, err := formatCommitHistory(nil)
	if err == nil {
		t.Fatal("Expected an error when there are no commits in the history")
	}

	expectedErrorMessage := "no commits found"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("Expected error message to be [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestOrchestrateLogExecution creates three sequential commits in an
// initialized repository, writes the latest commit hash to the default
// branch ref file, and invokes OrchestrateLogExecution. Verifies that
// the returned formatted output matches the full commit chain with
// correct hashes, messages, authors, and dates in reverse chronological order.
func TestOrchestrateLogExecution(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	store := objects.NewObjectStore(repoPath)

	author := objects.Author{
		Name:      testutils.RandomString(10),
		Email:     testutils.RandomString(15),
		Timestamp: time.Now(),
	}
	treeHash := testutils.RandomHash()

	message_first := testutils.RandomString(10)
	commitHash_first, err := createAndStoreCommit(treeHash, "", message_first, author, store)
	if err != nil {
		t.Fatalf("Failed to  create and store commit: %v", err)
	}

	message_second := testutils.RandomString(10)
	commitHash_second, err := createAndStoreCommit(treeHash, commitHash_first, message_second, author, store)
	if err != nil {
		t.Fatalf("Failed to  create and store commit: %v", err)
	}

	message_third := testutils.RandomString(10)
	commitHash_third, err := createAndStoreCommit(treeHash, commitHash_second, message_third, author, store)
	if err != nil {
		t.Fatalf("Failed to  create and store commit: %v", err)
	}

	refPath := filepath.Join(repoPath, constants.Gogit, "refs", "heads")
	if err := os.MkdirAll(refPath, constants.DirPerms); err != nil {
		t.Fatalf("Failed to create ref path directory: %v", err)
	}
	testutils.CreateTestFile(t, refPath, "main", []byte(commitHash_third+"\n"))

	historyOutput, err := OrchestrateLogExecution(repoPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	logEntries := []CommitLogEntry{
		newCommitLogEntry(commitHash_third, message_third, author),
		newCommitLogEntry(commitHash_second, message_second, author),
		newCommitLogEntry(commitHash_first, message_first, author),
	}

	var expectedHistoryOutput strings.Builder
	for _, commit := range logEntries {
		expectedHistoryOutput.WriteString(utils.FormatCommitLogEntry(
			commit.Hash,
			commit.Message,
			commit.Author.String(),
			commit.Author.Time(),
		))
	}

	if historyOutput != expectedHistoryOutput.String() {
		t.Fatalf("Expected commit history output to be [%s], got [%s]", expectedHistoryOutput.String(), historyOutput)
	}
}
