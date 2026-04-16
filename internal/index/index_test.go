package index

import (
	"slices"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestNewIndex verifies empty index creation.
func TestNewIndex(t *testing.T) {
	index := NewIndex()

	if index.CountEntries() != 0 {
		t.Fatalf("Expected empty index, got %d entries", index.CountEntries())
	}

	if index.Version() != constants.IndexVersion {
		t.Fatalf("Expected index versions [%d], got [%d] entries", constants.IndexVersion, index.Version())
	}
}

// TestIndex_AddEntry verifies entry insertion.
func TestIndex_AddEntry(t *testing.T) {
	index := NewIndex()

	entry, err := NewEntry(ModeRegularFile, testutils.RandomHash(), testutils.RandomString(10), testutils.RandomInt(100), time.Now())
	if err != nil {
		t.Fatalf("Failed to create entry: %v", err)
	}

	if err := index.AddEntry(entry); err != nil {
		t.Fatalf("Failed to add entry: %v", err)
	}

	if index.CountEntries() != 1 {
		t.Errorf("Expected 1 entry, got %d", index.CountEntries())
	}

	retrievedEntry := index.GetEntryList()[0]
	if retrievedEntry.Hash() != entry.Hash() {
		t.Errorf("Hash mismatch: expected [%s], got [%s]", entry.Hash(), retrievedEntry.Hash())
	}
}

// TestIndex_UpdateEntry verifies entry replacement for a file with updated hash.
func TestIndex_UpdateEntry(t *testing.T) {
	index := NewIndex()

	filePath := testutils.RandomString(10)
	originalEntry, err := NewEntry(ModeRegularFile, testutils.RandomHash(), filePath, testutils.RandomInt(100), time.Now())
	if err != nil {
		t.Fatalf("Failed to create entry: %v", err)
	}

	if err := index.AddEntry(originalEntry); err != nil {
		t.Fatalf("Failed to add entry: %v", err)
	}

	updatedEntry, err := NewEntry(ModeRegularFile, testutils.RandomHash(), filePath, testutils.RandomInt(100), time.Now())
	if err != nil {
		t.Fatalf("Failed to create entry: %v", err)
	}

	if err := index.AddEntry(updatedEntry); err != nil {
		t.Fatalf("Failed to add entry: %v", err)
	}

	if index.CountEntries() != 1 {
		t.Fatalf("Expected 1 entry after update, got %d", index.CountEntries())
	}

	retrievedEntry := index.GetEntryList()[0]
	if retrievedEntry.Hash() != updatedEntry.Hash() {
		t.Fatalf("Entry's hash is [%v], should be [%v]", retrievedEntry.Hash(), updatedEntry.Hash())
	}
	if retrievedEntry.FileSize() != updatedEntry.FileSize() {
		t.Fatalf("Entry's hash is [%v], should be [%v]", retrievedEntry.FileSize(), updatedEntry.FileSize())
	}
}

// TestIndex_RemoveEntry verifies entry deletion.
func TestIndex_RemoveEntry(t *testing.T) {
	index := NewIndex()

	entry, err := NewEntry(ModeRegularFile, testutils.RandomHash(), testutils.RandomString(10), testutils.RandomInt(100), time.Now())
	if err != nil {
		t.Fatalf("Failed to create entry: %v", err)
	}

	if err := index.AddEntry(entry); err != nil {
		t.Fatalf("Failed to add entry: %v", err)
	}

	if index.CountEntries() != 1 {
		t.Errorf("Expected 1 entry, got %d", index.CountEntries())
	}

	index.RemoveEntry(entry.Path())
	if index.CountEntries() != 0 {
		t.Fatalf("Expected 0 entries in the index after deletetion but got [%v]", index.CountEntries())
	}
}

// TestIndex_Entries_Sorted verifies sorted entry retrieval.
func TestIndex_Entries(t *testing.T) {
	idx := NewIndex()

	// Add entries in random order
	paths := []string{testutils.RandomString(5), testutils.RandomString(5), testutils.RandomString(5)}
	for _, path := range paths {
		entry, _ := NewEntry(ModeRegularFile, testutils.RandomHash(), path, testutils.RandomInt(10), time.Now())
		idx.AddEntry(entry)
	}

	entries := idx.GetEntryList()

	// Verify sorted order
	expectedOrder := slices.Clone(paths)
	slices.Sort(expectedOrder)
	for i, expected := range expectedOrder {
		if entries[i].Path() != expected {
			t.Errorf("Expected entry %d to be %s, got %s", i, expected, entries[i].Path())
		}
	}
}

// TestIndex_Clear verifies index reset.
func TestIndex_Clear(t *testing.T) {
	index := NewIndex()

	entriesFilePaths := []string{"z.txt", "a.html", "k.png"}

	sortedEntriesFilePaths := slices.Clone(entriesFilePaths)
	slices.Sort(sortedEntriesFilePaths)

	for _, entryFilePath := range entriesFilePaths {
		entry, err := NewEntry(ModeRegularFile, testutils.RandomHash(), entryFilePath, testutils.RandomInt(100), time.Now())
		if err != nil {
			t.Fatalf("Failed to create entry: %v", err)
		}

		if err := index.AddEntry(entry); err != nil {
			t.Fatalf("Failed to add entry: %v", err)
		}
	}

	index.Clear()
	if index.CountEntries() != 0 {
		t.Fatalf("Expected 0 entries after clearing the index")
	}
}
