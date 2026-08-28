package worktree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
)

// InspectCollisions uses the service's operation-scoped index snapshot to
// report untracked paths that would prevent targetSnapshot from being applied
// over originalSnapshot safely.
func (service *Service) InspectCollisions(originalSnapshot, targetSnapshot objects.TreeSnapshot) ([]Collision, error) {
	if err := validateWorktreeSnapshot(originalSnapshot); err != nil {
		return nil, fmt.Errorf("invalid original snapshot: %w", err)
	}
	if err := validateWorktreeSnapshot(targetSnapshot); err != nil {
		return nil, fmt.Errorf("invalid target snapshot: %w", err)
	}
	trackedPaths := buildTrackedPathSet(service.index)

	collisionSet := make(map[Collision]struct{}, 0)
	// inspect collisions per target path
	for targetPath := range targetSnapshot {
		targetCollisions, err := inspectTargetCollisions(
			service.repoPath,
			targetPath,
			trackedPaths,
		)
		if err != nil {
			return nil, err
		}

		for _, collision := range targetCollisions {
			collisionSet[collision] = struct{}{}
		}
	}

	// Inspect every path that snapshot application may remove. Paths present in
	// originalSnapshot but absent from the index are staged deletions, so any
	// object recreated at such a path is untracked and must be preserved.
	removedPathCollissions, err := inspectRemovedPathCollisions(
		service.repoPath,
		originalSnapshot,
		targetSnapshot,
		trackedPaths,
	)
	if err != nil {
		return nil, err
	}
	for _, collision := range removedPathCollissions {
		collisionSet[collision] = struct{}{}
	}

	collisions := make([]Collision, 0, len(collisionSet)+len(removedPathCollissions))
	for collision := range collisionSet {
		collisions = append(collisions, collision)
	}

	sortCollisions(collisions)
	return collisions, nil
}

// buildTrackedPathSet returns every index path for collision inspection.
func buildTrackedPathSet(idx *index.Index) map[string]struct{} {
	trackedPaths := make(map[string]struct{}, idx.CountEntries())

	for _, entry := range idx.GetEntryList() {
		trackedPaths[entry.Path()] = struct{}{}
	}

	return trackedPaths
}

// inspectTargetCollisions reports the highest-priority collisions for one
// target path.
func inspectTargetCollisions(repoPath, targetPath string, trackedPaths map[string]struct{}) ([]Collision, error) {
	parentCollision, err := inspectUntrackedParent(repoPath, targetPath, trackedPaths)
	if err != nil {
		return nil, err
	}
	if parentCollision.Path != "" {
		return []Collision{parentCollision}, nil
	}

	descendantCollisions, err := inspectTrackedPathDirectory(repoPath, targetPath, trackedPaths)
	if err != nil {
		return nil, err
	}
	if len(descendantCollisions) > 0 {
		return descendantCollisions, nil
	}

	if _, isTracked := trackedPaths[targetPath]; isTracked ||
		hasTrackedDescendant(targetPath, trackedPaths) {
		return nil, nil
	}

	exactCollision, err := inspectTargetPathCollision(repoPath, targetPath)
	if err != nil {
		return nil, err
	}
	if exactCollision.Path == "" {
		return nil, nil
	}

	return []Collision{exactCollision}, nil
}

// inspectUntrackedParent reports an untracked regular file that blocks
// creation of a directory required by targetPath.
// E.x. Disk contains : kappa
//
//	Index: doesn't track kappa
//	Target contains: kappa/hello.txt
func inspectUntrackedParent(repoPath, targetPath string, trackedPaths map[string]struct{}) (Collision, error) {
	for parentPath := path.Dir(targetPath); parentPath != "."; parentPath = path.Dir(parentPath) {
		if _, isTracked := trackedPaths[parentPath]; isTracked {
			continue
		}

		osParentPath, err := filepath.Localize(parentPath)
		if err != nil {
			return Collision{}, fmt.Errorf("failed to convert parent path to local os format [%s]: %w", parentPath, err)
		}

		fileInfo, err := os.Lstat(filepath.Join(repoPath, osParentPath))
		if err != nil {
			if isMissingPathError(err) {
				continue
			}
			return Collision{}, fmt.Errorf("failed to inspect path [%s]: %w", osParentPath, err)
		}

		if !fileInfo.IsDir() {
			return Collision{Path: parentPath, Kind: CollisionParentFile}, nil
		}
	}

	return Collision{}, nil
}

