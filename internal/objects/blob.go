package objects

import (
	"fmt"
	"os"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/hasher"
)

// Blob represents a Git blob object storing raw file content
// alongside its computed SHA-1 hash.
type Blob struct {
	content []byte
	hash    string
}

// NewBlob creates a Blob from raw content and computes its SHA-1 hash.
func NewBlob(content []byte) *Blob {
	hash := hasher.MustComputeHash(content, hasher.Blob)
	return &Blob{
		content: content,
		hash:    hash,
	}
}

// NewBlobFromFile reads the file at the given path and creates a Blob
// from its content. Returns an error if the file cannot be read.
func NewBlobFromFile(filepath string) (*Blob, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filepath, err)
	}
	return NewBlob(content), nil
}

// Hash returns the hex-encoded SHA-1 hash of the blob.
func (b *Blob) Hash() string {
	return b.hash
}

// Content returns the raw byte content of the blob.
func (b *Blob) Content() []byte {
	return b.content
}

// Size returns the byte length of the blob content.
func (b *Blob) Size() int {
	return len(b.content)
}

// Header returns Git object header.
func (b *Blob) Header() string {
	return fmt.Sprintf("%s%d%c", constants.BlobPrefix, b.Size(), constants.NullByte)
}

// Data returns complete Git object data including header.
func (b *Blob) Data() []byte {
	return append([]byte(b.Header()), b.Content()...)
}
