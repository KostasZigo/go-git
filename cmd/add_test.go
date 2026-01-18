package cmd

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/testutils"
)

// TestAddCommand_Success verifies file staging creates blob and updates index.
func TestAddCommand_Success(t *testing.T) {

	// set up repository path
	repoPath := testutils.SetupTestRepoWithInit(t)
	changeToRepoDir(t, repoPath)

	// Create test file
	testFileName := testutils.RandomString(10)
	testFileContent := []byte(testutils.RandomString(500))
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	testRootCommand := createTestRootCmd(addCmd)
	stdout := captureStdout(testRootCommand)

	// Execute add command
	testRootCommand.SetArgs([]string{constants.AddCmdName, testFileName})
	if err := testRootCommand.Execute(); err != nil {
		t.Fatalf("add command failed: %v", err)
	}

	// Verify Output
	output := stdout.String()
	expectedOutput := "add '" + testFileName + "'"
	if !strings.Contains(output, expectedOutput) {
		t.Fatalf("Expected output to contain %s, got: %s", expectedOutput, output)
	}

	// Verify index was updated
	indexManager := index.NewManager(repoPath)
	index, err := indexManager.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	entryList := index.GetEntryList()
	if len(entryList) != 1 {
		t.Fatal("Expected a single entry to exist in the index")
	}

	entry := entryList[0]
	if entry.Path() != testFileName {
		t.Errorf("Expected path %s, got %s", testFileName, entry.Path())
	}
}

// TestAddCommand_MultipleFiles verifies staging multiple files.
func TestAddCommand_MultipleFiles(t *testing.T) {
	// set up repository path
	repoPath := testutils.SetupTestRepoWithInit(t)
	changeToRepoDir(t, repoPath)

	// Create test files
	testFileNames := []string{testutils.RandomString(10), testutils.RandomString(10), testutils.RandomString(10)}
	testFileContents := []string{testutils.RandomString(100), testutils.RandomString(150), testutils.RandomString(200)}

	for i := range testFileNames {
		testutils.CreateTestFile(t, repoPath, testFileNames[i], []byte(testFileContents[i]))
	}

	testRootCommand := createTestRootCmd(addCmd)
	stdout := captureStdout(testRootCommand)

	// Execute add command
	testRootCommand.SetArgs(append([]string{constants.AddCmdName}, testFileNames...))
	if err := testRootCommand.Execute(); err != nil {
		t.Fatalf("add command failed: %v", err)
	}

	// Verify Output
	output := stdout.String()

	orderedTestFileNames := slices.Clone(testFileNames)
	slices.Sort(orderedTestFileNames)

	var sb strings.Builder
	for _, testFileName := range orderedTestFileNames {
		fmt.Fprintf(&sb, "add '%s'\n", testFileName)
	}
	expectedOutput := sb.String()

	if !strings.Contains(output, expectedOutput) {
		t.Fatalf("Expected output to contain %s, got: %s", expectedOutput, output)
	}

	// Verify all files appear as index entries
	indexManager := index.NewManager(repoPath)
	index, err := indexManager.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	indexEntryList := index.GetEntryList()
	if len(indexEntryList) != len(testFileNames) {
		t.Fatalf("Expected %d entries, got %d", len(testFileNames), index.CountEntries())
	}

	for i, indexEntry := range indexEntryList {
		if indexEntry.Path() != orderedTestFileNames[i] {
			t.Fatalf("File %s is missing from index or is in the right order, got [%s]", orderedTestFileNames[i], indexEntry.Path())
		}
	}
}

// TestAddCommand_FileNotFound verifies error for missing file.
func TestAddCommand_FileNotFound(t *testing.T) {
	// set up repository path
	repoPath := testutils.SetupTestRepoWithInit(t)
	changeToRepoDir(t, repoPath)

	// Create test file
	testFileName := testutils.RandomString(10)

	testRootCommand := createTestRootCmd(addCmd)
	stderr := captureStderr(testRootCommand)

	// Execute add command
	testRootCommand.SetArgs([]string{constants.AddCmdName, testFileName})
	if err := testRootCommand.Execute(); err == nil {
		t.Fatalf("Expected add command to fail with error for non-existent file but it succeeded: %v", err)
	}

	expectedErrorMessage := "Error: failed to add file " + testFileName + ": failed to stat file "
	if !strings.Contains(stderr.String(), expectedErrorMessage) {
		t.Errorf("Expected [%s] error, got: %v", expectedErrorMessage, stderr.String())
	}
}

// TestAddCommand_NotInRepository verifies error when outside repository.
func TestAddCommand_NotInRepository(t *testing.T) {
	tempDir := t.TempDir()
	changeToRepoDir(t, tempDir)

	testFileName := testutils.RandomString(10)
	testutils.CreateTestFile(t, tempDir, testFileName, []byte(testutils.RandomString(100)))

	testRootCmd := createTestRootCmd(addCmd)
	stderr := captureStderr(testRootCmd)

	testRootCmd.SetArgs([]string{constants.AddCmdName, testFileName})
	if err := testRootCmd.Execute(); err == nil {
		t.Fatal("Expected error when file not in a repository")
	}

	expectedErrorMessage := constants.Gogit + " directory not found"
	if !strings.Contains(stderr.String(), expectedErrorMessage) {
		t.Errorf("Expected [%s] error, got: %v", expectedErrorMessage, stderr.String())
	}
}

// TestAddCommand_UpdateExistingFile verifies updating already-staged file.
func TestAddCommand_UpdateExistingFile(t *testing.T) {

	// set up repository path
	repoPath := testutils.SetupTestRepoWithInit(t)
	changeToRepoDir(t, repoPath)

	// Create test file
	testFileName := testutils.RandomString(10)
	testFileContent := []byte(testutils.RandomString(500))
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	testRootCommand := createTestRootCmd(addCmd)
	captureStdout(testRootCommand)

	// Execute add command
	testRootCommand.SetArgs([]string{constants.AddCmdName, testFileName})
	if err := testRootCommand.Execute(); err != nil {
		t.Fatalf("add command failed: %v", err)
	}

	// Load hash from index
	indexManager := index.NewManager(repoPath)
	loadedIndex, err := indexManager.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}
	if loadedIndex.CountEntries() != 1 {
		t.Fatalf("Expected a single entry inside the index but got %d", loadedIndex.CountEntries())
	}
	entry := loadedIndex.GetEntryList()[0]
	originalHash := entry.Hash()
	originalFileSize := entry.FileSize()

	// Modify file
	modifiedFileContent := testutils.RandomString(1000)
	testutils.CreateTestFile(t, repoPath, testFileName, []byte(modifiedFileContent))

	// Run command again to add the modified dile
	if err := testRootCommand.Execute(); err != nil {
		t.Fatalf("add command failed: %v", err)
	}

	// Load modified hash from index
	indexManager = index.NewManager(repoPath)
	loadedIndex, err = indexManager.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}
	if loadedIndex.CountEntries() != 1 {
		t.Fatalf("Expected a single entry inside the index but got %d", loadedIndex.CountEntries())
	}
	entry = loadedIndex.GetEntryList()[0]
	modifiedHash := entry.Hash()
	modifiedFileSize := entry.FileSize()

	if originalHash == modifiedHash {
		t.Error("Expected hash to change after file modification")
	}

	if originalFileSize == modifiedFileSize {
		t.Error("Expected file size to change after file modification")
	}
}
