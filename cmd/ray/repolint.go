// Command surface for the repo-lint gate: runs the host repository's own
// configured linter against the solution files and turns the engine's
// four-way result into terminal output and an exit code.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/HyperMarble/ray/internal/repolint"
)

func newRepoLintCmd() *cobra.Command {
	var sourceRoot string
	var solutionFiles []string
	command := &cobra.Command{
		Use:          "repo-lint",
		Hidden:       true,
		Short:        "Run the host repository's own configured linter on the solution files",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sourceRoot == "" || len(solutionFiles) == 0 {
				return fmt.Errorf("repo-lint: needs --source-root and --solution-file")
			}
			result := repolint.Check(sourceRoot, solutionFiles)
			out := cmd.OutOrStdout()
			switch result.Status {
			case repolint.StatusNoConfig:
				fmt.Fprintln(out, "repo-lint: the repository configures no supported linter; no gate to enforce")
				return nil
			case repolint.StatusBlocked:
				return fmt.Errorf("repo-lint: blocked -- %s", result.Output)
			case repolint.StatusFindings:
				fmt.Fprintln(out, result.Output)
				return fmt.Errorf("repo-lint: the repository's own %s rejects the solution", result.Tool)
			default:
				fmt.Fprintf(out, "repo-lint: %s clean on %d solution file(s)\n", result.Tool, len(solutionFiles))
				return nil
			}
		},
	}
	command.Flags().StringVar(&sourceRoot, "source-root", "", "the applied source tree holding the repository's lint config")
	command.Flags().StringSliceVar(&solutionFiles, "solution-file", nil, "solution file, relative to --source-root (repeatable)")
	return command
}
