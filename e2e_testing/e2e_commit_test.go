package e2etesting

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/hasher"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
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

	addCmd := newGogitCmd(t, constants.AddCmdName, testFileName)
	addCmd.Dir = repoPath
	if output, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("add command failed: %v\nOutput: %s", err, output)
	}

	// Execute commit
	commitMessage := "initial commit"
	commitCmd := newGogitCmd(t, constants.CommitCmdName, "-m", commitMessage)
	commitCmd.Dir = repoPath
	output, err := commitCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("commit command failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)

	// Verify output contains the commit message
	if !strings.Contains(outputStr, commitMessage) {
		t.Fatalf("expected output to contain message [%s], got: [%s]", commitMessage, outputStr)
	}

	// Read ref file and verify it contains a valid hash
	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, constants.DefaultBranch)
	refContent, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("failed to read ref file: %v", err)
	}
	commitHash := strings.TrimSpace(string(refContent))

	if len(commitHash) != constants.HashStringLength {
		t.Fatalf("expected %d-char hash in ref file, got %d: %s", constants.HashStringLength, len(commitHash), commitHash)
	}

	// Verify output contains the short hash
	shortHash := commitHash[:7]
	if !strings.Contains(outputStr, shortHash) {
		t.Fatalf("expected output to contain short hash %q, got: %s", shortHash, outputStr)
	}

	// Verify commit object is readable and contains expected message
	commit := readCommitByHash(t, repoPath, commitHash)
	if commit.Message() != commitMessage {
		t.Fatalf("expected commit message to be [%s], got [%s]", commitMessage, commit.Message())
	}

	// Verify commit is an initial commit (no parent)
	if !commit.IsInitialCommit() {
		t.Fatalf("expected initial commit (with no parent), got parent [%s]", commit.ParentHash())
	}

	// Verify tree object is readable
	store := objects.NewObjectStore(repoPath)
	treeHash := commit.TreeHash()
	if len(treeHash) != constants.HashStringLength {
		t.Fatalf("expected %d-char tree hash, got %d: %q", constants.HashStringLength, len(treeHash), treeHash)
	}
	if _, err := store.ReadTree(treeHash); err != nil {
		t.Fatalf("failed to read tree object [%s]: %v", treeHash, err)
	}

	// Verify blob object for the staged file exists
	expectedBlobHash, err := hasher.ComputeHash(testFileContent, hasher.Blob)
	if err != nil {
		t.Fatalf("failed to compute expected blob hash: %v", err)
	}
	if !store.Exists(expectedBlobHash) {
		t.Fatalf("blob object [%s] not found in object store", expectedBlobHash)
	}
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

	addCmd1 := newGogitCmd(t, constants.AddCmdName, testFileName)
	addCmd1.Dir = repoPath
	if output, err := addCmd1.CombinedOutput(); err != nil {
		t.Fatalf("first add failed: %v\nOutput: %s", err, output)
	}

	commitCmd1 := newGogitCmd(t, constants.CommitCmdName, "-m", "first")
	commitCmd1.Dir = repoPath
	if output, err := commitCmd1.CombinedOutput(); err != nil {
		t.Fatalf("first commit failed: %v\nOutput: %s", err, output)
	}

	firstHashBytes, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("failed to read ref after first commit: %v", err)
	}
	firstHash := strings.TrimSpace(string(firstHashBytes))

	// Second: modify, re-stage, commit
	testutils.CreateTestFile(t, repoPath, testFileName, []byte("version 2"))

	addCmd2 := newGogitCmd(t, constants.AddCmdName, testFileName)
	addCmd2.Dir = repoPath
	if output, err := addCmd2.CombinedOutput(); err != nil {
		t.Fatalf("second add failed: %v\nOutput: %s", err, output)
	}

	commitCmd2 := newGogitCmd(t, constants.CommitCmdName, "-m", "second")
	commitCmd2.Dir = repoPath
	output, err := commitCmd2.CombinedOutput()
	if err != nil {
		t.Fatalf("second commit failed: %v\nOutput: %s", err, output)
	}

	secondHashBytes, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("failed to read ref after second commit: %v", err)
	}
	secondHash := strings.TrimSpace(string(secondHashBytes))

	// Hashes must differ
	if firstHash == secondHash {
		t.Fatal("first and second commit hashes must differ")
	}

	// Read both commits via ObjectStore
	firstCommit := readCommitByHash(t, repoPath, firstHash)
	secondCommit := readCommitByHash(t, repoPath, secondHash)

	// Verify first commit has no parent
	if !firstCommit.IsInitialCommit() {
		t.Fatalf("first commit should have no parent, got parent [%s]", firstCommit.ParentHash())
	}

	// Verify second commit references first as parent
	if secondCommit.ParentHash() != firstHash {
		t.Fatalf("second commit parent mismatch: expected [%s], got [%s]", firstHash, secondCommit.ParentHash())
	}

	// Verify tree hashes differ between commits with different file content
	firstTreeHash := firstCommit.TreeHash()
	secondTreeHash := secondCommit.TreeHash()
	if firstTreeHash == secondTreeHash {
		t.Fatal("tree hashes must differ between commits with different file content")
	}

	// Verify both tree objects are readable
	store := objects.NewObjectStore(repoPath)
	for _, treeHash := range []string{firstTreeHash, secondTreeHash} {
		if _, err := store.ReadTree(treeHash); err != nil {
			t.Fatalf("failed to read tree object [%s]: %v", treeHash, err)
		}
	}
}

