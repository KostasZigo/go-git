package branches

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestBranchRefPath verifies that logical branch names are converted to the
// expected operating-system path under refs/heads.
func TestBranchRefPath(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	testCases := []struct {
		name       string
		branchName string
	}{
		{name: "root branch", branchName: testutils.RandomString(8)},
		{
			name: "hierarchical branch",
			branchName: path.Join(
				testutils.RandomString(8),
				testutils.RandomString(8),
				testutils.RandomString(8),
			),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actualPath, err := branchRefPath(repoPath, testCase.branchName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expectedPath := expectedBranchRefPath(repoPath, testCase.branchName)
			if actualPath != expectedPath {
				t.Fatalf("expected ref path [%s], got [%s]", expectedPath, actualPath)
			}
		})
	}
}

// TestBranchRefPath_InvalidName verifies that path construction rejects an
// invalid logical branch name before accessing the filesystem.
func TestBranchRefPath_InvalidName(t *testing.T) {
	_, err := branchRefPath(t.TempDir(), `feature\invalid`)
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestReadBranchRef verifies that an existing hierarchical branch ref is read
// through its logical name and its stored hash is trimmed.
func TestReadBranchRef(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	branchName := path.Join(testutils.RandomString(8), testutils.RandomString(8))
	expectedHash := testutils.RandomHash()
	writeBranchRefFixture(t, repoPath, branchName, []byte("\n"+expectedHash+" \n"))

	actualHash, exists, err := readBranchRef(repoPath, branchName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatalf("expected branch [%s] ref to exist", branchName)
	}
	if actualHash != expectedHash {
		t.Fatalf("expected branch [%s] hash [%s], got [%s]", branchName, expectedHash, actualHash)
	}
}

// TestReadBranchRef_Missing verifies that a missing branch ref is reported as
// absent without being treated as malformed.
func TestReadBranchRef_Missing(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	hash, exists, err := readBranchRef(repoPath, testutils.RandomString(8))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Fatal("expected branch ref to be missing")
	}
	if hash != "" {
		t.Fatalf("expected empty hash for missing branch ref, got [%s]", hash)
	}
}

// TestReadBranchRef_InvalidContent verifies that an existing ref with empty or
// malformed content is distinguished from a missing ref.
func TestReadBranchRef_InvalidContent(t *testing.T) {
	testCases := []struct {
		name    string
		content []byte
	}{
		{name: "empty hash", content: []byte("\n")},
		{name: "malformed hash", content: testutils.RandomBytes(8)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repoPath := testutils.SetupTestRepoWithInit(t)
			branchName := testutils.RandomString(8)
			writeBranchRefFixture(t, repoPath, branchName, testCase.content)

			hash, exists, err := readBranchRef(repoPath, branchName)
			if err == nil {
				t.Fatal("expected error")
			}
			if !exists {
				t.Fatal("expected invalid branch ref to be reported as existing")
			}
			if hash != "" {
				t.Fatalf("expected empty hash for invalid branch ref, got [%s]", hash)
			}
		})
	}
}

// TestResolveCurrent verifies that symbolic HEAD resolves root and hierarchical
// logical branch names together with their current commit hashes.
func TestResolveCurrent(t *testing.T) {
	testCases := []struct {
		name       string
		branchName string
	}{
		{name: "root branch", branchName: testutils.RandomString(8)},
		{
			name: "hierarchical branch",
			branchName: path.Join(
				testutils.RandomString(8),
				testutils.RandomString(8),
				testutils.RandomString(8),
			),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repoPath := testutils.SetupTestRepoWithInit(t)
			expectedHash := testutils.RandomHash()
			headContent := constants.DefaultRefPrefix + testCase.branchName + "\n"
			testutils.WriteHEADFile(t, repoPath, []byte(headContent))
			writeBranchRefFixture(t, repoPath, testCase.branchName, []byte(expectedHash+"\n"))

			current, err := ResolveCurrent(repoPath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertReference(t, current, repoPath, testCase.branchName, expectedHash)

			testutils.AssertHEADContent(t, repoPath, headContent)
		})
	}
}

// TestResolveCurrent_UnbornBranch verifies that symbolic HEAD resolves to a
// valid current branch with an empty hash before its first commit.
func TestResolveCurrent_UnbornBranch(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	current, err := ResolveCurrent(repoPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertReference(t, current, repoPath, constants.DefaultBranch, "")
}

// TestResolveCurrent_DetachedHEAD verifies that a raw commit hash in HEAD is
// rejected when an operation requires a current branch.
func TestResolveCurrent_DetachedHEAD(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	detachedHash := testutils.RandomHash()
	testutils.WriteHEADFile(t, repoPath, []byte(detachedHash+"\n"))

	_, err := ResolveCurrent(repoPath)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrDetachedHEAD) {
		t.Fatalf("expected detached HEAD error, got [%v]", err)
	}
}

