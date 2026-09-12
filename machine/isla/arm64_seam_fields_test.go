// Seam checks compare emitted program text against the Isla parser source.
// They must read generated output and real Isla source, never a fixed list.
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

// islaSourceRoot names the checkout whose parser must accept our output.
const islaSourceRoot = "HYPERRAY_ISLA_SOURCE"

// TestEmittedFieldsReachTheParser fails when generated program text carries a
// key that the Isla parser never reads. Such a key is silently ignored.
func TestEmittedFieldsReachTheParser(t *testing.T) {
	parser := islaLitmusSource(t)
	unread := []string{}
	for _, key := range emittedKeys(t, generatedProgramText(t)) {
		if !strings.Contains(parser, `"`+key+`"`) {
			unread = append(unread, key)
		}
	}
	if len(unread) != 0 {
		t.Fatalf("Isla never reads these emitted keys: %s", strings.Join(unread, ", "))
	}
}

// islaLitmusSource joins every parser source file, because one key may be
// read by the litmus parser and another by the ARM64 memory parser.
func islaLitmusSource(t *testing.T) string {
	t.Helper()
	root := os.Getenv(islaSourceRoot)
	if root == "" {
		t.Skipf("%s is empty, so the seam cannot be compared", islaSourceRoot)
	}
	pattern := filepath.Join(root, "isla-axiomatic", "src", "*.rs")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("list Isla parser sources: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no Isla parser sources under %s", pattern)
	}
	var joined strings.Builder
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		joined.Write(content)
	}
	return joined.String()
}

// generatedProgramText builds one real program so the check reads output
// rather than the renderer's source.
func generatedProgramText(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "machine", "arm64", "leaky_relu", "leaky-relu-arm64-static"))
	if err != nil {
		t.Fatalf("read Leaky ReLU Mach-O: %v", err)
	}
	testCase := leakyCases(t)[0]
	memory := leakyMemoryInput(t, testCase.InputWords)
	program := leakyProgram(t, content, memory, leakyViolation(testCase.ExpectedWords, testCase.NegativeCount))
	return string(program.Content())
}

// emittedKeys returns every table name and assignment key in program text.
func emittedKeys(t *testing.T, text string) []string {
	t.Helper()
	assignment := regexp.MustCompile(`(?m)^([a-z_0-9]+) = `)
	table := regexp.MustCompile(`(?m)^\[\[?([a-z_0-9]+)`)
	seen := map[string]struct{}{}
	for _, match := range assignment.FindAllStringSubmatch(text, -1) {
		seen[match[1]] = struct{}{}
	}
	for _, match := range table.FindAllStringSubmatch(text, -1) {
		seen[match[1]] = struct{}{}
	}
	if len(seen) == 0 {
		t.Fatal("generated program text carried no keys")
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
