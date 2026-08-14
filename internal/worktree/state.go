// Package worktree inspects and applies repository snapshots to the filesystem.
package worktree

import (
	"slices"
	"strings"
)

// ChangeKind identifies how a tracked path differs from its baseline.
type ChangeKind string

const (
	// ChangeAdded indicates that a path was added.
	ChangeAdded ChangeKind = "added"

	// ChangeDeleted indicates that a path was deleted.
	ChangeDeleted ChangeKind = "deleted"

	// ChangeContentModified indicates that a file content changed.
	ChangeContentModified ChangeKind = "content-modified"

	// ChangeModeModified indicates that a file mode changed.
	ChangeModeModified ChangeKind = "mode-modified"

	// ChangeTypeModified indicates that the filesystem object type changed.
	// For example, a file name got changed to directory.
	ChangeTypeModified ChangeKind = "type-modified"
)

// Change describes a modification in the working-tree or staging area
type Change struct {
	Path string
	Kind ChangeKind
}

// CollisionKind identifies why a target cannot be applied safely.
type CollisionKind string

const (
	// CollisionUntrackedFile indicates that an untracked file occupies a
	// target path.
	CollisionUntrackedFile CollisionKind = "untracked-file"

	// CollisionUntrackedDirectory indicates that an untracked directory
	// occupies a target file path.
	CollisionUntrackedDirectory CollisionKind = "untracked-directory"

	// CollisionParentFile indicates that an untracked file prevents creation
	// of a target descendant.
	CollisionParentFile CollisionKind = "parent-file"

	// CollisionUntrackedDescendant indicates that an untracked descendant
	// prevents replacing a directory with a file.
	CollisionUntrackedDescendant CollisionKind = "untracked-descendant"
)

// Collision describes an untracked or structural obstruction.
type Collision struct {
	Path string
	Kind CollisionKind
}

// State contains the differences and collisions found during worktree
// inspection.
type State struct {
	StagedChanges   []Change
	WorktreeChanges []Change
	Collisions      []Collision
}

// HasTrackedChanges reports whether the index or working tree contains changes
// to tracked paths.
func (state State) HasTrackedChanges() bool {
	return len(state.StagedChanges) > 0 || len(state.WorktreeChanges) > 0
}

// HasCollisions reports whether applying the target snapshot would overwrite
// or be obstructed by untracked content.
func (state State) HasCollisions() bool {
	return len(state.Collisions) > 0
}

// sortChanges sorts a slice of Change elements first by their path and
// the by the kind of change.
func sortChanges(changes []Change) {
	slices.SortFunc(changes, func(a, b Change) int {
		if comparison := strings.Compare(a.Path, b.Path); comparison != 0 {
			return comparison
		}
		return strings.Compare(string(a.Kind), string(b.Kind))
	})
}

// sortCollisions sorts a slice of Collision elements first by their path and
// the by the kind of change.
func sortCollisions(collisions []Collision) {
	slices.SortFunc(collisions, func(a, b Collision) int {
		if comparison := strings.Compare(a.Path, b.Path); comparison != 0 {
			return comparison
		}
		return strings.Compare(string(a.Kind), string(b.Kind))
	})
}
