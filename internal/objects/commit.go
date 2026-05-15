package objects

import (
	"bytes"
	"fmt"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/hasher"
)

// Author represents commit author/committer
type Author struct {
	Name      string
	Email     string
	Timestamp time.Time
}

// String formats author as "Name <email>".
func (a Author) String() string {
	return fmt.Sprintf("%s <%s>",
		a.Name,
		a.Email)
}

// Time returns the author's commit timestamp.
func (a Author) Time() time.Time {
	return a.Timestamp
}

// DefaultAuthor returns a placeholder Author using the default name and
// email from constants. Intended as a temporary solution until git config
// based author resolution is implemented.
func DefaultAuthor() Author {
	return Author{
		Name:      constants.DefaultAuthorName,
		Email:     constants.DefaultAuthorEmail,
		Timestamp: time.Now(),
	}
}

// Commit represents a snapshot of the repository
type Commit struct {
	hash       string
	treeHash   string
	parentHash string
	author     Author
	committer  Author
	message    string
}

// NewCommit creates commit with parent reference.
func NewCommit(treeHash, parentHash, message string, author Author) (*Commit, error) {
	content := buildCommitContent(treeHash, parentHash, message, author)
	hash, err := hasher.ComputeHash(content, hasher.Commit)
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash for commit: %w", err)
	}

	return &Commit{
		hash:       hash,
		treeHash:   treeHash,
		parentHash: parentHash,
		author:     author,
		committer:  author,
		message:    message,
	}, nil
}

// NewInitialCommit creates root commit without parent.
func NewInitialCommit(treeHash, message string, author Author) (*Commit, error) {
	return NewCommit(treeHash, "", message, author)
}

// buildCommitContent constructs Git commit object format
func buildCommitContent(treeHash, parentHash, message string, author Author) []byte {
	var buf bytes.Buffer

	// Tree reference - tree hash\n
	fmt.Fprintf(&buf, "%s%s\n", constants.TreePrefix, treeHash)

	// Parent reference - parent hash\n
	if parentHash != "" {
		fmt.Fprintf(&buf, "%s%s\n", constants.CommitParentPrefix, parentHash)
	}

	// Author and commiter - author name <email> time timezone\n
	timezone := calculateTimezone(author.Timestamp)
	fmt.Fprintf(&buf, "%s%s %d %s\n",
		constants.CommitAuthorPrefix,
		author.String(),
		author.Timestamp.Unix(),
		timezone,
	)

	fmt.Fprintf(&buf, "%s%s %d %s\n",
		constants.CommitCommitterPrefix,
		author.String(),
		author.Timestamp.Unix(),
		timezone,
	)

	// Blank line before message
	buf.WriteByte('\n')

	// Commit message
	buf.WriteString(message)

	// Ensure message ends in newLine
	if len(message) > 0 && message[len(message)-1] != '\n' {
		buf.WriteByte('\n')
	}

	return buf.Bytes()
}

// calculateTimezone converts time.Time to Git timezone format (±HHMM).
func calculateTimezone(t time.Time) string {
	_, timeZoneOffset := t.Zone()

	// offset is in seconds, convert to ±HHMM format
	hours := timeZoneOffset / constants.SecondsPerHour
	minutes := (timeZoneOffset % constants.SecondsPerHour) / constants.SecondsPerMinute

	if minutes < 0 {
		minutes = -minutes
	}

	return fmt.Sprintf("%+03d%02d", hours, minutes)
}

// Hash returns the hex-encoded SHA-1 hash of the commit object.
func (c *Commit) Hash() string {
	return c.hash
}

// TreeHash returns the SHA-1 hash of the root tree object for this commit.
func (c *Commit) TreeHash() string {
	return c.treeHash
}

// ParentHash returns the SHA-1 hash of the parent commit,
// or an empty string for the initial commit.
func (c *Commit) ParentHash() string {
	return c.parentHash
}

// Message returns the commit message.
func (c *Commit) Message() string {
	return c.message
}

// Author returns the author metadata for this commit.
func (c *Commit) Author() Author {
	return c.author
}

// Content returns the raw commit object body without the header,
// reconstructed from the commit's stored fields.
func (c *Commit) Content() []byte {
	return buildCommitContent(c.treeHash, c.parentHash, c.message, c.author)
}

// Size returns the byte length of the commit content body.
func (c *Commit) Size() int {
	return len(c.Content())
}

// Header returns the Git object header for this commit ("commit <size>\0").
func (c *Commit) Header() string {
	return fmt.Sprintf("%s%d%c", constants.CommitPrefix, c.Size(), constants.NullByte)
}

// Data returns complete Git object data including header.
func (c *Commit) Data() []byte {
	return append([]byte(c.Header()), c.Content()...)
}

// IsInitialCommit checks whether this is the first commit of the repository
func (c *Commit) IsInitialCommit() bool {
	return c.parentHash == ""
}
