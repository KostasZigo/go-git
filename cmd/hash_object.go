package cmd

import (
	"fmt"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/utils"
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
	Args:         exactArgs(1, constants.HashObjectCmdName),
	RunE:         runHashObject,
}

var writeFlag bool

func init() {
	rootCmd.AddCommand(hashObjectCmd)

	// Add flag using Cobra's flag system
	hashObjectCmd.Flags().BoolVarP(&writeFlag, "write", "w", false, "Write the object into the objects folder")
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
		repoPath, err := utils.FindRepoRoot()
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
