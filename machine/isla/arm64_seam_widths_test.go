// This check compares declared SMT parameter widths against call arguments.
// It must read a captured solver script, never a rebuilt approximation.
//go:build isla_integration && arm64_acceptance

package isla_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// capturedScriptDirectory holds solver scripts kept by an earlier real run.
const capturedScriptDirectory = "HYPERRAY_ARM64_SMT_CAPTURE"

// TestMemoryPredicateWidthsAgree fails when a memory predicate is called with
// a value narrower than its declaration. Z3 rejects the whole script.
func TestMemoryPredicateWidthsAgree(t *testing.T) {
	script := capturedSolverScript(t)
	declared := declaredPredicateWidths(t, script)
	mismatches := []string{}
	for _, call := range predicateCalls(t, script) {
		width, known := declared[call.name]
		if !known {
			mismatches = append(mismatches, call.name+" is called but never declared")
			continue
		}
		if call.valueWidth != width {
			mismatches = append(mismatches,
				fmt.Sprintf("%s declares %d bits but receives %d", call.name, width, call.valueWidth))
		}
	}
	if len(mismatches) != 0 {
		t.Fatalf("memory predicate widths disagree: %s", strings.Join(mismatches, "; "))
	}
}

func capturedSolverScript(t *testing.T) string {
	t.Helper()
	directory := os.Getenv(capturedScriptDirectory)
	if directory == "" {
		t.Skipf("%s is empty, so no solver script can be read", capturedScriptDirectory)
	}
	paths, err := filepath.Glob(filepath.Join(directory, "*.smt2"))
	if err != nil {
		t.Fatalf("list solver scripts: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no solver script under %s", directory)
	}
	content, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("read %s: %v", paths[0], err)
	}
	return string(content)
}

func declaredPredicateWidths(t *testing.T, script string) map[string]int {
	t.Helper()
	pattern := regexp.MustCompile(`\(define-fun (last_write_to_\d+) \(\(addr \(_ BitVec \d+\)\) \(value \(_ BitVec (\d+)\)\)\)`)
	widths := map[string]int{}
	for _, match := range pattern.FindAllStringSubmatch(script, -1) {
		width, err := strconv.Atoi(match[2])
		if err != nil {
			t.Fatalf("declared width %q: %v", match[2], err)
		}
		widths[match[1]] = width
	}
	if len(widths) == 0 {
		t.Fatal("solver script declared no memory predicate")
	}
	return widths
}

type predicateCall struct {
	name       string
	valueWidth int
}

// predicateCalls reads the value argument width of each predicate call. A
// hexadecimal literal carries four bits per digit; a widened expression
// carries its stated target width.
func predicateCalls(t *testing.T, script string) []predicateCall {
	t.Helper()
	literal := regexp.MustCompile(`\((last_write_to_\d+) #x[0-9a-f]+ #x([0-9a-f]+)`)
	widened := regexp.MustCompile(`\((last_write_to_\d+) #x[0-9a-f]+ \(\(_ zero_extend (\d+)\) `)
	calls := []predicateCall{}
	for _, match := range literal.FindAllStringSubmatch(script, -1) {
		calls = append(calls, predicateCall{name: match[1], valueWidth: len(match[2]) * 4})
	}
	for _, match := range widened.FindAllStringSubmatch(script, -1) {
		added, err := strconv.Atoi(match[2])
		if err != nil {
			t.Fatalf("extension width %q: %v", match[2], err)
		}
		size, err := strconv.Atoi(strings.TrimPrefix(match[1], "last_write_to_"))
		if err != nil {
			t.Fatalf("predicate size %q: %v", match[1], err)
		}
		calls = append(calls, predicateCall{name: match[1], valueWidth: added + size})
	}
	if len(calls) == 0 {
		t.Fatal("solver script called no memory predicate")
	}
	return calls
}
