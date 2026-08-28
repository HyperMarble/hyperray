package enforce

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/HyperMarble/ray/internal/mutate"
)

// One mechanism, not a rule per case.
//
// Earlier versions grew a special case per kind of spec row -- delete the
// raise a failure row names, mutate the range operator a value row uses,
// substitute a constant to expose hardcodeable inputs. That list never
// terminates, and each entry is a guess about how a requirement can be
// broken.
//
// The general form has three parts, and none of them knows anything about
// kinds of rows:
//
//	generate  candidate wrong solutions, mechanically, from a fixed
//	          operator table over the code the task changed
//	filter    keep the candidates that actually behave differently from
//	          the real solution -- the reference solution is the oracle for
//	          what correct means, so any behavioural difference is a
//	          deviation from correct
//	judge     run the task's own verifier on a deviating candidate; if it
//	          passes, a wrong solution passes, which is a false positive
//
// Only the finding direction is claimed. "Differs on an input we tried"
// proves deviation. "Agreed on the inputs we tried" proves nothing, and is
// reported as unknown -- concluding equivalence from finite sampling is
// what produced nine wrong verdicts earlier.
//
// This is bounded because the task is fixed: finite mutation points in a
// finite patch, a finite operator table, finitely many probes.

// Deviation is a candidate solution shown to behave differently from the
// real one, together with the input that shows it.
type Deviation struct {
	Mutant  mutate.Mutant
	Input   string
	Before  string
	After   string
	Reason  string
	Checked bool
	// Accepted reports the verifier's answer: true means this proven-wrong
	// candidate passed the tests -- a false positive. False means it was
	// killed, and Killers names the failing tests, so enforcement can be
	// attributed to the rows the deviating probe observes.
	Accepted bool
	Killers  []string
}

// Discover finds candidate solutions that deviate from the real one, then
// asks the task's verifier about each.
//
// Returns every deviation the verifier was asked about. Accepted deviations
// are proven false positives; killed ones carry their killers, which is the
// evidence that attributes enforcement to the rows the probe observes.
func Discover(task Task, solutionFile string, mutants []mutate.Mutant, probes []Probe) ([]Deviation, error) {
	return DiscoverWith(task, solutionFile, mutants, probes, nil)
}

// DiscoverWith is Discover given a line map, which turns the cost of an
// adversary from one full verifier run into a couple of tests -- or into
// nothing at all when no test executes the line it changed.
func DiscoverWith(task Task, solutionFile string, mutants []mutate.Mutant, probes []Probe, lines LineMap) ([]Deviation, error) {
	if len(probes) == 0 {
		return nil, fmt.Errorf("enforce: no probes; an adversary cannot be shown to deviate without an input to run it on")
	}
	path := task.SourceRoot + "/" + solutionFile
	original, err := readSource(task, path)
	if err != nil {
		return nil, err
	}

	baseline := observe(task, probes)

	var found []Deviation
	for _, m := range mutants {
		if m.Source == "" || m.Source == original {
			continue
		}
		if err := writeSource(task, path, m.Source); err != nil {
			continue
		}
		after := observe(task, probes)
		dev, deviates := firstDifference(probes, baseline, after)
		if !deviates {
			// Not shown to deviate. Not "equivalent" -- unknown. Asking the
			// verifier about it would prove nothing either way.
			_ = writeSource(task, path, original)
			continue
		}
		dev.Mutant = m
		dev.Checked = true

		// An adversary on a line no test executes cannot be caught by
		// anything, so there is nothing to run: it deviates and the
		// verifier is blind to it.
		if tests, measured := lines.TestsForLine(solutionFile, m.Line); measured && len(tests) == 0 {
			_ = writeSource(task, path, original)
			dev.Reason = "no test executes this line, so nothing can reject it"
			found = append(found, dev)
			continue
		}

		passed, out := verifierPasses(task, lines, solutionFile, m.Line)
		_ = writeSource(task, path, original)
		if passed {
			dev.Accepted = true
			dev.Reason = fmt.Sprintf("verifier stayed green (%s)", lastLine(out))
		} else {
			dev.Reason = "killed"
			dev.Killers = FailedTestNames(out)
		}
		found = append(found, dev)
	}
	if err := writeSource(task, path, original); err != nil {
		return found, err
	}
	if got, _ := readSource(task, path); got != original {
		return found, fmt.Errorf("enforce: solution was not restored")
	}
	return found, nil
}

// observe runs every probe and records what came back.
func observe(task Task, probes []Probe) []string {
	if batched, ok := observeBatch(task, probes); ok {
		return batched
	}
	out := make([]string, len(probes))
	for i, p := range probes {
		code, text := run(task, p.Command, task.SourceRoot)
		out[i] = fmt.Sprintf("%d\n%s", code, text)
	}
	return out
}

// observeBatch runs every probe in one process when the task provides a
// batch command and every probe carries its script path. Falls back to the
// per-probe loop otherwise, and on any parse shortfall, so a batching
// problem can only cost time, never observations.
func observeBatch(task Task, probes []Probe) ([]string, bool) {
	if task.ProbeBatchCommand == "" {
		return nil, false
	}
	paths := make([]string, len(probes))
	for i, p := range probes {
		if p.Path == "" {
			return nil, false
		}
		paths[i] = p.Path
	}
	_, text := run(task, strings.ReplaceAll(task.ProbeBatchCommand, "{probes}", strings.Join(paths, " ")), task.SourceRoot)
	sections := map[string]string{}
	current := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "===RAY_PROBE ") && strings.HasSuffix(line, "===") {
			current = strings.TrimSuffix(strings.TrimPrefix(line, "===RAY_PROBE "), "===")
			continue
		}
		if current != "" {
			sections[current] += line + "\n"
		}
	}
	out := make([]string, len(probes))
	for i, p := range probes {
		section, found := sections[filepath.Base(p.Path)]
		if !found {
			return nil, false
		}
		out[i] = strings.TrimRight(section, "\n")
	}
	return out, true
}

func firstDifference(probes []Probe, before, after []string) (Deviation, bool) {
	for i := range probes {
		if i < len(before) && i < len(after) && before[i] != after[i] {
			return Deviation{
				Input:  probes[i].Name,
				Before: shorten(oneLine(before[i])),
				After:  shorten(oneLine(after[i])),
			}, true
		}
	}
	return Deviation{}, false
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

var failedNamePattern = regexp.MustCompile(`FAILED [^\s:]*::([A-Za-z0-9_]+)`)

// failedTestNames parses the verifier's own FAILED report lines. Parametrized
// case suffixes are dropped so one test name covers its whole matrix.
func FailedTestNames(output string) []string {
	seen := map[string]bool{}
	var names []string
	for _, match := range failedNamePattern.FindAllStringSubmatch(output, -1) {
		if !seen[match[1]] {
			seen[match[1]] = true
			names = append(names, match[1])
		}
	}
	return names
}
