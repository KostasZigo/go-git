package worktree

import (
	"errors"
)

// ErrPreflight indicates that snapshot application failed the inspection.
var ErrPreflight = errors.New("worktree preflight failed")

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
	return ErrPreflight.Error()
}

// Unwrap allows callers to identify preflight failures with errors.Is.
func (preflightError *PreflightError) Unwrap() error {
	return ErrPreflight
}
