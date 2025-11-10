package objects

import (
	"fmt"
	"os"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/utils"
)

type Blob struct {
	content []byte
	hash    string
}

func NewBlob(content []byte) *Blob {
	hash := utils.MustComputeHash(content, utils.BlobObjectType)
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

func (b *Blob) Hash() string {
	return b.hash
}

func (b *Blob) Content() []byte {
	return b.content
}

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
