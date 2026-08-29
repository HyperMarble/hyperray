package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/HyperMarble/hyperray/internal/coverage"
	"github.com/HyperMarble/hyperray/internal/enforce"
	"github.com/HyperMarble/hyperray/internal/mutate"
	"github.com/HyperMarble/hyperray/internal/specparser"
)

// newEnforceCmd asks the grader-soundness question for every expanded spec
// combination: can a solution that violates this requirement still pass the
// task's tests? The engine answers by running, not reading -- it applies the
// authored violation, proves the behaviour changed with the witness, then
// runs the task's own verifier. A passing verifier is a proven false
// positive; a red verifier counts as enforcement only when the declared test
// is what failed.
func newEnforceCmd() *cobra.Command {
	var obligationsPath string
	var applyFixes bool
	var discover bool
	var failToPass bool
	var baseRoot, testListCommand, oneTestCommand string
	var sourceRoot, testCommand, probeRunner, pythonPath string
	var solutionFiles []string
	command := &cobra.Command{
		Use:          "enforce <task-dir>",
		Hidden:       true,
		Short:        "Prove or refute that the task's tests enforce each spec obligation",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			taskDir, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			// Discovery mode is the fully automatic path the architecture
			// asks for: the human authors spec.md, and hyperray derives the wrong
			// solutions itself -- mechanical mutants of the reference --
			// proves each one deviates using the spec's bridge scripts as
			// probes, and asks the task's own verifier about it. A deviant
			// the verifier accepts is a proven false positive no one wrote
			// by hand.
			if failToPass {
				return runFailToPass(cmd, baseRoot, testListCommand, oneTestCommand)
			}
			if discover {
				return runDiscovery(cmd, taskDir, sourceRoot, testCommand, probeRunner, pythonPath, solutionFiles)
			}
			if obligationsPath == "" {
				obligationsPath = filepath.Join(taskDir, "obligations.json")
			}
			spec, err := enforce.LoadSpec(obligationsPath, taskDir)
			if err != nil {
				return fmt.Errorf("enforce: %w", err)
			}
			specSource, err := os.ReadFile(filepath.Join(taskDir, "spec.md"))
			if err != nil {
				return fmt.Errorf("enforce: %w", err)
			}
			tables, err := specparser.Parse(string(specSource))
			if err != nil {
				return fmt.Errorf("enforce: parse spec: %w", err)
			}
			covs, err := coverage.Generate(tables, "", 0)
			if err != nil {
				return fmt.Errorf("enforce: expand spec: %w", err)
			}
			obligations := enforce.Build(tables, covs, spec)
			if len(obligations) == 0 {
				return fmt.Errorf("enforce: the spec expanded to no obligations")
			}
			results, err := enforce.CheckAll(spec.Task, obligations)
			if err != nil {
				return err
			}
			counts := map[enforce.Verdict]int{}
			var falsePositives []enforce.Result
			for _, result := range results {
				counts[result.Verdict]++
				fmt.Fprintln(cmd.OutOrStdout(), result.String())
				if result.Verdict == enforce.FalsePositive {
					falsePositives = append(falsePositives, result)
				}
			}
			// --fix renders the test each proven false positive implies and
			// appends it under a marker naming hyperray. A rendered test is
			// derivation from the frozen spec row -- exactly as right as the
			// row, no more -- so every generated test is printed for the
			// author to review, and the loop reruns to confirm the holes
			// closed. The one wrong-row case nothing mechanical flags is a
			// row and reference agreeing on the same misreading of the
			// contract; the marker keeps a human in that loop.
			if applyFixes && len(falsePositives) > 0 {
				if spec.Fix == nil {
					return fmt.Errorf("enforce: --fix needs a \"fix\" section (file, template) in obligations.json")
				}
				fixPath := filepath.Join(taskDir, spec.Fix.File)
				body := "\n\n# --- hyperray-generated tests: derived from frozen spec rows, review before keeping ---\n"
				for _, result := range falsePositives {
					rendered := spec.Fix.Render(result.Obligation)
					body += "\n" + rendered + "\n"
					fmt.Fprintf(cmd.OutOrStdout(), "\ngenerated for %s:\n%s\n", result.Obligation.Section, rendered)
				}
				handle, err := os.OpenFile(fixPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
				if err != nil {
					return fmt.Errorf("enforce: append fixes: %w", err)
				}
				if _, writeErr := handle.WriteString(body); writeErr != nil {
					handle.Close()
					return fmt.Errorf("enforce: append fixes: %w", writeErr)
				}
				handle.Close()
				fmt.Fprintf(cmd.OutOrStdout(), "appended %d generated test(s) to %s; rerunning\n\n", len(falsePositives), spec.Fix.File)
				results, err = enforce.CheckAll(spec.Task, obligations)
				if err != nil {
					return err
				}
				counts = map[enforce.Verdict]int{}
				for _, result := range results {
					counts[result.Verdict]++
					fmt.Fprintln(cmd.OutOrStdout(), result.String())
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nenforced: %d  false positives: %d  misdeclared: %d  inconclusive: %d\n",
				counts[enforce.Enforced], counts[enforce.FalsePositive], counts[enforce.Misdeclared], counts[enforce.Inconclusive])
			if counts[enforce.FalsePositive] > 0 {
				return fmt.Errorf("proven false positives: %d", counts[enforce.FalsePositive])
			}
			return nil
		},
	}
	command.Flags().BoolVar(&failToPass, "fail-to-pass", false, "run every test against the base tree (no solution); a test that passes there enforces nothing the solution adds")
	command.Flags().StringVar(&baseRoot, "base-root", "", "fail-to-pass: source tree at the base commit with only the tests applied")
	command.Flags().StringVar(&testListCommand, "test-list", "", "fail-to-pass: command printing one test name per line, run in --base-root")
	command.Flags().StringVar(&oneTestCommand, "one-test", "", "fail-to-pass: command template running a single test; {test} is substituted")
	command.Flags().BoolVar(&discover, "discover", false, "derive wrong solutions mechanically and report every one the verifier accepts")
	command.Flags().StringVar(&sourceRoot, "source-root", "", "discover: the applied source tree the verifier runs against")
	command.Flags().StringSliceVar(&solutionFiles, "solution-file", nil, "discover: solution file to mutate, relative to --source-root (repeatable)")
	command.Flags().StringVar(&testCommand, "test-command", "", "discover: the task's verifier command, run in --source-root")
	command.Flags().StringVar(&probeRunner, "probe-runner", "", "discover: command template run per bridge probe; {probe} is the script path")
	command.Flags().StringVar(&pythonPath, "python", "python3", "discover: interpreter for the mutant generator")
	command.Flags().BoolVar(&applyFixes, "fix", false, "append the test each proven false positive implies (from fix.template) and rerun")
	command.Flags().StringVar(&obligationsPath, "obligations", "", "path to obligations.json (default: <task-dir>/obligations.json)")
	return command
}

// runDiscovery mutates each solution file, keeps mutants the spec's bridge
// probes show to behave differently, and reports every deviant the task's
// verifier still accepts.
func runDiscovery(cmd *cobra.Command, taskDir, sourceRoot, testCommand, probeRunner, pythonPath string, solutionFiles []string) error {
	if sourceRoot == "" || testCommand == "" || probeRunner == "" || len(solutionFiles) == 0 {
		return fmt.Errorf("enforce --discover needs --source-root, --solution-file, --test-command, and --probe-runner")
	}
	probesDir := filepath.Join(taskDir, "bridges")
	entries, err := os.ReadDir(probesDir)
	if err != nil {
		return fmt.Errorf("enforce: the spec's bridges directory is the probe source: %w", err)
	}
	var probes []enforce.Probe
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "wit_") || name == "wit_common.py" {
			continue
		}
		probes = append(probes, enforce.Probe{
			Name:    strings.TrimSuffix(strings.TrimPrefix(name, "wit_"), filepath.Ext(name)),
			Command: strings.ReplaceAll(probeRunner, "{probe}", filepath.Join(probesDir, name)),
			Path:    filepath.Join(probesDir, name),
		})
	}
	if len(probes) == 0 {
		return fmt.Errorf("enforce: no wit_* probe scripts in %s", probesDir)
	}
	// One interpreter answers every probe per observation round; each probe
	// still runs in full. The driver ships with hyperray.
	batchDriver := filepath.Join(hyperrayRepoRoot(), "third_party", "mutate", "run_probes.py")
	task := enforce.Task{
		SourceRoot:        sourceRoot,
		TestCwd:           sourceRoot,
		TestCommand:       testCommand,
		ProbeBatchCommand: strings.ReplaceAll(probeRunner, "{probe}", batchDriver) + " {probes}",
		AfterWrite:        "find " + sourceRoot + " -name __pycache__ -exec rm -rf {} +",
		Timeout:           10 * time.Minute,
	}
	total := 0
	for _, solutionFile := range solutionFiles {
		mutants, err := mutate.Generate(pythonPath, filepath.Join(sourceRoot, solutionFile), "python")
		if err != nil {
			return fmt.Errorf("enforce: generate mutants for %s: %w", solutionFile, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %d candidate wrong solutions\n", solutionFile, len(mutants))
		found, err := enforce.Discover(task, solutionFile, mutants, probes)
		if err != nil {
			return err
		}
		for _, deviation := range found {
			if !deviation.Accepted {
				continue
			}
			total++
			fmt.Fprintf(cmd.OutOrStdout(), "\nFALSE POSITIVE %d -- %s:%d %s\n  probe %q: %q -> %q\n  %s\n",
				total, solutionFile, deviation.Mutant.Line, deviation.Mutant.Operator+": "+deviation.Mutant.Original+" -> "+deviation.Mutant.Mutated,
				deviation.Input, deviation.Before, deviation.After, deviation.Reason)
		}
	}
	if total > 0 {
		return fmt.Errorf("proven false positives: %d", total)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "\nno mutant shown to deviate was accepted by the verifier")
	return nil
}

// hyperrayRepoRoot resolves hyperray's own repository root from this binary's source
// location, the same trick mutate uses to find its generator.
func hyperrayRepoRoot() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
}

