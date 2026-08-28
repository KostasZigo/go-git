package worktree

import (
	"fmt"
	"slices"
	"strings"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
)

// validateWorktreeSnapshot verifies that snapshot can be inspected without
// treating repository metadata as working-tree content.
func validateWorktreeSnapshot(snapshot objects.TreeSnapshot) error {
	if err := snapshot.Validate(); err != nil {
		return err
	}

	snapshotPaths := make([]string, 0, len(snapshot))
	for snapshotPath := range snapshot {
		snapshotPaths = append(snapshotPaths, snapshotPath)
	}
	slices.Sort(snapshotPaths)

	for _, targetPath := range snapshotPaths {
		if isRepositoryMetadataPath(targetPath) {
			return fmt.Errorf("%w: %q", ErrRepositoryMetadataTarget, targetPath)
		}
	}

	return nil
}

// isRepositoryMetadataPath reports whether logicalPath addresses gogit's
// internal metadata directory.
func isRepositoryMetadataPath(logicalPath string) bool {
	return logicalPath == constants.Gogit ||
		strings.HasPrefix(logicalPath, constants.Gogit+"/")
}
