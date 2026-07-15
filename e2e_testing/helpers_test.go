package e2etesting

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/hasher"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
	"github.com/fatih/color"
)

// e2eCommandTimeout bounds the time any single invocation of the
// gogit binary is allowed to take during E2E tests.
// This value only exists to fail fast on hangs for example deadlocks, instead of waiting
// for `go test -timeout` to terminate the whole package.
const e2eCommandTimeout = 30 * time.Second

// sharedBinaryPath stores compiled gogit binary path built once in TestMain.
// All E2E tests execute this binary to verify end-to-end behavior.
// Binary persists for test suite duration, cleaned up after all tests complete
var sharedBinaryPath string

// newGogitCmd builds an *exec.Cmd that invokes the shared gogit binary with the
// given arguments. The command is wired to a context and bounded by e2eCommandTimeout,
//
//	so:
//	 - The child process is automatically killed when the test ends.
//	 - A hanging or deadlocked invocation fails the test after timeout.
//
// The cancel function is registered with t.Cleanup so callers do not need to
// manage it.
func newGogitCmd(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), e2eCommandTimeout)
	t.Cleanup(cancel)
	return exec.CommandContext(ctx, sharedBinaryPath, args...)
}

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

	binaryName := "gogit"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	sharedBinaryPath = filepath.Join(tempDir, binaryName)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", sharedBinaryPath, ".")
	buildCmd.Dir = ".." // execute command on root folder
	if err := buildCmd.Run(); err != nil {
		panic("Failed to build binary: " + err.Error())
	}
	cancel()

	color.NoColor = true
	exitCode := m.Run()

	os.RemoveAll(tempDir)
	os.Exit(exitCode)
}

// setupTestRepo creates test directory.
func setupTestRepo(t *testing.T) (repoPath string) {
	t.Helper()

	repoPath = filepath.Join(t.TempDir(), "test-repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("failed to create test repo dir: %v", err)
	}

	return repoPath
}

// initializeRepository runs gogit init in test directory.
func initializeRepository(t *testing.T, repoPath string) {
	t.Helper()

	cmd := newGogitCmd(t, constants.InitCmdName)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to initialize repository: %v", err)
	}
}

// assertAddCommandOutputAndObjectCreation verifies add command output and blob
// object creation and content. Checks stdout contains the expected add message,
// reads the blob by its computed hash, and compares the stored content.
func assertAddCommandOutputAndObjectCreation(t *testing.T, testFileName string, output []byte, testFileContent []byte, repoPath string) {
	t.Helper()

	expectedOutput := fmt.Sprintf("add '%s'", filepath.ToSlash(testFileName))
	if !strings.Contains(string(output), expectedOutput) {
		t.Errorf("expected output to contain %q, got: %s", expectedOutput, string(output))
	}

	assertAddCommandObjectCreation(t, testFileName, testFileContent, repoPath)
}

// assertAddCommandObjectCreation verifies add command blob object creation and content.
// Checks stdout contains the expected add message,
// reads the blob by its computed hash, and compares the stored content.
func assertAddCommandObjectCreation(t *testing.T, testFileName string, testFileContent []byte, repoPath string) {
	t.Helper()

	expectedHash, err := hasher.ComputeHash(testFileContent, hasher.Blob)
	if err != nil {
		t.Fatalf("failed to compute hash: %v", err)
	}

	store := objects.NewObjectStore(repoPath)
	blob, err := store.ReadBlob(expectedHash)
	if err != nil {
		t.Fatalf("failed to read blob object [%s]: %v", expectedHash, err)
	}

	if !bytes.Equal(blob.Content(), testFileContent) {
		t.Errorf("blob content mismatch for [%s]: expected [%s], got [%s]", testFileName, testFileContent, blob.Content())
	}
}

// assertIndexCreationAndContent verifies index cretion and content
func assertIndexCreationAndContent(t *testing.T, repoPath string, expectedFiles map[string][]byte) {
	t.Helper()

	manager := index.NewManager(repoPath)
	idx, err := manager.Load()
	if err != nil {
		t.Fatalf("failed to load index: %v", err)
	}

	if idx.CountEntries() != len(expectedFiles) {
		t.Fatalf("index entry count mismatch: expected [%d], got [%d]", len(expectedFiles), idx.CountEntries())
	}

	// Sort expected keys with forward-slash normalization to match index ordering
	sortedKeys := make([]string, 0, len(expectedFiles))
	for key := range expectedFiles {
		sortedKeys = append(sortedKeys, filepath.ToSlash(key))
	}
	slices.Sort(sortedKeys)

	entries := idx.GetEntryList()
	for i, entry := range entries {
		expectedPath := sortedKeys[i]
		if entry.Path() != expectedPath {
			t.Fatalf("index entry [%d] path mismatch: expected [%s], got [%s]", i, expectedPath, entry.Path())
		}

		// Look up content using the original key (before normalization may differ on Windows)
		var content []byte
		for originalKey, fileContent := range expectedFiles {
			if filepath.ToSlash(originalKey) == expectedPath {
				content = fileContent
				break
			}
		}

		// Verify hash matches expected content
		expectedHash, err := hasher.ComputeHash(content, hasher.Blob)
		if err != nil {
			t.Fatalf("failed to compute expected hash for [%s]: %v", entry.Path(), err)
		}
		if entry.Hash() != expectedHash {
			t.Fatalf("hash mismatch for [%s]: expected [%s], got [%s]", entry.Path(), expectedHash, entry.Hash())
		}

		// Verify file size
		if entry.FileSize() != int64(len(content)) {
			t.Fatalf("size mismatch for [%s]: expected %d, got %d", entry.Path(), len(content), entry.FileSize())
		}
	}
}