// TestResolveCurrent_MissingHEAD verifies that a missing HEAD file returns a
// descriptive filesystem error rather than a detached-HEAD result.
func TestResolveCurrent_MissingHEAD(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithGogitDir(t)

	_, err := ResolveCurrent(repoPath)
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrDetachedHEAD) {
		t.Fatalf("expected missing HEAD error, got [%v]", err)
	}
}

// TestResolveCurrent_InvalidBranchName verifies that malformed branch names in
// symbolic HEAD are rejected by the existing branch-name validation policy.
func TestResolveCurrent_InvalidBranchName(t *testing.T) {
	testCases := []struct {
		name       string
		branchName string
	}{
		{name: "empty branch name", branchName: ""},
		{name: "malformed branch name", branchName: `feature\invalid`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repoPath := testutils.SetupTestRepoWithInit(t)
			testutils.WriteHEADFile(
				t,
				repoPath,
				[]byte(constants.DefaultRefPrefix+testCase.branchName+"\n"),
			)

			_, err := ResolveCurrent(repoPath)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

// TestResolveCurrent_InvalidRef verifies that an existing corrupt current ref
// is not treated as an unborn branch.
func TestResolveCurrent_InvalidRef(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	branchName := testutils.RandomString(8)
	testutils.WriteHEADFile(t, repoPath, []byte(constants.DefaultRefPrefix+branchName+"\n"))
	writeBranchRefFixture(t, repoPath, branchName, testutils.RandomBytes(8))

	current, err := ResolveCurrent(repoPath)
	if err == nil {
		t.Fatal("expected error")
	}
	if current != (Reference{}) {
		t.Fatalf("expected empty reference on corrupt ref, got [%v]", current)
	}
}

// TestResolve verifies that named root and hierarchical branches resolve
// without requiring a HEAD file.
func TestResolve(t *testing.T) {
	testCases := []struct {
		name       string
		branchName string
	}{
		{name: "root branch", branchName: testutils.RandomString(8)},
		{
			name: "hierarchical branch",
			branchName: path.Join(
				testutils.RandomString(8),
				testutils.RandomString(8),
				testutils.RandomString(8),
			),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repoPath := testutils.SetupTestRepoWithGogitDir(t)
			expectedHash := testutils.RandomHash()
			writeBranchRefFixture(t, repoPath, testCase.branchName, []byte(expectedHash+"\n"))

			branch, err := Resolve(repoPath, testCase.branchName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertReference(t, branch, repoPath, testCase.branchName, expectedHash)

			testutils.AssertFileNotExists(t, filepath.Join(repoPath, constants.Gogit, constants.Head))
		})
	}
}

// TestResolve_DoesNotModifyHEAD verifies that named branch resolution leaves
// existing HEAD content unchanged.
func TestResolve_DoesNotModifyHEAD(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	branchName := testutils.RandomString(8)
	writeBranchRefFixture(t, repoPath, branchName, []byte(testutils.RandomHash()+"\n"))
	originalHEADContent := testutils.RandomHash() + "\n"
	testutils.WriteHEADFile(t, repoPath, []byte(originalHEADContent))

	if _, err := Resolve(repoPath, branchName); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testutils.AssertHEADContent(t, repoPath, originalHEADContent)
}

// TestResolve_MissingBranch verifies that a missing named branch returns a
// distinguishable branch-not-found error.
func TestResolve_MissingBranch(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	branchName := testutils.RandomString(8)

	_, err := Resolve(repoPath, branchName)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrBranchNotFound) {
		t.Fatalf("expected branch not found error, got [%v]", err)
	}
}

// TestResolve_InvalidBranchName verifies that invalid input is rejected by
// branch-name validation rather than reported as a missing branch.
func TestResolve_InvalidBranchName(t *testing.T) {
	_, err := Resolve(t.TempDir(), `feature\invalid`)
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrBranchNotFound) {
		t.Fatalf("expected invalid branch name error, got [%v]", err)
	}
}

// TestCompareAndSwap_UpdatesExistingRef verifies that a matching expected hash
// is replaced without modifying HEAD or leaving a lock file behind.
func TestCompareAndSwap_UpdatesExistingRef(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	branchName := testutils.RandomString(8)
	hashes := randomDistinctHashes(t, 2)
	expectedHash := hashes[0]
	newHash := hashes[1]
	writeBranchRefFixture(t, repoPath, branchName, []byte(expectedHash+"\n"))
	originalHEADContent := testutils.ReadHEADFile(t, repoPath)

	if err := CompareAndSwap(repoPath, branchName, expectedHash, newHash); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertBranchRefHash(t, repoPath, branchName, newHash)
	assertBranchRefLockNotExists(t, repoPath, branchName)
	testutils.AssertHEADContent(t, repoPath, originalHEADContent)
}

// TestCompareAndSwap_CreatesMissingRef verifies that an empty expected hash
// creates root and hierarchical branch refs that are still absent.
func TestCompareAndSwap_CreatesRef(t *testing.T) {
	testCases := []struct {
		name       string
		branchName string
	}{
		{name: "root branch", branchName: testutils.RandomString(8)},
		{
			name: "hierarchical branch",
			branchName: path.Join(
				testutils.RandomString(8),
				testutils.RandomString(8),
				testutils.RandomString(8),
			),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repoPath := testutils.SetupTestRepoWithInit(t)
			newHash := testutils.RandomHash()
			originalHEADContent := testutils.ReadHEADFile(t, repoPath)

			if err := CompareAndSwap(repoPath, testCase.branchName, "", newHash); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertBranchRefHash(t, repoPath, testCase.branchName, newHash)
			assertBranchRefLockNotExists(t, repoPath, testCase.branchName)
			testutils.AssertHEADContent(t, repoPath, originalHEADContent)

			expectedRefPath := expectedBranchRefPath(repoPath, testCase.branchName)
			testutils.AssertFileExists(t, expectedRefPath)
		})
	}
}

