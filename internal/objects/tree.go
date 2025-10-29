package objects

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/KostasZigo/gogit/utils"
)

type FileMode string

const (
	ModeRegularFile FileMode = "100644" // Regular non-executable file
	ModeExecutable  FileMode = "100755" // Executable file
	ModeSymlink     FileMode = "120000" // Symbolic link
	ModeDirectory   FileMode = "040000" // Directory (tree)
	ModeSubmodule   FileMode = "160000" // Git submodule
)

func (m FileMode) IsValid() bool {
	switch m {
	case ModeRegularFile, ModeExecutable, ModeSymlink, ModeDirectory, ModeSubmodule:
		return true
	default:
		return false
	}
}

// TreeEntry represents a single entry in a tree object
type TreeEntry struct {
	mode FileMode
	name string
	hash string //This is the hex hash coming from the blob file hash
}

func NewTreeEntry(mode FileMode, name string, hash string) (*TreeEntry, error) {
	if !mode.IsValid() {
		return nil, fmt.Errorf("invalid file mode: %s", mode)
	}
	return &TreeEntry{
		mode: mode,
		name: name,
		hash: hash,
	}, nil
}

func (e *TreeEntry) Mode() FileMode {
	return e.mode
}

func (e *TreeEntry) Name() string {
	return e.name
}

func (e *TreeEntry) Hash() string {
	return e.hash
}

func (treeEntry *TreeEntry) IsDirectory() bool {
	return treeEntry.mode == ModeDirectory
}

func (treeEntry *TreeEntry) IsExecutable() bool {
	return treeEntry.mode == ModeExecutable
}

// Tree represents a Git tree object (directory)
type Tree struct {
	entries []TreeEntry
	hash    string
}

// NewTree creates a tree object from the list of Tree Entries
func NewTree(treeEntries []TreeEntry) (*Tree, error) {
	// GoGit requires entries to be sorted by name in ascending order
	entries := make([]TreeEntry, len(treeEntries))
	copy(entries, treeEntries)

	slices.SortStableFunc(entries, compareTreeEntries)

	treeContent := buildTreeContent(entries)
	hash, err := utils.ComputeHash(treeContent, utils.TreeObjectType)
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash for tree: %v", err)
	}

	return &Tree{
		entries: entries,
		hash:    hash,
	}, nil
}

// compareTreeEntries implements Git's tree entry sorting rules:
// - Entries are sorted by name
// - Directory names are treated as if they have a trailing "/" for comparison
// - This ensures correct ordering when directories and files have similar names
func compareTreeEntries(a, b TreeEntry) int {
	nameA := getSortableName(a)
	nameB := getSortableName(b)
	return strings.Compare(nameA, nameB)
}

// getSortableName returns the name used for sorting.
// For directories, appends "/" to follow Git's sorting convention.
func getSortableName(entry TreeEntry) string {
	if entry.IsDirectory() {
		return entry.Name() + "/"
	}
	return entry.Name()
}

// buildTreeContent creates the raw tree content in GoGit format
// <mode> <name>\0<20-byte binary SHA> , ex:
// 100644 README.md\0[binary SHA for README blob]
// 100644 main.go\0[binary SHA for main.go blob]
// 040000 src\0[binary SHA for src/ tree]
func buildTreeContent(entries []TreeEntry) []byte {
	var buf bytes.Buffer

	for _, entry := range entries {
		buf.WriteString(string(entry.Mode()))
		buf.WriteByte(' ')
		buf.WriteString(entry.Name())
		buf.WriteByte(0)

		// Convert hex hash to binary hash
		hashBytes, _ := hex.DecodeString(entry.Hash())
		buf.Write(hashBytes)
	}

	return buf.Bytes()
}

// Hash returns the SHA-1 hash of the tree
func (t *Tree) Hash() string {
	return t.hash
}

// Entries returns all tree entries
func (t *Tree) Entries() []TreeEntry {
	return t.entries
}

// Size returns the size of the tree content
func (t *Tree) Size() int {
	return len(buildTreeContent(t.entries))
}

// Content returns the raw tree content
func (t *Tree) Content() []byte {
	return buildTreeContent(t.entries)
}

// Header returns the Git object header
func (t *Tree) Header() string {
	return fmt.Sprintf("tree %d\x00", t.Size())
}

func (t *Tree) Data() []byte {
	header := t.Header()
	data := append([]byte(header), t.Content()...)
	return data
}

// String returns a human-readable representation
func (t *Tree) String() string {
	return fmt.Sprintf("Tree{hash: %s, entries: %d}", t.hash, len(t.entries))
}

// FindEntry finds an entry by name
func (t *Tree) FindEntry(name string) (*TreeEntry, bool) {
	for _, entry := range t.entries {
		if entry.Name() == name {
			return &entry, true
		}
	}
	return nil, false
}
