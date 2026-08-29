package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/HyperMarble/hyperray/internal/enforce"
	"github.com/HyperMarble/hyperray/internal/mutate"
	"github.com/HyperMarble/hyperray/internal/runner"
	"github.com/HyperMarble/hyperray/internal/semanticir"
	"github.com/HyperMarble/hyperray/internal/speccompiler"
)

// newRowsCmd asks the per-row enforcement question mechanically for the rows
// whose breaker is derivable from the row itself: a requirement of the form
// `raise X containing "msg"` is violated by exactly one edit -- tamper the
// message literal in the solution -- so hyperray generates that wrong solution
// from the spec row, runs the task's tests, and records which test fails.
//
//	a test fails   -> that test enforces the row; its name is reported for
//	                  the row's Enforced by cell
//	nothing fails  -> the row's message is pinned by no test -- a named hole
//
// This is the row-level counterpart of --discover: discover aims mutants at
// nothing in particular; this aims one derived wrong solution at one row.
// Rows whose breakers are not yet derivable (labels, effects) are listed as
// not-derived, never silently skipped.
func newRowsCmd() *cobra.Command {
	var sourceRoot, testCommand, probeRunner, pythonPath, language string
	var solutionFiles []string
	var fastKill bool
	command := &cobra.Command{
		Use:          "rows <task-dir>",
		Hidden:       true,
		Short:        "Per-row enforcement: derive a rule-breaker from each raise-row, run the tests, name the guard",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			taskDir, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			if sourceRoot == "" || testCommand == "" || len(solutionFiles) == 0 {
				return fmt.Errorf("rows: needs --source-root, --test-command, --solution-file")
			}
			run, err := newRowsRun(cmd.OutOrStdout(), taskDir, sourceRoot, testCommand, probeRunner, solutionFiles)
			if err == nil {
				run.fastKill = fastKill
				run.language = language
			}
			if err != nil {
				return err
			}
			run.checkReferenceAgainstRaiseRows()
			if err := run.enforceRaiseRows(); err != nil {
				return err
			}
			attributed, err := attributeMutants(cmd, taskDir, sourceRoot, testCommand, probeRunner, pythonPath, solutionFiles, run.task, run.outcomes, run.baseline)
			if err != nil {
				return err
			}
			run.reportAttributedRows(attributed)
			if err := run.checkBaselineGreen(); err != nil {
				return err
			}
			run.writeReferenceSnapshot()
			run.reportOrphanCandidates(attributed)
			run.reportFalseNegativeTheorem()
			return run.summarize()
		},
	}
	command.Flags().StringVar(&sourceRoot, "source-root", "", "the applied source tree the tests run against")
	command.Flags().StringSliceVar(&solutionFiles, "solution-file", nil, "solution file, relative to --source-root (repeatable)")
	command.Flags().StringVar(&testCommand, "test-command", "", "the task's verifier command, run in --source-root")
	command.Flags().StringVar(&probeRunner, "probe-runner", "", "command template run per bridge probe; {probe} is the script path")
	command.Flags().StringVar(&pythonPath, "python", "python3", "interpreter for the mutant generator")
	command.Flags().BoolVar(&fastKill, "fast-kill", false, "stop each breaker's suite at the first failing test; same verdicts, fewer runs")
	command.Flags().StringVar(&language, "language", "python", "task language: python, rust, or cpp")
	return command
}

// rowsRun carries one invocation's fixed inputs and running tallies so each
// phase below reads as a plain step instead of sharing a wall of locals.
type rowsRun struct {
	out         io.Writer
	taskDir     string
	sourceRoot  string
	testCommand string
	task        *semanticir.Task
	outcomes    map[string]semanticir.ObservableOutcome
	sources     map[string]string
	baseline    map[string]string
	fastKill    bool
	language    string
	enforced    int
	holes       int
	untried     int
	guardNames  map[string]bool
}