// TestCompareAndSwap_StaleRef verifies that a stale expected hash returns a
// distinguishable error without overwriting the current ref or HEAD.
func TestCompareAndSwap_StaleRef(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	branchName := testutils.RandomString(8)
	hashes := randomDistinctHashes(t, 3)
	actualHash := hashes[0]
	expectedHash := hashes[1]
	newHash := hashes[2]
	writeBranchRefFixture(t, repoPath, branchName, []byte(actualHash+"\n"))
	originalHEADContent := testutils.ReadHEADFile(t, repoPath)

	err := CompareAndSwap(repoPath, branchName, expectedHash, newHash)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrReferenceChanged) {
		t.Fatalf("expected reference changed error, got [%v]", err)
	}

	assertBranchRefHash(t, repoPath, branchName, actualHash)
	assertBranchRefLockNotExists(t, repoPath, branchName)
	testutils.AssertHEADContent(t, repoPath, originalHEADContent)
}

// TestCompareAndSwap_ExpectedRefMissing verifies that an update expecting an
// existing ref fails when the ref was removed before the update.
func TestCompareAndSwap_ExpectedRefMissing(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	branchName := testutils.RandomString(8)
	hashes := randomDistinctHashes(t, 2)
	expectedHash := hashes[0]
	newHash := hashes[1]
	originalHEADContent := testutils.ReadHEADFile(t, repoPath)

	err := CompareAndSwap(repoPath, branchName, expectedHash, newHash)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrReferenceChanged) {
		t.Fatalf("expected reference changed error, got [%v]", err)
	}

	refPath := expectedBranchRefPath(repoPath, branchName)
	testutils.AssertFileNotExists(t, refPath)
	assertBranchRefLockNotExists(t, repoPath, branchName)
	testutils.AssertHEADContent(t, repoPath, originalHEADContent)
}

// TestCompareAndSwap_ExpectedMissingRefExists verifies that ref creation fails
// when another operation created the ref after the caller observed it missing.
func TestCompareAndSwap_ExpectedMissingRefExists(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	branchName := testutils.RandomString(8)
	hashes := randomDistinctHashes(t, 2)
	actualHash := hashes[0]
	newHash := hashes[1]
	writeBranchRefFixture(t, repoPath, branchName, []byte(actualHash+"\n"))
	originalHEADContent := testutils.ReadHEADFile(t, repoPath)

	err := CompareAndSwap(repoPath, branchName, "", newHash)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrReferenceChanged) {
		t.Fatalf("expected reference changed error, got [%v]", err)
	}

	assertBranchRefHash(t, repoPath, branchName, actualHash)
	assertBranchRefLockNotExists(t, repoPath, branchName)
	testutils.AssertHEADContent(t, repoPath, originalHEADContent)
}

