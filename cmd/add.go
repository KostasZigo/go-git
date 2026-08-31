package cmd

import (
	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/repository"
	"github.com/KostasZigo/gogit/internal/staging"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <file>...",
	Short: "Add file to the staging area",
	Long: `Stage selected working-tree changes in the index.

Explicit paths limit staging to those paths and any indexed ancestor or
descendant conflicts required to represent them. A sole "." reconciles the
complete repository with the index, including tracked deletions and
file/directory transitions. Regular files are stored as blob objects;
unsupported filesystem objects are rejected.

Examples:
  # Stage single file
  gogit add README.md

  # Stage multiple files
  gogit add main.go utils.go

	# Stage all repository changes
  gogit add .`,
	SilenceUsage: true,
	Args:         minimumArgs(1, constants.AddCmdName),
	RunE:         runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

// runAdd stages selected additions, modifications, and deletions and reports
// each changed index path in deterministic order.
func runAdd(cmd *cobra.Command, args []string) error {
	// Find repository root path
	repoPath, err := repository.FindRoot()
	if err != nil {
		return err
	}

	addedFiles, deletedFiles, err := staging.OrchestrateAddExecution(repoPath, args)
	if err != nil {
		return err
	}

	for _, file := range deletedFiles {
		cmd.Printf("deleted '%s'\n", file)
	}
	for _, file := range addedFiles {
		cmd.Printf("add '%s'\n", file)
	}

	return nil
}
