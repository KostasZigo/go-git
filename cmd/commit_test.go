package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestCommitCommand_FirstCommitWithStagedFiles
// Stages a file via the add command, then executes commit via the CLI.
// Verifies: stdout contains short hash + message, ref file created with
// matching hash, commit object readable with correct tree and message.
func TestCommitCommand_FirstCommitWithStagedFiles(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	command := createTestRootCmd(commitCmd)
	stdout := captureStdout(command)

	stageRandomFile(t, repoPath)

	// Execute commit command
	message := testutils.RandomString(10)
	command.SetArgs([]string{constants.CommitCmdName, "-m", message})
	if err := command.Execute(); err != nil {
		t.Fatalf("commit command failed: %v", err)
	}

	// Verify stdout contains short hash and message
	output := stdout.String()
	if !strings.Contains(output, message) {
		t.Fatalf("expected output to contain message [%s], got: [%s]", message, output)
	}

	// Extract commit hash from ref file
	commitHash := testutils.ReadDefaultRefFile(t, repoPath)
	if len(commitHash) != constants.HashStringLength {
		t.Fatalf("expected %d-char hash in ref file, got [%d]: %s", constants.HashStringLength, len(commitHash), commitHash)
	}

	// Verify stdout contains the short hash from ref file
	expectedOutputHash := commitHash[:7]
	if !strings.Contains(output, expectedOutputHash) {
		t.Fatalf("expected output to contain short hash [%s], got [%s]", expectedOutputHash, output)
	}

	// Verify commit object is readable with correct fields
	store := objects.NewObjectStore(repoPath)
	commit, err := store.ReadCommit(commitHash)
	if err != nil {
		t.Fatalf("failed to read commit object: %v", err)
	}

	if commit.Message() != message {
		t.Fatalf("expected commit message [%s], got [%s]", message, commit.Message())
	}

	if commit.ParentHash() != "" {
		t.Fatalf("expected empty parent hash for first commit, got [%s]", commit.ParentHash())
	}

	// Verify tree referenced by commit is readable
	_, err = store.ReadTree(commit.TreeHash())
	if err != nil {
		t.Fatalf("failed to read tree referenced by commit: %v", err)
	}
}

// TestCommitCommand_SecondCommitAfterModification
// Commits once, modifies and re-stages the file via add command, commits
// again. Verifies: second commit's parent points to first, tree hashes
// differ, ref file updated to second commit.
func TestCommitCommand_SecondCommitAfterModification(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	fileName := testutils.RandomString(10)
	fileContent := testutils.RandomByteSlice(100)
	stageFile(t, fileName, repoPath, fileContent)

	// Execute initial commit command
	message := testutils.RandomString(10)
	command := createTestRootCmd(commitCmd)
	captureStdout(command)
	command.SetArgs([]string{constants.CommitCmdName, "-m", message})
	if err := command.Execute(); err != nil {
		t.Fatalf("commit command failed: %v", err)
	}

	firstHash := testutils.ReadDefaultRefFile(t, repoPath)

	// Commit modified file
	updatedFileContent := slices.Concat(fileContent, []byte("-v2"))
	stageFile(t, fileName, repoPath, updatedFileContent)
	if err := command.Execute(); err != nil {
		t.Fatalf("commit command failed: %v", err)
	}

	secondHash := testutils.ReadDefaultRefFile(t, repoPath)
	if firstHash == secondHash {
		t.Fatal("first and second commit hashes must differ")
	}

	// Verify parent chain and tree difference
	store := objects.NewObjectStore(repoPath)
	firstCommit, err := store.ReadCommit(firstHash)
	if err != nil {
		t.Fatalf("failed to read first commit: %v", err)
	}

	secondCommit, err := store.ReadCommit(secondHash)
	if err != nil {
		t.Fatalf("failed to read second commit: %v", err)
	}

	if secondCommit.ParentHash() != firstHash {
		t.Fatalf("second commit parent [%s] does not match first commit [%s]", secondCommit.ParentHash(), firstHash)
	}

	if firstCommit.TreeHash() == secondCommit.TreeHash() {
		t.Fatalf("tree hashes must differ after file modification - they are both [%s]", firstCommit.TreeHash())
	}
}

// TestCommitCommand_EmptyIndex
// Executes commit with no staged files. Verifies error output contains
// "nothing to commit".
func TestCommitCommand_EmptyIndex(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	command := createTestRootCmd(commitCmd)
	outErr := captureStderr(command)
	command.SetArgs([]string{constants.CommitCmdName, "-m", testutils.RandomString(10)})
	if err := command.Execute(); err == nil {
		t.Fatalf("expected commit with no staged files to fail with 'nothing to cmmit'")
	}

	expectedErrorMessage := "nothing to commit"
	errorOutput := outErr.String()
	if !strings.Contains(errorOutput, expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], but got [%s]", expectedErrorMessage, errorOutput)
	}
}

