package commits

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/utils"
	"github.com/KostasZigo/gogit/utils/indexutils"
)

// ResolvedTarget holds the result of resolving a checkout target string.
// IsBranch indicates whether the target was resolved via a branch ref file or direct commit.
// Hash contains the commit hash the target points to.
type ResolvedTarget struct {
	IsBranch bool
	Hash     string
}

// ResolveTarget resolves a checkout target string to a commit hash.
// Resolution order follows convention: branch refs are checked first,
// then the object store for direct commit hashes.
// Returns an error if the target is empty, not found, or unreadable.
func ResolveTarget(repoPath, target string) (resolvedTarget *ResolvedTarget, err error) {
	if target == "" {
		return nil, fmt.Errorf("checkout target cannot be empty")
	}

	resolvedTarget, err = searchForTargetInRefs(repoPath, target)
	if err != nil {
		return nil, fmt.Errorf("failed search for checkout target [%s] in refs: %w", target, err)
	}
	if resolvedTarget != nil {
		return resolvedTarget, nil
	}

	resolvedTarget, err = searchForTargetInCommitObjects(repoPath, target)
	if err != nil {
		return nil, fmt.Errorf("failed search for checkout target [%s] in commit objects: %w", target, err)
	}
	if resolvedTarget == nil {
		return nil, fmt.Errorf("checkout target [%s] not found as branch or commit", target)
	}
	return resolvedTarget, nil
}

// searchForTargetInRefs checks if target matches a branch name under refs/heads/.
// Returns nil, nil if no matching branch exists (not an error).
// Returns an error only for filesystem failures.
func searchForTargetInRefs(repoPath, target string) (*ResolvedTarget, error) {
	branchPath := filepath.Join(repoPath, constants.Gogit, constants.Refs, constants.Heads, target)
	content, err := os.ReadFile(branchPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read branch file: %w", err)
	}

	hash := strings.TrimSpace(string(content))
	return &ResolvedTarget{
		IsBranch: true,
		Hash:     hash,
	}, nil
}

// searchForTargetInCommitObjects checks if target is a valid SHA-1 hash
// pointing to an existing commit object in the store.
// Returns nil, nil if the target is not a valid hash or the object does not exist.
// Returns an error only if the object exists but cannot be read as a commit.
func searchForTargetInCommitObjects(repoPath, target string) (*ResolvedTarget, error) {
	if !utils.IsValidSHA1Hash(target) {
		return nil, nil
	}

	store := objects.NewObjectStore(repoPath)
	if !store.Exists(target) {
		return nil, nil
	}

	commit, err := store.ReadCommit(target)
	if err != nil {
		return nil, fmt.Errorf("failed to read commit for checkout target [%s]: %w", target, err)
	}
	return &ResolvedTarget{
		IsBranch: false,
		Hash:     commit.Hash(),
	}, nil
}

// CleanWorkingTree removes all files referenced by the provided index entries
// from the working directory. Operates in two passes: first deletes all tracked
// files, then collects unique parent directories and prunes empty ones deepest-first
// up to (but not including) repoPath. Files already missing on disk are silently skipped.
func CleanWorkingTree(repoPath string, indexEntries []*index.IndexEntry) error {
	uniqueDirs := map[string]struct{}{}
	for _, entry := range indexEntries {
		relPath, err := filepath.Localize(entry.Path())
		if err != nil {
			return fmt.Errorf("failed to convert file path to local os specific format for file [%s]: %w", entry.Path(), err)
		}
		absPath := filepath.Join(repoPath, relPath)
		if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove file [%s]: %w", relPath, err)
		}

		dir := filepath.Dir(absPath)
		if dir == repoPath {
			continue
		}
		uniqueDirs[dir] = struct{}{}
	}

	dirs := slices.Collect(maps.Keys(uniqueDirs))
	slices.SortFunc(dirs, func(a, b string) int {
		aCount := strings.Count(a, string(os.PathSeparator))
		bCount := strings.Count(b, string(os.PathSeparator))
		if aCount > bCount {
			return -1
		}
		if aCount == bCount {
			return 0
		}
		return 1
	})

	for _, dir := range dirs {
		if err := pruneEmptyDirectories(repoPath, dir); err != nil {
			return err
		}
	}

	return nil
}

