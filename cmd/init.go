package cmd

import (
	"fmt"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/repository"
	"github.com/KostasZigo/gogit/utils"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Initialize a new GoGit repository",
	Long: `The 'init' command sets up a new GoGit repository in the current directory.
It creates a .gogit directory and necessary configuration files, allowing you to start tracking your project's history.
If a repository already exists, the command will not overwrite existing data.`,
	SilenceUsage: true,
	Args:         maximumArgs(1),
	RunE:         runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// maximumArgs validates command receives at most n positional arguments.
// Returns error with usage help if argument limit exceeded.
func maximumArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > n {
			cmd.SilenceUsage = false
			return fmt.Errorf("init command accepts at most %d arg(s), received %d", n, len(args))
		}
		return nil
	}
}

// runInit executes repository initialization at specified or current directory.
func runInit(cmd *cobra.Command, args []string) error {
	dirPath := "."
	if len(args) > 0 {
		dirPath = args[0]
	}

	if err := repository.InitRepository(dirPath); err != nil {
		return fmt.Errorf("failed to initialize repository - %w", err)
	}

	cmd.Printf("Initialized empty GoGit repository in %s\n", utils.BuildDirPath(dirPath, constants.Gogit))
	return nil
}