// inspectTrackedPathDirectory reports untracked files below a Git-owned path
// that currently exists as a directory on disk.
// E.x. Disk contains : kappa/hello.txt and kappa/world.txt
//
//	Index contains: kappa/world.txt
//	Target contains: kappa -> converts tracked dir to file
//
// or
// Disk contains: kappa/hello.txt
// Index contains: kappa
// Target contains: kappa
func inspectTrackedPathDirectory(repoPath, targetPath string, trackedPaths map[string]struct{}) ([]Collision, error) {
	_, isDirectlyTracked := trackedPaths[targetPath]
	if !isDirectlyTracked && !hasTrackedDescendant(targetPath, trackedPaths) {
		return nil, nil
	}

	osTargetPath, err := filepath.Localize(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to convert target path [%s] to local os format: %w", targetPath, err)
	}

	osPath := filepath.Join(repoPath, osTargetPath)
	fileInfo, err := os.Lstat(osPath)
	if err != nil {
		if isMissingPathError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to stat target path [%s]: %w", osTargetPath, err)
	}

	if !fileInfo.IsDir() {
		return nil, nil
	}

	collisions := make([]Collision, 0)

	var walkDirCollisionInspect fs.WalkDirFunc = func(currentPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		filePath, err := filepath.Rel(repoPath, currentPath)
		if err != nil {
			return fmt.Errorf("failed to determine relative path for [%s]: %w", currentPath, err)
		}
		gitFilePath := filepath.ToSlash(filePath)

		if _, exists := trackedPaths[gitFilePath]; !exists {
			collisions = append(collisions, Collision{Path: gitFilePath, Kind: CollisionUntrackedDescendant})
		}
		return nil
	}

	if err := filepath.WalkDir(osPath, walkDirCollisionInspect); err != nil {
		return collisions, fmt.Errorf("failed during walk of file directory [%s]: %w", osPath, err)
	}
	return collisions, nil
}

// hasTrackedDescendant reports whether directoryPath is an implicit tracked
// directory because the index contains at least one path below it.
func hasTrackedDescendant(directoryPath string, trackedPaths map[string]struct{}) bool {
	for trackedPath := range trackedPaths {
		if strings.HasPrefix(trackedPath, (directoryPath + "/")) {
			return true
		}
	}
	return false
}

// inspectTargetPathCollision checks whether an untracked filesystem object
// already occupies targetPath.
// E.x. Disk contains : kappa
//
//	Index contains: not tracking kappa
//	Target contains: kappa
//
// # OR
//
// E.x. Disk contains : kappa/hello.txt
//
//	Index contains: not tracking kappa
//	Target contains: kappa
func inspectTargetPathCollision(repoPath, targetPath string) (Collision, error) {
	osTargetPath, err := filepath.Localize(targetPath)
	if err != nil {
		return Collision{}, fmt.Errorf("failed to convert target file path to local os path: %w", err)
	}

	fileInfo, err := os.Lstat(filepath.Join(repoPath, osTargetPath))
	if err != nil {
		if isMissingPathError(err) {
			return Collision{}, nil
		}
		return Collision{}, fmt.Errorf("failed to inspect target path [%s]: %w", targetPath, err)
	}

	if fileInfo.IsDir() {
		return Collision{Path: targetPath, Kind: CollisionUntrackedDirectory}, nil
	}
	return Collision{Path: targetPath, Kind: CollisionUntrackedFile}, nil
}

// inspectRemovedPathCollisions reports untracked objects at paths that
// snapshot application will remove because they are owned by originalSnapshot
// or the current index but absent from targetSnapshot.
func inspectRemovedPathCollisions(repoPath string, originalSnapshot, targetSnapshot objects.TreeSnapshot, trackedPaths map[string]struct{}) ([]Collision, error) {
	collisions := make([]Collision, 0)
	removedPaths := make(map[string]struct{}, len(originalSnapshot)+len(trackedPaths))

	for trackedPath := range trackedPaths {
		if _, exists := targetSnapshot[trackedPath]; exists {
			continue
		}
		removedPaths[trackedPath] = struct{}{}
	}
	for originalPath := range originalSnapshot {
		if _, exists := targetSnapshot[originalPath]; exists {
			continue
		}
		removedPaths[originalPath] = struct{}{}
	}

	for removedPath := range removedPaths {
		_, isTracked := trackedPaths[removedPath]
		if !isTracked && !hasTrackedDescendant(removedPath, trackedPaths) {
			collision, err := inspectTargetPathCollision(repoPath, removedPath)
			if err != nil {
				return nil, fmt.Errorf("failed to inspect removed path %q: %w", removedPath, err)
			}
			if collision.Path != "" {
				collisions = append(collisions, collision)
			}
			continue
		}

		pathCollisions, err := inspectTrackedPathDirectory(repoPath, removedPath, trackedPaths)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect removed tracked path %q: %w", removedPath, err)
		}

		collisions = append(collisions, pathCollisions...)
	}

	return collisions, nil
}

// isMissingPathError reports whether an inspected path is absent, including
// the Unix case where an ancestor exists but is not a directory.
func isMissingPathError(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}
