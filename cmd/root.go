package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gogit",
	Short: "A simplified Git implementation in GO",
	Long: `GoGit is a simplified Git Implementation developed in GO that offers the main capabilites
	and features expected from a Git project like init, add, commit etc.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