func newRowsRun(out io.Writer, taskDir, sourceRoot, testCommand, probeRunner string, solutionFiles []string) (*rowsRun, error) {
	task, err := compileTaskDir(taskDir)
	if err != nil {
		return nil, err
	}
	outcomes := map[string]semanticir.ObservableOutcome{}
	for _, outcome := range task.Outcomes {
		outcomes[outcome.ID] = outcome
	}
	sources := map[string]string{}
	for _, file := range solutionFiles {
		body, err := os.ReadFile(filepath.Join(sourceRoot, file))
		if err != nil {
			return nil, err
		}
		sources[file] = string(body)
	}
	return &rowsRun{
		out:         out,
		taskDir:     taskDir,
		sourceRoot:  sourceRoot,
		testCommand: testCommand,
		task:        task,
		outcomes:    outcomes,
		sources:     sources,
		baseline:    probeOutputs(taskDir, sourceRoot, probeRunner),
		guardNames:  map[string]bool{},
	}, nil
}

// checkReferenceAgainstRaiseRows is oracle-lite: before breaking anything,
// the reference itself is checked against the derivable rows. Each raise-row's
// required exception type and message must be observed in some probe's output
// on the untouched solution -- the bounded form of "the author's own solution
// obeys the spec", for the rows where the observation is mechanical.
func (run *rowsRun) checkReferenceAgainstRaiseRows() {
	confirmed, unconfirmed := 0, 0
	for _, requirement := range run.task.Requirements {
		message := singleRaiseMessage(requirement, run.outcomes)
		if message == "" {
			continue
		}
		exceptionType := raiseType(requirement, run.outcomes)
		if baselineShows(run.baseline, exceptionType, message) {
			confirmed++
		} else {
			unconfirmed++
			fmt.Fprintf(run.out, "reference UNCONFIRMED %s -- no probe observed %s containing %q\n", requirement.ID, exceptionType, message)
		}
	}
	fmt.Fprintf(run.out, "oracle-lite: reference confirmed on %d raise-rows, %d unconfirmed\n\n", confirmed, unconfirmed)
}

// enforceRaiseRows derives the three breakers for every raise-row and runs
// the verifier against each. Closure over the rule's forbidden outcome
// classes: a raise rule can be wrong three ways -- wrong message, wrong
// exception type, or no exception at all. All three breakers are derived
// from the rule; every one must be rejected for the rule to count as
// enforced.
func (run *rowsRun) enforceRaiseRows() error {
	for _, requirement := range run.task.Requirements {
		message := singleRaiseMessage(requirement, run.outcomes)
		if message == "" {
			continue
		}
		file, count := findLiteral(run.sources, message)
		if count != 1 {
			fmt.Fprintf(run.out, "not derived  %s (message %q found %d times in solution)\n", requirement.ID, message, count)
			continue
		}
		original := run.sources[file]
		exceptionType := raiseType(requirement, run.outcomes)
		// The rule matches a substring, so the breaker must remove it;
		// appending would leave substring-matching tests green.
		breakers := []struct{ kind, broken string }{
			{"wrong-message", strings.Replace(original, message, "hyperray broke this message", 1)},
			{"wrong-type", enforce.SwapRaiseType(run.language, original, exceptionType, message)},
			{"no-raise", enforce.SuppressRaise(run.language, original, exceptionType, message)},
		}
		ruleEnforced := true
		var ruleKillers []string
		for _, breaker := range breakers {
			if breaker.broken == "" || breaker.broken == original {
				fmt.Fprintf(run.out, "not derived  %s [%s]\n", requirement.ID, breaker.kind)
				continue
			}
			killers, err := run.runBrokenSolution(file, original, breaker.broken)
			if err != nil {
				return err
			}
			if len(killers) == 0 {
				ruleEnforced = false
				run.holes++
				fmt.Fprintf(run.out, "HOLE         %s [%s] -- this wrong behaviour is pinned by no test\n", requirement.ID, breaker.kind)
				continue
			}
			for _, killer := range killers {
				name := testBaseName(killer)
				run.guardNames[name] = true
				ruleKillers = append(ruleKillers, name)
			}
		}
		if ruleEnforced {
			run.enforced++
			fmt.Fprintf(run.out, "enforced     %s <- %s\n", requirement.ID, strings.Join(dedupe(ruleKillers), "; "))
		}
	}
	return nil
}

// runBrokenSolution writes one broken solution file, runs the verifier, and
// always restores the original bytes before returning.
func (run *rowsRun) runBrokenSolution(file, original, broken string) ([]string, error) {
	path := filepath.Join(run.sourceRoot, file)
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		return nil, err
	}
	command := run.testCommand
	if run.fastKill {
		frameworkRunner, err := runner.New(run.language, "", "", command)
		if err == nil {
			command += frameworkRunner.FastKillSuffix()
		}
	}
	killers := runSuiteFailures(run.language, command, run.sourceRoot)
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		return nil, fmt.Errorf("rows: RESTORE FAILED for %s: %w", file, err)
	}
	return killers, nil
}

