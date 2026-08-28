// Command surface for the repo-lint gate: runs every linter the host
// repository configures against the solution files and turns the engine's
// results into terminal output and an exit code.
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
		Short:        "Run the host repository's own configured linters on the solution files",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sourceRoot == "" || len(solutionFiles) == 0 {
				return fmt.Errorf("repo-lint: needs --source-root and --solution-file")
			}
			results := repolint.Check(sourceRoot, solutionFiles)
			out := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintln(out, "repo-lint: the repository configures no supported linter for these files; no gate to enforce")
				return nil
			}
			failed := 0
			for _, result := range results {
				switch result.Status {
				case repolint.StatusBlocked:
					failed++
					fmt.Fprintf(out, "repo-lint: %s blocked -- %s\n", result.Tool, result.Output)
				case repolint.StatusFindings:
					failed++
					fmt.Fprintf(out, "repo-lint: %s rejects the solution:\n%s\n", result.Tool, result.Output)
				default:
					fmt.Fprintf(out, "repo-lint: %s clean on %d solution file(s)\n", result.Tool, len(solutionFiles))
				}
			}
			if failed > 0 {
				return fmt.Errorf("repo-lint: %d of %d configured gate(s) did not pass", failed, len(results))
			}
			return nil
		},
	}
	command.Flags().StringVar(&sourceRoot, "source-root", "", "the applied source tree holding the repository's lint config")
	command.Flags().StringSliceVar(&solutionFiles, "solution-file", nil, "solution file, relative to --source-root (repeatable)")
	return command
}
