package objects

import (
	"crypto/sha1"
	"fmt"
	"os"
)

const headerFormat string = "blob %d\x00"

type Blob struct {
	content []byte
	hash    string
}

func NewBlob(content []byte) *Blob {
	hash := computeBlobHash(content)
	return &Blob{
		content: content,
		hash:    hash,
	}
}

func NewBlobFromFile(filepath string) (*Blob, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filepath, err)
	}
	return NewBlob(content), nil
}

func computeBlobHash(content []byte) string {
	header := fmt.Sprintf(headerFormat, len(content))
	data := append([]byte(header), content...)
	hash := sha1.Sum(data)
	return fmt.Sprintf("%x", hash)
}

func (b *Blob) Hash() string {
	return b.hash
}

func (b *Blob) Content() []byte {
	return b.content
}

func (b *Blob) Size() int {
	return len(b.content)
}

func (b *Blob) Header() string {
	return fmt.Sprintf(headerFormat, b.Size())
}

func (b *Blob) Data() []byte {
	header := b.Header()
	data := append([]byte(header), b.Content()...)
	return data
}

func (b *Blob) String() string {
	return fmt.Sprintf("Blob{hash: %s, size: %d bytes}", b.hash, b.Size())
}
