package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/spf13/cobra"
)

var hashObjectCmd = &cobra.Command{
	Use:   "hash-object <filepath>",
	Short: "Compute object hash and optionally create and store a blob from a file",
	Long: `Compute the object hash (SHA-1 hash) for a file's content.
Optionally write the resulting object's blob into the objects folder.

Examples:
  # Compute hash without storing
  gogit hash-object myfile.txt

  # Compute hash and store in .gogit/objects
  gogit hash-object -w myfile.txt`,
	SilenceUsage: true,
	Args:         exactArgs(1),
	RunE:         runHashObject,
}

var writeFlag bool

func init() {
	rootCmd.AddCommand(hashObjectCmd)

	// Add flag using Cobra's flag system
	hashObjectCmd.Flags().BoolVarP(&writeFlag, "write", "w", false, "Write the object into the objects folder")
}

// exactArgs validates command receives exactly n positional arguments.
// enables usage printing in case of error
func exactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			cmd.SilenceUsage = false
			return fmt.Errorf("hash-object command requires exactly %d argument (filepath), received %d", n, len(args))
		}
		return nil
	}
}

// runHashObject computes hash and optionally stores blob object.
func runHashObject(cmd *cobra.Command, args []string) error {
	// Create blob from file's contents]
	blob, err := objects.NewBlobFromFile(args[0])
	if err != nil {
		return err
	}

	// Print hash to stdout
	fmt.Fprintln(cmd.OutOrStdout(), blob.Hash())

	if writeFlag {
		repoPath, err := findRepoRoot()
		if err != nil {
			return err
		}

		store := objects.NewObjectStore(repoPath)
		if err := store.Store(blob); err != nil {
			return fmt.Errorf("failed to store object: %w", err)
		}
	}

	return nil
}

// findRepoRoot locates .gogit directory by walking up directory tree.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		gogitPath := filepath.Join(dir, constants.Gogit)
		if info, err := os.Stat(gogitPath); err == nil && info.IsDir() {
			return dir, nil
		}

		// Dir returns all but the last element of path
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding .gogit
			return "", fmt.Errorf("%s directory not found", constants.Gogit)
		}
		dir = parent
	}
}
