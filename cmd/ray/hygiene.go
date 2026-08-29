package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/HyperMarble/ray/internal/runner"
)

// newHygieneCmd checks the verifier is an instrument: the same tree must
// yield the same verdict on every run and in every order. A test that flakes
// or depends on its neighbours poisons every other verdict ray produces, so
// this runs before any per-row work is trusted.
func newHygieneCmd() *cobra.Command {
	var sourceRoot, testCommand, pythonPath, testFile, language string
	command := &cobra.Command{
		Use:          "hygiene <task-dir>",
		Hidden:       true,
		Short:        "Same tree, same verdict: run twice and in reverse order; name any test that differs",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			frameworkRunner, err := runner.New(language, pythonPath, testFile, testCommand)
			if err != nil {
				return err
			}
			runOnce := func(command string) (bool, []string) {
				sub := exec.Command("sh", "-c", command)
				sub.Dir = sourceRoot
				output, runErr := sub.CombinedOutput()
				return runErr == nil, frameworkRunner.FailedNames(string(output))
			}
			firstPass, firstFailed := runOnce(frameworkRunner.SuiteCommand())
			secondPass, secondFailed := runOnce(frameworkRunner.SuiteCommand())
			if firstPass != secondPass || strings.Join(firstFailed, ",") != strings.Join(secondFailed, ",") {
				fmt.Fprintf(out, "FLAKY: two identical runs disagree (run1 failed=%v, run2 failed=%v)\n", firstFailed, secondFailed)
				return fmt.Errorf("hygiene: flaky verifier")
			}
			collect := exec.Command("sh", "-c", frameworkRunner.ListCommand())
			collect.Dir = sourceRoot
			listing, err := collect.Output()
			if err != nil {
				return fmt.Errorf("hygiene: collect tests: %w", err)
			}
			// One id per line: parametrized ids contain spaces, so
			// whitespace-splitting shreds them into nonsense arguments.
			var ids []string
			for _, line := range strings.Split(strings.TrimSpace(string(listing)), "\n") {
				if line = strings.TrimSpace(line); line != "" {
					ids = append(ids, line)
				}
			}
			for left, right := 0, len(ids)-1; left < right; left, right = left+1, right-1 {
				ids[left], ids[right] = ids[right], ids[left]
			}
			reversedPass, reversedFailed := runOnce(frameworkRunner.OrderedCommand(ids))
			if reversedPass != firstPass || strings.Join(reversedFailed, ",") != strings.Join(firstFailed, ",") {
				fmt.Fprintf(out, "ORDER-DEPENDENT: reversed order disagrees (normal failed=%v, reversed failed=%v)\n", firstFailed, reversedFailed)
				return fmt.Errorf("hygiene: order-dependent verifier")
			}
			fmt.Fprintf(out, "hygiene: %d tests -- stable across repeat and reverse order\n", len(ids))
			return nil
		},
	}
	command.Flags().StringVar(&sourceRoot, "source-root", "", "tree the tests run in")
	command.Flags().StringVar(&testCommand, "test-command", "", "the verifier command")
	command.Flags().StringVar(&pythonPath, "python", "python3", "interpreter (python tasks)")
	command.Flags().StringVar(&testFile, "test-file", "", "test file for ordered collection (python tasks)")
	command.Flags().StringVar(&language, "language", "python", "task language: python, rust, or cpp")
	return command
}
