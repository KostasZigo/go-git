package e2etesting

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/testutils"
	"github.com/KostasZigo/gogit/utils"
)

// sharedBinaryPath stores compiled gogit binary path built once in TestMain.
// All E2E tests execute this binary to verify end-to-end behavior.
// Binary persists for test suite duration, cleaned up after all tests complete
var sharedBinaryPath string

// TestMain executes before all tests to build gogit binary once.
// Binary stored in temporary directory, removed after test suite completes.
//
// Execution flow:
//  1. Create temporary directory for binary storage
//  2. Build gogit binary with platform-specific extension
//  3. Store binary path in package-level sharedBinaryPath variable
//  4. Execute all Test* functions via m.Run()
//  5. Clean up temporary directory and binary
//  6. Exit with test suite status code
func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "gogit-e2e-*")
	if err != nil {
		panic("Failed to create temp directory: " + err.Error())
	}
	defer os.RemoveAll(tempDir)

	binaryName := "gogit"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	sharedBinaryPath = filepath.Join(tempDir, binaryName)

	buildCmd := exec.Command("go", "build", "-o", sharedBinaryPath, ".")
	buildCmd.Dir = ".." // execute command on root folder
	if err := buildCmd.Run(); err != nil {
		panic("Failed to build binary: " + err.Error())
	}

	os.Exit(m.Run())
}

// setupTestRepo creates test directory.
func setupTestRepo(t *testing.T) (repoPath string) {
	t.Helper()

	repoPath = filepath.Join(t.TempDir(), "test-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create test repo dir: %v", err)
	}

	return repoPath
}

// initializeRepository runs gogit init in test directory.
func initializeRepository(t *testing.T, repoPath string) {
	t.Helper()

	cmd := exec.Command(sharedBinaryPath, constants.InitCmdName)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}
}

// decompressObject reads and decompresses any git object file.
// Returns the full decompressed content including the header.
func decompressObject(t *testing.T, objectPath string) []byte {
	t.Helper()

	compressedData, err := os.ReadFile(objectPath)
	if err != nil {
		t.Fatalf("Failed to read object file: %v", err)
	}

	reader, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		t.Fatalf("Failed to create zlib reader: %v", err)
	}
	defer reader.Close()

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(reader); err != nil {
		t.Fatalf("Failed to decompress object: %v", err)
	}

	return buffer.Bytes()
}

// assertBlobContent verifies blob object format and content.
func assertBlobContent(t *testing.T, decompressedData, expectedContent []byte) {
	t.Helper()

	if !bytes.HasPrefix(decompressedData, []byte("blob ")) {
		t.Fatal("Object is not a blob")
	}

	nullByteIndex := bytes.IndexByte(decompressedData, 0)
	if nullByteIndex == -1 {
		t.Fatal("Invalid blob format: no null byte found")
	}

	content := decompressedData[nullByteIndex+1:]
	if !bytes.Equal(content, expectedContent) {
		t.Errorf("Content mismatch: expected %q, got %q", expectedContent, content)
	}
}

// assertAddCommandOutputAndObjectCreation verifies add command output and blob object creation and content.
func assertAddCommandOutputAndObjectCreation(t *testing.T, testFileName string, output []byte, testFileContent []byte, repoPath string) {
	expectedOutput := fmt.Sprintf("add '%s'", filepath.ToSlash(testFileName))
	if !strings.Contains(string(output), expectedOutput) {
		t.Errorf("Expected output to contain %q, got: %s", expectedOutput, string(output))
	}

	expectedHash, err := utils.ComputeHash(testFileContent, utils.BlobObjectType)
	if err != nil {
		t.Fatalf("Failed to compute hash: %v", err)
	}

	objectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, expectedHash[:constants.HashDirPrefixLength], expectedHash[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, objectPath)

	//Verify File content
	decompressedContent := decompressObject(t, objectPath)
	assertBlobContent(t, decompressedContent, testFileContent)
}

// assertIndexCreationAndContent verifies index cretion and content
func assertIndexCreationAndContent(t *testing.T, repoPath string, expectedFiles map[string][]byte) {
	indexPath := filepath.Join(repoPath, constants.Gogit, constants.Index)
	testutils.AssertFileExists(t, indexPath)

	assertIndexContent(t, indexPath, expectedFiles)
}

// assertIndexContent verifies index content
func assertIndexContent(t *testing.T, indexPath string, expectedFiles map[string][]byte) {
	t.Helper()

	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("Failed to read index file: %v", err)
	}

	reader := bytes.NewReader(indexData)

	readAndAssertIndexHeader(t, reader, expectedFiles)
	readAndAssertIndexEntries(t, reader, expectedFiles)
	_, err = reader.ReadByte()
	if err == nil {
		t.Fatal("Expeceted an error when trying to read form the index while its meant to have reached EOF")
	}
}

// readAndAssertIndexHeader verifies index header content
func readAndAssertIndexHeader(t *testing.T, reader *bytes.Reader, expectedFiles map[string][]byte) {
	signature := make([]byte, 4)
	if _, err := io.ReadFull(reader, signature); err != nil {
		t.Fatalf("Failed to read signature: %v", err)
	}
	if string(signature) != constants.IndexSignature {
		t.Fatalf("Invalid signature: expected %s, got %s", constants.IndexSignature, string(signature))
	}

	var version uint32
	if err := binary.Read(reader, binary.BigEndian, &version); err != nil {
		t.Fatalf("Failed to read version: %v", err)
	}
	if version != constants.IndexVersion {
		t.Fatalf("Invalid version: expected %d, got %d", constants.IndexVersion, version)
	}

	var entryCount uint32
	if err := binary.Read(reader, binary.BigEndian, &entryCount); err != nil {
		t.Fatalf("Failed to read entry count: %v", err)
	}
	if int(entryCount) != len(expectedFiles) {
		t.Fatalf("Entry count mismatch: expected %d, got %d", len(expectedFiles), entryCount)
	}
}

