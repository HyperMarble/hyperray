// Package coverage generates combinatorial test coverage from spec.md's
// declared parameters, using PICT (github.com/microsoft/pict) as the
// underlying combinatorial engine.
package coverage

import (
	"encoding/json"
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
		if err != nil || unsupported != "" || len(domains) != len(tb.Columns)-1 {
			continue
		}

		combos, err := runPict(pictPath, domains, strength)
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

	args := []string{f.Name(), "/d:" + pictSeparator, "/f:json"}
	if strength > 0 {
		args = append(args, fmt.Sprintf("/o:%d", strength))
	}

	out, err := exec.Command(pictPath, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("pict: %w", err)
	}

	var raw [][]struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing pict output: %w", err)
	}

	combos := make([]Combination, len(raw))
	for i, row := range raw {
		c := make(Combination, len(row))
		for _, kv := range row {
			c[kv.Key] = kv.Value
		}
		combos[i] = c
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
