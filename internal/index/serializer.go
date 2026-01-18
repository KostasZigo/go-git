package index

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
)

// WriteIndex serializes index to binary format
func WriteIndex(writer io.Writer, index *Index) error {
	var buffer bytes.Buffer

	// Write header
	if err := writeIndexHeader(&buffer, index); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write entries
	entries := index.GetEntryList()
	for _, entry := range entries {
		if err := writeEntry(&buffer, entry); err != nil {
			return fmt.Errorf("failed to write entry %s: %w", entry.Path(), err)
		}
	}

	// Write buffered data to output
	if _, err := buffer.WriteTo(writer); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}

	return nil
}

// writeIndexHeader writes format (signature, version, entry count)
func writeIndexHeader(writer io.Writer, index *Index) error {
	// Signature 4 bytes
	if _, err := writer.Write([]byte(constants.IndexSignature)); err != nil {
		return fmt.Errorf("failed to write signature: %w", err)
	}

	// Version 4 bytes - uint32
	if err := binary.Write(writer, binary.BigEndian, index.Version()); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}

	// Entry Count
	count := uint32(index.CountEntries())
	if err := binary.Write(writer, binary.BigEndian, count); err != nil {
		return fmt.Errorf("failed to write entry count: %w", err)
	}

	return nil
}

// writeEntry serializes single index entry
func writeEntry(writer io.Writer, entry *IndexEntry) error {
	// File mode - 4 bytes
	if err := binary.Write(writer, binary.BigEndian, entry.Mode()); err != nil {
		return fmt.Errorf("failed to write file mode: %w", err)
	}

	// Object Hash - 20 bytes
	hashBinary, err := hex.DecodeString(entry.Hash())
	if err != nil {
		return fmt.Errorf("invalid hash format: %w", err)
	}
	if len(hashBinary) != constants.HashByteLength {
		return fmt.Errorf("invalid hash length: expected %d bytes, got %d", constants.HashByteLength, len(hashBinary))
	}
	if _, err := writer.Write(hashBinary); err != nil {
		return fmt.Errorf("failed to write object hash: %w", err)
	}

	// File size - 8 bytes
	if err := binary.Write(writer, binary.BigEndian, entry.FileSize()); err != nil {
		return fmt.Errorf("failed to write file size: %w", err)
	}

	// Modified time - 8 bytes
	if err := binary.Write(writer, binary.BigEndian, entry.LastModified().Unix()); err != nil {
		return fmt.Errorf("failed to write last modified timestamp: %w", err)
	}

	// Path length - bytes
	if err := binary.Write(writer, binary.BigEndian, uint16(len(entry.Path()))); err != nil {
		return fmt.Errorf("failed to write path length: %w", err)
	}

	// Path
	if _, err := writer.Write([]byte(entry.Path())); err != nil {
		return fmt.Errorf("failed to write path: %w", err)
	}

	// Null terminator
	if _, err := writer.Write([]byte{constants.NullByte}); err != nil {
		return fmt.Errorf("failed to write null terminator byte: %w", err)
	}

	return nil
}

// ReadIndex deserializesindex from binary format
func ReadIndex(reader io.Reader) (*Index, error) {
	// Read index header
	index, entriesCount, err := readHeader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read index header: %w", err)
	}

	// Read index entries
	for i := 0; i < int(entriesCount); i++ {
		entry, err := readEntry(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read entry %d: %w", i, err)
		}
		if err := index.AddEntry(entry); err != nil {
			return nil, fmt.Errorf("failed to add entry %s to index: %w", entry.Path(), err)
		}
	}

	return index, nil
}

// readHeader parses index header and returns index with entry count
func readHeader(reader io.Reader) (*Index, uint32, error) {
	// Read signature
	signature := make([]byte, len(constants.IndexSignature))
	if _, err := io.ReadFull(reader, signature); err != nil {
		return nil, 0, fmt.Errorf("failed to read signature: %w", err)
	}
	if string(signature) != constants.IndexSignature {
		return nil, 0, fmt.Errorf("invalid index signature: expected [%s], got [%s]", constants.IndexSignature, string(signature))
	}

	// Read version
	var version uint32
	if err := binary.Read(reader, binary.BigEndian, &version); err != nil {
		return nil, 0, fmt.Errorf("failed to read version: %w", err)
	}
	if version != constants.IndexVersion {
		return nil, 0, fmt.Errorf("invalid index version: expected [%d], got [%d]", constants.IndexVersion, version)
	}

	// Read entries count
	var entriesCount uint32
	if err := binary.Read(reader, binary.BigEndian, &entriesCount); err != nil {
		return nil, 0, fmt.Errorf("failed to read entries count: %w", err)
	}

	return &Index{
		entries: make(map[string]*IndexEntry, entriesCount),
		version: uint32(version),
	}, entriesCount, nil
}

// readEntry deserializes single index entry.
func readEntry(reader io.Reader) (*IndexEntry, error) {
	// Read File Mode
	var fileMode FileMode
	if err := binary.Read(reader, binary.BigEndian, &fileMode); err != nil {
		return nil, fmt.Errorf("failed to read file mode: %w", err)
	}

	// Read Object hash
	hashBytes := make([]byte, constants.HashByteLength)
	if _, err := io.ReadFull(reader, hashBytes); err != nil {
		return nil, fmt.Errorf("failed to read hash: %w", err)
	}
	hash := hex.EncodeToString(hashBytes)

	// Read file size
	var fileSize int64
	if err := binary.Read(reader, binary.BigEndian, &fileSize); err != nil {
		return nil, fmt.Errorf("failed to read file size: %w", err)
	}

	// Read modified time
	var modTime int64
	if err := binary.Read(reader, binary.BigEndian, &modTime); err != nil {
		return nil, fmt.Errorf("failed to read last modified timestamp: %w", err)
	}
	lastModified := time.Unix(modTime, 0)

	// Read path length
	var pathLength uint16
	if err := binary.Read(reader, binary.BigEndian, &pathLength); err != nil {
		return nil, fmt.Errorf("failed to read path length: %w", err)
	}

	// Read path
	pathBytes := make([]byte, pathLength)
	if _, err := io.ReadFull(reader, pathBytes); err != nil {
		return nil, fmt.Errorf("failed to read path: %w", err)
	}

	// Read null terminator
	var nullByte byte
	if err := binary.Read(reader, binary.BigEndian, &nullByte); err != nil {
		return nil, fmt.Errorf("failed to read null terminator: %w", err)
	}
	if nullByte != constants.NullByte {
		return nil, fmt.Errorf("invalid null terminator: expected 0x%02x, got 0x%02x", constants.NullByte, nullByte)
	}

	return NewEntry(fileMode, hash, string(pathBytes), fileSize, lastModified)
}
