package commits

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
)

// LoadIndexEntries returns all entries of staged files for repository's index
func loadIndexEntries(repoPath string) ([]*index.IndexEntry, error) {
	indexManager := index.NewManager(repoPath)
	idx, err := indexManager.Load()

	if err != nil {
		return nil, fmt.Errorf("Failed to load index from path [%s]: %w", repoPath, err)
	}

	return idx.GetEntryList(), nil
}

// fileEntry holds the minimal data needed to create a tree entry for a file.
type fileEntry struct {
	name string
	mode objects.FileMode
	hash string
}

// directoryNode represents a node in the in-memory directory tree.
// Files are stored as entries, subdirectories as child nodes.
type directoryNode struct {
	files    []fileEntry
	children map[string]*directoryNode
}

// buildDirectoryTree constructs an in-memory directory tree from flat index entries.
// Each index path is split into segments and inserted into the tree at the correct depth.
func buildDirectoryTree(entries []*index.IndexEntry) *directoryNode {
	root := &directoryNode{children: make(map[string]*directoryNode)}

	for _, entry := range entries {
		// Index paths are stored as forward-slash-separated (normalized at add time).
		pathSegments := strings.Split(entry.Path(), "/")
		current := root

		// Create intermediate directories
		for _, dirName := range pathSegments[:len(pathSegments)-1] {
			if current.children[dirName] == nil {
				current.children[dirName] = &directoryNode{children: make(map[string]*directoryNode)}
			}
			current = current.children[dirName]
		}

		// Add file to final directory
		fileName := pathSegments[len(pathSegments)-1]
		current.files = append(current.files, fileEntry{
			name: fileName,
			mode: index.ToObjectFileMode(entry.Mode()),
			hash: entry.Hash(),
		})
	}

	return root
}

// writeTree recursively creates and stores tree objects from a directoryNode.
// Processes children first (bottom-up) so parent trees can reference child hashes.
func writeTree(node *directoryNode, store *objects.ObjectStore) (string, error) {
	var treeEntries []objects.TreeEntry

	// Add file entries (blobs)
	for _, file := range node.files {
		treeEntry, err := objects.NewTreeEntry(file.mode, file.name, file.hash)
		if err != nil {
			return "", fmt.Errorf("failed to create file tree entry for %s: %w", file.name, err)
		}
		treeEntries = append(treeEntries, *treeEntry)
	}

	// Recurse into subdirectories, get their tree hashes
	for dirName, childNode := range node.children {
		childHash, err := writeTree(childNode, store)
		if err != nil {
			return "", fmt.Errorf("failed to write sub-tree for %s: %w", dirName, err)
		}

		treeEntry, err := objects.NewTreeEntry(objects.ModeDirectory, dirName, childHash)
		if err != nil {
			return "", fmt.Errorf("failed to create directory tree entry for %s: %w", dirName, err)
		}
		treeEntries = append(treeEntries, *treeEntry)
	}

	// Create and store tree
	tree, err := objects.NewTree(treeEntries)
	if err != nil {
		return "", fmt.Errorf("failed to create new tree object: %w", err)
	}

	if err := store.Store(tree); err != nil {
		return "", fmt.Errorf("failed to store tree object: %w", err)
	}

	return tree.Hash(), nil
}

// resolveHEADRef reads the .gogit/HEAD file and resolves the symbolic reference
// to the full filesystem path of the current branch ref file.
// Returns the absolute path to the ref file (e.g., .gogit/refs/heads/master).
func resolveHEADRef(repoPath string) (string, error) {
	headContent, err := os.ReadFile(filepath.Join(repoPath, constants.Gogit, constants.Head))
	if err != nil {
		return "", fmt.Errorf("failed to read HEAD file: %w", err)
	}

	trimmed := strings.TrimSpace(string(headContent))

	const refPrefix = "ref: "
	if !strings.HasPrefix(trimmed, refPrefix) {
		return "", fmt.Errorf("HEAD is not a symbolic ref: %q", trimmed)
	}

	relRefPath := strings.TrimPrefix(trimmed, refPrefix)
	fullRefPath := filepath.Join(repoPath, constants.Gogit, relRefPath)
	return fullRefPath, nil
}