// readAndAssertIndexEntries verifies index entries content
func readAndAssertIndexEntries(t *testing.T, reader *bytes.Reader, expectedFiles map[string][]byte) {
	// Sort the map keys that correspond to the filepats as they are expected to be sorted inside the index
	keys := make([]string, 0, len(expectedFiles))
	for k := range expectedFiles {
		keys = append(keys, (k))
	}
	slices.Sort(keys)

	for _, key := range keys {
		// Verify expected file was indexed
		expectedContent, exists := expectedFiles[key]
		if !exists {
			t.Fatalf("Expected entry for key [%s] to exist", key)
		}
		normalizedKey := filepath.ToSlash(key) // File path Separator is converted to fornt-slash during add command for OS unaware reasons
		parseAndAssertIndexEntry(t, reader, normalizedKey, expectedContent)
	}
}

// parseIndexEntry reads single entry from binary stream.
func parseAndAssertIndexEntry(t *testing.T, reader *bytes.Reader, filepath string, expectedContent []byte) {
	t.Helper()

	// File mode (4 bytes)
	var fileMode uint32
	if err := binary.Read(reader, binary.BigEndian, &fileMode); err != nil {
		t.Fatalf("Failed to read file mode: %v", err)
	}

	// Object hash (20 bytes)
	hashBytes := make([]byte, constants.HashByteLength)
	if _, err := io.ReadFull(reader, hashBytes); err != nil {
		t.Fatalf("Failed to read hash: %v", err)
	}

	// Verify hash matches file content
	expectedHash, err := utils.ComputeHash(expectedContent, utils.BlobObjectType)
	if err != nil {
		t.Fatalf("Failed to compute expected hash for %s: %v", filepath, err)
	}

	hash := fmt.Sprintf("%x", hashBytes)
	if hash != expectedHash {
		t.Fatalf("Hash mismatch for %s: expected %s, got %s", filepath, expectedHash, hash)
	}

	// File size (8 bytes)
	var fileSize int64
	if err := binary.Read(reader, binary.BigEndian, &fileSize); err != nil {
		t.Fatalf("Failed to read file size: %v", err)
	}
	// Verify file size
	if fileSize != int64(len(expectedContent)) {
		t.Fatalf("Size mismatch for %s: expected %d, got %d", filepath, len(expectedContent), fileSize)
	}

	// Modified time (8 bytes)
	var lastModified int64
	if err := binary.Read(reader, binary.BigEndian, &lastModified); err != nil {
		t.Fatalf("Failed to read modified time: %v", err)
	}

	// Path length (2 bytes)
	var pathLength uint16
	if err := binary.Read(reader, binary.BigEndian, &pathLength); err != nil {
		t.Fatalf("Failed to read path length: %v", err)
	}

	// Path (N bytes)
	pathBytes := make([]byte, pathLength)
	if _, err := io.ReadFull(reader, pathBytes); err != nil {
		t.Fatalf("Failed to read path: %v", err)
	}
	// verify expected file path
	if string(pathBytes) != filepath {
		t.Fatalf("Expected file path to be [%s] but got [%s]", filepath, string(pathBytes))
	}

	// Null terminator (1 byte)
	var nullByte byte
	if err := binary.Read(reader, binary.BigEndian, &nullByte); err != nil {
		t.Fatalf("Failed to read null terminator: %v", err)
	}
	if nullByte != constants.NullByte {
		t.Errorf("Invalid null terminator: expected 0x00, got 0x%02x", nullByte)
	}
}

// extractObjectBody strips the git object header ("type size\0") and returns
// the body content as a string. Fails the test if the null separator is missing.
func extractObjectBody(t *testing.T, data []byte) string {
	t.Helper()

	nullIndex := bytes.IndexByte(data, 0)
	if nullIndex == -1 {
		t.Fatal("Invalid object format: no null byte separator found")
	}

	return string(data[nullIndex+1:])
}

// extractFieldFromCommitBody scans the commit body for a line starting with
// "<field> " and returns the value after the space.
// Returns empty string if the field is not found.
func extractFieldFromCommitBody(t *testing.T, body, field string) string {
	t.Helper()

	prefix := field + " "
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}

	return ""
}

// assertCommitObjectContent verifies a decompressed commit object has the correct
// type header and contains the expected commit message.
func assertCommitObjectContent(t *testing.T, decompressedData []byte, expectedMessage string) {
	t.Helper()

	if !bytes.HasPrefix(decompressedData, []byte("commit ")) {
		t.Fatalf("Object is not a commit, starts with: %q", string(decompressedData[:20]))
	}

	body := extractObjectBody(t, decompressedData)

	if !strings.Contains(body, "tree ") {
		t.Fatal("Commit object missing 'tree' field")
	}

	if !strings.Contains(body, "author ") {
		t.Fatal("Commit object missing 'author' field")
	}

	if !strings.Contains(body, expectedMessage) {
		t.Fatalf("Commit object missing message %q.\nCommit body:\n%s", expectedMessage, body)
	}
}
