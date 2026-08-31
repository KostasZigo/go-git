// Package index implements the gogit staging area, providing data structures
// and operations for tracking file entries between the working tree and the
// object store.
package index

import (
	"fmt"
	"slices"
	"strings"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
)

// Index represents the staging area as canonical repository paths mapped to
// their blob identity and filesystem metadata.
type Index struct {
	entries map[string]*Entry // Map [filepath : entry] for fast lookup
	version uint32
}

// NewIndex creates an empty index with the default format version.
func NewIndex() *Index {
	return &Index{
		entries: make(map[string]*Entry),
		version: constants.IndexVersion,
	}
}

// AddEntry inserts or replaces an entry using its canonical path as identity.
func (idx *Index) AddEntry(entry *Entry) error {
	if entry == nil {
		return fmt.Errorf("cannot add empty entry to idx")
	}

	idx.entries[entry.path] = entry
	return nil
}

// RemoveEntries removes the entries at paths from the index. Paths that are
// not present are ignored.
func (idx *Index) RemoveEntries(paths ...string) {
	for _, entryPath := range paths {
		delete(idx.entries, entryPath)
	}
}

// GetEntry returns the index entry for the given path, or nil if no entry
// exists.
func (idx *Index) GetEntry(path string) *Entry {
	return idx.entries[path]
}

// GetEntryList returns all entries in deterministic canonical-path order.
func (idx *Index) GetEntryList() []*Entry {
	entries := make([]*Entry, 0, len(idx.entries))
	for _, entry := range idx.entries {
		entries = append(entries, entry)
	}

	// Sort by path
	slices.SortStableFunc(entries, func(a, b *Entry) int {
		return strings.Compare(a.path, b.path)
	})
	return entries
}

// CountEntries returns number of staged entries.
func (idx *Index) CountEntries() int {
	return len(idx.entries)
}

// Clear removes all entries from index.
func (idx *Index) Clear() {
	clear(idx.entries)
}

// Version returns index format version.
func (idx *Index) Version() uint32 {
	return idx.version
}

// ToTreeSnapshot converts the staging index into the snapshot representation
// used for comparing repository trees.
func (idx *Index) ToTreeSnapshot() (objects.TreeSnapshot, error) {
	snapshot := make(objects.TreeSnapshot, idx.CountEntries())

	for _, entry := range idx.GetEntryList() {
		snapshot[entry.Path()] = objects.SnapshotEntry{
			Hash: entry.Hash(),
			Mode: ToObjectFileMode(entry.Mode()),
		}
	}

	if err := snapshot.Validate(); err != nil {
		return nil, fmt.Errorf("invalid snapshot created from index: %w", err)
	}

	return snapshot, nil
}