// TestCompareAndSwap_ConcurrentWriters verifies that two writers using the
// same expected hash cannot both advance a branch.
func TestCompareAndSwap_ConcurrentWriters(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	branchName := testutils.RandomString(8)
	hashes := randomDistinctHashes(t, 3)
	expectedHash := hashes[0]
	newHashes := hashes[1:]
	writeBranchRefFixture(t, repoPath, branchName, []byte(expectedHash+"\n"))

	start := make(chan struct{}) // notification channel
	results := make(chan error, len(newHashes))
	for _, newHash := range newHashes {
		go func() {
			<-start
			results <- CompareAndSwap(repoPath, branchName, expectedHash, newHash)
		}()
	}
	close(start)

	successCount := 0
	for range newHashes {
		err := <-results
		if err == nil {
			successCount++
			continue
		}
		if !errors.Is(err, ErrReferenceLocked) && !errors.Is(err, ErrReferenceChanged) {
			t.Fatalf("expected reference locked or changed error, got [%v]", err)
		}
	}
	if successCount != 1 {
		t.Fatalf("expected exactly one successful update, got [%d]", successCount)
	}

	actualHash, exists, err := readBranchRef(repoPath, branchName)
	if err != nil {
		t.Fatalf("failed to read branch ref: %v", err)
	}
	if !exists {
		t.Fatalf("expected branch [%s] ref to exist", branchName)
	}
	if actualHash != newHashes[0] && actualHash != newHashes[1] {
		t.Fatalf("expected one competing hash, got [%s]", actualHash)
	}
	assertBranchRefLockNotExists(t, repoPath, branchName)
}

// TestCompareAndSwap_LockedRef verifies that an existing lock prevents an
// update without changing either the ref or the lock owner’s file.
func TestCompareAndSwap_LockedRef(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	branchName := testutils.RandomString(8)
	hashes := randomDistinctHashes(t, 2)
	currentHash := hashes[0]
	newHash := hashes[1]
	writeBranchRefFixture(t, repoPath, branchName, []byte(currentHash+"\n"))
	refPath := expectedBranchRefPath(repoPath, branchName)
	lockPath := refPath + ".lock"
	lockContent := testutils.RandomByteSlice(20)
	if err := os.WriteFile(lockPath, lockContent, constants.FilePerms); err != nil {
		t.Fatalf("failed to create branch ref lock: %v", err)
	}

	err := CompareAndSwap(repoPath, branchName, currentHash, newHash)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrReferenceLocked) {
		t.Fatalf("expected reference locked error, got [%v]", err)
	}

	assertBranchRefHash(t, repoPath, branchName, currentHash)
	testutils.AssertFileContent(t, lockPath, lockContent)
}

// TestCompareAndSwap_InvalidHashes verifies that invalid expected or new hashes
// are rejected before a ref or lock file is created.
func TestCompareAndSwap_InvalidHashes(t *testing.T) {
	testCases := []struct {
		name         string
		expectedHash string
		newHash      string
	}{
		{name: "invalid expected hash", expectedHash: "invalid-hash", newHash: testutils.RandomHash()},
		{name: "empty new hash", expectedHash: "", newHash: ""},
		{name: "invalid new hash", expectedHash: "", newHash: "invalid-hash"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repoPath := testutils.SetupTestRepoWithInit(t)
			branchName := testutils.RandomString(8)

			err := CompareAndSwap(
				repoPath,
				branchName,
				testCase.expectedHash,
				testCase.newHash,
			)
			if err == nil {
				t.Fatal("expected error")
			}

			refPath := expectedBranchRefPath(repoPath, branchName)
			testutils.AssertFileNotExists(t, refPath)
			testutils.AssertFileNotExists(t, refPath+".lock")
		})
	}
}

// TestCompareAndSwap_InvalidCurrentRef verifies that failure to read a corrupt
// current ref preserves its content and removes the acquired lock.
func TestCompareAndSwap_InvalidCurrentRef(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	branchName := testutils.RandomString(8)
	invalidContent := testutils.RandomBytes(8)
	writeBranchRefFixture(t, repoPath, branchName, invalidContent)
	hashes := randomDistinctHashes(t, 2)

	err := CompareAndSwap(repoPath, branchName, hashes[0], hashes[1])
	if err == nil {
		t.Fatal("expected error")
	}

	refPath := expectedBranchRefPath(repoPath, branchName)
	testutils.AssertFileContent(t, refPath, invalidContent)
	assertBranchRefLockNotExists(t, repoPath, branchName)
}

