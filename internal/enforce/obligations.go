package enforce

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HyperMarble/ray/internal/coverage"
	"github.com/HyperMarble/ray/internal/specparser"
)

// Spec is the authored companion to a frozen spec.md: how to run the
// task's verifier, and how to violate each obligation.
//
// It is deliberately a separate file rather than more columns in
// spec.md. spec.md states WHAT is required; this states how to break one
// requirement and how to run the tests, which is machinery, not
// contract. The contract stays readable and stays frozen.
type Spec struct {
	Task        Task              `json:"task"`
	Violations  []ScopedViolation `json:"violations"`
	Fix         *FixSpec          `json:"fix,omitempty"`
	ViolationOf map[string]string `json:"-"`
}

// FixSpec lets ray render the missing test for a proven false positive. The
// task owns the shape of its tests via Template; ray owns only the row facts
// substituted into it, so one mechanism serves every language:
//
//	{name}       the obligation's combo, flattened to an identifier
//	{<param>}    a combo value verbatim ("true")
//	{<param>^}   the same value capitalized ("True"), for Python literals
//	{behavior}   the row's Required-outcomes cell verbatim ("return 0")
//	{value}      that cell with a leading "return " stripped ("0")
//
// A rendered test is derivation from the frozen spec row -- exactly as right
// as the row, no more. Generated tests are appended under a marker naming
// ray, and the report lists them, so a human reviews what the axiom implied.
type FixSpec struct {
	File     string `json:"file"`
	Template string `json:"template"`
}

// Render produces the test Template implies for one obligation.
func (f FixSpec) Render(ob Obligation) string {
	out := f.Template
	out = strings.ReplaceAll(out, "{name}", comboName(ob.Combo))
	out = strings.ReplaceAll(out, "{behavior}", ob.Behavior)
	out = strings.ReplaceAll(out, "{value}", strings.TrimPrefix(ob.Behavior, "return "))
	for k, v := range ob.Combo {
		out = strings.ReplaceAll(out, "{"+k+"^}", capitalize(v))
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}

func comboName(combo map[string]string) string {
	keys := make([]string, 0, len(combo))
	for k := range combo {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"_"+combo[k])
	}
	name := strings.Join(parts, "_")
	return strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' || r == '.' {
			return '_'
		}
		return r
	}, name)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ScopedViolation applies to every expanded combination whose values
// match When. When is a SUBSET of the combination's parameters, so one
// authored violation can discharge several combinations -- a reversed
// range is broken the same way whatever the step happens to be.
type ScopedViolation struct {
	Section   string            `json:"section"`
	When      map[string]string `json:"when"`
	Violation Violation         `json:"violation"`
}

// LoadSpec reads the obligations file beside spec.md and resolves its
// relative paths against the task folder.
func LoadSpec(path, taskDir string) (*Spec, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Spec
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filepath.Base(path), err)
	}
	if !filepath.IsAbs(s.Task.SourceRoot) {
		s.Task.SourceRoot = filepath.Join(taskDir, s.Task.SourceRoot)
	}
	if s.Task.TestCwd == "" {
		s.Task.TestCwd = taskDir
	} else if !filepath.IsAbs(s.Task.TestCwd) {
		s.Task.TestCwd = filepath.Join(taskDir, s.Task.TestCwd)
	}
	return &s, nil
}

// Matches reports whether a scoped violation applies to a combination.
func (sv ScopedViolation) Matches(section string, combo coverage.Combination) bool {
	if sv.Section != "" && sv.Section != section {
		return false
	}
	for k, want := range sv.When {
		if combo[k] != want {
			return false
		}
	}
	return true
}

// Build pairs every expanded combination with the test the frozen spec
// declares enforces it and the authored violation that breaks it.
//
// Combinations with no authored violation are still returned, carrying an
// empty Violation. They come back inconclusive rather than being dropped:
// an obligation nobody wrote a violation for is unverified, and silently
// omitting it would turn "not checked" into "no problem found".
func Build(tables []specparser.Table, covs []coverage.TableCoverage, spec *Spec) []Obligation {
	return BuildWith(tables, covs, spec, nil)
}

// BuildWith additionally consults violations ray derived itself, used only
// where the author wrote none. Authored always wins: an author can express
// what synthesis cannot.
func BuildWith(tables []specparser.Table, covs []coverage.TableCoverage, spec *Spec, derived []ScopedViolation) []Obligation {
	bySection := map[string]specparser.Table{}
	for _, tb := range tables {
		bySection[tb.Section] = tb
	}

	var obs []Obligation
	for _, cov := range covs {
		tb, ok := bySection[cov.Section]
		if !ok {
			continue
		}
		domains, _, err := specparser.ParseParams(tb.Params)
		if err != nil {
			continue
		}
		for _, combo := range cov.Combinations {
			ob := Obligation{Section: cov.Section, Combo: combo}
			for _, row := range tb.Rows {
				if specparser.RowMatches(row, domains, combo) {
					ob.Test = tb.EnforcedBy(row)
					ob.Behavior = tb.RequiredBehavior(row)
					break
				}
			}
			for _, sv := range spec.Violations {
				if sv.Matches(cov.Section, combo) {
					ob.Violation = sv.Violation
					break
				}
			}
			if ob.Unauthored() {
				for _, sv := range derived {
					if sv.Matches(cov.Section, combo) {
						ob.Violation = sv.Violation
						ob.Violation.File = spec.Task.SolutionFile
						break
					}
				}
			}
			obs = append(obs, ob)
		}
	}
	return obs
}

// Unauthored reports whether no violation was written for an obligation.
func (o Obligation) Unauthored() bool { return o.Violation.Cut == "" }