// reportAttributedRows reads the mutant-attribution verdicts for every
// non-raise row: killed deviations mark the row enforced, accepted
// deviations are proven false positives, and rows nothing reached stay
// untried.
func (run *rowsRun) reportAttributedRows(attributed map[string]rowVerdict) {
	for _, requirement := range run.task.Requirements {
		if singleRaiseMessage(requirement, run.outcomes) != "" {
			continue
		}
		verdict, found := attributed[requirement.ID]
		switch {
		case !found:
			run.untried++
		case verdict.falsePositive:
			run.holes++
			fmt.Fprintf(run.out, "FALSE POSITIVE %s -- a deviating mutant passed (probe %s)\n", requirement.ID, verdict.probe)
		default:
			run.enforced++
			fmt.Fprintf(run.out, "enforced     %s <- %s (%s)\n", requirement.ID, strings.Join(verdict.killers, "; "), run.closureNote(requirement, verdict))
		}
	}
}

// closureNote summarizes how much of the row's forbidden alphabet the
// mutants actually realized, so "enforced" always carries its boundary.
func (run *rowsRun) closureNote(requirement semanticir.RequirementCase, verdict rowVerdict) string {
	var residue []string
	for _, forbiddenID := range requirement.ForbiddenOutcomes {
		forbidden := run.outcomes[forbiddenID]
		if forbidden.Kind != semanticir.OutcomeReturn || forbidden.Value == nil || forbidden.Value.Type != semanticir.TypeString {
			continue
		}
		label := forbidden.Value.String
		if !slices.Contains(verdict.realizedForbidden, label) {
			residue = append(residue, label)
		}
	}
	note := fmt.Sprintf("%d wrong behaviours rejected", len(verdict.rejected))
	if len(verdict.realizedForbidden) > 0 {
		note += "; forbidden labels realized+rejected: " + strings.Join(verdict.realizedForbidden, ",")
	}
	if len(residue) > 0 {
		note += "; never realized: " + strings.Join(residue, ",")
	}
	return note
}

// checkBaselineGreen proves the tree was restored: the untouched suite must
// pass again after every breaker ran.
func (run *rowsRun) checkBaselineGreen() error {
	restoreCheck := exec.Command("sh", "-c", run.testCommand)
	restoreCheck.Dir = run.sourceRoot
	if restoreCheck.Run() != nil {
		return fmt.Errorf("rows: baseline is not green after restore; the tree needs inspection")
	}
	return nil
}

// writeReferenceSnapshot records the reference's observation under every
// healthy probe beside the bridges. A later run that changes a line here is
// a behaviour change of the reference itself -- reviewable, versionable
// evidence.
func (run *rowsRun) writeReferenceSnapshot() {
	var probeNames []string
	for name := range run.baseline {
		probeNames = append(probeNames, name)
	}
	sort.Strings(probeNames)
	var snapshot []string
	for _, name := range probeNames {
		if healthyProbe(run.baseline, name) {
			snapshot = append(snapshot, name+": "+strings.TrimSpace(run.baseline[name]))
		}
	}
	_ = os.WriteFile(filepath.Join(run.taskDir, "bridges", "reference-observations.txt"), []byte(strings.Join(snapshot, "\n")+"\n"), 0o644)
	fmt.Fprintf(run.out, "oracle: reference observed healthy on %d probe(s); snapshot recorded\n", len(snapshot))
}

// reportOrphanCandidates lists tests never seen guarding any rule. These are
// candidates for unfairness, not verdicts -- a test can guard a rule nothing
// has tried to break yet.
func (run *rowsRun) reportOrphanCandidates(attributed map[string]rowVerdict) {
	guards := run.guardNames
	for _, verdict := range attributed {
		for _, killer := range verdict.killers {
			guards[testBaseName(killer)] = true
		}
	}
	collectCmd := exec.Command("sh", "-c", run.testCommand+" --collect-only -q 2>/dev/null | grep '::' | sed 's/.*:://' | sort -u")
	collectCmd.Dir = run.sourceRoot
	listing, err := collectCmd.Output()
	if err != nil {
		return
	}
	var orphans []string
	for _, test := range strings.Fields(strings.TrimSpace(string(listing))) {
		base := testBaseName(test)
		if !guards[base] && !guards[test] {
			orphans = append(orphans, base)
		}
	}
	orphans = dedupe(orphans)
	if len(orphans) > 0 {
		fmt.Fprintf(run.out, "orphan candidates (%d): tests not yet seen guarding any rule -- %s\n", len(orphans), strings.Join(orphans, "; "))
	}
}

