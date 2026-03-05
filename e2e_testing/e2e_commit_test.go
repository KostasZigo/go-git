package e2etesting

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/testutils"
	"github.com/KostasZigo/gogit/utils"
)

// TestE2E_CommitCommand_FirstCommit verifies the full init → add → commit workflow.
// Stages a file and commits with a message. Verifies exit code 0, stdout contains
// the short commit hash and message, ref file is created, and the commit object
// is stored with correct tree reference.
func TestE2E_CommitCommand_FirstCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	// Create and stage a file
	testFileName := testutils.RandomString(10)
	testFileContent := []byte(testutils.RandomString(100))
	testutils.CreateTestFile(t, repoPath, testFileName, testFileContent)

	addCmd := exec.Command(sharedBinaryPath, constants.AddCmdName, testFileName)
	addCmd.Dir = repoPath
	if output, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("add command failed: %v\nOutput: %s", err, output)
	}

	// Execute commit
	commitMessage := "initial commit"
	commitCmd := exec.Command(sharedBinaryPath, constants.CommitCmdName, "-m", commitMessage)
	commitCmd.Dir = repoPath
	output, err := commitCmd.CombinedOutput()

	if err != nil {
		t.Fatalf("commit command failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)

	// Verify output contains the commit message
	if !strings.Contains(outputStr, commitMessage) {
		t.Fatalf("Expected output to contain message [%s], got: [%s]", commitMessage, outputStr)
	}

	// Read ref file and verify it contains a valid hash
	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, constants.DefaultBranch)
	refContent, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("Failed to read ref file: %v", err)
	}
	commitHash := strings.TrimSpace(string(refContent))

	if len(commitHash) != constants.HashStringLength {
		t.Fatalf("Expected %d-char hash in ref file, got %d: %s", constants.HashStringLength, len(commitHash), commitHash)
	}

	// Verify output contains the short hash
	shortHash := commitHash[:7]
	if !strings.Contains(outputStr, shortHash) {
		t.Fatalf("Expected output to contain short hash %q, got: %s", shortHash, outputStr)
	}

	// Verify commit object exists
	commitObjectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, commitHash[:constants.HashDirPrefixLength], commitHash[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, commitObjectPath)

	commitData := decompressObject(t, commitObjectPath)
	assertCommitObjectContent(t, commitData, commitMessage)

	// Verify tree object exists and has correct type header
	commitBody := extractObjectBody(t, commitData)
	treeHash := extractFieldFromCommitBody(t, commitBody, "tree")
	if len(treeHash) != constants.HashStringLength {
		t.Fatalf("Expected %d-char tree hash, got %d: %q", constants.HashStringLength, len(treeHash), treeHash)
	}

	treeObjectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, treeHash[:constants.HashDirPrefixLength], treeHash[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, treeObjectPath)

	treeData := decompressObject(t, treeObjectPath)
	if !bytes.HasPrefix(treeData, []byte("tree ")) {
		t.Fatalf("Tree object has wrong type header: %q", string(treeData[:20]))
	}

	// Verify blob object for the staged file exists
	expectedBlobHash, err := utils.ComputeHash(testFileContent, utils.BlobObjectType)
	if err != nil {
		t.Fatalf("Failed to compute expected blob hash: %v", err)
	}
	blobObjectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, expectedBlobHash[:constants.HashDirPrefixLength], expectedBlobHash[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, blobObjectPath)
}

