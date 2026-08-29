// Package runner abstracts the task's test framework: list tests, run one,
// run in a given order, and parse failing names. The rungs never carry
// framework syntax themselves.
package runner

import (
	"fmt"
	"regexp"
	"strings"
)

// Runner is one language's answers. All commands are shell strings executed
// with the task's source root as working directory.
type Runner struct {
	language    string
	python      string
	testFile    string
	testCommand string
	// gtestBinary, when set on a cpp task, switches the runner from ctest
	// to driving that googletest binary directly -- the shape real
	// platform C++ tasks use (test.sh execs the binary with a filter).
	gtestBinary string
}

// New picks the language's runner. python interpreter and testFile matter
// only to the Python runner; testCommand is the task's frozen verifier.
func New(language, python, testFile, testCommand string) (Runner, error) {
	switch language {
	case "", "python", "rust", "cpp":
		return Runner{language: normalize(language), python: python, testFile: testFile, testCommand: testCommand}, nil
	default:
		return Runner{}, fmt.Errorf("runner: unsupported language %q (python, rust, cpp)", language)
	}
}

// WithGtestBinary switches a cpp runner to googletest-binary mode.
func (r Runner) WithGtestBinary(binary string) Runner {
	r.gtestBinary = binary
	return r
}

func normalize(language string) string {
	if language == "" {
		return "python"
	}
	return language
}

// Language reports the normalized language name.
func (r Runner) Language() string { return r.language }

// ListCommand prints one test id per line.
func (r Runner) ListCommand() string {
	switch r.language {
	case "rust":
		return r.cargoInvocation() + " -- --list 2>/dev/null | sed -n 's/: test$//p'"
	case "cpp":
		if r.gtestBinary != "" {
			// The frozen verifier's own filter scopes the listing to the
			// task's suite; without it the whole binary's thousands of
			// tests flood every per-test rung.
			return r.gtestBinary + r.frozenGtestFilter() + ` --gtest_list_tests | awk '/^[A-Za-z_].*\.$/{suite=$1} /^  /{print suite $1}'`
		}
		return "ctest -N | sed -n 's/^ *Test *#[0-9]*: //p'"
	default:
		return fmt.Sprintf("%s -m pytest -q -o addopts= --collect-only %s | grep '::'", r.python, r.testFile)
	}
}

// OneTestCommand runs exactly the named test.
func (r Runner) OneTestCommand(test string) string {
	switch r.language {
	case "rust":
		return fmt.Sprintf("%s '%s' -- --exact", r.cargoInvocation(), test)
	case "cpp":
		if r.gtestBinary != "" {
			return fmt.Sprintf("%s --gtest_filter='%s'", r.gtestBinary, test)
		}
		return fmt.Sprintf("ctest --output-on-failure -R '^%s$'", regexp.QuoteMeta(test))
	default:
		return fmt.Sprintf("%s -m pytest -q -o addopts= '%s::%s'", r.python, r.testFile, test)
	}
}

// SuiteCommand runs the whole suite so that failing test names appear in
// the output FailedNames can read.
func (r Runner) SuiteCommand() string {
	switch r.language {
	case "rust", "cpp":
		return r.testCommand
	default:
		return r.testCommand + " -rf"
	}
}

// FastKillSuffix stops the suite at the first failure where the framework
// supports it; breaker runs only need "did anything fail".
func (r Runner) FastKillSuffix() string {
	switch r.language {
	case "rust":
		return "" // libtest reports all failures; no stable stop-at-first flag
	case "cpp":
		if r.gtestBinary != "" {
			return " --gtest_fail_fast"
		}
		return " --stop-on-failure"
	default:
		return " -x"
	}
}

// OrderedCommand runs exactly the given tests in the given order. Pytest
// honours the order of node ids in one invocation; cargo and ctest get one
// invocation per test, chained, with a preserved combined exit code.
func (r Runner) OrderedCommand(ids []string) string {
	if r.language == "python" {
		quoted := make([]string, len(ids))
		for index, id := range ids {
			// Parametrized ids carry [brackets]; unquoted they are shell
			// globs and pytest receives nothing.
			quoted[index] = "'" + id + "'"
		}
		return fmt.Sprintf("%s -m pytest -q -o addopts= %s", r.python, strings.Join(quoted, " "))
	}
	var steps []string
	for _, id := range ids {
		steps = append(steps, r.OneTestCommand(id)+" || rayfail=1")
	}
	return "rayfail=0; " + strings.Join(steps, "; ") + "; exit $rayfail"
}

var (
	pytestFailedPattern = regexp.MustCompile(`FAILED [^\s:]*::([A-Za-z0-9_\[\]-]+)`)
	gtestFailedPattern  = regexp.MustCompile(`(?m)^\[  FAILED  \] (\S+?)(?: \(.*\))?$`)
	cargoFailedPattern  = regexp.MustCompile("(?m)^(?:test )?(\\S+) (?:\\.\\.\\.|---) FAILED$")
	ctestFailedPattern  = regexp.MustCompile(`(?m)^\s*\d+ - (\S+) \((Failed|Timeout|Subprocess aborted)\)`)
)

// frozenGtestFilter lifts the --gtest_filter argument out of the frozen
// verifier command, so listing and the suite agree on scope.
func (r Runner) frozenGtestFilter() string {
	for _, field := range strings.Fields(r.testCommand) {
		if strings.HasPrefix(field, "--gtest_filter=") {
			return " '" + field + "'"
		}
	}
	return ""
}

// cargoInvocation reuses the task's own frozen cargo command -- its
// package, test-target, and feature flags scope compilation to the task --
// falling back to a bare cargo test when the verifier is a wrapper script.
func (r Runner) cargoInvocation() string {
	if strings.HasPrefix(r.testCommand, "cargo test") {
		return r.testCommand
	}
	return "cargo test --quiet"
}

// FailedNames reads failing test names from the framework's own report.
func (r Runner) FailedNames(output string) []string {
	pattern := pytestFailedPattern
	switch r.language {
	case "rust":
		pattern = cargoFailedPattern
	case "cpp":
		pattern = ctestFailedPattern
		if r.gtestBinary != "" {
			pattern = gtestFailedPattern
		}
	}
	seen := map[string]bool{}
	var names []string
	for _, match := range pattern.FindAllStringSubmatch(output, -1) {
		if !seen[match[1]] {
			seen[match[1]] = true
			names = append(names, match[1])
		}
	}
	return names
}
