// Package hasher provides content-addressable hashing for gogit objects.
// It implements the SHA-1 hash computation used to identify blobs, trees,
// and commits in the object store.
package hasher

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"

	"github.com/KostasZigo/gogit/internal/constants"
)

// ObjectType identifies the category of a Git object for header construction.
type ObjectType string

// ObjectType constants enumerate the supported Git object types.
const (
	Blob   ObjectType = "blob"
	Tree   ObjectType = "tree"
	Commit ObjectType = "commit"
)

// IsValid reports whether the ObjectType is a recognized Git object type.
func (ot ObjectType) IsValid() bool {
	switch ot {
	case Blob, Tree, Commit:
		return true
	default:
		return false
	}
}

// ComputeHash calculates the SHA-1 hash of content with a Git object header.
// The header format is: "<objectType> <size>\0<content>".
// Returns an error if the object type is invalid.
func ComputeHash(content []byte, objectType ObjectType) (string, error) {
	if !objectType.IsValid() {
		return "", fmt.Errorf("invalid object type: %s - hash not computed", objectType)
	}

	header := fmt.Sprintf("%v %d\x00", objectType, len(content))
	data := append([]byte(header), content...)
	hash := sha1.Sum(data)
	return fmt.Sprintf("%x", hash), nil
}

// MustComputeHash is like Compute but panics on error.
// Use only when the object type is statically known to be valid.
func MustComputeHash(content []byte, objectType ObjectType) string {
	hash, err := ComputeHash(content, objectType)
	if err != nil {
		panic(err)
	}
	return hash
}

// IsValidSHA1 checks whether the given string is a well-formed SHA-1 hash:
// exactly 40 characters of valid hexadecimal (0-9, a-f, A-F).
func IsValidSHA1(hash string) bool {
	if len(hash) != constants.HashStringLength {
		return false
	}
	_, err := hex.DecodeString(hash)
	return err == nil
}
