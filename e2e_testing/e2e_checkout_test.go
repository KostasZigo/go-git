package e2etesting

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index/indextest"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestE2E_CheckoutCommand_IdempotentBranchSwitch commits a file on main,
// creates a feature branch pointing to the same commit, checks out the
// feature branch, then checks out the feature branch a second time.
// Verifies: both checkouts succeed with exit code 0, stdout contains
// the branch name, HEAD points to the feature branch, file content is
// unchanged between the two checkout calls.
func TestE2E_CheckoutCommand_IdempotentBranchSwitch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	// Create and commit a file on main
	fileName := testutils.RandomString(10)
	fileContent := testutils.RandomByteSlice(100)
	commitWithFile(t, repoPath, fileName, fileContent)
	commitHash := testutils.ReadDefaultRefFile(t, repoPath)

	// Create feature branch pointing to the same commit
	featureBranch := testutils.RandomString(10)
	writeRefFile(t, repoPath, featureBranch, commitHash)

	// First checkout to feature branch
	cmd := newGogitCmd(t, constants.CheckoutCmdName, featureBranch)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("checkout command failed: %v\nOutput: %s", err, output)
	}

	assertCheckoutOutput(t, string(output), featureBranch)
	assertHEADContent(t, repoPath, constants.DefaultRefPrefix+featureBranch+"\n")
	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), fileContent)

	// Second checkout to the same feature branch (idempotent)
	cmd = newGogitCmd(t, constants.CheckoutCmdName, featureBranch)
	cmd.Dir = repoPath
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("idempotent checkout command failed: %v\nOutput: %s", err, output)
	}

	assertCheckoutOutput(t, string(output), featureBranch)
	assertHEADContent(t, repoPath, constants.DefaultRefPrefix+featureBranch+"\n")
	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), fileContent)
}

// TestE2E_CheckoutCommand_DirtyWorkingTreeRejection commits a file, modifies
// it on disk without staging, then attempts checkout to the same branch.
// Verifies: command fails with non-zero exit code, error output identifies the
// worktree change, HEAD remains unchanged, modified file content is preserved.
func TestE2E_CheckoutCommand_DirtyWorkingTreeRejection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	// Create and commit a file on main
	fileName := testutils.RandomString(10)
	fileContent := testutils.RandomByteSlice(100)
	commitWithFile(t, repoPath, fileName, fileContent)
	commitHash := testutils.ReadDefaultRefFile(t, repoPath)

	// Create feature branch at the same commit
	featureBranch := testutils.RandomString(10)
	writeRefFile(t, repoPath, featureBranch, commitHash)

	// Second commit so checkout has actual work to do (different tree)
	secondFileName := testutils.RandomString(10)
	secondFileContent := testutils.RandomByteSlice(100)
	commitWithFile(t, repoPath, secondFileName, secondFileContent)

	// Modify file on disk (dirty working tree)
	dirtyContent := testutils.RandomByteSlice(200)
	testutils.CreateTestFile(t, repoPath, fileName, dirtyContent)

	// Attempt checkout — should fail
	cmd := newGogitCmd(t, constants.CheckoutCmdName, featureBranch)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected checkout to fail on dirty working tree, but it succeeded\nOutput: %s", output)
	}

	outputStr := string(output)
	expectedErrorMessage := "worktree preflight failed: Worktree change [content-modified] found for path [" + fileName + "]"
	if !strings.Contains(outputStr, expectedErrorMessage) {
		t.Fatalf("expected error output to contain [%s], got: [%s]", expectedErrorMessage, outputStr)
	}

	// HEAD must remain on main
	assertHEADContent(t, repoPath, constants.DefaultRefPrefix+constants.DefaultBranch+"\n")

	// Dirty file content must be preserved
	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), dirtyContent)
}

// TestE2E_CheckoutCommand_NonexistentTarget runs checkout with a target that
// does not exist as a branch or commit hash. Verifies: command fails with
// non-zero exit code, error output indicates the target was not found.
func TestE2E_CheckoutCommand_NonexistentTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	nonexistentTarget := testutils.RandomString(10)

	cmd := newGogitCmd(t, constants.CheckoutCmdName, nonexistentTarget)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected checkout to fail for nonexistent target [%s], but it succeeded\nOutput: %s", nonexistentTarget, output)
	}

	outputStr := string(output)
	expectedMessage := "not found"
	if !strings.Contains(outputStr, expectedMessage) {
		t.Fatalf("expected error output to contain [%s], got: [%s]", expectedMessage, outputStr)
	}
}

