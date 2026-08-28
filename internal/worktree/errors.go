package worktree

import (
	"errors"
	"fmt"
	"strings"
)

// ErrPreflight indicates that inspection or planning prevented snapshot application.
var ErrPreflight = errors.New("worktree preflight failed")

// ErrRollback identifies a failure while restoring the worktree after an
// unsuccessful snapshot application.
var ErrRollback = errors.New("rollback failed")

// ErrRepositoryMetadataTarget indicates that a target snapshot attempts to
// address gogit's internal repository metadata directory.
var ErrRepositoryMetadataTarget = errors.New(
	"target snapshot contains repository metadata path",
)

// PreflightError reports the inspection state that prevented snapshot
// application.
type PreflightError struct {
	State State
}

// Error implements the error interface.
func (preflightError *PreflightError) Error() string {
	var builder strings.Builder
	for _, s := range preflightError.State.Collisions {
		fmt.Fprintf(&builder, "Collision [%s] found for path [%s]\n", s.Kind, s.Path)
	}
	for _, s := range preflightError.State.StagedChanges {
		fmt.Fprintf(&builder, "Staged change [%s] found for path [%s]\n", s.Kind, s.Path)
	}
	for _, s := range preflightError.State.WorktreeChanges {
		fmt.Fprintf(&builder, "Worktree change [%s] found for path [%s]\n", s.Kind, s.Path)
	}
	return fmt.Sprintf("%s: %s", ErrPreflight.Error(), strings.TrimSuffix(builder.String(), "\n"))
}

// Unwrap allows callers to identify preflight failures with errors.Is.
func (preflightError *PreflightError) Unwrap() error {
	return ErrPreflight
}