// TestCommitCommand_DuplicateCommitNoChanges
// Stages and commits once, then attempts a second commit without any
// modifications. Verifies error output contains "nothing to commit".
func TestCommitCommand_DuplicateCommitNoChanges(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	fileName := testutils.RandomString(10)
	fileContent := testutils.RandomByteSlice(100)
	stageFile(t, fileName, repoPath, fileContent)

	// Execute initial commit command
	message := testutils.RandomString(10)
	command := createTestRootCmd(commitCmd)
	captureStdout(command)
	errOut := captureStderr(command)
	command.SetArgs([]string{constants.CommitCmdName, "-m", message})
	if err := command.Execute(); err != nil {
		t.Fatalf("commit command failed: %v", err)
	}

	// Execute commit command again without any staged files
	if err := command.Execute(); err == nil {
		t.Fatalf("expected commit with no staged files to fail with 'nothing to cmmit'")
	}

	errorOutput := errOut.String()
	expectedErrorMessage := "nothing to commit: working tree clean"
	if !strings.Contains(errorOutput, expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], but got [%s]", expectedErrorMessage, errorOutput)
	}
}

// TestCommitCommand_MissingMessageFlag
// Executes commit without the -m flag. Verifies error output contains
// "commit message required".
func TestCommitCommand_MissingMessageFlag(t *testing.T) {
	// Reset package-level flag to prevent leakage from prior tests.
	// Cobra binds flags to package-level variables via StringVarP and
	// does not reset them between Execute() calls within the same process.
	messageFlag = ""

	command := createTestRootCmd(commitCmd)
	outErr := captureStderr(command)
	command.SetArgs([]string{constants.CommitCmdName})
	if err := command.Execute(); err == nil {
		t.Fatalf("expected commit with no message flag to fail")
	}

	expectedErrorMessage := "commit message required: use -m \"your message\""
	errorOutput := outErr.String()
	if !strings.Contains(errorOutput, expectedErrorMessage) {
		t.Fatalf("expected error message to contain [%s], but got [%s]", expectedErrorMessage, errorOutput)
	}
}

// TestCommitCommand_DeeplyNestedDirectoryStructure
// Commit with deeply nested directory structure (a/b/c/d.go)
// Stages a file four directories deep and commits via CLI. Walks the
// tree chain root → a → b → c → d.go verifying all intermediate tree
// objects are stored and each parent tree references its child correctly.
func TestCommitCommand_DeeplyNestedDirectoryStructure(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	// Create deeply nested file and stage via add command
	firstLevel := testutils.RandomString(10)
	secondLevel := testutils.RandomString(10)
	thirdLevel := testutils.RandomString(10)
	nestedPath := filepath.Join(repoPath, firstLevel, secondLevel, thirdLevel)
	nestedFileName := testutils.RandomString(10)
	nestedContent := testutils.RandomByteSlice(100)

	if err := os.MkdirAll(nestedPath, constants.DirPerms); err != nil {
		t.Fatalf("failed to create directory %s: %v", nestedPath, err)
	}

	stageFile(t, nestedFileName, nestedPath, nestedContent)

	// Execute commit
	commitRoot := createTestRootCmd(commitCmd)
	captureStdout(commitRoot)
	commitRoot.SetArgs([]string{constants.CommitCmdName, "-m", testutils.RandomString(10)})
	if err := commitRoot.Execute(); err != nil {
		t.Fatalf("commit command failed: %v", err)
	}

	// Read commit and walk tree chain
	commitHash := testutils.ReadDefaultRefFile(t, repoPath)
	store := objects.NewObjectStore(repoPath)

	commit, err := store.ReadCommit(commitHash)
	if err != nil {
		t.Fatalf("failed to read commit: %v", err)
	}

	// root tree → must have single entry
	rootTree, err := store.ReadTree(commit.TreeHash())
	if err != nil {
		t.Fatalf("failed to read root tree: %v", err)
	}
	rootEntries := rootTree.Entries()
	if len(rootEntries) != 1 || rootEntries[0].Name() != firstLevel {
		t.Fatalf("expected single root entry [%s], got %v", firstLevel, treeEntryNames(rootEntries))
	}

	treeA, err := store.ReadTree(rootEntries[0].Hash())
	if err != nil {
		t.Fatalf("failed to read 1st level tree: %v", err)
	}
	entriesA := treeA.Entries()
	if len(entriesA) != 1 || entriesA[0].Name() != secondLevel {
		t.Fatalf("expected single entry [%s] in first level tree, got %v", secondLevel, treeEntryNames(entriesA))
	}

	treeB, err := store.ReadTree(entriesA[0].Hash())
	if err != nil {
		t.Fatalf("failed to read second level tree: %v", err)
	}
	entriesB := treeB.Entries()
	if len(entriesB) != 1 || entriesB[0].Name() != thirdLevel {
		t.Fatalf("expected single entry [%s] in second level tree, got %v", thirdLevel, treeEntryNames(entriesB))
	}

	treeC, err := store.ReadTree(entriesB[0].Hash())
	if err != nil {
		t.Fatalf("failed to read third level tree: %v", err)
	}
	entriesC := treeC.Entries()
	if len(entriesC) != 1 || entriesC[0].Name() != nestedFileName {
		t.Fatalf("expected single entry [%s] in tree 'c', got %v", nestedFileName, treeEntryNames(entriesC))
	}

	// Verify blob content
	blob, err := store.ReadBlob(entriesC[0].Hash())
	if err != nil {
		t.Fatalf("failed to read blob 'd.go': %v", err)
	}
	if !bytes.Equal(blob.Content(), nestedContent) {
		t.Fatalf("expected blob content [%s], got [%s]", nestedContent, blob.Content())
	}
}

