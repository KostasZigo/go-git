package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/index"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/utils"
	"github.com/spf13/cobra"
)

// ExecutableFileMask is used on bitwise opertations for identifying executable files
const ExecutableFileMask = 0111

var addCmd = &cobra.Command{
	Use:   "add <file>...",
	Short: "Add file to the staging area",
	Long: `Add file contents to the index (staging area).
Files are hashed and stored as blob objects, with entries recorded in .gogit/index.

Examples:
  # Stage single file
  gogit add README.md

  # Stage multiple files
  gogit add main.go utils.go

  # Stage all files in directory (future feature)
  gogit add .`,
	SilenceUsage: true,
	Args:         minimumArgs(1),
	RunE:         runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

// minimumArgs validates command receives at least n positional arguments.
// Returns error with usage help if argument limit exceeded.
func minimumArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < n {
			cmd.SilenceUsage = false
			return fmt.Errorf("%s command accepts at least %d arg(s), received %d", constants.AddCmdName, n, len(args))
		}
		return nil
	}
}

// runAdd stages files in index and creates blob objects.
func runAdd(cmd *cobra.Command, args []string) error {
	// Find repository root path
	repoPath, err := utils.FindRepoRoot()
	if err != nil {
		return err
	}

	// Load existing index
	indexManager := index.NewManager(repoPath)
	idx, err := indexManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}
	// Create object store
	store := objects.NewObjectStore(repoPath)

	var filePaths []string
	// Check if all files should be added
	if len(args) == 1 && args[0] == "." {
		filePaths, err = collectAllRepoFiles(repoPath)
		if err != nil {
			return fmt.Errorf("failed to collect repository files: %w", err)
		}
	} else {
		// Individual file arguments
		filePaths = slices.Clone(args)
	}

	// Sort paths for deterministic processing
	slices.Sort(filePaths)

	// Process each file
	for _, filePath := range filePaths {
		if err := addFile(cmd, repoPath, filePath, idx, store); err != nil {
			return fmt.Errorf("failed to add file %s: %w", filePath, err)
		}
	}

	// Save updated index
	if err := indexManager.Save(idx); err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}

	return nil
}

// collectAllRepoFiles recursively walks repository collecting non-ignored files.
// Returns relative paths from repository root suitable for staging.
func collectAllRepoFiles(repoPath string) ([]string, error) {
	var filePaths []string
	goGitDir := filepath.Join(repoPath, constants.Gogit)

	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to access path %s: %w", path, err)
		}

		// Skip .gogit directory entirely
		if d.IsDir() && path == goGitDir {
			return filepath.SkipDir
		}

		// Skip hidden directories (starting with .)
		if d.IsDir() && filepath.Base(path)[0] == '.' && path != repoPath {
			return filepath.SkipDir
		}

		// Collect regular files only
		if d.Type().IsRegular() {
			relPath, err := filepath.Rel(repoPath, path)
			if err != nil {
				return fmt.Errorf("failed to compute relative path for %s: %w", path, err)
			}
			filePaths = append(filePaths, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk repository: %w", err)
	}

	return filePaths, nil
}

// addFile stages single file by creating blob and updating index.
func addFile(cmd *cobra.Command, repoPath, filePath string, idx *index.Index, store *objects.ObjectStore) error {
	// Get absolute path
	absolutePath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Verify file exists
	fileInfo, err := os.Stat(absolutePath)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", absolutePath, err)
	}
	if fileInfo.IsDir() {
		return fmt.Errorf("cannot add directory (not yet implemented)")
	}

	// Compute relative path from repository root
	absoluteRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute repo path: %w", err)
	}

	relativeFilePath, err := filepath.Rel(absoluteRepoPath, absolutePath)
	if err != nil {
		return fmt.Errorf("file is not inside the repository: %w", err)
	}

	// Create blob from file
	blob, err := objects.NewBlobFromFile(absolutePath)
	if err != nil {
		return fmt.Errorf("failed to create blob from file %s: %w", absolutePath, err)
	}

	// Store Blob in objects/
	if err := store.Store(blob); err != nil {
		return fmt.Errorf("failed to strore file blob: %w", err)
	}

	// Determine file mode
	fileMode := detectFileMode(fileInfo)

	// create index entry
	entry, err := index.NewEntry(
		fileMode,
		blob.Hash(),
		relativeFilePath,
		fileInfo.Size(),
		fileInfo.ModTime().Truncate(time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to create index entry for %s: %w", absolutePath, err)
	}

	// Add to index
	if err := idx.AddEntry(entry); err != nil {
		return fmt.Errorf("failed to add [%s] entry to index: %w", absolutePath, err)
	}

	cmd.Printf("add '%s'\n", relativeFilePath)
	return nil
}

// detectFileMode converts os.FileInfo mode to Git index FileMode.
func detectFileMode(info os.FileInfo) index.FileMode {
	mode := info.Mode()

	if mode.IsDir() {
		return index.ModeDirectory
	}

	if mode&os.ModeSymlink != 0 {
		return index.ModeSymlink
	}

	// Check if file is regular (standard file with no special type bits set)
	if mode.IsRegular() {
		// Extract permission bits and check execute flags across user/group/other
		// Non-zero result means at least one execute bit is set
		if mode.Perm()&ExecutableFileMask != 0 {
			return index.ModeExecutable
		}
		return index.ModeRegularFile
	}

	return index.ModeRegularFile
}