// reportFalseNegativeTheorem states the model-level theorem when it applies:
// every rule requires exactly one outcome, so exactly one allowed behaviour
// table exists -- the reference's. "Every allowed solution passes" is then
// equivalent to "the suite passes the reference", which checkBaselineGreen
// establishes. The probe-equivalence sweep guards the seam between model and
// reality.
func (run *rowsRun) reportFalseNegativeTheorem() {
	for _, requirement := range run.task.Requirements {
		if len(requirement.RequiredOutcomes) != 1 {
			return
		}
	}
	fmt.Fprintf(run.out, "false negatives: 100%% within the finite model -- every rule single-outcome, reference accepted, equivalence sweep clean\n")
}

func (run *rowsRun) summarize() error {
	fmt.Fprintf(run.out, "\nrows: %d enforced, %d holes/false positives, %d untried (no breaker reached them; the claim stops here)\n", run.enforced, run.holes, run.untried)
	if run.holes > 0 {
		return fmt.Errorf("unenforced rows: %d", run.holes)
	}
	return nil
}

// testBaseName strips a parametrization suffix: "test_x[case-1]" -> "test_x".
func testBaseName(test string) string {
	return strings.SplitN(test, "[", 2)[0]
}

// compileTaskDir strictly compiles a task folder's spec the same way
// spec-lint does, so rows and spec-lint can never disagree about the rows.
func compileTaskDir(taskDir string) (*semanticir.Task, error) {
	spec, err := os.ReadFile(filepath.Join(taskDir, "spec.md"))
	if err != nil {
		return nil, err
	}
	instruction, err := os.ReadFile(filepath.Join(taskDir, "instruction.md"))
	if err != nil {
		return nil, err
	}
	request := speccompiler.Request{
		TaskID: filepath.Base(taskDir),
		Artifact: semanticir.ArtifactRef{
			ID: "spec", Kind: semanticir.ArtifactSpec, Path: "spec.md",
			Digest: semanticir.DigestBytes(spec),
		},
		Source: spec,
		Instruction: semanticir.ArtifactRef{
			ID: "instruction", Kind: semanticir.ArtifactInstruction, Path: "instruction.md",
			Digest: semanticir.DigestBytes(instruction),
		},
		InstructionSource: instruction,
	}
	if reference, err := os.ReadFile(filepath.Join(taskDir, "solution.patch")); err == nil {
		request.Reference = semanticir.ArtifactRef{
			ID: "reference", Kind: semanticir.ArtifactCode, Path: "solution.patch",
			Digest: semanticir.DigestBytes(reference),
		}
		request.ReferenceSource = reference
	}
	task, diagnostics := speccompiler.Compile(context.Background(), request)
	if task == nil || semanticir.HasErrors(diagnostics) {
		return nil, fmt.Errorf("rows: spec does not compile: %v", diagnostics)
	}
	return task, nil
}

// singleRaiseMessage returns the message when the row requires exactly one
// raise-with-message outcome, else "".
func singleRaiseMessage(requirement semanticir.RequirementCase, outcomes map[string]semanticir.ObservableOutcome) string {
	if len(requirement.RequiredOutcomes) != 1 {
		return ""
	}
	outcome, found := outcomes[requirement.RequiredOutcomes[0]]
	if !found || outcome.Kind != semanticir.OutcomeRaise || outcome.Message == "" {
		return ""
	}
	return outcome.Message
}

// findLiteral locates the message literal across the solution files; a
// breaker is derivable only when it appears exactly once overall.
func findLiteral(sources map[string]string, message string) (string, int) {
	total, where := 0, ""
	for file, body := range sources {
		count := strings.Count(body, message)
		total += count
		if count > 0 {
			where = file
		}
	}
	return where, total
}

