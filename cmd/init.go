package cmd

import (
	// "fmt"
	"fmt"

	"github.com/KostasZigo/gogit/internal/repository"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new GoGit repository",
	Long: `The 'init' command sets up a new GoGit repository in the current directory.
It creates a .gogit directory and necessary configuration files, allowing you to start tracking your project's history.
If a repository already exists, the command will not overwrite existing data.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			fmt.Println("usage: gogit init [<directory>]")
			return nil
		}

		filepath := "."
		if len(args) > 0 {
			filepath = args[0]
		}

		if err := repository.InitRepository(filepath); err != nil {
			return fmt.Errorf("failed to initialize repository: %w", err)
		}

		fmt.Printf("Initialized empty GoGit repository in %s/.gogit/\n", filepath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
