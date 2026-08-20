package worktree

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
)

// TestServiceInspectStagedChanges_IndexMatchesHead_NoChanges verifies that a
// staging index identical to HEAD produces no staged changes.
func TestServiceInspectStagedChanges_IndexMatchesHead_NoChanges(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	fileName := testutils.RandomString(10)
	fileHash := testutils.RandomHash()
	fileMode := objects.ModeRegularFile
	modifTime := time.Now()

	headSnapshot := objects.TreeSnapshot{
		fileName: {
			Hash: fileHash,
			Mode: fileMode,
		},
	}

	idx := index.NewIndex()
	addIndexEntry(t, idx, fileMode, fileHash, fileName, modifTime)
	saveIndex(t, repoPath, idx)

	workTreeService := NewService(repoPath)
	changes, err := workTreeService.InspectStagedChanges(headSnapshot)
	if err != nil {
		t.Fatalf("failed to inspect staged chagnes versus the head snapshot: %v", err)
	}

	if len(changes) != 0 {
		t.Fatalf("expected to get no changes when head and index match, got [%d] : [%#v]", len(changes), changes)
	}
}

// TestServiceInspectStagedChanges_ChangesFound_DeterministicOrder verifies
// addition, deletion, content, and mode changes are all reported in stable order.
func TestServiceInspectStagedChanges_ChangesFound_DeterministicOrder(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	modifTime := time.Now().UTC()

	unchangedHash := testutils.RandomHash()
	unchangedFile := testutils.RandomString(10)

	changedHeadHash := testutils.RandomHash()
	changedFile := testutils.RandomString(11)

	modeOnlyHash := testutils.RandomHash()
	modeOnlyFile := testutils.RandomString(12)

	bothModifsHash := testutils.RandomHash()
	bothModifsFile := testutils.RandomString(13)

	deletedHash := testutils.RandomHash()
	deletedFile := testutils.RandomString(14)

	// Head snapshot content
	headSnapshot := objects.TreeSnapshot{
		bothModifsFile: {Mode: objects.ModeRegularFile, Hash: bothModifsHash},
		changedFile:    {Mode: objects.ModeRegularFile, Hash: changedHeadHash},
		deletedFile:    {Mode: objects.ModeRegularFile, Hash: deletedHash},
		modeOnlyFile:   {Mode: objects.ModeRegularFile, Hash: modeOnlyHash},
		unchangedFile:  {Mode: objects.ModeRegularFile, Hash: unchangedHash},
	}

	// Index snapshot content representing the changes based on the file names
	bothModifsNewHash := testutils.RandomHash()
	for bothModifsNewHash == bothModifsHash {
		bothModifsNewHash = testutils.RandomHash()
	}

	changedHeadNewHash := testutils.RandomHash()
	for changedHeadNewHash == changedHeadHash {
		changedHeadNewHash = testutils.RandomHash()
	}

	addedHash := testutils.RandomHash()
	addedFile := testutils.RandomString(20)

	indexSnapshot := objects.TreeSnapshot{
		bothModifsFile: {Mode: objects.ModeExecutable, Hash: bothModifsNewHash},
		changedFile:    {Mode: objects.ModeRegularFile, Hash: changedHeadNewHash},
		addedFile:      {Mode: objects.ModeRegularFile, Hash: addedHash},
		modeOnlyFile:   {Mode: objects.ModeExecutable, Hash: modeOnlyHash},
		unchangedFile:  {Mode: objects.ModeRegularFile, Hash: unchangedHash},
	}

	// create index -> add entries -> save index
	idx := index.NewIndex()
	for path, entry := range indexSnapshot {
		addIndexEntry(t, idx, entry.Mode, entry.Hash, path, modifTime)
	}
	saveIndex(t, repoPath, idx)

	// Inspect stage changes
	changes, err := NewService(repoPath).InspectStagedChanges(headSnapshot)
	if err != nil {
		t.Fatalf("failed to inspect staged changes against HEAD snapshot: %v", err)
	}

	expectedChengesNo := 6
	if len(changes) != expectedChengesNo {
		t.Fatalf("expected number of changes to be [%d], got [%d]", expectedChengesNo, len(changes))
	}

	// Assert deterministic result due to sorting
	expectedChanges := []Change{
		{Path: addedFile, Kind: ChangeAdded},
		{Path: bothModifsFile, Kind: ChangeContentModified},
		{Path: bothModifsFile, Kind: ChangeModeModified},
		{Path: changedFile, Kind: ChangeContentModified},
		{Path: deletedFile, Kind: ChangeDeleted},
		{Path: modeOnlyFile, Kind: ChangeModeModified},
	}
	sortChanges(expectedChanges)

	if !slices.Equal(changes, expectedChanges) {
		t.Fatalf("expected chagnes [%#v], got [%#v]", expectedChanges, changes)
	}
}

// TestServiceInspectStagedChanges_InvalidHeadSnapshot verifies that inspection
// rejects a HEAD snapshot that cannot represent a valid Git tree.
func TestServiceInspectStagedChanges_InvalidHeadSnapshot(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)

	headSnapshot := objects.TreeSnapshot{
		testutils.RandomString(10): {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeDirectory, // directory mode is invalid for a snapshot entry
		},
	}

	service := NewService(repoPath)
	_, err := service.InspectStagedChanges(headSnapshot)
	if err == nil {
		t.Fatal("expected an error to occur when head snapshot is invalid")
	}

	expectedErrorMessage := "invalid HEAD snapshot:"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expectrer error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
}

// TestServiceInspectStagedChanges_InvalidIndexSnapshot verifies that inspection
// rejects a persisted index containing a non-canonical logical path.
func TestServiceInspectStagedChanges_InvalidIndexSnapshot(t *testing.T) {
	repoPath := testutils.SetupTestRepoWithInit(t)
	headSnapshot := objects.TreeSnapshot{
		testutils.RandomString(10): {
			Hash: testutils.RandomHash(),
			Mode: objects.ModeRegularFile,
		},
	}

	idx := index.NewIndex()
	validPath := "invalid/path.txt"
	invalidPath := `invalid\path.txt`
	addIndexEntry(t, idx, objects.ModeRegularFile, testutils.RandomHash(), validPath, time.Now())
	saveIndex(t, repoPath, idx)

	indexPath := filepath.Join(repoPath, constants.Gogit, constants.Index)
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read persisted index: %v", err)
	}
	corruptIndexData := bytes.Replace(indexData, []byte(validPath), []byte(invalidPath), 1)
	if bytes.Equal(corruptIndexData, indexData) {
		t.Fatal("expected persisted index path to be replaced")
	}
	if err := os.WriteFile(indexPath, corruptIndexData, constants.FilePerms); err != nil {
		t.Fatalf("failed to write corrupt index: %v", err)
	}

	service := NewService(repoPath)
	_, err = service.InspectStagedChanges(headSnapshot)
	if err == nil {
		t.Fatal("expected an error to occur when index snapshot is invalid")
	}

	expectedErrorMessage := "failed to load index:"
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expectrer error message to contain [%s], got [%s]", expectedErrorMessage, err.Error())
	}
	if !strings.Contains(err.Error(), "path cannot contain backslashes") {
		t.Fatalf("expected invalid path error, got [%s]", err.Error())
	}
}
