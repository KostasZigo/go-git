// Package index implements the gogit staging area, providing data structures
// and operations for tracking file entries between the working tree and the
// object store.
package index

import (
	"fmt"
	"slices"
	"strings"

	"github.com/KostasZigo/gogit/internal/constants"
)

// Index represents the staging area containing file entries
type Index struct {
	entries map[string]*Entry // Map [filepath : entry] for fast lookup
	version uint32
}

// NewIndex creates an empty index with default version
func NewIndex() *Index {
	return &Index{
		entries: make(map[string]*Entry),
		version: constants.IndexVersion,
	}
}

// AddEntry inserts or updates an entry in index
func (index *Index) AddEntry(entry *Entry) error {
	if entry == nil {
		return fmt.Errorf("cannot add empty entry to index")
	}

	index.entries[entry.path] = entry
	return nil
}

// GetEntry returns the index entry for the given path, or nil if no entry
// exists.
func (index *Index) GetEntry(path string) *Entry {
	return index.entries[path]
}

// RemoveEntry deletes an entry from the index based on its path
func (index *Index) RemoveEntry(path string) {
	delete(index.entries, path)
}

// GetEntryList returns sorted slice of all entries
func (index *Index) GetEntryList() []*Entry {
	entries := make([]*Entry, 0, len(index.entries))
	for _, entry := range index.entries {
		entries = append(entries, entry)
	}

	// Sort by path
	slices.SortStableFunc(entries, func(a, b *Entry) int {
		return strings.Compare(a.path, b.path)
	})
	return entries
}

// CountEntries returns number of staged entries.
func (index *Index) CountEntries() int {
	return len(index.entries)
}

// Clear removes all entries from index.
func (index *Index) Clear() {
	clear(index.entries)
}

// Version returns index format version.
func (index *Index) Version() uint32 {
	return index.version
}
