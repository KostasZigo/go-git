package cmd

import "github.com/spf13/cobra"

var checkoutCmd = &cobra.Command{
	Use:   "checkout <target>",
	Short: "Checkout to a given target",
	Long: `Checkout to a given target.
	Directories are hashed and stored as tree objects, files and sub-directories are added as tree entries.
	A commit object is also hashed and stored as stored as commit object. The current refs/heads/<branch-name> file mentioned in 
	HEAD file is updated with the commit's hash.

Examples:
  # Checkout with checking if working directory is dirty
  gogit checkout target

  # Commit with without checking whether working directory is clean - force
  gogit checkout -f target
  gogit checkout --force target`,

	SilenceUsage: true,
	Args:         exactArgs(1),
	RunE:         runCheckout,
}

func init() {
	rootCmd.AddCommand(checkoutCmd)
}

func runCheckout(cmd *cobra.Command, args []string) error {

	return nil
}
