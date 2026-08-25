// Command ray verifies that a coding task's spec, tests, and solution are
// internally consistent, using formal methods layered with testing.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "ray",
		Short: "Verify that an agent's (or human's) code logic is actually correct",
	}
	root.AddCommand(newSpecLintCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
