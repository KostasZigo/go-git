package objects

import (
	"fmt"
	"path"
	"strings"

	"github.com/KostasZigo/gogit/internal/hasher"
)

// snapshotDirectory represents one implied directory while rebuilding a
// recursive Git tree from a flat snapshot.
type snapshotDirectory struct {
	files    map[string]SnapshotEntry
	children map[string]*snapshotDirectory
}

// newSnapshotDirectory creates an empty implied snapshot directory.
func newSnapshotDirectory() *snapshotDirectory {
	return &snapshotDirectory{
		files:    make(map[string]SnapshotEntry),
		children: make(map[string]*snapshotDirectory),
	}
}

// add inserts a logical snapshot path into its implied directory.
func (sd *snapshotDirectory) add(relativePath string, entry SnapshotEntry) {
	pathSegments := strings.Split(relativePath, "/")
	currentDirectory := sd

	for _, directoryName := range pathSegments[:len(pathSegments)-1] {
		childDirectory, exists := currentDirectory.children[directoryName]
		if !exists {
			childDirectory = newSnapshotDirectory()
			currentDirectory.children[directoryName] = childDirectory
		}
		currentDirectory = childDirectory
	}

	fileName := pathSegments[len(pathSegments)-1]
	currentDirectory.files[fileName] = entry
}

// storeSnapshotDirectory stores the whole git tree recursively, stores child trees before their parent and returns
// the tree hash.
func (store *ObjectStore) storeSnapshotDirectory(directory *snapshotDirectory, directoryPath string) (string, error) {
	treeEntries := make([]TreeEntry, 0, len(directory.files)+len(directory.children))

	// create tree entries from files
	for fileName, snapEntry := range directory.files {
		treeEntry, err := NewTreeEntry(snapEntry.Mode, fileName, snapEntry.Hash)
		if err != nil {
			return "", fmt.Errorf("failed to create file tree entry for [%s]: %w", path.Join(directoryPath, fileName), err)
		}
		treeEntries = append(treeEntries, *treeEntry)
	}

	// create tree etries from directories
	for dirName, childDir := range directory.children {
		childPath := path.Join(directoryPath, dirName)

		childTreeHash, err := store.storeSnapshotDirectory(childDir, childPath)
		if err != nil {
			return "", err
		}

		treeEntry, err := NewTreeEntry(ModeDirectory, dirName, childTreeHash)
		if err != nil {
			return "", fmt.Errorf("failed to create directory tree entry [%s]: %w", childPath, err)
		}

		treeEntries = append(treeEntries, *treeEntry)
	}

	tree, err := NewTree(treeEntries)
	if err != nil {
		return "", fmt.Errorf("failed to create tree for directory [%s]: %w", directoryPath, err)
	}
	if err := store.Store(tree); err != nil {
		return "", fmt.Errorf("failed to store tree for directory [%s]: %w", directoryPath, err)
	}

	return tree.Hash(), nil
}

// StoreTreeSnapshot builds and stores recursive Git tree objects for snapshot
// and returns the hash of the root tree object.
func (store *ObjectStore) StoreTreeSnapshot(snapshot TreeSnapshot) (string, error) {
	if err := snapshot.Validate(); err != nil {
		return "", fmt.Errorf("invalid tree snapshot: %w", err)
	}

	rootDirectory := newSnapshotDirectory()
	for relativePath, entry := range snapshot {
		rootDirectory.add(relativePath, entry)
	}

	rootTreeHash, err := store.storeSnapshotDirectory(rootDirectory, "")
	if err != nil {
		return "", fmt.Errorf("failed to store tree snapshot: %w", err)
	}

	return rootTreeHash, nil
}

// flattenTreeSnapshot recursively reads tree objects and adds only leaf
// entries to snapshot. treePath is always a logical forward-slash path.
func (store *ObjectStore) flattenTreeSnapshot(treeHash, treePath string, snapshot TreeSnapshot) error {
	tree, err := store.ReadTree(treeHash)
	if err != nil {
		if treePath == "" {
			treePath = "." // for root tree
		}
		return fmt.Errorf("failed to read tree at path [%s]: %w", treePath, err)
	}

	for _, entry := range tree.Entries() {
		entryPath := path.Join(treePath, entry.Name())

		if entry.IsDirectory() {
			if err := store.flattenTreeSnapshot(entry.Hash(), entryPath, snapshot); err != nil {
				return err
			}
			continue
		}

		snapshot[entryPath] = SnapshotEntry{
			Mode: entry.Mode(),
			Hash: entry.Hash(),
		}
	}

	return nil
}

// ReadTreeSnapshot reads a recursive Git tree and returns its leaf entries
// keyed by logical repository paths.
func (store *ObjectStore) ReadTreeSnapshot(treeHash string) (TreeSnapshot, error) {
	if !hasher.IsValidSHA1(treeHash) {
		return nil, fmt.Errorf("invalid root tree hash %s", treeHash)
	}

	snapshot := make(TreeSnapshot)
	if err := store.flattenTreeSnapshot(treeHash, "", snapshot); err != nil {
		return nil, err
	}

	if err := snapshot.Validate(); err != nil {
		return nil, fmt.Errorf("invalid snapshot read from tree %s: %w", treeHash, err)
	}

	return snapshot, nil
}
