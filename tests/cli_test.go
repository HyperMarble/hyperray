package tests

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIStartAndCheckShareFailClosedPipeline(t *testing.T) {
	binary := buildRayCLI(t)
	root := t.TempDir()
	var outputs []string
	for _, subcommand := range []string{"check", "start"} {
		command := exec.Command(binary, subcommand, root)
		output, err := command.CombinedOutput()
		if err == nil {
			t.Fatalf("ray %s succeeded without a frozen task", subcommand)
		}
		text := string(output)
		if !strings.Contains(text, "freeze: blocked") || !strings.Contains(text, "PROOF BLOCKED") {
			t.Fatalf("ray %s did not expose the fail-closed verdict:\n%s", subcommand, text)
		}
		outputs = append(outputs, text)
	}
	if outputs[0] != outputs[1] {
		t.Fatalf("check and start diverged:\ncheck:\n%s\nstart:\n%s", outputs[0], outputs[1])
	}
}

func TestCLISpecLintUsesStrictCompiler(t *testing.T) {
	binary := buildRayCLI(t)
	root := t.TempDir()
	instruction := "Return zero in zero mode and one in one mode.\n"
	spec := `# Strict spec

Inputs: choose(mode: string).
Grounding: choose.mode."zero" = when mode == "zero"; witness {"mode":"zero"}.
Grounding: choose.mode."one" = when mode == "one"; witness {"mode":"one"}.

Parameters: ` + "`mode`" + ` (zero / one).

| mode | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Input witnesses | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|---|
| zero | req-zero | choose | reachable | return 0 | return 1; other outcome | none | none | [{"mode":"zero"}] | none | 1 | — |
| one | req-one | choose | reachable | return 1 | return 0; other outcome | none | none | [{"mode":"one"}] | none | 1 | — |
`
	writeFixtureFile(t, root, "instruction.md", instruction, 0o644)
	writeFixtureFile(t, root, "spec.md", spec, 0o644)
	command := exec.Command(binary, "spec-lint", filepath.Join(root, "spec.md"),
		"--instruction", filepath.Join(root, "instruction.md"), "--task-id", "cli-spec-lint")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("strict spec-lint failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "spec: complete\nir: sha256:") || !strings.Contains(string(output), "\nfrozen-semantics: sha256:") {
		t.Fatalf("strict spec-lint omitted compiled evidence:\n%s", output)
	}
}

func buildRayCLI(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve repository root")
	}
	repository := filepath.Dir(filepath.Dir(source))
	binary := filepath.Join(t.TempDir(), "ray")
	command := exec.Command("go", "build", "-o", binary, "./cmd/ray")
	command.Dir = repository
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build ray CLI: %v\n%s", err, output)
	}
	return binary
}
