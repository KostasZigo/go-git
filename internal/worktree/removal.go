package worktree

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// RemovePaths removes all logical worktree paths from the file system.
// Operates in two passes: first deletes all
// files, then collects unique parent directories and prunes empty ones deepest-first
// up to (but not including) repoPath. Files already missing on disk are silently skipped.
func RemovePaths(repoPath string, pathsToRemove []string) error {
	uniqueDirs := map[string]struct{}{}
	for _, filePath := range pathsToRemove {
		relPath, err := filepath.Localize(filePath)
		if err != nil {
			return fmt.Errorf("failed to convert path to local os specific format for [%s]: %w", filePath, err)
		}
		absPath := filepath.Join(repoPath, relPath)
		if err := os.Remove(absPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("failed to remove [%s]: %w", relPath, err)
		}

		dir := filepath.Dir(absPath)
		if dir == repoPath {
			continue
		}
		uniqueDirs[dir] = struct{}{}
	}

	dirs := slices.Collect(maps.Keys(uniqueDirs))
	sortLocalPathsDepthFirst(dirs)

	for _, dir := range dirs {
		if err := pruneEmptyDirectories(repoPath, dir); err != nil {
			return err
		}
	}

	return nil
}

// pruneEmptyDirectories walks upward from dirPath toward repoPath, removing each
// directory that is empty after file deletion. Stops at repoPath or the first
// non-empty directory.
func pruneEmptyDirectories(repoPath, dirPath string) error {
	for {
		if repoPath == dirPath {
			return nil
		}

		parentDir := filepath.Dir(dirPath)
		isEmpty, err := isDirEmpty(dirPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("failed to check if directory [%s] is empty: %w", dirPath, err)
		}

		if isEmpty {
			if err := os.Remove(dirPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("failed to remove empty directory [%s]: %w", dirPath, err)
			}
			dirPath = parentDir
		} else {
			return nil
		}
	}
}

// isDirEmpty reports whether the directory at path contains no entries.
// Uses a single Readdirnames call to avoid reading the entire directory listing.
func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	_, err = f.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	return false, err
}

// sortLogicalPathsDepthFirst sorts slash-separated Git paths in place from
// deepest to shallowest.
func sortLogicalPathsDepthFirst(paths []string) {
	sortPathsDepthFirst(paths, "/")
}

// sortLocalPathsDepthFirst sorts OS-specific filesystem paths in place from
// deepest to shallowest.
func sortLocalPathsDepthFirst(paths []string) {
	sortPathsDepthFirst(paths, string(os.PathSeparator))
}

// sortPathsDepthFirst sorts paths in place by counting the supplied separator.
// Paths at the same depth are ordered lexically for deterministic processing.
func sortPathsDepthFirst(paths []string, sep string) {
	slices.SortFunc(paths, func(a, b string) int {
		aDepth := strings.Count(a, sep)
		bDepth := strings.Count(b, sep)

		if aDepth != bDepth {
			return cmp.Compare(bDepth, aDepth)
		}
		return strings.Compare(a, b)
	})
}
