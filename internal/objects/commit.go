package objects

import (
	"bytes"
	"fmt"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/utils"
)

// Represents commit author/committer
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
	hash, err := utils.ComputeHash(content, utils.CommitObjectType)
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash for commit: %v", err)
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

func (c *Commit) Hash() string {
	return c.hash
}

func (c *Commit) Content() []byte {
	return buildCommitContent(c.treeHash, c.parentHash, c.message, c.author)
}

func (c *Commit) Size() int {
	return len(c.Content())
}

func (c *Commit) Header() string {
	return fmt.Sprintf("%s%d%c", constants.CommitPrefix, c.Size(), constants.NullByte)
}

// Data returns complete Git object data including header.
func (c *Commit) Data() []byte {
	return append([]byte(c.Header()), c.Content()...)
}

func (c *Commit) IsInitialCommit() bool {
	return c.parentHash == ""
}
