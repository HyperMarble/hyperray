// Command surface for the prose gate: checks the task's problem statement
// against the platform's own rejections (non-ASCII), the style guide's word
// budget, and promise-word coverage against the compiled spec's anchors.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/HyperMarble/ray/internal/proselint"
)

const proseWordBudget = 500

func newProseLintCmd() *cobra.Command {
	command := &cobra.Command{
		Use:          "prose-lint <task-dir>",
		Hidden:       true,
		Short:        "Check the problem statement: ASCII, word budget, and promise-word coverage by spec rows",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			taskDir, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			body, err := os.ReadFile(filepath.Join(taskDir, "instruction.md"))
			if err != nil {
				return fmt.Errorf("prose-lint: read instruction.md: %w", err)
			}
			text := string(body)
			out := cmd.OutOrStdout()
			failed := false

			if position, char, found := proselint.NonASCII(text); found {
				failed = true
				fmt.Fprintf(out, "prose-lint: non-ASCII character %q at byte %d -- the platform rejects it\n", char, position)
			}

			if words := proselint.CountWords(text); words > proseWordBudget {
				fmt.Fprintf(out, "prose-lint: %d words exceeds the style guide budget of %d (warning)\n", words, proseWordBudget)
			}

			task, err := compileTaskDir(taskDir)
			if err != nil {
				return fmt.Errorf("prose-lint: spec must compile before coverage can be checked: %w", err)
			}
			rowsPerLine := map[int]int{}
			for _, requirement := range task.Requirements {
				for _, source := range requirement.InstructionSources {
					location := source.Location
					end := location.EndLine
					if end < location.StartLine {
						end = location.StartLine
					}
					for line := location.StartLine; line <= end; line++ {
						rowsPerLine[line]++
					}
				}
			}
			lines := proselint.PromiseLines(text, rowsPerLine)
			for _, line := range lines {
				fmt.Fprintln(out, "prose-lint: "+proselint.Describe(line))
			}
			uncovered := proselint.Uncovered(lines)
			for _, line := range uncovered {
				failed = true
				fmt.Fprintf(out, "prose-lint: UNCOVERED PROMISE -- %s\n", proselint.Describe(line))
			}

			if failed {
				return fmt.Errorf("prose-lint: the statement makes promises the spec does not carry")
			}
			fmt.Fprintf(out, "prose-lint: statement clean -- %d promise line(s), all anchored\n", len(lines))
			return nil
		},
	}
	return command
}
