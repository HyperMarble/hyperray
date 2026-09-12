// This check type checks a captured solver script with z3 itself.
// It must use the solver as the authority, never a copy of its type rules.
//go:build isla_integration && arm64_acceptance

package isla_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// solverPath names the z3 executable that decides the verdict.
const solverPath = "HYPERRAY_Z3"

// capturedScriptDirectory holds solver scripts kept by an earlier real run.
const capturedScriptDirectory = "HYPERRAY_ARM64_SMT_CAPTURE"

// TestCapturedScriptTypeChecks fails when z3 rejects any declaration or call
// in a captured script. One rejected term makes the whole verdict void.
func TestCapturedScriptTypeChecks(t *testing.T) {
	solver := requiredSolver(t)
	failures := []string{}
	for _, path := range capturedScriptPaths(t) {
		if report := solverRejection(t, solver, path); report != "" {
			failures = append(failures, filepath.Base(path)+": "+report)
		}
	}
	if len(failures) != 0 {
		t.Fatalf("z3 rejected captured scripts: %s", strings.Join(failures, "; "))
	}
}

func requiredSolver(t *testing.T) string {
	t.Helper()
	solver := os.Getenv(solverPath)
	if solver == "" {
		t.Skipf("%s is empty, so no solver can type check the script", solverPath)
	}
	return solver
}

func capturedScriptPaths(t *testing.T) []string {
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
	return paths
}

// solverRejection returns z3's first complaint, or an empty string when every
// term type checks. The search commands are removed so only typing is tested.
func solverRejection(t *testing.T, solver string, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	declarations := declarationsOnly(string(content))
	stripped := filepath.Join(t.TempDir(), filepath.Base(path))
	if err := os.WriteFile(stripped, []byte(declarations), 0o600); err != nil {
		t.Fatalf("write stripped script: %v", err)
	}
	// z3 exits nonzero when it rejects a term, so its output carries the
	// result and an exit status alone cannot separate rejection from failure.
	output, runError := exec.CommandContext(t.Context(), solver, stripped).CombinedOutput()
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "(error") {
			return line
		}
	}
	if runError != nil {
		t.Fatalf("%s failed without naming a rejected term: %v", solver, runError)
	}
	return ""
}

// declarationsOnly keeps every definition and assertion, and drops the search
// so that a slow or unsatisfiable query cannot hide a typing failure.
func declarationsOnly(script string) string {
	end := strings.Index(script, "(check-sat")
	if end < 0 {
		return script
	}
	return script[:end]
}
