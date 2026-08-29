// hyperray version: print which build this is. Release builds stamp the number
// in at compile time; a source build says dev.
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the hyperray version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "hyperray "+version)
		},
	}
}
