// Package repolint runs the host repository's own configured linter against
// the solution files. A reference solution must satisfy the lint gates the
// repository enforces on every contribution (for example Ruff's pydocstyle
// rules): a solution that violates them is rejected in review even when
// every test passes. Ray never invents style rules of its own -- when the
// repository configures no linter there is no gate, and when the tool is
// not installed the check is blocked, never silently passed.
package repolint

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Status is the four-way outcome of the gate.
type Status string

const (
	// StatusNoConfig means the repository configures no supported linter,
	// so there is no gate to enforce.
	StatusNoConfig Status = "no-config"
	// StatusBlocked means the repository configures a linter but the tool
	// is not installed here, so the gate cannot be evaluated.
	StatusBlocked Status = "blocked"
	// StatusClean means the repository's own linter accepted every file.
	StatusClean Status = "clean"
	// StatusFindings means the repository's own linter rejected the files;
	// Output carries the tool's report.
	StatusFindings Status = "findings"
)

// Result is one gate evaluation.
type Result struct {
	Status Status
	Tool   string
	Output string
}

// Check evaluates the repository's configured linter against the given
// files (paths relative to sourceRoot). Today the supported linter is Ruff
// for Python; other language linters are added as further detect/run pairs.
func Check(sourceRoot string, files []string) Result {
	if !ruffConfigured(sourceRoot) {
		return Result{Status: StatusNoConfig}
	}
	ruff, err := exec.LookPath("ruff")
	if err != nil {
		return Result{Status: StatusBlocked, Tool: "ruff", Output: "ruff is configured by the repository but not installed"}
	}
	command := exec.Command(ruff, append([]string{"check"}, files...)...)
	command.Dir = sourceRoot
	output, err := command.CombinedOutput()
	if err != nil {
		return Result{Status: StatusFindings, Tool: "ruff", Output: strings.TrimSpace(string(output))}
	}
	return Result{Status: StatusClean, Tool: "ruff", Output: strings.TrimSpace(string(output))}
}

// ruffConfigured reports whether the repository itself sets up Ruff: a
// dedicated ruff config file, or a [tool.ruff] section in pyproject.toml.
func ruffConfigured(sourceRoot string) bool {
	for _, name := range []string{".ruff.toml", "ruff.toml"} {
		if _, err := os.Stat(filepath.Join(sourceRoot, name)); err == nil {
			return true
		}
	}
	pyproject, err := os.ReadFile(filepath.Join(sourceRoot, "pyproject.toml"))
	if err != nil {
		return false
	}
	return strings.Contains(string(pyproject), "[tool.ruff")
}
