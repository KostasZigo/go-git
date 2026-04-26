package cmd

import (
	"fmt"

	"github.com/KostasZigo/gogit/internal/commits"
	"github.com/KostasZigo/gogit/internal/utils"
	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "log history of commits",
	Long: `Display the commit history starting from the current branch HEAD.
	Walks the parent chain from the latest commit to the root, printing
	each commit's hash, message, author, and date in reverse chronological order.

Examples:
  # Show full commit history
  gogit log`,

	SilenceUsage: true,
	Args:         cobra.NoArgs,
	RunE:         runLog,
}

func init() {
	rootCmd.AddCommand(logCmd)
}

// runLog resolves the repository root, delegates to OrchestrateLogExecution
// to collect and format the commit history, and prints the result to stdout.
func runLog(cmd *cobra.Command, _ []string) error {
	// Find repository root path
	repoPath, err := utils.FindRepoRoot()
	if err != nil {
		return err
	}

	logOutput, err := commits.OrchestrateLogExecution(repoPath)
	if err != nil {
		return fmt.Errorf("log command failed: %w", err)
	}
	cmd.Print(logOutput)

	return nil
}
