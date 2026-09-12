// This check compares nested table fields against Isla's exact field sets.
// It must read the real fields() calls, never a copy of the expected names.
//go:build isla_integration && arm64_acceptance

package isla_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestNestedTablesMatchExactFieldSets fails when a nested table we emit does
// not carry exactly the fields Isla requires. Isla rejects any difference.
func TestNestedTablesMatchExactFieldSets(t *testing.T) {
	required := requiredFieldSets(t)
	emitted := emittedNestedTables(t, generatedProgramText(t))
	failures := []string{}
	for _, table := range emitted {
		if !matchesOneSet(table, required) {
			failures = append(failures, "{"+strings.Join(table, ", ")+"}")
		}
	}
	if len(failures) != 0 {
		t.Fatalf("no Isla field set accepts these emitted tables: %s", strings.Join(failures, " "))
	}
}

func matchesOneSet(table []string, required [][]string) bool {
	for _, set := range required {
		if strings.Join(table, ",") == strings.Join(set, ",") {
			return true
		}
	}
	return false
}

// requiredFieldSets reads every exact set that the ARM64 memory parser demands.
func requiredFieldSets(t *testing.T) [][]string {
	t.Helper()
	root := os.Getenv(islaSourceRoot)
	if root == "" {
		t.Skipf("%s is empty, so field sets cannot be compared", islaSourceRoot)
	}
	content, err := os.ReadFile(filepath.Join(root, "isla-axiomatic", "src", "arm64_memory.rs"))
	if err != nil {
		t.Fatalf("read ARM64 memory parser: %v", err)
	}
	pattern := regexp.MustCompile(`fields\([^,]+,\s*&\[([^\]]+)\]`)
	matches := pattern.FindAllStringSubmatch(string(content), -1)
	if len(matches) == 0 {
		t.Fatal("ARM64 memory parser declared no field sets")
	}
	sets := make([][]string, 0, len(matches))
	for _, match := range matches {
		names := regexp.MustCompile(`"([a-z_0-9]+)"`).FindAllStringSubmatch(match[1], -1)
		set := make([]string, 0, len(names))
		for _, name := range names {
			set = append(set, name[1])
		}
		sort.Strings(set)
		sets = append(sets, set)
	}
	return sets
}

// emittedNestedTables returns the field names of each inline table we emit.
func emittedNestedTables(t *testing.T, text string) [][]string {
	t.Helper()
	inline := regexp.MustCompile(`\{ ([a-z_0-9]+ = [^}]*)\}`)
	key := regexp.MustCompile(`([a-z_0-9]+) = `)
	tables := [][]string{}
	for _, match := range inline.FindAllStringSubmatch(text, -1) {
		names := key.FindAllStringSubmatch(match[1], -1)
		table := make([]string, 0, len(names))
		for _, name := range names {
			table = append(table, name[1])
		}
		sort.Strings(table)
		tables = append(tables, table)
	}
	if len(tables) == 0 {
		t.Fatal("generated program text carried no inline tables")
	}
	return tables
}