// TestCommitCommand_ThreeSequentialCommits_ParentChain
// Stages, modifies, and commits three times via CLI. Walks from the
// latest commit back through parent hashes to the initial commit.
// Validates the full history chain is intact and the root commit has
// no parent.
func TestCommitCommand_ThreeSequentialCommits_ParentChain(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testutils.ChangeToDir(t, repoPath)

	fileName := testutils.RandomString(10)
	firstMessage := testutils.RandomString(10)
	secondMessage := testutils.RandomString(10)
	thirdMessage := testutils.RandomString(10)
	messages := []string{firstMessage, secondMessage, thirdMessage}
	hashes := make([]string, len(messages))

	for i, msg := range messages {
		// Write distinct content for each commit
		content := testutils.RandomByteSlice(50)
		stageFile(t, fileName, repoPath, content)

		command := createTestRootCmd(commitCmd)
		captureStdout(command)
		command.SetArgs([]string{constants.CommitCmdName, "-m", msg})
		if err := command.Execute(); err != nil {
			t.Fatalf("commit [%s] failed: %v", msg, err)
		}

		hashes[i] = testutils.ReadDefaultRefFile(t, repoPath)
	}

	// Ref file must point to the latest commit
	latestCommitHash := testutils.ReadDefaultRefFile(t, repoPath)
	if hashes[2] != latestCommitHash {
		t.Fatalf("ref file expected to point to commit [%s], got [%s]", hashes[2], latestCommitHash)
	}

	// Walk parent chain: third → second → first → empty
	store := objects.NewObjectStore(repoPath)

	thirdCommit, err := store.ReadCommit(hashes[2])
	if err != nil {
		t.Fatalf("failed to read third commit: %v", err)
	}
	if thirdCommit.ParentHash() != hashes[1] {
		t.Fatalf("third commit parent [%s] != second commit [%s]", thirdCommit.ParentHash(), hashes[1])
	}
	if thirdCommit.Message() != thirdMessage {
		t.Fatalf("third commit message: expected [%s], got [%s]", thirdMessage, thirdCommit.Message())
	}

	secondCommit, err := store.ReadCommit(hashes[1])
	if err != nil {
		t.Fatalf("failed to read second commit: %v", err)
	}
	if secondCommit.ParentHash() != hashes[0] {
		t.Fatalf("second commit parent [%s] != first commit [%s]", secondCommit.ParentHash(), hashes[0])
	}
	if secondCommit.Message() != secondMessage {
		t.Fatalf("second commit message: expected [%s], got [%s]", secondMessage, secondCommit.Message())
	}

	firstCommit, err := store.ReadCommit(hashes[0])
	if err != nil {
		t.Fatalf("failed to read first commit: %v", err)
	}
	if firstCommit.ParentHash() != "" {
		t.Fatalf("first commit must have empty parent, got [%s]", firstCommit.ParentHash())
	}
	if firstCommit.Message() != firstMessage {
		t.Fatalf("first commit message: expected [%s], got [%s]", firstMessage, firstCommit.Message())
	}
}
