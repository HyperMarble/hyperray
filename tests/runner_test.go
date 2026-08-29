// Tests for the language runner abstraction: each language answers the
// same four questions, and failing test names are read from each
// framework's real report format.
package tests

import (
	"strings"
	"testing"

	"github.com/HyperMarble/ray/internal/runner"
)

func mustRunner(t *testing.T, language string) runner.Runner {
	t.Helper()
	r, err := runner.New(language, "python3", "tests/test_x.py", "run-suite")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRunner_UnsupportedLanguageIsAnError(t *testing.T) {
	if _, err := runner.New("cobol", "", "", ""); err == nil {
		t.Fatal("expected an error for an unsupported language")
	}
}

func TestRunner_EmptyLanguageMeansPython(t *testing.T) {
	r := mustRunner(t, "")
	if r.Language() != "python" {
		t.Fatalf("expected python, got %q", r.Language())
	}
}

func TestRunner_PythonReadsPytestFailures(t *testing.T) {
	output := "FAILED tests/test_x.py::test_alpha - AssertionError\nFAILED tests/test_x.py::test_beta[case-1] - ValueError\n"
	names := mustRunner(t, "python").FailedNames(output)
	if len(names) != 2 || names[0] != "test_alpha" || names[1] != "test_beta[case-1]" {
		t.Fatalf("unexpected names: %v", names)
	}
}

func TestRunner_RustReadsCargoFailures(t *testing.T) {
	output := "test splits::chains_in_order ... ok\ntest splits::rejects_empty ... FAILED\n\nfailures:\n"
	names := mustRunner(t, "rust").FailedNames(output)
	if len(names) != 1 || names[0] != "splits::rejects_empty" {
		t.Fatalf("unexpected names: %v", names)
	}
}

func TestRunner_CppReadsCtestFailures(t *testing.T) {
	output := "The following tests FAILED:\n\t  3 - splitter_rejects_empty (Failed)\n"
	names := mustRunner(t, "cpp").FailedNames(output)
	if len(names) != 1 || names[0] != "splitter_rejects_empty" {
		t.Fatalf("unexpected names: %v", names)
	}
}

func TestRunner_OneTestCommandsPerLanguage(t *testing.T) {
	if got := mustRunner(t, "python").OneTestCommand("test_a"); !strings.Contains(got, "'tests/test_x.py::test_a'") {
		t.Fatalf("python one-test: %s", got)
	}
	if got := mustRunner(t, "rust").OneTestCommand("mod::test_a"); !strings.Contains(got, "cargo test --quiet 'mod::test_a' -- --exact") {
		t.Fatalf("rust one-test: %s", got)
	}
	if got := mustRunner(t, "cpp").OneTestCommand("test_a"); !strings.Contains(got, "-R '^test_a$'") {
		t.Fatalf("cpp one-test: %s", got)
	}
}

func TestRunner_OrderedCommandChainsNonPython(t *testing.T) {
	got := mustRunner(t, "rust").OrderedCommand([]string{"b", "a"})
	if !strings.Contains(got, "rayfail=0") || !strings.Contains(got, "exit $rayfail") {
		t.Fatalf("expected a chained command with preserved exit code: %s", got)
	}
	if strings.Index(got, "'b'") > strings.Index(got, "'a'") {
		t.Fatalf("expected b before a: %s", got)
	}
}

func TestRunner_PythonSuiteAddsReportingFlag(t *testing.T) {
	if got := mustRunner(t, "python").SuiteCommand(); !strings.HasSuffix(got, " -rf") {
		t.Fatalf("expected -rf suffix: %s", got)
	}
	if got := mustRunner(t, "rust").SuiteCommand(); got != "run-suite" {
		t.Fatalf("rust suite must be the frozen command untouched: %s", got)
	}
}

func TestRunner_GtestModeListsFiltersAndParses(t *testing.T) {
	r := mustRunner(t, "cpp").WithGtestBinary("./build/test/opt/test_opt")
	if got := r.ListCommand(); !strings.Contains(got, "--gtest_list_tests") {
		t.Fatalf("gtest list: %s", got)
	}
	if got := r.OneTestCommand("Suite.Case"); !strings.Contains(got, "--gtest_filter='Suite.Case'") {
		t.Fatalf("gtest one-test: %s", got)
	}
	output := "[ RUN      ] CopyPropArrayComponentTest.KeepsLoad\n[  FAILED  ] CopyPropArrayComponentTest.KeepsLoad (12 ms)\n[  FAILED  ] 1 test, listed below:\n[  FAILED  ] CopyPropArrayComponentTest.KeepsLoad\n"
	names := r.FailedNames(output)
	if len(names) != 1 || names[0] != "CopyPropArrayComponentTest.KeepsLoad" {
		t.Fatalf("gtest failure names: %v", names)
	}
}