// getRefCommitHash reads the commit hash stored in the given branch ref file.
// Returns an empty string if the ref file does not exist (first commit scenario).
// Returns an error for any other filesystem failure.
func getRefCommitHash(refPath string) (string, error) {
	parentCommitHash, err := os.ReadFile(refPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read ref file %q: %w", refPath, err)
	}

	return strings.TrimSpace(string(parentCommitHash)), nil
}

// createAndStoreCommit creates and stores commit in the file sytem and returns the commit hash
func createAndStoreCommit(treeHash, parentHash, message string, author objects.Author, store *objects.ObjectStore) (string, error) {
	commit, err := objects.NewCommit(treeHash, parentHash, message, author)
	if err != nil {
		return "", fmt.Errorf("failed to create commit: %w", err)
	}

	if err := store.Store(commit); err != nil {
		return "", fmt.Errorf("failed to store commit: %w", err)
	}

	return commit.Hash(), nil
}

// updateRefFile is updating the commit hash in the branch ref file by creating a temporary
// file, writing the commit hash in said file and then replacing the ref file with the temporary file.
// This is covering safety cases for updates in file system.
func updateRefFile(repoPath, commitHash string) error {
	refPath, err := resolveHEADRef(repoPath)
	if err != nil {
		return err
	}

	// Create temporary file to write the commit hash
	// If everything is successful this file will replace the existing commit file
	tempFile, err := os.CreateTemp(filepath.Dir(refPath), ".commit-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary commit file: %w", err)
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

	// write commit hash to tempFile
	_, err = tempFile.Write([]byte(commitHash + "\n"))
	if err != nil {
		return fmt.Errorf("failed to write commit to temporary file: %w", err)
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

	// Replace ref file with temporary file
	if err := os.Rename(tempPath, refPath); err != nil {
		return fmt.Errorf("failed to rename temporary file to commit ref: %w", err)
	}

	succeeded = true
	return nil
}

// OrchestrateCommitExecution orchestrates the full commit workflow:
// loads the index, builds the tree hierarchy, resolves the parent commit,
// creates and stores the commit object, and updates the current branch ref.
func OrchestrateCommitExecution(repoPath string, message string, author objects.Author) (string, error) {
	// 1. load staged files entries from index
	entries, err := loadIndexEntries(repoPath)
	if err != nil {
		return "", err
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("nothing to commit")
	}

	// 2. construct full directory node in-memory - to be used in order to create and store Tree entries
	directoryNode := buildDirectoryTree(entries)

	// 3. Create and store recursively and bottom up all trees and return root tree hash
	store := objects.NewObjectStore(repoPath)
	rootTreeHash, err := writeTree(directoryNode, store)
	if err != nil {
		return "", fmt.Errorf("failed to create commit tree directory: %w", err)
	}

	// 4. resolve commit parent hash
	refPath, err := resolveHEADRef(repoPath)
	if err != nil {
		return "", err
	}

	parentHash, err := getRefCommitHash(refPath)
	if err != nil {
		return "", err
	}

	// 4.5 reject commit if tree is unchanged from parent
	if parentHash != "" {
		parentCommit, err := store.ReadCommit(parentHash)
		if err != nil {
			return "", fmt.Errorf("failed to read parent commit %q: %w", parentHash, err)
		}
		if parentCommit.TreeHash() == rootTreeHash {
			return "", fmt.Errorf("nothing to commit: working tree clean")
		}
	}

	// 5. create and store commit in the filesystem
	commitHash, err := createAndStoreCommit(rootTreeHash, parentHash, message, author, store)
	if err != nil {
		return "", err
	}

	// 6. update reference file with the new commit hash
	if err := updateRefFile(repoPath, commitHash); err != nil {
		return "", fmt.Errorf("failed to update ref file [%s]: %w", refPath, err)
	}

	return commitHash, nil
}
