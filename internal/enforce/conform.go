package enforce

import (
	"fmt"
	"regexp"
	"strings"
)

// Obligation A: the real implementation must satisfy every requirement in
// the frozen spec.
//
// The scope is fixed, so this is exhaustive rather than approximate: every
// expanded combination is checked, using the witness already authored for
// obligation B. One input, two questions --
//
//	B: does the verifier reject a solution that violates this row?
//	A: does the real solution actually do what this row requires?
//
// Only a Required-behavior clause written in a checkable form can be
// decided here. Anything else is inconclusive and blocks certification;
// it is never counted as satisfied, because "could not check" is not
// "no problem found".

// Expectation is a Required-behavior clause reduced to something a run can
// be judged against.
type Expectation struct {
	// Raises is the exception/error type the row requires, if any.
	Raises string
	// Contains is text the row requires the failure to carry.
	Contains string
	// Succeeds is set when the row requires normal completion.
	Succeeds bool
}

// A row states its requirement in prose, but the part that matters is
// written to a fixed shape by the spec skill: `raise <Type> containing
// "<text>"`. That is deliberately the only form decided automatically --
// inventing a parser for arbitrary English would reintroduce exactly the
// guessing this tool exists to remove.
var raisePattern = regexp.MustCompile(
	`raise\s+([A-Za-z_][A-Za-z_0-9.]*)\s+containing\s+"([^"]+)"`)

// ParseExpectation reduces a Required-behavior cell to a checkable
// expectation. The second result is false when the clause is not in a
// decidable form.
func ParseExpectation(behavior string) (Expectation, bool) {
	behavior = strings.TrimSpace(behavior)
	if m := raisePattern.FindStringSubmatch(behavior); m != nil {
		return Expectation{Raises: m[1], Contains: m[2]}, true
	}
	// A row that requires an error without naming the carried text.
	if m := regexp.MustCompile(`raise\s+([A-Za-z_][A-Za-z_0-9.]*)`).FindStringSubmatch(behavior); m != nil {
		return Expectation{Raises: m[1]}, true
	}
	// A row that states an outcome rather than a failure requires the
	// solution to complete on that input. The row's exact VALUE is not
	// judged here -- prose is not a value oracle. It is judged by the
	// task's own test, and obligation B is what proves that test really
	// enforces this row. The two together close the row; neither does
	// alone, which is why the detail string says which half this is.
	if behavior != "" {
		return Expectation{Succeeds: true}, true
	}
	return Expectation{}, false
}

// Conform decides obligation A for one expanded combination by running the
// witness against the UNMODIFIED solution and judging what came back.
func Conform(task Task, ob Obligation) Result {
	if ob.Behavior == "" {
		return Result{ob, Inconclusive, "row states no Required behavior"}
	}
	if ob.Violation.Witness == "" {
		return Result{ob, Inconclusive, "no witness authored for this obligation"}
	}
	want, ok := ParseExpectation(ob.Behavior)
	if !ok {
		return Result{ob, Inconclusive,
			fmt.Sprintf("Required behavior is not in a decidable form: %q", shorten(ob.Behavior))}
	}

	// A witness is only evidence about this row if it actually exercises
	// this row. Caught by a real authoring slip: the non-string-expression
	// witness passed a string, so it never reached the rule, the solution
	// completed normally, and obligation A reported the solution VIOLATED
	// when the solution was fine and the witness was wrong.
	//
	// Obligation B's demonstration answers exactly this -- if breaking the
	// rule does not change what the witness observes, the witness is blind
	// to the rule. Reuse it rather than trusting the author.
	if exercises, err := witnessExercises(task, ob); err != nil {
		return Result{ob, Inconclusive, "could not establish whether the witness exercises this row"}
	} else if !exercises {
		return Result{ob, Inconclusive,
			"witness does not exercise this row; it observes nothing when the rule is broken"}
	}

	code, out := run(task, ob.Violation.Witness, task.SourceRoot)

	if want.Raises != "" {
		if code == 0 {
			return Result{ob, Violated,
				fmt.Sprintf("spec requires %s, but the solution completed normally", want.Raises)}
		}
		if !strings.Contains(out, want.Raises) {
			return Result{ob, Violated,
				fmt.Sprintf("spec requires %s, but the solution failed with something else: %s",
					want.Raises, shorten(lastLine(out)))}
		}
		if want.Contains != "" && !strings.Contains(out, want.Contains) {
			return Result{ob, Violated,
				fmt.Sprintf("spec requires %s carrying %q, which the failure does not: %s",
					want.Raises, want.Contains, shorten(lastLine(out)))}
		}
		return Result{ob, Satisfied,
			fmt.Sprintf("solution raises %s as required", want.Raises)}
	}

	if code != 0 {
		return Result{ob, Violated,
			fmt.Sprintf("spec requires normal completion, but the solution failed: %s", shorten(lastLine(out)))}
	}
	return Result{ob, Satisfied,
		"solution completes on this input; the row's value is enforced by its declared test (obligation B)"}
}

// ConformAll decides obligation A for every obligation in order.
func ConformAll(task Task, obs []Obligation) []Result {
	results := make([]Result, 0, len(obs))
	for _, ob := range obs {
		results = append(results, Conform(task, ob))
	}
	return results
}

func shorten(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 110 {
		return s[:107] + "..."
	}
	return s
}
