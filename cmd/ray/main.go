// Command ray verifies that a coding task's spec, tests, and solution are
// internally consistent, using formal methods layered with testing.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "ray",
		Short:         "Prove statement, solution, and tests agree over a bounded scope",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(newCheckCmd())
	root.AddCommand(newEnforceCmd())
	root.AddCommand(newSpecInitCmd())
	root.AddCommand(newOneshotCmd())
	root.AddCommand(newDepHarvestCmd())
	root.AddCommand(newRowsCmd())
	root.AddCommand(newBridgesGenCmd())
	root.AddCommand(newHygieneCmd())
	root.AddCommand(newRepoLintCmd())
	root.AddCommand(newProseLintCmd())
	root.AddCommand(newStartCmd())
	root.AddCommand(newStrictSpecLintCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newUpdateCmd())
	return root
}
