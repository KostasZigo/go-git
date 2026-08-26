package worktree

import (
	"fmt"

	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
)

// Service provides one operation-scoped view of the worktree and index.
// Its inspectors and snapshot application all use the index loaded by
// NewService, preventing one operation from observing different index states.
type Service struct {
	repoPath string
	index    *index.Index
}

// NewService loads the current index once and creates a worktree service whose
// methods share that index snapshot for the lifetime of the service.
func NewService(repoPath string) (*Service, error) {
	idx, err := index.NewManager(repoPath).Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load current index: %w", err)
	}

	return &Service{
		repoPath: repoPath,
		index:    idx,
	}, nil
}

// ResolveRepositoryState inspects staged changes against originSnapshot, tracked
// worktree changes against the index, and collisions with applying targetSnapshot
// over originSnapshot.
func (service *Service) ResolveRepositoryState(originSnapshot, targetSnapshot objects.TreeSnapshot) (*State, error) {
	stagedChanges, err := service.InspectStagedChanges(originSnapshot)
	if err != nil {
		return nil, fmt.Errorf("failed inspection for staged changes: %w", err)
	}

	worktreeChanges, err := service.InspectWorktreeChanges()
	if err != nil {
		return nil, fmt.Errorf("failed inspection for worktree changes: %w", err)
	}

	collisions, err := service.InspectCollisions(originSnapshot, targetSnapshot)
	if err != nil {
		return nil, fmt.Errorf("failed inspection for target collisions: %w", err)
	}

	return &State{
		StagedChanges:   stagedChanges,
		WorktreeChanges: worktreeChanges,
		Collisions:      collisions,
	}, nil
}
