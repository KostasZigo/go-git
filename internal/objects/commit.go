package objects

import (
	"bytes"
	"fmt"
	"time"

	"github.com/KostasZigo/gogit/utils"
)

// Represents commit author/committer
type Author struct {
	Name      string
	Email     string
	Timestamp time.Time
}

func (a Author) String() string {
	return fmt.Sprintf("%s <%s>",
		a.Name,
		a.Email)
}

// Represents a snapshot of the repository
type Commit struct {
	hash       string
	treeHash   string
	parentHash string
	author     Author
	committer  Author
	message    string
}

func NewCommit(treeHash, parentHash, message string, author Author) (*Commit, error) {

	content := buildCommitContent(treeHash, parentHash, message, author)
	hash, err := utils.ComputeHash(content, utils.CommitObjectType)
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash for commit: %v", err)
	}

	committer := author

	return &Commit{
		hash:       hash,
		treeHash:   treeHash,
		parentHash: parentHash,
		author:     author,
		committer:  committer,
		message:    message,
	}, nil
}

func NewInitialCommit(treeHash, message string, author Author) (*Commit, error) {
	return NewCommit(treeHash, "", message, author)
}

func buildCommitContent(treeHash, parentHash, message string, author Author) []byte {
	var buf bytes.Buffer

	// Tree reference
	fmt.Fprintf(&buf, "tree %s\n", treeHash)

	// Parent reference --rwta
	if parentHash != "" {
		fmt.Fprintf(&buf, "parent %s\n", parentHash)
	}

	// Author and commiter
	_, timeZoneOffeset := author.Timestamp.Zone()
	timezone := calculateTimezone(timeZoneOffeset)
	fmt.Fprintf(&buf, "author %s <%s> %d %s\n", author.Name, author.Email, author.Timestamp.Unix(), timezone)

	fmt.Fprintf(&buf, "committer %s <%s> %d %s\n", author.Name, author.Email, author.Timestamp.Unix(), timezone)

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

func calculateTimezone(offset int) string {
	// offset is in seconds, convert to ±HHMM format
	hours := offset / 3600
	minutes := (offset % 3600) / 60

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
	return fmt.Sprintf("commit %d\x00", c.Size())
}

func (c *Commit) Data() []byte {
	return append([]byte(c.Header()), c.Content()...)
}

func (c *Commit) IsInitialCommit() bool {
	return c.parentHash == ""
}

func (c *Commit) String() string {
	return fmt.Sprintf("Commit{hash: %s, tree: %s, parent: %s, author: %s, message: %q}",
		c.hash, c.treeHash, c.parentHash, c.author.String(), c.message)
}