// commitWithSingleFile creates a random file, stages it via the add binary
// command, and commits it with a random message via the commit binary command.
// Fails the test if any step produces an error.
func commitWithSingleFile(t *testing.T, repoPath string) {
	t.Helper()

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
	commitMessage := testutils.RandomString(10)
	commitCmd := newGogitCmd(t, constants.CommitCmdName, "-m", commitMessage)
	commitCmd.Dir = repoPath
	output, err := commitCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("commit command failed: %v\nOutput: %s", err, output)
	}
}

// readCommitFromDefaultRef reads the commit hash from the default branch ref
// file and returns the parsed Commit object via ObjectStore.
func readCommitFromDefaultRef(t *testing.T, repoPath string) *objects.Commit {
	t.Helper()

	commitHash := testutils.ReadDefaultRefFile(t, repoPath)
	return readCommitByHash(t, repoPath, commitHash)
}

// readCommitByHash reads and parses a commit object from the ObjectStore by its
// hash.
func readCommitByHash(t *testing.T, repoPath, commitHash string) *objects.Commit {
	t.Helper()

	store := objects.NewObjectStore(repoPath)
	commit, err := store.ReadCommit(commitHash)
	if err != nil {
		t.Fatalf("failed to read commit object [%s]: %v", commitHash, err)
	}
	return commit
}

// commitWithFile creates a file with the given name and content, stages it via
// the add binary command, and commits it with a random message via the commit
// binary command. Fails the test if any step produces an error.
func commitWithFile(t *testing.T, repoPath, fileName string, fileContent []byte) {
	t.Helper()

	testutils.CreateTestFile(t, repoPath, fileName, fileContent)

	addCmd := newGogitCmd(t, constants.AddCmdName, fileName)
	addCmd.Dir = repoPath
	if output, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("add command failed: %v\nOutput: %s", err, output)
	}

	commitMessage := testutils.RandomString(10)
	commitCmd := newGogitCmd(t, constants.CommitCmdName, "-m", commitMessage)
	commitCmd.Dir = repoPath
	if output, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("commit command failed: %v\nOutput: %s", err, output)
	}
}

// writeRefFile writes a commit hash into the branch ref file at
// .gogit/refs/heads/<branchName>.
func writeRefFile(t *testing.T, repoPath, branchName, commitHash string) {
	t.Helper()

	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, branchName)
	if err := os.WriteFile(refPath, []byte(commitHash+"\n"), constants.FilePerms); err != nil {
		t.Fatalf("failed to write ref file for branch %s: %v", branchName, err)
	}
}

// assertCheckoutOutput verifies that the checkout command output contains the
// expected "checked out [<target>]" message.
func assertCheckoutOutput(t *testing.T, output, target string) {
	t.Helper()

	expected := fmt.Sprintf("checked out [%s]\n", target)
	if !strings.Contains(output, expected) {
		t.Fatalf("expected checkout output to contain [%s], got: [%s]", expected, output)
	}
}

// assertHEADContent reads .gogit/HEAD and verifies its raw content matches
// the expected string exactly.
func assertHEADContent(t *testing.T, repoPath, expectedContent string) {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(repoPath, constants.Gogit, constants.Head))
	if err != nil {
		t.Fatalf("failed to read HEAD file: %v", err)
	}
	if string(content) != expectedContent {
		t.Fatalf("expected HEAD to be [%s], got [%s]", expectedContent, string(content))
	}
}

// runBranchCommand executes `gogit branch <branchName>` inside repoPath and
// returns combined output and execution error.
func runBranchCommand(t *testing.T, repoPath, branchName string) ([]byte, error) {
	t.Helper()

	cmd := newGogitCmd(t, constants.BranchCmdName, branchName)
	cmd.Dir = repoPath
	return cmd.CombinedOutput()
}

// assertBranchOutput verifies that branch command output contains the expected
// success message for created branch name.
func assertBranchOutput(t *testing.T, output, branchName string) {
	t.Helper()

	expected := fmt.Sprintf("created branch [%s]\n", branchName)
	if !strings.Contains(output, expected) {
		t.Fatalf("expected branch output to contain [%s], got: [%s]", expected, output)
	}
}

// readBranchRefHash reads refs/heads/<branchName> and returns the trimmed hash.
func readBranchRefHash(t *testing.T, repoPath, branchName string) string {
	t.Helper()

	refPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, filepath.FromSlash(branchName))
	content, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("failed to read branch ref file for [%s]: %v", branchName, err)
	}

	return string(bytes.TrimSpace(content))
}