// TestE2E_CommitCommand_DeleteAllFilesCommitsEmptyTree verifies the complete
// add/commit/delete/add/commit workflow. Repository-wide staging must report
// every deletion in path order, persist an empty index, and create a commit
// whose canonical empty tree replaces the non-empty parent tree.
func TestE2E_CommitCommand_DeleteAllFilesCommitsEmptyTree(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	rootPath := "a-" + testutils.RandomString(10)
	nestedDirectory := "z-" + testutils.RandomString(10)
	nestedPath := filepath.Join(nestedDirectory, testutils.RandomString(11))
	testutils.CreateTestFile(t, repoPath, rootPath, testutils.RandomBytes(20))
	testutils.CreateTestFileWithDirs(t, repoPath, nestedPath, testutils.RandomBytes(21))

	addCommand := newGogitCmd(t, constants.AddCmdName, ".")
	addCommand.Dir = repoPath
	if output, err := addCommand.CombinedOutput(); err != nil {
		t.Fatalf("failed to stage initial repository: %v\nOutput: %s", err, output)
	}
	commitCommand := newGogitCmd(t, constants.CommitCmdName, "-m", "non-empty parent")
	commitCommand.Dir = repoPath
	if output, err := commitCommand.CombinedOutput(); err != nil {
		t.Fatalf("failed to create non-empty parent commit: %v\nOutput: %s", err, output)
	}
	parentHash := readBranchRefHash(t, repoPath, constants.DefaultBranch)
	parentCommit := readCommitByHash(t, repoPath, parentHash)
	if parentCommit.TreeHash() == testutils.CanonicalEmptyTreeHash {
		t.Fatal("expected initial commit to contain a non-empty tree")
	}

	if err := os.Remove(filepath.Join(repoPath, rootPath)); err != nil {
		t.Fatalf("failed to remove root tracked file: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(repoPath, nestedDirectory)); err != nil {
		t.Fatalf("failed to remove nested tracked file: %v", err)
	}

	addCommand = newGogitCmd(t, constants.AddCmdName, ".")
	addCommand.Dir = repoPath
	deletionOutput, err := addCommand.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to stage all deletions: %v\nOutput: %s", err, deletionOutput)
	}
	expectedDeletionOutput := "deleted '" + filepath.ToSlash(rootPath) + "'\n" +
		"deleted '" + filepath.ToSlash(nestedPath) + "'\n"
	if string(deletionOutput) != expectedDeletionOutput {
		t.Fatalf("expected sorted deletion output [%q], got [%q]", expectedDeletionOutput, deletionOutput)
	}
	assertIndexCreationAndContent(t, repoPath, map[string][]byte{})

	commitCommand = newGogitCmd(t, constants.CommitCmdName, "-m", "delete all files")
	commitCommand.Dir = repoPath
	if output, err := commitCommand.CombinedOutput(); err != nil {
		t.Fatalf("failed to commit empty tree: %v\nOutput: %s", err, output)
	}
	commitHash := readBranchRefHash(t, repoPath, constants.DefaultBranch)
	commit := readCommitByHash(t, repoPath, commitHash)
	if commit.ParentHash() != parentHash {
		t.Fatalf("expected parent hash [%s], got [%s]", parentHash, commit.ParentHash())
	}
	if commit.TreeHash() != testutils.CanonicalEmptyTreeHash {
		t.Fatalf("expected empty tree hash [%s], got [%s]", testutils.CanonicalEmptyTreeHash, commit.TreeHash())
	}
	if _, err := objects.NewObjectStore(repoPath).ReadTree(commit.TreeHash()); err != nil {
		t.Fatalf("failed to read committed empty tree: %v", err)
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

	commitCmd := newGogitCmd(t, constants.CommitCmdName, "-m", "empty commit")
	commitCmd.Dir = repoPath
	output, err := commitCmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected non-zero exit code for commit with no staged files")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "nothing to commit") {
		t.Fatalf("expected error containing 'nothing to commit', got: %s", outputStr)
	}

	// Verify ref file was NOT created
	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, constants.DefaultBranch)
	if _, err := os.Stat(refPath); err == nil {
		t.Fatal("ref file should not exist when commit fails")
	}
}
