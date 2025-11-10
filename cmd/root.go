package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd defines the base command for the gogit CLI.
// All subcommands (init, add, commit, etc.) register under this root.
// Uses cobra for command parsing, flag handling, and help generation.
var rootCmd = &cobra.Command{
	Use:   "gogit",
	Short: "A simplified Git implementation in GO",
	Long: `GoGit is a simplified Git Implementation developed in GO that offers the main capabilites
	and features expected from a Git project like init, add, commit etc.`,
}

// Execute runs the root command and handles exit codes.
// Called from main.go to start CLI execution.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