// TestCleanupRefLock_CloseFailure verifies that a close failure is returned
// while lock removal is still attempted.
func TestCleanupRefLock_CloseFailure(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), testutils.RandomString(8)+".lock")
	lockFile, err := os.Create(lockPath)
	if err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}
	if err := lockFile.Close(); err != nil {
		t.Fatalf("failed to close lock file fixture: %v", err)
	}

	err = cleanupRefLock(lockFile, lockPath, testutils.RandomString(8), false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("expected closed file error, got [%v]", err)
	}
	testutils.AssertFileNotExists(t, lockPath)
}

// TestCleanupRefLock_RemoveFailure verifies that lock removal failures are
// returned instead of being silently discarded.
func TestCleanupRefLock_RemoveFailure(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), testutils.RandomString(8)+".lock")
	if err := os.Mkdir(lockPath, constants.DirPerms); err != nil {
		t.Fatalf("failed to create lock directory fixture: %v", err)
	}
	testutils.CreateTestFile(
		t,
		lockPath,
		testutils.RandomString(8),
		testutils.RandomByteSlice(20),
	)
	lockFile, err := os.Open(lockPath)
	if err != nil {
		t.Fatalf("failed to open lock directory fixture: %v", err)
	}

	err = cleanupRefLock(lockFile, lockPath, testutils.RandomString(8), false)
	if err == nil {
		t.Fatal("expected error")
	}
	testutils.AssertDirExists(t, lockPath)
}

// writeBranchRefFixture writes raw ref content using the expected filesystem
// representation of a logical branch name.
func writeBranchRefFixture(t *testing.T, repoPath, branchName string, content []byte) {
	t.Helper()

	refPath := expectedBranchRefPath(repoPath, branchName)
	if err := os.MkdirAll(filepath.Dir(refPath), constants.DirPerms); err != nil {
		t.Fatalf("failed to create branch ref directory: %v", err)
	}
	if err := os.WriteFile(refPath, content, constants.FilePerms); err != nil {
		t.Fatalf("failed to write branch ref: %v", err)
	}
}

// assertBranchRefHash verifies the validated hash stored for a logical branch.
func assertBranchRefHash(t *testing.T, repoPath, branchName, expectedHash string) {
	t.Helper()

	actualHash, exists, err := readBranchRef(repoPath, branchName)
	if err != nil {
		t.Fatalf("failed to read branch [%s] ref: %v", branchName, err)
	}
	if !exists {
		t.Fatalf("expected branch [%s] ref to exist", branchName)
	}
	if actualHash != expectedHash {
		t.Fatalf("expected branch [%s] hash [%s], got [%s]", branchName, expectedHash, actualHash)
	}
}

// assertBranchRefLockNotExists verifies that no lock remains for a branch ref.
func assertBranchRefLockNotExists(t *testing.T, repoPath, branchName string) {
	t.Helper()

	refPath := expectedBranchRefPath(repoPath, branchName)
	testutils.AssertFileNotExists(t, refPath+".lock")
}

// assertReference verifies a resolved branch's logical name, hash, and ref path.
func assertReference(t *testing.T, actual Reference, repoPath, branchName, expectedHash string) {
	t.Helper()

	if actual.Name != branchName {
		t.Fatalf("expected branch name [%s], got [%s]", branchName, actual.Name)
	}
	if actual.Hash != expectedHash {
		t.Fatalf("expected branch hash [%s], got [%s]", expectedHash, actual.Hash)
	}
	expectedRefPath := expectedBranchRefPath(repoPath, branchName)
	if actual.FilePath != expectedRefPath {
		t.Fatalf("expected branch ref path [%s], got [%s]", expectedRefPath, actual.FilePath)
	}
}

// expectedBranchRefPath returns the expected filesystem path for a logical branch name.
func expectedBranchRefPath(repoPath, branchName string) string {
	return filepath.Join(
		repoPath,
		constants.Gogit,
		constants.Refs,
		constants.Heads,
		filepath.FromSlash(branchName),
	)
}

// randomDistinctHashes returns the requested number of unique SHA-1 hashes.
func randomDistinctHashes(t *testing.T, count int) []string {
	t.Helper()

	hashes := make([]string, 0, count)
	seen := make(map[string]struct{}, count)
	for len(hashes) < count {
		hash := testutils.RandomHash()
		if _, exists := seen[hash]; exists {
			continue
		}
		seen[hash] = struct{}{}
		hashes = append(hashes, hash)
	}

	return hashes
}
