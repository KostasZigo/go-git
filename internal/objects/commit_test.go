package objects

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
)

// TestNewCommit_InitialCommit verifies initial commit creation without parent.
func TestNewCommit_InitialCommit(t *testing.T) {
	treeHash := "randomTreeHash"
	author := createTestAuthor("Alexander the Great", "alaexander@great.com")
	message := "Init commit"

	commit, err := NewInitialCommit(treeHash, message, author)
	if err != nil {
		t.Fatal("Expected commit to be created")
	}

	if commit.hash == "" {
		t.Fatal("Expected commit hash to be set")
	}
	if !commit.IsInitialCommit() {
		t.Fatal("Expected it to be an initial commit")
	}
	if commit.treeHash != treeHash {
		t.Fatalf("Expected tree hash to be %s,  but got %s", treeHash, commit.treeHash)
	}

	assertCommitFields(t, commit, treeHash, "", message, author)
}

// TestNewCommit verifies commit creation with parent reference.
func TestNewCommit(t *testing.T) {
	treeHash := "aTreeHash"
	parentHash := "aParentHash"
	message := "Second Commit"
	author := createTestAuthor("Ioannis Kappodistrias", "john.kapo@gmail.com")

	commit, err := NewCommit(treeHash, parentHash, message, author)
	if err != nil {
		t.Fatal("Expected for commit to be created")
	}

	if commit.hash == "" {
		t.Fatal("Expected commit hash to be set")
	}
	if commit.IsInitialCommit() {
		t.Fatal("Expected it to be non-initial commit (has parent)")
	}
	if commit.treeHash != treeHash {
		t.Fatalf("Expected tree hash to be [%s],  but got [%s]", treeHash, commit.treeHash)
	}

	assertCommitFields(t, commit, treeHash, parentHash, message, author)
}

// TestCommit_ContentFormat verifies commit content matches Git format.
func TestCommit_ContentFormat(t *testing.T) {
	treeHash := "tree123"
	parentHash := "parent456"
	location := time.FixedZone("EST", -5*3600)
	author := Author{
		Name:      "Test User",
		Email:     "test@example.com",
		Timestamp: time.Now().In(location).Truncate(time.Second),
	}
	message := "Test commit message"

	commit, err := NewCommit(treeHash, parentHash, message, author)
	if err != nil {
		t.Fatalf("Failed to create commit: %v", err)
	}
	content := string(commit.Content())

	// Verify content contains required lines
	timezone := calculateTimezone(author.Timestamp)
	expectedLines := []string{
		constants.TreePrefix + treeHash,
		constants.CommitParentPrefix + parentHash,
		fmt.Sprintf("%s%s %d %s", constants.CommitAuthorPrefix, author.String(), author.Timestamp.Unix(), timezone),
		fmt.Sprintf("%s%s %d %s", constants.CommitCommitterPrefix, author.String(), author.Timestamp.Unix(), timezone),
		"\n",
		message,
	}

	for _, line := range expectedLines {
		if !strings.Contains(content, line) {
			t.Fatalf("expected line [%s] to appear in content [%s]", line, content)
		}
	}
}

// TestCommit_MessageWithMultipleLines verifies multi-line commit messages are preserved.
func TestCommit_MessageWithMultipleLines(t *testing.T) {
	treeHash := "tree123"
	author := createTestAuthor("Test User", "test@example.com")
	message := "Fist line\n\n" + "Second paragraph\n" + "Third line"

	commit, err := NewInitialCommit(treeHash, message, author)
	if err != nil {
		t.Fatalf("Failed to create initial commit: %v", err)
	}

	if commit.message != message {
		t.Fatalf("Multi-line message not preserved correctly. Expected [%s] got [%s]", message, commit.message)
	}
}
