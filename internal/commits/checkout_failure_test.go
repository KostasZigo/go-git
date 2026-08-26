package commits

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/testutils"
	"github.com/KostasZigo/gogit/internal/worktree"
)

// TestCheckout_Orchestrate_ApplicationFailureRestoresStateBeforeHeadUpdate
// verifies that an index-save failure after worktree mutation restores the
// original files and index while leaving HEAD and every branch ref unchanged.
func TestCheckout_Orchestrate_ApplicationFailureRestoresStateBeforeHeadUpdate(t *testing.T) {
	originalPath := testutils.RandomString(10)
	originalContent := testutils.RandomBytes(20)
	targetPath := filepath.ToSlash(filepath.Join(testutils.RandomString(11), testutils.RandomString(12)))
	targetContent := testutils.RandomBytes(21)
	fixture := newCheckoutFixture(t,
		map[string]checkoutFile{
			originalPath: {
				content: originalContent,
				mode:    objects.ModeRegularFile,
			},
		},
		map[string]checkoutFile{
			targetPath: {
				content: targetContent,
				mode:    objects.ModeRegularFile,
			},
		})
	metadataBefore := captureCheckoutMetadata(t, fixture.repoPath, constants.DefaultBranch, fixture.targetBranch)
	saveErr := errors.New("injected checkout index save failure")
	injectIndexSaveFailure(t, saveErr)

	err := OrchestrateCheckoutExecution(fixture.repoPath, fixture.targetBranch, false)

	if !errors.Is(err, saveErr) {
		t.Fatalf("expected injected index-save error, got [%v]", err)
	}
	if errors.Is(err, worktree.ErrRollback) {
		t.Fatalf("expected checkout rollback to succeed, got [%v]", err)
	}
	assertCheckoutMetadataUnchanged(t, fixture.repoPath, metadataBefore)
	testutils.AssertFileContent(t, filepath.Join(fixture.repoPath, originalPath), originalContent)
	testutils.AssertFileNotExists(t, filepath.Join(fixture.repoPath, filepath.FromSlash(targetPath)))
}

// TestCheckout_Orchestrate_ApplicationAndRollbackFailuresPreserveBothErrors
// verifies that checkout returns both the index-save and rollback errors when
// an original blob cannot be restored, without updating HEAD or branch refs.
func TestCheckout_Orchestrate_ApplicationAndRollbackFailuresPreserveBothErrors(t *testing.T) {
	originalPath := testutils.RandomString(10)
	targetPath := testutils.RandomString(11)
	fixture := newCheckoutFixture(t, map[string]checkoutFile{originalPath: {content: testutils.RandomBytes(20), mode: objects.ModeRegularFile}}, map[string]checkoutFile{targetPath: {content: testutils.RandomBytes(21), mode: objects.ModeRegularFile}})
	metadataBefore := captureCheckoutMetadata(t, fixture.repoPath, constants.DefaultBranch, fixture.targetBranch)
	removeStoredObject(t, fixture.repoPath, fixture.headSnapshot[originalPath].Hash)
	saveErr := errors.New("injected checkout index save failure")
	injectIndexSaveFailure(t, saveErr)

	err := OrchestrateCheckoutExecution(fixture.repoPath, fixture.targetBranch, false)

	if !errors.Is(err, saveErr) {
		t.Fatalf("expected injected index-save error, got [%v]", err)
	}
	if !errors.Is(err, worktree.ErrRollback) {
		t.Fatalf("expected joined rollback error, got [%v]", err)
	}
	if !strings.Contains(err.Error(), originalPath) {
		t.Fatalf("expected rollback error to identify path [%s], got [%v]", originalPath, err)
	}
	assertCheckoutMetadataUnchanged(t, fixture.repoPath, metadataBefore)
	testutils.AssertFileNotExists(t, filepath.Join(fixture.repoPath, originalPath))
	testutils.AssertFileNotExists(t, filepath.Join(fixture.repoPath, targetPath))
}
