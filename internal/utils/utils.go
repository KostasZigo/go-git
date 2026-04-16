package utils

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
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

// ComputeHash calculates SHA-1 hash for Object content
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

// MustComputeHash is a non-validating version of Compute Hash
func MustComputeHash(content []byte, objectType ObjectType) string {
	hash, err := ComputeHash(content, objectType)
	if err != nil {
		panic(err)
	}
	return hash
}

// IsValidSHA1Hash checks whether the given string is a well-formed SHA-1 hash:
// exactly 40 characters of valid hexadecimal (0-9, a-f, A-F).
func IsValidSHA1Hash(hash string) bool {
	if len(hash) != constants.HashStringLength {
		return false
	}
	_, err := hex.DecodeString(hash)
	return err == nil
}

// BuildDirPath constructs os-agnostic display direcotry path with trailing separator preserving all components.
// Unlike filepath.Join, does not normalize "." or remove redundant separators.
func BuildDirPath(dirs ...string) string {
	return strings.Join(dirs, string(filepath.Separator)) + string(filepath.Separator)
}

// FindRepoRoot locates .gogit directory by walking up directory tree.
func FindRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		gogitPath := filepath.Join(dir, constants.Gogit)
		if info, err := os.Stat(gogitPath); err == nil && info.IsDir() {
			return dir, nil
		}

		// Dir returns all but the last element of path
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding .gogit
			return "", fmt.Errorf("%s directory not found", constants.Gogit)
		}
		dir = parent
	}
}

// FormatCommitLogEntry renders a single commit log line with colored hash,
// author, and date fields.
func FormatCommitLogEntry(hash, message, author string, time time.Time) string {
	return fmt.Sprintf("%s %s %s %s\n",
		constants.HashColor(hash),
		message,
		constants.AuthorColor("Author: "+author),
		constants.DateColor("Date: "+time.Format(constants.CommitDateFormat)))
}