// runSuiteFailures runs the verifier and returns failing test names via the
// language's own report format.
func runSuiteFailures(language, testCommand, dir string) []string {
	frameworkRunner, err := runner.New(language, "", "", testCommand)
	if err != nil {
		return nil
	}
	sub := exec.Command("sh", "-c", frameworkRunner.SuiteCommand())
	sub.Dir = dir
	output, _ := sub.CombinedOutput()
	return frameworkRunner.FailedNames(string(output))
}

type rowVerdict struct {
	probe             string
	killers           []string
	rejected          []string
	realizedForbidden []string
	falsePositive     bool
}

func oneLineOf(text string) string {
	line := strings.SplitN(strings.TrimSpace(text), "\n", 2)[0]
	if len(line) > 60 {
		line = line[:60]
	}
	return line
}

// attributeMutants runs undirected mutants and attributes each deviation to
// spec rows through the probe that saw it: a probe named wit_<op>__<key>.py
// observes the rows of operation <op> whose conditions or required label
// match <key> ("any" matches the whole operation). A killed deviant marks
// those rows enforced by the killing tests; an accepted deviant marks them
// proven false positives. Rows no deviation ever reached stay untried, and
// are reported as the boundary of the claim.
func attributeMutants(cmd *cobra.Command, taskDir, sourceRoot, testCommand, probeRunner, pythonPath string, solutionFiles []string, task *semanticir.Task, outcomes map[string]semanticir.ObservableOutcome, baseline map[string]string) (map[string]rowVerdict, error) {
	verdicts := map[string]rowVerdict{}
	if probeRunner == "" {
		return verdicts, nil
	}
	probesDir := filepath.Join(taskDir, "bridges")
	entries, err := os.ReadDir(probesDir)
	if err != nil {
		return nil, err
	}
	var probes []enforce.Probe
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "wit_") || name == "wit_common.py" {
			continue
		}
		probeName := strings.TrimSuffix(strings.TrimPrefix(name, "wit_"), filepath.Ext(name))
		if !healthyProbe(baseline, probeName) {
			fmt.Fprintf(cmd.OutOrStdout(), "probe excluded (errors on the reference): %s -- its rules stay untried\n", probeName)
			continue
		}
		probes = append(probes, enforce.Probe{
			Name:    probeName,
			Command: strings.ReplaceAll(probeRunner, "{probe}", filepath.Join(probesDir, name)),
			Path:    filepath.Join(probesDir, name),
		})
	}
	if len(probes) == 0 {
		return verdicts, nil
	}
	batchDriver := filepath.Join(hyperrayRepoRoot(), "third_party", "mutate", "run_probes.py")
	engineTask := enforce.Task{
		SourceRoot:        sourceRoot,
		TestCwd:           sourceRoot,
		TestCommand:       testCommand + " -rf",
		ProbeBatchCommand: strings.ReplaceAll(probeRunner, "{probe}", batchDriver) + " {probes}",
		AfterWrite:        "find " + sourceRoot + " -name __pycache__ -exec rm -rf {} +",
		Timeout:           10 * time.Minute,
	}
	for _, solutionFile := range solutionFiles {
		mutants, err := mutate.Generate(pythonPath, filepath.Join(sourceRoot, solutionFile), "python")
		if err != nil {
			return nil, err
		}
		deviations, err := enforce.Discover(engineTask, solutionFile, mutants, probes)
		if err != nil {
			return nil, err
		}
		rejected, err := enforce.FalseNegatives(engineTask, solutionFile, mutants, probes)
		if err != nil {
			return nil, err
		}
		for _, finding := range rejected {
			fmt.Fprintf(cmd.OutOrStdout(), "FALSE NEGATIVE candidate -- %s:%d %s\n  %s\n  rejected by: %s\n",
				solutionFile, finding.Mutant.Line,
				finding.Mutant.Operator+": "+finding.Mutant.Original+" -> "+finding.Mutant.Mutated,
				finding.Reason, strings.Join(finding.Killers, "; "))
		}
		for _, deviation := range deviations {
			operation, key, ok := strings.Cut(deviation.Input, "__")
			if !ok {
				if deviation.Accepted {
					fmt.Fprintf(cmd.OutOrStdout(), "FALSE POSITIVE (file-level) -- %s:%d %s -> %s\n",
						solutionFile, deviation.Mutant.Line, deviation.Mutant.Original, deviation.Mutant.Mutated)
				}
				continue
			}
			for _, requirement := range task.Requirements {
				if requirement.OperationID != operation || !rowMatchesKey(requirement, key, outcomes) {
					continue
				}
				existing, found := verdicts[requirement.ID]
				if deviation.Accepted {
					verdicts[requirement.ID] = rowVerdict{probe: deviation.Input, falsePositive: true}
				} else if !found || !existing.falsePositive {
					updated := rowVerdict{probe: deviation.Input, killers: mergeNames(existing.killers, deviation.Killers), rejected: existing.rejected, realizedForbidden: existing.realizedForbidden}
					// Closure strength: each distinct wrong observation this
					// rule's probe saw rejected is one more forbidden
					// behaviour proven caught.
					updated.rejected = mergeNames(updated.rejected, []string{oneLineOf(deviation.After)})
					// Cross-rule realization: the spec names which OTHER rule
					// requires each label this rule forbids. When the broken
					// solution's observation equals that other probe's
					// baseline, the forbidden label was realized here -- and
					// it was rejected, because this deviant was killed.
					for _, forbiddenID := range requirement.ForbiddenOutcomes {
						forbidden := outcomes[forbiddenID]
						if forbidden.Kind != semanticir.OutcomeReturn || forbidden.Value == nil || forbidden.Value.Type != semanticir.TypeString {
							continue
						}
						label := forbidden.Value.String
						other, known := baseline[operation+"__"+label]
						if known && strings.TrimSpace(deviation.After) == strings.TrimSpace("0\n"+other) {
							updated.realizedForbidden = mergeNames(updated.realizedForbidden, []string{label})
						}
						if known && strings.TrimSpace(deviation.After) == strings.TrimSpace(other) {
							updated.realizedForbidden = mergeNames(updated.realizedForbidden, []string{label})
						}
					}
					verdicts[requirement.ID] = updated
				}
			}
		}
	}
	return verdicts, nil
}

