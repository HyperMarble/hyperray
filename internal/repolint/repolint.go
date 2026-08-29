// Package repolint runs the host repository's own configured linters on the
// solution files: a gate exists only where the repo configures one, and a
// missing tool blocks rather than silently passing.
package repolint

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Status is the four-way outcome of one linter's gate.
type Status string

const (
	// StatusNoConfig means the repository configures no supported linter,
	// so there is no gate to enforce.
	StatusNoConfig Status = "no-config"
	// StatusBlocked means the repository configures a linter but the tool
	// cannot run here, so the gate cannot be evaluated.
	StatusBlocked Status = "blocked"
	// StatusClean means the repository's own linter accepted every file.
	StatusClean Status = "clean"
	// StatusFindings means the repository's own linter rejected the files;
	// Output carries the tool's report.
	StatusFindings Status = "findings"
)

// Result is one linter's gate evaluation.
type Result struct {
	Status Status
	Tool   string
	Output string
}

// linter pairs a config detector with the command that runs the tool.
// applies keeps a configured linter quiet when the solution never touches
// its language.
type linter struct {
	tool       string
	configured func(sourceRoot string) bool
	applies    func(files []string) bool
	run        func(sourceRoot string, files []string) Result
}

var linters = []linter{
	{
		tool:       "ruff",
		configured: ruffConfigured,
		applies:    touchesExtensions(".py"),
		run:        runRuff,
	},
	{
		tool:       "cargo clippy",
		configured: clippyConfigured,
		applies:    touchesExtensions(".rs"),
		run:        runClippy,
	},
	{
		tool:       "clang-format",
		configured: hasRootFile(".clang-format"),
		applies:    touchesExtensions(".c", ".cc", ".cpp", ".cxx", ".h", ".hpp"),
		run:        runClangFormat,
	},
	{
		tool:       "clang-tidy",
		configured: hasRootFile(".clang-tidy"),
		applies:    touchesExtensions(".c", ".cc", ".cpp", ".cxx", ".h", ".hpp"),
		run:        runClangTidy,
	},
}

// Check evaluates every linter the repository configures against the given
// files (paths relative to sourceRoot). An empty slice means the repository
// configures no linter relevant to these files: there is no gate.
func Check(sourceRoot string, files []string) []Result {
	var results []Result
	for _, l := range linters {
		if !l.configured(sourceRoot) || !l.applies(files) {
			continue
		}
		results = append(results, l.run(sourceRoot, files))
	}
	return results
}

// touchesExtensions reports whether any solution file has one of the
// language's extensions.
func touchesExtensions(extensions ...string) func([]string) bool {
	return func(files []string) bool {
		for _, file := range files {
			for _, extension := range extensions {
				if strings.HasSuffix(file, extension) {
					return true
				}
			}
		}
		return false
	}
}

// hasRootFile reports whether the repository root carries the named config
// file.
func hasRootFile(name string) func(string) bool {
	return func(sourceRoot string) bool {
		_, err := os.Stat(filepath.Join(sourceRoot, name))
		return err == nil
	}
}

// runTool runs one lint command in the source root and folds its exit code
// into the four-way result. Lint tools report violations through a nonzero
// exit, so any error with output is findings, and an error without output
// is blocked.
func runTool(tool, sourceRoot string, arguments []string) Result {
	binary := arguments[0]
	if _, err := exec.LookPath(binary); err != nil {
		return Result{Status: StatusBlocked, Tool: tool, Output: binary + " is configured by the repository but not installed"}
	}
	command := exec.Command(binary, arguments[1:]...)
	command.Dir = sourceRoot
	output, err := command.CombinedOutput()
	report := strings.TrimSpace(string(output))
	if err != nil {
		return Result{Status: StatusFindings, Tool: tool, Output: report}
	}
	return Result{Status: StatusClean, Tool: tool, Output: report}
}

// ruffConfigured reports whether the repository itself sets up Ruff: a
// dedicated ruff config file, or a [tool.ruff] section in pyproject.toml.
func ruffConfigured(sourceRoot string) bool {
	if hasRootFile(".ruff.toml")(sourceRoot) || hasRootFile("ruff.toml")(sourceRoot) {
		return true
	}
	pyproject, err := os.ReadFile(filepath.Join(sourceRoot, "pyproject.toml"))
	if err != nil {
		return false
	}
	return strings.Contains(string(pyproject), "[tool.ruff")
}

func runRuff(sourceRoot string, files []string) Result {
	return runTool("ruff", sourceRoot, append([]string{"ruff", "check"}, files...))
}

// clippyConfigured reports whether the repository itself sets up clippy: a
// clippy.toml, or a [lints] table in Cargo.toml.
func clippyConfigured(sourceRoot string) bool {
	if hasRootFile("clippy.toml")(sourceRoot) || hasRootFile(".clippy.toml")(sourceRoot) {
		return true
	}
	manifest, err := os.ReadFile(filepath.Join(sourceRoot, "Cargo.toml"))
	if err != nil {
		return false
	}
	return strings.Contains(string(manifest), "[lints")
}

// runClippy lints the whole workspace: clippy has no per-file mode, and the
// repository's own gate runs workspace-wide too.
func runClippy(sourceRoot string, files []string) Result {
	return runTool("cargo clippy", sourceRoot, []string{"cargo", "clippy", "--quiet", "--no-deps", "--", "--deny", "warnings"})
}

func runClangFormat(sourceRoot string, files []string) Result {
	return runTool("clang-format", sourceRoot, append([]string{"clang-format", "--dry-run", "--Werror"}, files...))
}

// runClangTidy needs the compilation database the repository's own gate
// uses; without one the check is blocked, not skipped.
func runClangTidy(sourceRoot string, files []string) Result {
	if !hasRootFile("compile_commands.json")(sourceRoot) && !hasRootFile(filepath.Join("build", "compile_commands.json"))(sourceRoot) {
		return Result{Status: StatusBlocked, Tool: "clang-tidy", Output: "clang-tidy is configured but no compile_commands.json exists in the repository"}
	}
	return runTool("clang-tidy", sourceRoot, append([]string{"clang-tidy"}, files...))
}
