package utils

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"strings"
)

type ObjectType string

const (
	BlobObjectType   ObjectType = "blob"
	TreeObjectType   ObjectType = "tree"
	CommitObjectType ObjectType = "commit"
)

func (ot ObjectType) IsValid() bool {
	switch ot {
	case BlobObjectType, TreeObjectType, CommitObjectType:
		return true
	default:
		return false
	}
}

// computeHash calculates SHA-1 hash for Object content
func ComputeHash(content []byte, objectType ObjectType) (string, error) {
	if !objectType.IsValid() {
		return "", fmt.Errorf("invalid object type: %s - hash not computed", objectType)
	}

	// format: "ObjectType <size>\0<content>"
	header := fmt.Sprintf("%v %d\x00", objectType, len(content))
	data := append([]byte(header), content...)
	hash := sha1.Sum(data)
	return fmt.Sprintf("%x", hash), nil
}

// BuildDirPath constructs os-agnostic display direcotry path with trailing separator preserving all components.
// Unlike filepath.Join, does not normalize "." or remove redundant separators.
func BuildDirPath(dirs ...string) string {
	return strings.Join(dirs, string(filepath.Separator)) + string(filepath.Separator)
}
