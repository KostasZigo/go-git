package constants

import "os"

// Command name constants used in tests and error messages.
// Cobra Use fields remain inline for CLI discoverability.
const (
	InitCmdName       = "init"
	HashObjectCmdName = "hash-object"
)

// Repository directory and file names define the gogit metadata structure.
const (
	// Gogit is the repository metadata directory.
	Gogit = ".gogit"

	// Objects stores content-addressable objects (blobs, trees, commits).
	Objects = "objects"

	// Refs contains branch and tag references.
	Refs = "refs"

	// Heads stores branch pointers under refs/.
	Heads = "heads"

	// Tags stores tag pointers under refs/.
	Tags = "tags"

	// Head points to current branch or detached commit.
	Head = "HEAD"
)

// Default repository values.
const (
	// DefaultBranch is the initial branch name for new repositories.
	DefaultBranch = "main"

	// DefaultRefPrefix is prepended to branch names in HEAD file.
	DefaultRefPrefix = "ref: refs/heads/"
)

// File system permissions for created files and directories.
const (
	// DirPerms grants read/write/execute to owner, read/execute to others (rwxr-xr-x).
	DirPerms os.FileMode = 0755

	// FilePerms grants read/write to owner, read-only to others (rw-r--r--).
	FilePerms os.FileMode = 0644
)

// Cryptographic hash properties.
const (
	// HashByteLength is byte length of SHA-1 hash (20 bytes).
	HashByteLength = 20

	// HashStringLength is hex string length of SHA-1 hash (40 characters).
	HashStringLength = 40

	// HashDirPrefixLength is subdirectory prefix length under objects/ (2 characters).
	HashDirPrefixLength = 2
)

// Git object type prefixes used in object headers and commit metadata.
const (
	// BlobPrefix identifies blob objects in headers ("blob <size>\0").
	BlobPrefix = "blob "

	// TreePrefix identifies tree objects in headers ("tree <size>\0").
	TreePrefix = "tree "

	// CommitPrefix identifies commit objects in headers ("commit <size>\0").
	CommitPrefix = "commit "

	// CommitParentPrefix marks parent commit lines in commit objects.
	CommitParentPrefix = "parent "

	// CommitAuthorPrefix marks author metadata in commit objects.
	CommitAuthorPrefix = "author "

	// CommitCommitterPrefix marks committer metadata in commit objects.
	CommitCommitterPrefix = "committer "
)

// Object format constants.
const (
	// NullByte separates header from content in Git objects.
	NullByte = '\x00'
)

// Time conversion constants for timezone formatting.
const (
	SecondsPerHour   = 3600
	SecondsPerMinute = 60
)