// TestE2E_CommitCommand_FullWorkflow verifies the multi-commit workflow:
// init → add → commit → modify → add → commit. Verifies two distinct commits
// are created, the second commit's parent references the first, both commit
// objects exist in the object store, and the ref file points to the latest commit.
func TestE2E_CommitCommand_FullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	testFileName := testutils.RandomString(10)
	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, constants.DefaultBranch)

	// First: create, stage, commit
	testutils.CreateTestFile(t, repoPath, testFileName, []byte("version 1"))

	addCmd1 := exec.Command(sharedBinaryPath, constants.AddCmdName, testFileName)
	addCmd1.Dir = repoPath
	if output, err := addCmd1.CombinedOutput(); err != nil {
		t.Fatalf("first add failed: %v\nOutput: %s", err, output)
	}

	commitCmd1 := exec.Command(sharedBinaryPath, constants.CommitCmdName, "-m", "first")
	commitCmd1.Dir = repoPath
	if output, err := commitCmd1.CombinedOutput(); err != nil {
		t.Fatalf("first commit failed: %v\nOutput: %s", err, output)
	}

	firstHashBytes, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("Failed to read ref after first commit: %v", err)
	}
	firstHash := strings.TrimSpace(string(firstHashBytes))

	// Second: modify, re-stage, commit
	testutils.CreateTestFile(t, repoPath, testFileName, []byte("version 2"))

	addCmd2 := exec.Command(sharedBinaryPath, constants.AddCmdName, testFileName)
	addCmd2.Dir = repoPath
	if output, err := addCmd2.CombinedOutput(); err != nil {
		t.Fatalf("second add failed: %v\nOutput: %s", err, output)
	}

	commitCmd2 := exec.Command(sharedBinaryPath, constants.CommitCmdName, "-m", "second")
	commitCmd2.Dir = repoPath
	output, err := commitCmd2.CombinedOutput()
	if err != nil {
		t.Fatalf("second commit failed: %v\nOutput: %s", err, output)
	}

	secondHashBytes, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("Failed to read ref after second commit: %v", err)
	}
	secondHash := strings.TrimSpace(string(secondHashBytes))

	// Hashes must differ
	if firstHash == secondHash {
		t.Fatal("First and second commit hashes must differ")
	}

	// Both commit objects must exist
	firstObjectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, firstHash[:constants.HashDirPrefixLength], firstHash[constants.HashDirPrefixLength:])
	secondObjectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, secondHash[:constants.HashDirPrefixLength], secondHash[constants.HashDirPrefixLength:])
	testutils.AssertFileExists(t, firstObjectPath)
	testutils.AssertFileExists(t, secondObjectPath)

	// Verify second commit references first as parent
	secondCommitData := decompressObject(t, secondObjectPath)
	secondCommitBody := extractObjectBody(t, secondCommitData)

	if !strings.Contains(secondCommitBody, "parent "+firstHash) {
		t.Fatalf("Second commit missing parent reference to first commit.\nExpected parent: %s\nCommit body:\n%s", firstHash, secondCommitBody)
	}

	// Verify first commit has no parent
	firstCommitData := decompressObject(t, firstObjectPath)
	firstCommitBody := extractObjectBody(t, firstCommitData)

	if strings.Contains(firstCommitBody, "parent ") {
		t.Fatalf("First commit should have no parent.\nCommit body:\n%s", firstCommitBody)
	}

	// Extract tree hashes from both commits and verify they differ
	firstTreeHash := extractFieldFromCommitBody(t, firstCommitBody, "tree")
	secondTreeHash := extractFieldFromCommitBody(t, secondCommitBody, "tree")

	if firstTreeHash == secondTreeHash {
		t.Fatal("Tree hashes must differ between commits with different file content")
	}

	// Verify both tree objects exist and have correct type header
	for _, treeHash := range []string{firstTreeHash, secondTreeHash} {
		treeObjectPath := filepath.Join(repoPath, constants.Gogit, constants.Objects, treeHash[:constants.HashDirPrefixLength], treeHash[constants.HashDirPrefixLength:])
		testutils.AssertFileExists(t, treeObjectPath)

		treeData := decompressObject(t, treeObjectPath)
		if !bytes.HasPrefix(treeData, []byte("tree ")) {
			t.Fatalf("Object %s is not a tree: %q", treeHash[:7], string(treeData[:20]))
		}
	}

	// Verify ref file points to second commit
	if secondHash != strings.TrimSpace(string(secondHashBytes)) {
		t.Fatalf("Ref file should point to second commit %s", secondHash)
	}
}

// TestE2E_CommitCommand_NoStagedFiles verifies commit fails with non-zero exit code
// and appropriate error message when no files have been staged.
func TestE2E_CommitCommand_NoStagedFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	commitCmd := exec.Command(sharedBinaryPath, constants.CommitCmdName, "-m", "empty commit")
	commitCmd.Dir = repoPath
	output, err := commitCmd.CombinedOutput()

	if err == nil {
		t.Fatal("Expected non-zero exit code for commit with no staged files")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "nothing to commit") {
		t.Fatalf("Expected error containing 'nothing to commit', got: %s", outputStr)
	}

	// Verify ref file was NOT created
	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, constants.DefaultBranch)
	if _, err := os.Stat(refPath); err == nil {
		t.Fatal("Ref file should not exist when commit fails")
	}
}
