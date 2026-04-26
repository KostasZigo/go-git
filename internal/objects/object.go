// Package objects implements the content-addressable object store used by gogit.
// It provides types and operations for blobs, trees, and commits, along with
// SHA-1 hashing, zlib compression, and on-disk storage under .gogit/objects/.
package objects

// Object represents any GoGit object that can be stored
// All GoGit objects (blobs, trees, commits, tags) must implement this interface
type Object interface {
	// Hash returns the SHA-1 hash of the object
	Hash() string

	// Data returns the complete object data including header
	// Format: "<type> <size>\0<content>"
	Data() []byte
}
