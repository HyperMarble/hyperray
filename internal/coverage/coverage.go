// Package coverage generates combinatorial test coverage from spec.md's
// declared parameters, using PICT (github.com/microsoft/pict) as the
// underlying combinatorial engine.
package coverage

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/HyperMarble/ray/internal/specparser"
)

// Combination is one generated test case: parameter name -> value.
type Combination map[string]string

type TableCoverage struct {
	Section      string        `json:"section"`
	Line         int           `json:"line"`
	Combinations []Combination `json:"combinations"`
}

// pictSeparator is used for PICT's own value list, distinct from the
// spec.md grammar's "/" — a domain value can never contain "|", since
// specparser already splits table cells on it, so this never collides
// with a real value.
const pictSeparator = "|"

// Generate runs PICT against every table in tables that has a fully
// declared Parameters domain, at the given t-way strength (0 uses
// PICT's own default, pairwise). Tables spec-lint would flag (no
// Params, unsupported domain, schema mismatch) are silently skipped —
// coverage assumes the spec already passed spec-lint.
func Generate(tables []specparser.Table, pictPath string, strength int) ([]TableCoverage, error) {
	if pictPath == "" {
		pictPath = "pict"
	}
	if _, err := exec.LookPath(pictPath); err != nil {
		return nil, fmt.Errorf("pict binary not found (%q): %w", pictPath, err)
	}

	var results []TableCoverage
	for _, tb := range tables {
		if tb.Params == "" || len(tb.Columns) < 2 {
			continue
		}
		domains, unsupported, err := specparser.ParseParams(tb.Params)
		if err != nil || unsupported != "" || len(domains) != tb.ParamColumns() {
			continue
		}

		// EXHAUSTIVE, not sampled. PICT's default is pairwise: every pair
		// of values appears somewhere, but a specific three-way case can
		// be omitted entirely. Measured: three binary parameters have 8
		// combinations and PICT's default returns 4. ray was reporting
		// that sample as "every combination", so a clean coverage result
		// was weaker than it read.
		//
		// The task is fixed and bounded, so the whole Cartesian product is
		// the right answer and needs no external tool. PICT stays available
		// through runPict for a future strength-limited mode, but a
		// sampled result must never be reported as complete.
		combos, err := cartesian(domains)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", tb.Section, err)
		}
		results = append(results, TableCoverage{
			Section:      tb.Section,
			Line:         tb.Line,
			Combinations: combos,
		})
	}
	return results, nil
}

// maxCombinations bounds the expansion. A fixed task cannot legitimately
// need more; past this, the spec's parameters are too coarse and the
// caller is told rather than handed a sample.
const maxCombinations = 20000

// cartesian returns every complete combination of the declared values.
func cartesian(domains []specparser.Domain) ([]Combination, error) {
	total := 1
	for _, d := range domains {
		if len(d.Values) == 0 {
			return nil, fmt.Errorf("parameter %q declares no values", d.Name)
		}
		total *= len(d.Values)
		if total > maxCombinations {
			return nil, fmt.Errorf(
				"more than %d combinations — decompose the parameters; a sampled subset "+
					"must not be reported as complete coverage", maxCombinations)
		}
	}

	combos := make([]Combination, 0, total)
	idx := make([]int, len(domains))
	for {
		c := make(Combination, len(domains))
		for i, d := range domains {
			c[d.Name] = d.Values[idx[i]]
		}
		combos = append(combos, c)

		pos := len(domains) - 1
		for pos >= 0 {
			idx[pos]++
			if idx[pos] < len(domains[pos].Values) {
				break
			}
			idx[pos] = 0
			pos--
		}
		if pos < 0 {
			return combos, nil
		}
	}
}

func runPict(pictPath string, domains []specparser.Domain, strength int) ([]Combination, error) {
	model := buildModel(domains)

	f, err := os.CreateTemp("", "ray-pict-model-*.txt")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(model); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}

	args := []string{f.Name(), "/d:" + pictSeparator}
	if strength > 0 {
		args = append(args, fmt.Sprintf("/o:%d", strength))
	}

	out, err := exec.Command(pictPath, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("pict: %w", err)
	}
	return parsePict(string(out))
}

// parsePict reads PICT's real output: tab-separated columns, one header
// row naming the parameters, one row per generated combination.
//
// An earlier version asked for "/f:json". That option does not exist --
// pict 3.7.4 rejects it with "Unknown option" and exits 3, so the whole
// coverage layer failed on every task and reported itself skipped. It
// was never caught because a skipped layer looks like a missing binary,
// not like a bug.
func parsePict(out string) ([]Combination, error) {
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	var header []string
	var combos []Combination
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if header == nil {
			header = fields
			continue
		}
		if len(fields) != len(header) {
			return nil, fmt.Errorf("pict row has %d fields, header has %d", len(fields), len(header))
		}
		c := make(Combination, len(fields))
		for i, name := range header {
			c[name] = fields[i]
		}
		combos = append(combos, c)
	}
	if header == nil {
		return nil, fmt.Errorf("pict produced no output")
	}
	return combos, nil
}

func buildModel(domains []specparser.Domain) string {
	var b strings.Builder
	for _, d := range domains {
		fmt.Fprintf(&b, "%s: %s\n", d.Name, strings.Join(d.Values, " "+pictSeparator+" "))
	}
	return b.String()
}
