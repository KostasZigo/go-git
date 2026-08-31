package cmd

import (
	"fmt"

	"github.com/KostasZigo/gogit/internal/commits"
	"github.com/KostasZigo/gogit/internal/objects"
	"github.com/KostasZigo/gogit/internal/repository"
	"github.com/spf13/cobra"
)

var commitCmd = &cobra.Command{
	Use:   "commit -m <message>",
	Short: "Record the staged snapshot",
	Long: `Record the current index as a commit on the active branch.

The staged snapshot is stored as recursive tree objects and linked to the
current commit as its parent. The commit is rejected when its tree is unchanged
from the parent or when an initial commit has an empty index. An empty index can
be committed after a non-empty parent to record deletion of all tracked files.

Examples:
  # Commit with message
  gogit commit -m "<message>"`,

	SilenceUsage: true,
	Args:         cobra.NoArgs,
	RunE:         runCommit,
}

var messageFlag string

func init() {
	rootCmd.AddCommand(commitCmd)

	commitCmd.Flags().StringVarP(&messageFlag, "message", "m", "", "commit message")
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

	// Author configuration is not implemented yet, so commits use the default
	// identity until repository or user configuration is introduced.
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
