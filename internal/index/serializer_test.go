package index

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestWrite_EmptyIndex verifies serialization of empty index.
func TestWrite_EmptyIndex(t *testing.T) {
	index := NewIndex()

	var buffer bytes.Buffer
	err := WriteIndex(&buffer, index)
	if err != nil {
		t.Fatalf("failed to write index: %v", err)
	}

	// Verify minimum header size (signature:4 + version:4 + entryCount:4 = 12 bytes)
	if buffer.Len() < 12 {
		t.Fatalf("invalid header: expected at least 12 bytes, got %d", buffer.Len())
	}

	data := buffer.Bytes()

	// Validate signature
	signature := string(data[0:4])
	if signature != constants.IndexSignature {
		t.Errorf("invalid signature: expected [%s], got [%s]", constants.IndexSignature, signature)
	}

	// Validate version
	version := binary.BigEndian.Uint32(data[4:8])
	if version != constants.IndexVersion {
		t.Errorf("invalid version: expected %d, got %d", constants.IndexVersion, version)
	}

	// Validate entry count (should be 0 for empty index)
	entryCount := binary.BigEndian.Uint32(data[8:])
	if entryCount != 0 {
		t.Errorf("invalid entry count: expected 0, got %d", entryCount)
	}
}

// Test_WriteAndRead_Index verifies serialization/deserialization consistency.
func Test_WriteAndRead_Index(t *testing.T) {
	index := NewIndex()

	filePaths := []string{testutils.RandomString(5), testutils.RandomString(5), testutils.RandomString(5)}
	for _, filePath := range filePaths {
		entry, err := NewEntry(ModeRegularFile, testutils.RandomHash(), filePath, testutils.RandomInt(10), time.Now())
		if err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
		if err := index.AddEntry(entry); err != nil {
			t.Fatalf("failed to add entry: %v", err)
		}
	}

	var buffer bytes.Buffer
	err := WriteIndex(&buffer, index)
	if err != nil {
		t.Fatalf("failed to write index: %v", err)
	}

	readIndex, err := ReadIndex(&buffer)
	if err != nil {
		t.Fatalf("failed to read index: %v", err)
	}

	// Verify entry count matches
	if readIndex.CountEntries() != index.CountEntries() {
		t.Fatalf("entry count mismatch: expected %d, got %d", index.CountEntries(), readIndex.CountEntries())
	}

	// Verify each entry
	for i, entry := range index.GetEntryList() {
		readEntry := readIndex.GetEntryList()[i]

		if readEntry.Path() != entry.Path() {
			t.Fatalf("path mismatch for %d element: expected %s, got %s", i, entry.Path(), readEntry.Path())
		}
		if readEntry.Hash() != entry.Hash() {
			t.Fatalf("hash mismatch for %s: expected %s, got %s", entry.Path(), readEntry.Hash(), entry.Hash())
		}
		if readEntry.FileSize() != entry.FileSize() {
			t.Fatalf("size mismatch for %s : expected %d, got %d", entry.Path(), readEntry.FileSize(), entry.FileSize())
		}
		if readEntry.Mode() != entry.Mode() {
			t.Fatalf("mode mismatch for %s : expected %v, got %v", entry.Path(), readEntry.Mode(), entry.Mode())
		}
	}
}
