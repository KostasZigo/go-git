package worktree

import "github.com/KostasZigo/gogit/internal/index"

// Service provides worktree inspection and snapshot-application operations.
type Service struct {
	repoPath     string
	indexManager *index.Manager
}

// NewService creates a worktree service for repoPath.
func NewService(repoPath string) *Service {
	return &Service{
		repoPath:     repoPath,
		indexManager: index.NewManager(repoPath),
	}
}
