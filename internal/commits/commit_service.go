// Package commits orchestrates the high-level version control operations that
// act on or produce commit objects. It bridges the cmd layer and the lower-level
// internal packages (objects, index, utils) to implement commit creation,
// history traversal and working-tree checkout.
package commits

import (
	"fmt"
	"strings"

	"github.com/KostasZigo/gogit/internal/branches"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
)

// LoadIndexEntries returns all entries of staged files for repository's index
func loadIndexEntries(repoPath string) ([]*index.Entry, error) {
	indexManager := index.NewManager(repoPath)
	idx, err := indexManager.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load index from path [%s]: %w", repoPath, err)
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
func buildDirectoryTree(entries []*index.Entry) *directoryNode {
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

// createAndStoreCommit creates and stores commit in the file system and returns the commit hash
func createAndStoreCommit(treeHash, parentHash, message string, author objects.Author, store *objects.ObjectStore) (string, error) {
	var commit *objects.Commit
	var err error
	if parentHash == "" {
		commit, err = objects.NewInitialCommit(treeHash, message, author)
	} else {
		commit, err = objects.NewCommit(treeHash, parentHash, message, author)
	}
	if err != nil {
		return "", fmt.Errorf("failed to create commit: %w", err)
	}

	if err := store.Store(commit); err != nil {
		return "", fmt.Errorf("failed to store commit: %w", err)
	}

	return commit.Hash(), nil
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

	// 2. resolve the current branch and commit parent before creating objects
	currentBranch, err := branches.ResolveCurrent(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve current branch: %w", err)
	}
	parentHash := currentBranch.Hash

	// 3. construct full directory node in-memory - to be used in order to create and store Tree entries
	directoryNode := buildDirectoryTree(entries)

	// 4. Create and store recursively and bottom up all trees and return root tree hash
	store := objects.NewObjectStore(repoPath)
	rootTreeHash, err := writeTree(directoryNode, store)
	if err != nil {
		return "", fmt.Errorf("failed to create commit tree directory: %w", err)
	}

	// 5. reject commit if tree is unchanged from parent
	if parentHash != "" {
		parentCommit, err := store.ReadCommit(parentHash)
		if err != nil {
			return "", fmt.Errorf("failed to read parent commit %q: %w", parentHash, err)
		}
		if parentCommit.TreeHash() == rootTreeHash {
			return "", fmt.Errorf("nothing to commit: working tree clean")
		}
	}

	// 6. create and store commit in the filesystem
	commitHash, err := createAndStoreCommit(rootTreeHash, parentHash, message, author, store)
	if err != nil {
		return "", err
	}

	// 7. advance the current branch only if it still points to the parent commit
	if err := branches.CompareAndSwap(
		repoPath,
		currentBranch.Name,
		parentHash,
		commitHash,
	); err != nil {
		return "", fmt.Errorf("failed to update branch [%s]: %w", currentBranch.Name, err)
	}

	return commitHash, nil
}