// pruneEmptyDirectories walks upward from dirPath toward repoPath, removing each
// directory that is empty after file deletion. Stops at repoPath or the first
// non-empty directory.
func pruneEmptyDirectories(repoPath, dirPath string) error {
	for {
		if repoPath == dirPath {
			return nil
		}

		parentDir := filepath.Dir(dirPath)
		isEmpty, err := isDirEmpty(dirPath)
		if err != nil {
			return fmt.Errorf("failed to check if directory [%s] is empty: %w", dirPath, err)
		}

		if isEmpty {
			if err := os.Remove(dirPath); err != nil {
				return fmt.Errorf("failed to remove empty directory [%s]: %w", dirPath, err)
			}
			dirPath = parentDir
		} else {
			return nil
		}
	}

}

// isDirEmpty reports whether the directory at path contains no entries.
// Uses a single Readdirnames call to avoid reading the entire directory listing.
func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// RestoreTreeAndRebuildIndex reads a stored tree object, reconstructs its contents on the
// filesystem under repoPath, and rebuilds the index from the restored files.
// The index is saved to disk after the tree walk completes.
func RestoreTreeAndRebuildIndex(repoPath, treeHash string) error {
	store := objects.NewObjectStore(repoPath)
	idx := index.NewIndex()

	if err := restoreTreeRecursive(repoPath, "", treeHash, store, idx); err != nil {
		return err
	}

	idxManager := index.NewManager(repoPath)
	if err := idxManager.Save(idx); err != nil {
		return fmt.Errorf("failed to save rebuilt index: %w", err)
	}

	return nil
}

// restoreTreeRecursive walks a tree object and writes its entries to dirPath.
// relDir accumulates the forward-slash relative path prefix from the repository root,
// used to construct index entry paths. Subtree entries are created as directories
// and descended into recursively. Blob entries are written as files and added to the index.
func restoreTreeRecursive(dirPath, relDir, treeHash string, store *objects.ObjectStore, idx *index.Index) error {
	tree, err := store.ReadTree(treeHash)
	if err != nil {
		return fmt.Errorf("failed to read tree [%s]: %w", treeHash, err)
	}

	for _, treeEntry := range tree.Entries() {
		entryPath := filepath.Join(dirPath, treeEntry.Name())
		entryRelPath := path.Join(relDir, treeEntry.Name())

		if !treeEntry.IsDirectory() {
			if err := createFileFromBlob(store, treeEntry.Hash(), entryPath); err != nil {
				return err
			}
			if err := addFileToRebuiltIndex(entryPath, entryRelPath, treeEntry.Hash(), idx); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(entryPath, constants.DirPerms); err != nil {
			return fmt.Errorf("failed to create directory [%s]: %w", entryPath, err)
		}
		if err := restoreTreeRecursive(entryPath, entryRelPath, treeEntry.Hash(), store, idx); err != nil {
			return err
		}
	}

	return nil
}

// createFileFromBlob reads a blob from the object store and writes its content
// to the specified file path.
func createFileFromBlob(store *objects.ObjectStore, blobHash, filePath string) error {
	blob, err := store.ReadBlob(blobHash)
	if err != nil {
		return fmt.Errorf("failed to create file [%s] from blob [%s]: %w", filePath, blobHash, err)
	}

	if err := os.WriteFile(filePath, blob.Content(), constants.FilePerms); err != nil {
		return fmt.Errorf("failed to write [%s] file: %w", filePath, err)
	}

	return nil
}

// addFileToRebuiltIndex stats the file at absPath to obtain size, mode, and
// modification time, then creates an index entry using the provided relative
// path and blob hash, and adds it to the index.
func addFileToRebuiltIndex(absPath, relPath, hash string, idx *index.Index) error {
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", absPath, err)
	}

	fileMode := indexutils.DetectIndexFileMode(fileInfo)

	entry, err := index.NewEntry(
		fileMode,
		hash,
		relPath,
		fileInfo.Size(),
		fileInfo.ModTime().Truncate(time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to create index entry for %s: %w", relPath, err)
	}

	if err := idx.AddEntry(entry); err != nil {
		return fmt.Errorf("failed to add [%s] entry to index: %w", relPath, err)
	}

	return nil
}

// checkIfDirty compares each index entry against its on-disk counterpart.
// Uses a file-size + modified-time fast path to skip unchanged files, falling back to a
// full content hash comparison when metadata differs. Collects all dirty
// entries into a single error rather than failing on the first mismatch.
func checkIfDirty(repoPath string, idxEntries []*index.IndexEntry) error {
	var errorBuilder strings.Builder
	for _, idxEntry := range idxEntries {
		absPath := filepath.Join(repoPath, filepath.FromSlash(idxEntry.Path()))
		info, err := os.Stat(absPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				fmt.Fprintf(&errorBuilder, "dirty: [%s] file got deleted\n\n", idxEntry.Path())
			} else {
				fmt.Fprintf(&errorBuilder, "dirty: [%s] failed to stat file: %s\n\n", idxEntry.Path(), err.Error())
			}
			continue
		}

		if info.Size() == idxEntry.FileSize() && info.ModTime().Truncate(time.Second).Equal(idxEntry.LastModified().Truncate(time.Second)) {
			continue
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			fmt.Fprintf(&errorBuilder, "dirty: [%s] failed to read file: %s\n\n", idxEntry.Path(), err.Error())
			continue
		}
		hash := utils.MustComputeHash(content, utils.BlobObjectType)
		if hash != idxEntry.Hash() {
			fmt.Fprintf(&errorBuilder, "dirty: [%s] file was modified \n\n", idxEntry.Path())
		}
	}

	if errorBuilder.Len() != 0 {
		return fmt.Errorf("working directory contains dirty files:\n\n%s", errorBuilder.String())
	}
	return nil
}

