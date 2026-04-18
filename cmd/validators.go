package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// minimumArgs validates command receives at least n positional arguments.
// Returns error with usage help if argument limit exceeded.
func minimumArgs(n int, cmdName string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < n {
			cmd.SilenceUsage = false
			return fmt.Errorf("%s command accepts at least %d arg(s), received %d", cmdName, n, len(args))
		}
		return nil
	}
}

// maximumArgs validates command receives at most n positional arguments.
// Returns error with usage help if argument limit exceeded.
func maximumArgs(n int, cmdName string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > n {
			cmd.SilenceUsage = false
			return fmt.Errorf("%s command accepts at most %d arg(s), received %d", cmdName, n, len(args))
		}
		return nil
	}
}

// exactArgs validates command receives exactly n positional arguments.
// Returns error with usage help if argument count does not match.
func exactArgs(n int, cmdName string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			cmd.SilenceUsage = false
			return fmt.Errorf("%s command requires exactly %d argument (filepath), received %d", cmdName, n, len(args))
		}
		return nil
	}
}
