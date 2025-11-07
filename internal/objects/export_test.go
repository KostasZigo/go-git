package objects

import (
	"testing"
	"time"

	"github.com/KostasZigo/gogit/testutils"
	"github.com/KostasZigo/gogit/utils"
)

// assertBlobHash verifies blob hash matches expected value for given content.
func assertBlobHash(t *testing.T, blob *Blob, content []byte) {
	t.Helper()

	expectedHash, err := utils.ComputeHash(content, utils.BlobObjectType)
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

// createTreeEntry creates tree entry and fails test on error.
func createTreeEntry(t *testing.T, mode FileMode, name, hash string) TreeEntry {
	t.Helper()

	entry, err := NewTreeEntry(mode, name, hash)
	if err != nil {
		t.Fatalf("Failed to create tree entry: %v", err)
	}

	return *entry
}

// createTree creates tree from entries and fails test on error.
func createTree(t *testing.T, entries []TreeEntry) *Tree {
	t.Helper()

	tree, err := NewTree(entries)
	if err != nil {
		t.Fatalf("Failed to create tree: %v", err)
	}

	return tree
}

// createAndStoreTree creates tree from entries, stores it, and returns tree.
func createAndStoreTree(t *testing.T, store *ObjectStore, entries []TreeEntry) *Tree {
	t.Helper()

	tree := createTree(t, entries)
	if err := store.Store(tree); err != nil {
		t.Fatalf("Failed to store tree: %v", err)
	}

	return tree
}

// assertTreeEntryEqual verifies two tree entries match.
func assertTreeEntryEqual(t *testing.T, actual, expected TreeEntry) {
	t.Helper()

	if actual.Name() != expected.Name() {
		t.Errorf("Entry name mismatch: expected %s, got %s", expected.Name(), actual.Name())
	}
	if actual.Hash() != expected.Hash() {
		t.Errorf("Entry hash mismatch: expected %s, got %s", expected.Hash(), actual.Hash())
	}
	if actual.Mode() != expected.Mode() {
		t.Errorf("Entry mode mismatch: expected %s, got %s", expected.Mode(), actual.Mode())
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
