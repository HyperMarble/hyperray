package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/HyperMarble/ray/internal/repolint"
	"github.com/HyperMarble/ray/internal/runner"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

// oneshotConfig is plumbing metadata only -- where things are and how to run
// them. It carries no judgement: no requirement, outcome, or verdict can be
// expressed here. Judgement lives in spec.md alone.
type oneshotConfig struct {
	Oneshot struct {
		Python        string   `toml:"python"`
		SourceRoot    string   `toml:"source_root"`
		TestFile      string   `toml:"test_file"`
		TestCommand   string   `toml:"test_command"`
		SolutionFiles []string `toml:"solution_files"`
		Language      string   `toml:"language"`
		// Container, when set, runs every command inside this running
		// docker container instead of on the host: the task's frozen box.
		// source_root then names the path inside the container.
		Container string `toml:"container"`
		// RequiredTestFiles are files the test patch must deliver -- the
		// canonical runner above all. A regenerated patch that drops one
		// fails here instead of inside the platform's container.
		RequiredTestFiles []string `toml:"required_test_files"`
		// FastKill makes the rows rung stop each breaker's suite at the
		// first failing test: same verdicts, far fewer test executions.
		FastKill bool `toml:"fast_kill"`
	} `toml:"oneshot"`
}

// newOneshotCmd chains the authoring-time ladder into one command with one
// exit code: strict spec compilation, the fail-to-pass check (every test
// must fail without the solution), and mechanical false-positive discovery.
// Any rung failing fails the run; a rung that cannot run is reported as
// blocked rather than silently skipped.
func newOneshotCmd() *cobra.Command {
	command := &cobra.Command{
		Use:          "verify <task-dir>",
		Aliases:      []string{"oneshot"},
		Short:        "Run the whole authoring ladder on one task: spec-lint, fail-to-pass, discovery",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			taskDir, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			var cfg oneshotConfig
			raw, err := os.ReadFile(filepath.Join(taskDir, "ray.toml"))
			if err != nil {
				return fmt.Errorf("oneshot: read ray.toml: %w", err)
			}
			if err := toml.Unmarshal(raw, &cfg); err != nil {
				return fmt.Errorf("oneshot: parse ray.toml: %w", err)
			}
			c := cfg.Oneshot
			if c.Python == "" {
				c.Python = "python3"
			}
			// The frozen box: wrap every command in docker exec. The same
			// four rungs run unchanged; only where they run moves.
			if c.Container != "" {
				c.TestCommand = "docker exec -w " + c.SourceRoot + " " + c.Container + " " + c.TestCommand
				c.Python = "docker exec -w " + c.SourceRoot + " " + c.Container + " " + c.Python
			}
			out := cmd.OutOrStdout()
			self, err := os.Executable()
			if err != nil {
				return err
			}
			run := func(name string, extra ...string) error {
				fmt.Fprintf(out, "== %s\n", name)
				sub := exec.Command(self, extra...)
				sub.Stdout = out
				sub.Stderr = out
				return sub.Run()
			}

			// Rung 0: generate any probe files the spec implies but the task
			// lacks; a generated-but-unfilled probe keeps its rules "untried".
			_ = run("probes", "bridges-gen", taskDir)

			// Rung 1: the spec is the axiom; nothing runs on a broken axiom.
			reference := filepath.Join(taskDir, "solution.patch")
			lintArgs := []string{"spec-lint", filepath.Join(taskDir, "spec.md"),
				"--instruction", filepath.Join(taskDir, "instruction.md"),
				"--task-id", filepath.Base(taskDir)}
			if _, err := os.Stat(reference); err == nil {
				lintArgs = append(lintArgs, "--reference", reference)
			}
			if err := run("spec-lint", lintArgs...); err != nil {
				return fmt.Errorf("oneshot: spec-lint failed")
			}

			// Rung 1.1: bundle completeness. The container's canonical
			// runner ships inside the test patch; a patch that lost it
			// fails every verify remotely and nothing locally.
			if len(c.RequiredTestFiles) > 0 {
				patchBytes, err := os.ReadFile(filepath.Join(taskDir, "test.patch"))
				if err != nil {
					return fmt.Errorf("oneshot: required_test_files declared but test.patch unreadable: %w", err)
				}
				if missing := repolint.MissingBundleFiles(string(patchBytes), c.RequiredTestFiles); len(missing) > 0 {
					return fmt.Errorf("oneshot: test.patch does not deliver required file(s): %s", strings.Join(missing, ", "))
				}
				fmt.Fprintf(out, "bundle: test.patch delivers all %d required file(s)\n", len(c.RequiredTestFiles))
			}

			// Rung 1.25: the statement itself. Non-ASCII bytes bounce at the
			// platform, and a promise-bearing line with no spec row anchored
			// to it is behavior nothing downstream will check.
			if err := run("prose-lint", "prose-lint", taskDir); err != nil {
				return fmt.Errorf("oneshot: prose lint failed")
			}

			// Rung 1.5: the host repository's own lint gate. Review bounces a
			// solution that violates the repo's configured linter even when
			// every test passes, so the gate runs here with the repo's own
			// tool and config. Container mode is warned about, not failed:
			// the lint config lives in the source tree, which container
			// tasks expose only inside the box.
			if c.Container != "" {
				fmt.Fprintln(out, "repo-lint: not run in container mode -- run it on a host checkout")
			} else if c.SourceRoot != "" && len(c.SolutionFiles) > 0 {
				repoLintArgs := []string{"repo-lint", "--source-root", c.SourceRoot}
				for _, file := range c.SolutionFiles {
					repoLintArgs = append(repoLintArgs, "--solution-file", file)
				}
				if err := run("repo-lint", repoLintArgs...); err != nil {
					return fmt.Errorf("oneshot: repo lint gate failed")
				}
			}

			// Rung 2: every test must fail without the solution. Needs a git
			// tree so the solution can be stashed and restored; without one
			// the rung is blocked, never silently skipped.
			if c.SourceRoot == "" || c.TestCommand == "" || c.TestFile == "" {
				return fmt.Errorf("oneshot: blocked -- ray.toml needs source_root, test_command, test_file for fail-to-pass and discovery")
			}
			if _, err := os.Stat(filepath.Join(c.SourceRoot, ".git")); err != nil {
				return fmt.Errorf("oneshot: blocked -- fail-to-pass needs source_root to be a git tree so the solution can be stashed and restored")
			}
			stash := exec.Command("git", "stash", "--quiet")
			stash.Dir = c.SourceRoot
			if err := stash.Run(); err != nil {
				return fmt.Errorf("oneshot: stash solution: %w", err)
			}
			frameworkRunner, runnerErr := runner.New(c.Language, c.Python, c.TestFile, c.TestCommand)
			if runnerErr != nil {
				return runnerErr
			}
			testList := frameworkRunner.ListCommand()
			if frameworkRunner.Language() == "python" {
				testList += " | sed 's/.*:://' | sort -u"
			}
			failToPassErr := run("fail-to-pass",
				"enforce", taskDir, "--fail-to-pass",
				"--base-root", c.SourceRoot,
				"--test-list", testList,
				"--one-test", frameworkRunner.OneTestCommand("{test}"))
			restore := exec.Command("git", "stash", "pop", "--quiet")
			restore.Dir = c.SourceRoot
			if err := restore.Run(); err != nil {
				return fmt.Errorf("oneshot: RESTORE FAILED after fail-to-pass; the tree may be missing the solution: %w", err)
			}
			if failToPassErr != nil {
				return fmt.Errorf("oneshot: fail-to-pass failed")
			}

			// Discovery is subsumed by the rows rung: the same mutants run
			// once, attributed where a probe owns them and reported as
			// file-level findings where none does. One sweep, not two.

			// Rung 3.5: test hygiene. A verifier must be an instrument:
			// same tree, same verdict. Run twice (flakiness), then run the
			// tests in reverse order (order dependence). Differences are
			// named tests, not vibes.
			if err := run("hygiene", "hygiene", taskDir,
				"--source-root", c.SourceRoot,
				"--test-command", c.TestCommand,
				"--python", c.Python,
				"--test-file", c.TestFile,
				"--language", c.Language); err != nil {
				return fmt.Errorf("oneshot: test hygiene failed")
			}

			// Rung 4: per-row -- oracle-lite on the reference, then a derived
			// breaker per rule, guards named, boundary listed.
			probeRunner := c.Python + " {probe}"
			if c.Language == "rust" || c.Language == "cpp" {
				probeRunner = "sh {probe}"
			}
			rowsArgs := []string{"rows", taskDir,
				"--source-root", c.SourceRoot,
				"--test-command", c.TestCommand,
				"--probe-runner", probeRunner,
				"--python", c.Python}
			if c.FastKill {
				rowsArgs = append(rowsArgs, "--fast-kill")
			}
			if c.Language != "" {
				rowsArgs = append(rowsArgs, "--language", c.Language)
			}
			for _, file := range c.SolutionFiles {
				rowsArgs = append(rowsArgs, "--solution-file", file)
			}
			if err := run("rows", rowsArgs...); err != nil {
				return fmt.Errorf("oneshot: per-row verification failed")
			}

			fmt.Fprintf(out, "\nONESHOT PASS: spec compiles, every test requires the solution, no constructed wrong solution was accepted, every derivable row is guarded\n")
			return nil
		},
	}
	return command
}
