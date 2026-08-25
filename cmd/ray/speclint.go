package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSpecLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "spec-lint <spec.md>",
		Short: "Check a spec.md's condition tables for completeness and disjointness",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("spec-lint: not yet implemented")
		},
	}
}
