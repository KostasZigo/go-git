package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/KostasZigo/gogit/internal/constants"
	"github.com/KostasZigo/gogit/internal/repository"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Initialize a new GoGit repository",
	Long: `The 'init' command sets up a new GoGit repository in the current directory.
It creates a .gogit directory and necessary configuration files, allowing you to start tracking your project's history.
If a repository already exists, the command will not overwrite existing data.`,
	SilenceUsage: true,
	Args:         maximumArgs(1, constants.InitCmdName),
	RunE:         runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
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

	cmd.Printf("Initialized empty GoGit repository in %s\n", buildDirPath(dirPath, constants.Gogit))
	return nil
}

// buildDirPath constructs an os-agnostic display path with a trailing separator.
func buildDirPath(dirs ...string) string {
	return strings.Join(dirs, string(filepath.Separator)) + string(filepath.Separator)
}