// TestE2E_CheckoutCommand_NestedDirectoryCleanupAndRestore commits files under
// a/b/c/, creates a branch pointing to a flat-structure commit (single root
// file), checks out that branch, then checks out back. Verifies: nested
// directories are removed on first checkout and only the root file exists,
// nested structure is fully restored on second checkout.
func TestE2E_CheckoutCommand_NestedDirectoryCleanupAndRestore(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	// First commit: single root-level file
	rootFileName := testutils.RandomString(10)
	rootFileContent := testutils.RandomByteSlice(100)
	commitWithFile(t, repoPath, rootFileName, rootFileContent)
	firstCommitHash := testutils.ReadDefaultRefFile(t, repoPath)

	// Second commit: add nested file under a/b/c/
	dirA := testutils.RandomString(5)
	dirB := testutils.RandomString(5)
	dirC := testutils.RandomString(5)
	nestedDir := filepath.Join(dirA, dirB, dirC)
	if err := os.MkdirAll(filepath.Join(repoPath, nestedDir), constants.DirPerms); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}

	nestedFileName := filepath.Join(nestedDir, testutils.RandomString(10))
	nestedFileContent := testutils.RandomByteSlice(100)
	commitWithFile(t, repoPath, nestedFileName, nestedFileContent)
	secondCommitHash := testutils.ReadDefaultRefFile(t, repoPath)

	// Create branch pointing to first commit (flat structure)
	flatBranch := testutils.RandomString(10)
	writeRefFile(t, repoPath, flatBranch, firstCommitHash)

	// Checkout flat branch — nested directories should be removed
	cmd := newGogitCmd(t, constants.CheckoutCmdName, flatBranch)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("checkout to flat branch failed: %v\nOutput: %s", err, output)
	}

	assertCheckoutOutput(t, string(output), flatBranch)
	testutils.AssertFileContent(t, filepath.Join(repoPath, rootFileName), rootFileContent)
	testutils.AssertDirNotExists(t, filepath.Join(repoPath, dirA))
	indextest.AssertIndexEntryPaths(t, repoPath, 1, []string{rootFileName})

	// Create branch pointing to second commit (nested structure)
	nestedBranch := testutils.RandomString(10)
	writeRefFile(t, repoPath, nestedBranch, secondCommitHash)

	// Checkout nested branch — nested directories should be restored
	cmd = newGogitCmd(t, constants.CheckoutCmdName, nestedBranch)
	cmd.Dir = repoPath
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("checkout to nested branch failed: %v\nOutput: %s", err, output)
	}

	assertCheckoutOutput(t, string(output), nestedBranch)
	testutils.AssertFileContent(t, filepath.Join(repoPath, rootFileName), rootFileContent)
	testutils.AssertFileContent(t, filepath.Join(repoPath, nestedFileName), nestedFileContent)
	testutils.AssertDirExists(t, filepath.Join(repoPath, dirA, dirB, dirC))
	indextest.AssertIndexEntryPaths(t, repoPath, 2, []string{rootFileName, filepath.ToSlash(nestedFileName)})
}

// TestE2E_CheckoutCommand_DetachedHEADRoundTrip creates two commits on main
// (A then B), checks out commit A by hash (detached HEAD), then checks out
// main again. Verifies: detached checkout sets HEAD to raw hash with correct
// file state, returning to main restores symbolic ref and latest file state.
func TestE2E_CheckoutCommand_DetachedHEADRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	repoPath := setupTestRepo(t)
	initializeRepository(t, repoPath)

	// Commit A
	fileName := testutils.RandomString(10)
	fileContentA := testutils.RandomByteSlice(100)
	commitWithFile(t, repoPath, fileName, fileContentA)
	commitHashA := testutils.ReadDefaultRefFile(t, repoPath)

	// Commit B (update same file)
	fileContentB := testutils.RandomByteSlice(200)
	commitWithFile(t, repoPath, fileName, fileContentB)

	// Checkout commit A by hash (detached HEAD)
	cmd := newGogitCmd(t, constants.CheckoutCmdName, commitHashA)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("detached checkout failed: %v\nOutput: %s", err, output)
	}

	assertCheckoutOutput(t, string(output), commitHashA)
	assertHEADContent(t, repoPath, commitHashA+"\n")
	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), fileContentA)
	indextest.AssertIndexEntryPaths(t, repoPath, 1, []string{fileName})

	// Checkout back to main (symbolic ref)
	cmd = newGogitCmd(t, constants.CheckoutCmdName, constants.DefaultBranch)
	cmd.Dir = repoPath
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("checkout back to main failed: %v\nOutput: %s", err, output)
	}

	assertCheckoutOutput(t, string(output), constants.DefaultBranch)
	assertHEADContent(t, repoPath, constants.DefaultRefPrefix+constants.DefaultBranch+"\n")
	testutils.AssertFileContent(t, filepath.Join(repoPath, fileName), fileContentB)
	indextest.AssertIndexEntryPaths(t, repoPath, 1, []string{fileName})
}