// runFailToPass is the cheapest check in the ladder and the one a stale
// test slips past everything else: a test that PASSES on the base tree --
// before the solution exists -- cannot be evidence for anything the
// solution adds, because the empty change already satisfies it. Every test
// must fail on base and pass on the solution; this runs the base half.
func runFailToPass(cmd *cobra.Command, baseRoot, testListCommand, oneTestCommand string) error {
	if baseRoot == "" || testListCommand == "" || oneTestCommand == "" {
		return fmt.Errorf("enforce --fail-to-pass needs --base-root, --test-list, and --one-test")
	}
	listCmd := exec.Command("sh", "-c", testListCommand)
	listCmd.Dir = baseRoot
	listOut, err := listCmd.Output()
	if err != nil {
		return fmt.Errorf("enforce: list tests: %w", err)
	}
	var passing []string
	total := 0
	for _, line := range strings.Split(strings.TrimSpace(string(listOut)), "\n") {
		test := strings.TrimSpace(line)
		if test == "" {
			continue
		}
		total++
		testCmd := exec.Command("sh", "-c", strings.ReplaceAll(oneTestCommand, "{test}", test))
		testCmd.Dir = baseRoot
		if testCmd.Run() == nil {
			passing = append(passing, test)
			fmt.Fprintf(cmd.OutOrStdout(), "PASSES ON BASE -- %s\n", test)
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n%d tests, %d pass without the solution\n", total, len(passing))
	if len(passing) > 0 {
		return fmt.Errorf("tests enforcing nothing new: %d", len(passing))
	}
	return nil
}
