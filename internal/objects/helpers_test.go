package objects

import (
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/hasher"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// assertBlobHash verifies blob hash matches expected value for given content.
func assertBlobHash(t *testing.T, blob *Blob, content []byte) {
	t.Helper()

	expectedHash, err := hasher.ComputeHash(content, hasher.Blob)
	if err != nil {
		t.Fatalf("Hash computation failed: %v", err)
	}

	if blob.Hash() != expectedHash {
		t.Fatalf("Expected hash [%s], got [%s]", expectedHash, blob.Hash())
	}
}

// assertBlobContent verifies blob stores exact content and correct size.
func assertBlobContent(t *testing.T, blob *Blob, expectedContent []byte) {
	t.Helper()

	if blob.Size() != len(expectedContent) {
		t.Fatalf("Expected size %d, got %d", len(expectedContent), blob.Size())
	}

	if string(blob.Content()) != string(expectedContent) {
		t.Fatalf("Expected content [%q], got [%q]", expectedContent, blob.Content())
	}
}

// createTestAuthor returns test author with UTC timezone.
func createTestAuthor(name, email string) Author {
	return Author{
		Name:      name,
		Email:     email,
		Timestamp: time.Now().UTC().Truncate(time.Second),
	}
}

// createAndStoreInitialCommit creates initial commit, stores it, and returns commit.
func createAndStoreInitialCommit(t *testing.T, store *ObjectStore) *Commit {
	t.Helper()

	author := createTestAuthor(testutils.RandomString(10), testutils.RandomString(20))
	commit, err := NewInitialCommit(testutils.RandomHash(), testutils.RandomString(50), author)
	if err != nil {
		t.Fatalf("Failed to create initial commit: %v", err)
	}

	if err := store.Store(commit); err != nil {
		t.Fatalf("Failed to store commit: %v", err)
	}

	return commit
}

// createAndStoreCommit creates commit, stores it, and returns commit.
func createAndStoreCommit(t *testing.T, parentHash string, store *ObjectStore) *Commit {
	t.Helper()

	author := createTestAuthor(testutils.RandomString(10), testutils.RandomString(20))
	commit, err := NewCommit(testutils.RandomHash(), parentHash, testutils.RandomString(50), author)
	if err != nil {
		t.Fatalf("Failed to create commit: %v", err)
	}

	if err := store.Store(commit); err != nil {
		t.Fatalf("Failed to store commit: %v", err)
	}

	return commit
}

// assertCommitFields verifies commit fields match expected values.
func assertCommitFields(t *testing.T, commit *Commit, treeHash, parentHash, message string, author Author) {
	t.Helper()

	if commit.treeHash != treeHash {
		t.Errorf("Expected tree hash [%s], got [%s]", treeHash, commit.treeHash)
	}

	if commit.parentHash != parentHash {
		t.Errorf("Expected parent hash [%s], got [%s]", parentHash, commit.parentHash)
	}

	if commit.message != message {
		t.Errorf("Expected message [%s], got [%s]", message, commit.message)
	}

	if commit.author.String() != author.String() {
		t.Errorf("Expected author [%s], got [%s]", author.String(), commit.author.String())
	}

	if !commit.author.Timestamp.Equal(author.Timestamp) {
		t.Errorf("Expected timestamp [%s], got [%s]", author.Timestamp, commit.author.Timestamp)
	}
}

// assertCommitEqual verifies two commits match in all fields.
func assertCommitEqual(t *testing.T, actual, expected *Commit) {
	t.Helper()

	if actual.hash != expected.hash {
		t.Errorf("Hash mismatch: expected [%s], got [%s]", expected.hash, actual.hash)
	}

	if actual.treeHash != expected.treeHash {
		t.Errorf("Tree hash mismatch: expected [%s], got [%s]", expected.treeHash, actual.treeHash)
	}

	if actual.message != expected.message {
		t.Errorf("Message mismatch: expected [%s], got [%s]", expected.message, actual.message)
	}

	if actual.author.String() != expected.author.String() {
		t.Errorf("Author mismatch: expected [%s], got [%s]", expected.author.String(), actual.author.String())
	}

	if !actual.author.Timestamp.Equal(expected.author.Timestamp) {
		t.Errorf("Author timestamp mismatch: expected [%s], got [%s]",
			expected.author.Timestamp.Format("2006-01-02 15:04:05 -0700"),
			actual.author.Timestamp.Format("2006-01-02 15:04:05 -0700"))
	}
}
