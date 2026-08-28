package cmd

import (
	"github.com/KostasZigo/gogit/internal/commits"
	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/repository"
	"github.com/spf13/cobra"
)

var checkoutCmd = &cobra.Command{
	Use:   "checkout <target>",
	Short: "Switch to a branch or commit",
	Long: `Switch the working directory to the state of a given branch or commit.

Resolves the target as a branch name first (under refs/heads/), then as a
direct commit hash. Staged or tracked worktree changes block checkout unless
--force is set; collisions with untracked content always block it. On success,
applies the target snapshot to the worktree and index, then updates HEAD.

Examples:
  # Checkout a branch
  gogit checkout feature-branch

  # Checkout a specific commit (detached HEAD)
  gogit checkout abc123...

  # Force checkout, discarding uncommitted changes
  gogit checkout -f main
  gogit checkout --force main`,

	SilenceUsage: true,
	Args:         exactArgs(1, constants.CheckoutCmdName),
	RunE:         runCheckout,
}

// forceFlag permits staged and tracked worktree changes to be discarded.
var forceFlag bool

func init() {
	checkoutCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "discard staged and tracked worktree changes")
	rootCmd.AddCommand(checkoutCmd)
}

// runCheckout resolves the repository and delegates to the checkout orchestrator.
func runCheckout(cmd *cobra.Command, args []string) error {
	// Find repository root path
	repoPath, err := repository.FindRoot()
	if err != nil {
		return err
	}

	if err := commits.OrchestrateCheckoutExecution(repoPath, args[0], forceFlag); err != nil {
		return err
	}

	cmd.Printf("checked out [%s]\n", args[0])
	return nil
}
