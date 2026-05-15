package cmd

import (
	"fmt"

	"github.com/KostasZigo/gogit/internal/commits"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/repository"
	"github.com/spf13/cobra"
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Commit all staged files",
	Long: `Commit all staged files.
	Directories are hashed and stored as tree objects, files and sub-directories are added as tree entries.
	A commit object is also hashed and stored as stored as commit object. The current refs/heads/<branch-name> file mentioned in 
	HEAD file is updated with the commit's hash.

Examples:
  # Commit with no message
  gogit commit

  # Commit with message
  gogit commit -m "<message>"`,

	SilenceUsage: true,
	Args:         cobra.NoArgs,
	RunE:         runCommit,
}

var messageFlag string

func init() {
	rootCmd.AddCommand(commitCmd)

	// Add flag using Cobra's flag system
	commitCmd.Flags().StringVarP(&messageFlag, "message", "m", "", "Add message to commit")
}

func runCommit(cmd *cobra.Command, _ []string) error {
	if messageFlag == "" {
		return fmt.Errorf("commit message required: use -m \"your message\"")
	}

	// Find repository root path
	repoPath, err := repository.FindRoot()
	if err != nil {
		return err
	}

	// author should be resolved from git config
	// use *default* author for now. Git config author resolution to be implemented later
	author := objects.DefaultAuthor()
	commitHash, err := commits.OrchestrateCommitExecution(repoPath, messageFlag, author)
	if err != nil {
		return err
	}

	if len(commitHash) == 0 {
		return fmt.Errorf("commit succeeded but returned empty hash")
	}

	cmd.Printf("[%s] %s\n", commitHash[:7], messageFlag)
	return nil
}
