package objects

import (
	"fmt"
	"slices"
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
		t.Fatal("expected commit to be created")
	}

	if commit.hash == "" {
		t.Fatal("expected commit hash to be set")
	}
	if !commit.IsInitialCommit() {
		t.Fatal("expected it to be an initial commit")
	}
	if commit.treeHash != treeHash {
		t.Fatalf("expected tree hash to be %s,  but got %s", treeHash, commit.treeHash)
	}

	assertCommitFields(t, commit, treeHash, message, nil, author)
}

// TestNewCommit verifies commit creation with parent reference.
func TestNewCommit(t *testing.T) {
	treeHash := "aTreeHash"
	parentHash := "aParentHash"
	message := "Second Commit"
	author := createTestAuthor("Ioannis Kappodistrias", "john.kapo@gmail.com")

	commit, err := NewCommit(treeHash, parentHash, message, author)
	if err != nil {
		t.Fatal("expected for commit to be created")
	}

	if commit.hash == "" {
		t.Fatal("expected commit hash to be set")
	}
	if commit.IsInitialCommit() {
		t.Fatal("expected it to be non-initial commit (has parent)")
	}
	if commit.treeHash != treeHash {
		t.Fatalf("expected tree hash to be [%s],  but got [%s]", treeHash, commit.treeHash)
	}

	assertCommitFields(t, commit, treeHash, message, []string{parentHash}, author)
}

// TestNewCommit_EmptyParent verifies that an ordinary commit requires a parent reference.
func TestNewCommit_EmptyParent(t *testing.T) {
	_, err := NewCommit(
		"aTreeHash",
		"",
		"Commit message",
		createTestAuthor("Test User", "test@example.com"),
	)
	if err == nil {
		t.Fatal("expected error")
	}
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
		t.Fatalf("failed to create commit: %v", err)
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
		t.Fatalf("failed to create initial commit: %v", err)
	}

	if commit.message != message {
		t.Fatalf("multi-line message not preserved correctly. Expected [%s] got [%s]", message, commit.message)
	}
}

// TestNewMergeCommit verifies that a merge commit preserves its two parents
// in order and serializes both parent lines in Git commit format.
func TestNewMergeCommit(t *testing.T) {
	treeHash := "mergeTreeHash"
	firstParentHash := "currentBranchTip"
	secondParentHash := "mergedBranchTip"
	location := time.FixedZone("EST", -5*3600)
	author := Author{
		Name:      "Test User",
		Email:     "test@example.com",
		Timestamp: time.Now().In(location).Truncate(time.Second),
	}
	message := "Merge branch 'feature' into main"

	commit, err := NewMergeCommit(treeHash, firstParentHash, secondParentHash, message, author)
	if err != nil {
		t.Fatalf("failed to create merge commit: %v", err)
	}

	expectedParentHashes := []string{firstParentHash, secondParentHash}
	if !slices.Equal(commit.ParentHashes(), expectedParentHashes) {
		t.Fatalf("expected parent hashes [%v], got [%v]", expectedParentHashes, commit.ParentHashes())
	}
	if commit.ParentHash() != firstParentHash {
		t.Fatalf("expected first parent hash [%s], got [%s]", firstParentHash, commit.ParentHash())
	}
	if commit.IsInitialCommit() {
		t.Fatal("expected merge commit to be non-initial")
	}

	timezone := calculateTimezone(author.Timestamp)
	expectedContent := fmt.Sprintf(
		"%s%s\n%s%s\n%s%s\n%s%s %d %s\n%s%s %d %s\n\n%s\n",
		constants.TreePrefix,
		treeHash,
		constants.CommitParentPrefix,
		firstParentHash,
		constants.CommitParentPrefix,
		secondParentHash,
		constants.CommitAuthorPrefix,
		author.String(),
		author.Timestamp.Unix(),
		timezone,
		constants.CommitCommitterPrefix,
		author.String(),
		author.Timestamp.Unix(),
		timezone,
		message,
	)
	if string(commit.Content()) != expectedContent {
		t.Fatalf("expected content [%s], got [%s]", expectedContent, commit.Content())
	}
}

// TestCommit_ParentHashesReturnsCopy verifies that callers cannot mutate a
// commit's parent list through the ParentHashes accessor.
func TestCommit_ParentHashesReturnsCopy(t *testing.T) {
	firstParentHash := "currentBranchTip"
	secondParentHash := "mergedBranchTip"
	commit, err := NewMergeCommit(
		"mergeTreeHash",
		firstParentHash,
		secondParentHash,
		"merge message",
		createTestAuthor("Test User", "test@example.com"),
	)
	if err != nil {
		t.Fatalf("failed to create merge commit: %v", err)
	}

	parentHashes := commit.ParentHashes()
	parentHashes[0] = "mutatedParentHash"

	expectedParentHashes := []string{firstParentHash, secondParentHash}
	if !slices.Equal(commit.ParentHashes(), expectedParentHashes) {
		t.Fatalf("expected parent hashes [%v], got [%v]", expectedParentHashes, commit.ParentHashes())
	}
}

// TestNewMergeCommit_ParentOrderAffectsIdentity verifies that parent order is
// part of the serialized commit content and therefore its content-addressed hash.
func TestNewMergeCommit_ParentOrderAffectsIdentity(t *testing.T) {
	treeHash := "mergeTreeHash"
	firstParentHash := "parentA"
	secondParentHash := "parentB"
	message := "merge message"
	author := createTestAuthor("Test User", "test@example.com")

	commit, err := NewMergeCommit(treeHash, firstParentHash, secondParentHash, message, author)
	if err != nil {
		t.Fatalf("failed to create merge commit: %v", err)
	}
	reversedCommit, err := NewMergeCommit(treeHash, secondParentHash, firstParentHash, message, author)
	if err != nil {
		t.Fatalf("failed to create merge commit with reversed parents: %v", err)
	}

	if slices.Equal(commit.Content(), reversedCommit.Content()) {
		t.Fatal("expected reversed parents to produce different commit content")
	}
	if commit.Hash() == reversedCommit.Hash() {
		t.Fatal("expected reversed parents to produce different commit hashes")
	}
}

// TestNewMergeCommit_InvalidParents verifies that merge commits require two
// distinct, non-empty parent hashes.
func TestNewMergeCommit_InvalidParents(t *testing.T) {
	testCases := []struct {
		name             string
		firstParentHash  string
		secondParentHash string
	}{
		{name: "empty first parent", firstParentHash: "", secondParentHash: "parentB"},
		{name: "empty second parent", firstParentHash: "parentA", secondParentHash: ""},
		{name: "identical parents", firstParentHash: "sameParent", secondParentHash: "sameParent"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewMergeCommit(
				"mergeTreeHash",
				testCase.firstParentHash,
				testCase.secondParentHash,
				"merge message",
				createTestAuthor("Test User", "test@example.com"),
			)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
