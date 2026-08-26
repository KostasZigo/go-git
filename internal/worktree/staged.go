package worktree

import (
	"fmt"

	"github.com/KostasZigo/gogit/internal/objects"
)

// InspectStagedChanges compares headSnapshot with the service's operation-scoped
// index snapshot and returns every staged difference in deterministic order.
func (service *Service) InspectStagedChanges(headSnapshot objects.TreeSnapshot) ([]Change, error) {
	if err := headSnapshot.Validate(); err != nil {
		return nil, fmt.Errorf("invalid HEAD snapshot: %w", err)
	}

	indexSnapshot, err := service.index.ToTreeSnapshot()
	if err != nil {
		return nil, err
	}

	return compareSnapshots(indexSnapshot, headSnapshot), nil
}

// compareSnapshots returns every change between 2 snapshots.
// The result is sorted by logical path and then change kind.
func compareSnapshots(s1, s2 objects.TreeSnapshot) []Change {
	changes := make([]Change, 0)

	// Find all files present to s1 that where modified or added compared to s2
	for path, e1 := range s1 {
		e2, exists := s2[path]
		if !exists {
			changes = append(changes, Change{Path: path, Kind: ChangeAdded})
			continue
		}
		if e2.Hash != e1.Hash {
			changes = append(changes, Change{Path: path, Kind: ChangeContentModified})
		}
		if e2.Mode != e1.Mode {
			changes = append(changes, Change{Path: path, Kind: ChangeModeModified})
		}
	}

	// Find all file present to s2 that don't exist in s1 (got deleted)
	for path := range s2 {
		if _, exists := s1[path]; !exists {
			changes = append(changes, Change{Path: path, Kind: ChangeDeleted})
		}
	}

	// sorting the changes makes the result determenistic
	sortChanges(changes)
	return changes
}