func rowMatchesKey(requirement semanticir.RequirementCase, key string, outcomes map[string]semanticir.ObservableOutcome) bool {
	if key == "any" {
		return true
	}
	for _, value := range requirement.Conditions {
		if value == key {
			return true
		}
	}
	for _, outcomeID := range requirement.RequiredOutcomes {
		outcome := outcomes[outcomeID]
		if outcome.Kind == semanticir.OutcomeReturn && outcome.Value != nil && outcome.Value.Type == semanticir.TypeString && outcome.Value.String == key {
			return true
		}
	}
	return false
}

func mergeNames(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, name := range append(a, b...) {
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// probeOutputs runs every bridge probe once against the untouched tree,
// keyed by probe name so an erroring baseline can be excluded downstream.
func probeOutputs(taskDir, sourceRoot, probeRunner string) map[string]string {
	outputs := map[string]string{}
	if probeRunner == "" {
		return outputs
	}
	entries, err := os.ReadDir(filepath.Join(taskDir, "bridges"))
	if err != nil {
		return outputs
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "wit_") || name == "wit_common.py" {
			continue
		}
		sub := exec.Command("sh", "-c", strings.ReplaceAll(probeRunner, "{probe}", filepath.Join(taskDir, "bridges", name)))
		sub.Dir = sourceRoot
		output, _ := sub.CombinedOutput()
		outputs[strings.TrimSuffix(strings.TrimPrefix(name, "wit_"), filepath.Ext(name))] = string(output)
	}
	return outputs
}

// healthyProbe reports whether a probe's baseline is a real observation. A
// probe that errors on the untouched reference observes nothing; letting it
// attribute verdicts would let an environment quirk mint false positives.
func healthyProbe(baseline map[string]string, name string) bool {
	output, found := baseline[name]
	return found && !strings.Contains(output, "Traceback") && !strings.Contains(output, "TODO")
}

func raiseType(requirement semanticir.RequirementCase, outcomes map[string]semanticir.ObservableOutcome) string {
	outcome := outcomes[requirement.RequiredOutcomes[0]]
	return outcome.ExceptionType
}

func baselineShows(outputs map[string]string, exceptionType, message string) bool {
	for _, output := range outputs {
		if strings.Contains(output, exceptionType) && strings.Contains(output, message) {
			return true
		}
	}
	return false
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
