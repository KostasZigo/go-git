package worktree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
)

// InspectCollisions reports untracked paths that would prevent target from
// being applied safely.
func (service *Service) InspectCollisions(targetSnapshot objects.TreeSnapshot) ([]Collision, error) {
	if err := validateWorktreeSnapshot(targetSnapshot); err != nil {
		return nil, fmt.Errorf("invalid target snapshot: %w", err)
	}

	idx, err := service.indexManager.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load index: %w", err)
	}
	trackedPaths := buildTrackedPathSet(idx)

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

	// inspect collisions derived from files removed by target
	removedPathCollissions, err := inspectRemovedTrackedPathCollisions(service.repoPath, targetSnapshot, trackedPaths)
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
			if errors.Is(err, fs.ErrNotExist) {
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
		if errors.Is(err, fs.ErrNotExist) {
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
		if errors.Is(err, fs.ErrNotExist) {
			return Collision{}, nil
		}
		return Collision{}, fmt.Errorf("failed to inspect target path [%s]: %w", targetPath, err)
	}

	if fileInfo.IsDir() {
		return Collision{Path: targetPath, Kind: CollisionUntrackedDirectory}, nil
	}
	return Collision{Path: targetPath, Kind: CollisionUntrackedFile}, nil
}

// inspectRemovedTrackedPathCollisions reports untracked descendants below
// tracked paths that are absent from the target snapshot.
func inspectRemovedTrackedPathCollisions(repoPath string, targetSnapshot objects.TreeSnapshot, trackedPaths map[string]struct{}) ([]Collision, error) {
	collisions := make([]Collision, 0)

	for trackedPath := range trackedPaths {
		if _, exists := targetSnapshot[trackedPath]; exists {
			continue
		}

		pathCollisions, err := inspectTrackedPathDirectory(repoPath, trackedPath, trackedPaths)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect removed tracked path %q: %w", trackedPath, err)
		}

		collisions = append(collisions, pathCollisions...)
	}

	return collisions, nil
}
