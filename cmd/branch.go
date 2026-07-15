package cmd

import (
	"strings"

	"github.com/KostasZigo/gogit/internal/branches"
	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/repository"
	"github.com/spf13/cobra"
)

var branchCmd = &cobra.Command{
	Use:   "branch <name>",
	Short: "create new branch",
	Long: `Create a new branch pointing to the current HEAD commit.

Reads the commit hash that HEAD currently references (either directly or
through a symbolic ref) and writes a new file under refs/heads/<name>
containing that hash. Fails if a branch with the given name already exists.

Examples:
  # Create a new branch from current HEAD
  gogit branch feature-login`,

	SilenceUsage: true,
	Args:         exactArgs(1, constants.BranchCmdName),
	RunE:         runBranch,
}

func init() {
	rootCmd.AddCommand(branchCmd)
}

// runBranch locates the repository root and delegates branch creation to the branches service.
func runBranch(cmd *cobra.Command, args []string) error {
	branchName := strings.TrimSpace(args[0])

	// Find repository root path
	repoPath, err := repository.FindRoot()
	if err != nil {
		return err
	}

	if err := branches.OrchestrateBranchCreation(repoPath, branchName); err != nil {
		return err
	}

	cmd.Printf("created branch [%s]\n", branchName)
	return nil
}