// updateHEAD atomically replaces the HEAD file with the given content using
// a temp-file-then-rename strategy identical to IndexManager.Save.
// For branch targets, content should be "ref: refs/heads/<branch>\n".
// For detached commits, content should be "<commit-hash>\n".
func updateHEAD(repoPath string, content string) error {
	headFilePath := filepath.Join(repoPath, constants.Gogit, constants.Head)
	tempFile, err := os.CreateTemp(filepath.Dir(headFilePath), "HEAD-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary HEAD file: %w", err)
	}
	tempPath := tempFile.Name()

	// Track success for cleanup
	succeeded := false
	defer func() {
		if !succeeded {
			tempFile.Close()
			os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.WriteString(content); err != nil {
		return fmt.Errorf("failed to write content to HEAD file: %w", err)
	}

	// Force OS to flush data to physical disk before rename
	// Critical for durability guarantees on power loss
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}

	// Close temp file descriptor before rename
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Replace HEAD file with temporary file
	if err := os.Rename(tempPath, headFilePath); err != nil {
		return fmt.Errorf("failed to rename temporary file to HEAD: %w", err)
	}
	succeeded = true

	return nil
}

// OrchestrateCheckoutExecution orchestrates the full checkout workflow:
// resolves the target to a commit hash, reads the commit to obtain its tree,
// loads the current index, cleans the working tree, restores the target
// commit's tree (rebuilding the index), and updates HEAD.
//
// When force is true the dirty-check is skipped, allowing recovery from a
// previously interrupted checkout. Steps 4-6 are destructive and
// non-transactional: if restore fails after clean, the working tree is left
// in a partial state. Re-running with force=true is the recovery path.
func OrchestrateCheckoutExecution(repoPath, target string, force bool) error {
	// 1. Resolve target to commit hash
	resolvedTarget, err := ResolveTarget(repoPath, target)
	if err != nil {
		return fmt.Errorf("failure while resolving target: %w", err)
	}

	// 2. Read the commit object to obtain its tree hash
	store := objects.NewObjectStore(repoPath)
	commit, err := store.ReadCommit(resolvedTarget.Hash)
	if err != nil {
		return fmt.Errorf("failed to read commit [%s]: %w", resolvedTarget.Hash, err)
	}

	// 3. Check if working directory is dirty
	idxManager := index.NewManager(repoPath)
	idx, err := idxManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load index before checking if working tree is dirty: %w", err)
	}

	idxEntries := idx.GetEntryList()
	if !force {
		if err := checkIfDirty(repoPath, idxEntries); err != nil {
			return err
		}
	}

	// 4. Clean working directory
	if err := CleanWorkingTree(repoPath, idxEntries); err != nil {
		return fmt.Errorf("failed to clean working directory: %w", err)
	}

	// 5. Restore file structure for repository snapshot and rebuild index
	if err := RestoreTreeAndRebuildIndex(repoPath, commit.TreeHash()); err != nil {
		return fmt.Errorf("failed to restore file structure from tree [%s] of commit [%s]: %w", commit.TreeHash(), resolvedTarget.Hash, err)
	}

	// 6. Update HEAD file reference
	var headFileContent string
	if resolvedTarget.IsBranch {
		headFileContent = constants.DefaultRefPrefix + target + "\n"
	} else {
		headFileContent = resolvedTarget.Hash + "\n"
	}

	if err := updateHEAD(repoPath, headFileContent); err != nil {
		return fmt.Errorf("failure during updating HEAD file: %w", err)
	}

	return nil
}
