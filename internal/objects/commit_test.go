package objects

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewCommit_InitialCommit(t *testing.T) {
	treeHash := "randomTreeHash"
	author := Author{
		Name:      "Alexander the Great",
		Email:     "alaexander@great.com",
		Timestamp: time.Now().UTC().Truncate(time.Second),
	}
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
	if commit.message != message {
		t.Fatalf("Expected message to be %s,  but got %s", message, commit.message)
	}
	if commit.author.String() != author.String() {
		t.Fatalf("Expected author to be %s,  but got %s", author.String(), commit.author.String())
	}
	if !commit.author.Timestamp.Equal(author.Timestamp) {
		t.Fatalf("Expected author timestamp to be %s,  but got %s", author.Timestamp.String(), commit.author.Timestamp.String())
	}
}

func TestNewCommit(t *testing.T) {
	treeHash := "aTreeHash"
	parentHash := "aParentHash"
	message := "Second Commit"
	author := Author{
		Name:      "Ioannis Kappodistrias",
		Email:     "john.kapo@gmail.com",
		Timestamp: time.Now().UTC().Truncate(time.Second),
	}

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
		t.Fatalf("Expected tree hash to be %s,  but got %s", treeHash, commit.treeHash)
	}
	if commit.parentHash != parentHash {
		t.Fatalf("Expected parent hash to be %s,  but got %s", parentHash, commit.parentHash)
	}
	if commit.message != message {
		t.Fatalf("Expected message to be %s,  but got %s", message, commit.message)
	}
	if commit.author.String() != author.String() {
		t.Fatalf("Expected author to be %s,  but got %s", author.String(), commit.author.String())
	}
	if !commit.author.Timestamp.Equal(author.Timestamp) {
		t.Fatalf("Expected author timestamp to be %s,  but got %s", author.Timestamp.String(), commit.author.Timestamp.String())
	}
}

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
		t.Fatal("Expected for commit to be created")
	}
	content := string(commit.Content())

	// Verify content contains required lines
	_, timeZoneOffset := author.Timestamp.Zone()
	timezone := calculateTimezone(timeZoneOffset)
	expectedLines := []string{
		"tree " + treeHash,
		"parent " + parentHash,
		"author Test User <" + author.Email + "> " + fmt.Sprint(author.Timestamp.Unix()) + " " + timezone,
		"committer Test User <" + author.Email + "> " + fmt.Sprint(author.Timestamp.Unix()) + " " + timezone,
		"\n",
		message,
	}

	for _, line := range expectedLines {
		if !strings.Contains(content, line) {
			t.Fatalf("expected line [%s] to appear in content [%s]", line, content)
		}
	}
}

func TestCommit_MessageWithMultipleLines(t *testing.T) {
	treeHash := "tree123"
	author := Author{
		Name:      "Test User",
		Email:     "test@example.com",
		Timestamp: time.Now().UTC().Truncate(time.Second),
	}
	message := "Fist line\n\n" + "Second paragraph\n" + "Thrid line"

	commit, err := NewInitialCommit(treeHash, message, author)
	if err != nil {
		t.Fatal("Expected for initial commit to be created")
	}

	if commit.message != message {
		t.Fatalf("Multi-line message not preserved correctly. Expected [%s] got [%s]", message, commit.message)
	}
}
