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
	Args:         minimumArgs(1, constants.AddCmdName),
	RunE:         runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

// runAdd stages files in index and creates blob objects.
func runAdd(cmd *cobra.Command, args []string) error {
	// Find repository root path
	repoPath, err := repository.FindRoot()
	if err != nil {
		return err
	}

	addedFiles, err := staging.OrchestrateAddExecution(repoPath, args)
	if err != nil {
		return err
	}

	for _, file := range addedFiles {
		cmd.Printf("add '%s'\n", file)
	}
	return nil
}
