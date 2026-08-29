package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/internal/semanticir"
	"github.com/HyperMarble/hyperray/internal/speccompiler"
	"github.com/HyperMarble/hyperray/internal/specparser"
)

func readSkillFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{"..", "skills"}, parts...)...)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func normalizeSkillText(source string) string {
	return strings.Join(strings.Fields(source), " ")
}

func TestSpecSkillWorkflow(t *testing.T) {
	skill := readSkillFile(t, "spec", "SKILL.md")
	normalized := normalizeSkillText(skill)

	phase1 := strings.Index(skill, "## Phase 1: derive semantics before tests")
	freeze := strings.Index(skill, "### Freeze before test access")
	phase2 := strings.Index(skill, "## Phase 2: map and compare the real tests")
	if phase1 < 0 || freeze <= phase1 || phase2 <= freeze {
		t.Fatalf("workflow order is not derive -> freeze -> compare tests: phase1=%d freeze=%d phase2=%d", phase1, freeze, phase2)
	}

	pretest := skill[phase1:phase2]
	for _, source := range []string{
		"complete instruction, issue, or PR description",
		"declared base commit",
		"complete reference solution or PR diff",
		"Relevant environment facts",
		"relevant callers",
		"dependency behavior",
	} {
		if !strings.Contains(pretest, source) {
			t.Errorf("pre-test derivation omits %q", source)
		}
	}
	for _, boundary := range []string{
		"do not open, search, run, summarize, or ask another agent to inspect test files",
		"Tests never supply a parameter, domain value, constraint, required outcome, effect, state transition, or provenance anchor",
		"apply only the reference patch",
		"Do not apply or inspect the test patch",
		"every `Enforced by` cell is `none`",
		"preserve immutable pre-test bytes",
	} {
		if !strings.Contains(normalized, boundary) {
			t.Errorf("workflow omits evidence boundary %q", boundary)
		}
	}

	posttest := normalizeSkillText(skill[phase2:])
	for _, rule := range []string{
		"both public and hidden tests",
		"runner, setup/teardown, ordering, shared state, test command, and authoritative pass signal",
		"leave `none` when no test actually observes a row",
		"one global predicate over a full implementation behavior",
		"False-positive query",
		"False-negative query",
		"T(C) = true",
		"Only `Enforced by` cells may change",
		"restart phase 1",
	} {
		if !strings.Contains(posttest, rule) {
			t.Errorf("post-freeze comparison omits %q", rule)
		}
	}

	record := normalizeSkillText(readSkillFile(t, "spec", "templates", "authoring-record.md"))
	for _, field := range []string{
		"Test access during authoring",
		"not accessed",
		"Phase-1 source manifest",
		"Exact finite domains",
		"Impossible-case constraints",
		"Full-N-way row review",
		"Cross-clause and state interactions",
		"Source disagreements",
		"Frozen pre-test bytes and SHA-256 recorded before test access",
	} {
		if !strings.Contains(record, field) {
			t.Errorf("authoring record omits %q", field)
		}
	}
}

func TestSpecSkillSchema(t *testing.T) {
	skill := readSkillFile(t, "spec", "SKILL.md")
	normalized := normalizeSkillText(skill)
	for _, rule := range []string{
		"operation-local domains",
		"full Cartesian product",
		"exactly one row",
		"`Constraint reason`",
		"Full-N-way means simultaneous clauses remain simultaneous",
		"return or success result, exact value, type, label, and shape",
		"reads, writes, calls, outputs, mutations, and state transitions",
		"permitted alternatives",
		"Required and forbidden sets must be nonempty, disjoint",
		"globally unique stable requirement or constraint ID",
		// The Evidence column replaced the instruction-only span: a bare span
		// cites the instruction, reference:<span> cites the reference solution.
		"a span into the frozen instruction or the frozen reference",
		"reference:94-101",
		"Prose outside the strict table is not a graded property",
		"patch-shaped benchmark",
	} {
		if !strings.Contains(normalized, rule) {
			t.Errorf("schema guidance omits %q", rule)
		}
	}

	template := readSkillFile(t, "spec", "templates", "spec.md")
	tables, err := specparser.Parse(template)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	if len(tables) != 2 {
		t.Fatalf("template tables=%d, want two operation-local examples", len(tables))
	}
	requiredHeaders := map[string]bool{
		speccompiler.HeaderID:                true,
		speccompiler.HeaderOperation:         true,
		speccompiler.HeaderReachability:      true,
		speccompiler.HeaderRequiredOutcomes:  true,
		speccompiler.HeaderForbiddenOutcomes: true,
		speccompiler.HeaderEffects:           true,
		speccompiler.HeaderInvariants:        true,
		speccompiler.HeaderInputWitnesses:    true,
		speccompiler.HeaderEnforcedBy:        true,
		speccompiler.HeaderEvidence:          true,
		speccompiler.HeaderConstraintReason:  true,
	}
	for _, table := range tables {
		domains, unsupported, parseErr := specparser.ParseParams(table.Params)
		if parseErr != nil || unsupported != "" || len(domains) != 2 {
			t.Fatalf("table %q domains=%v unsupported=%q err=%v", table.Section, domains, unsupported, parseErr)
		}
		wantRows := 1
		for _, domain := range domains {
			wantRows *= len(domain.Values)
		}
		if len(table.Rows) != wantRows {
			t.Errorf("table %q rows=%d, want complete full product %d", table.Section, len(table.Rows), wantRows)
		}
		seen := map[string]int{}
		for index, column := range table.Columns {
			seen[column] = index
		}
		for header := range requiredHeaders {
			if _, exists := seen[header]; !exists {
				t.Errorf("table %q omits strict header %q", table.Section, header)
			}
		}
		mappingIndex := seen[speccompiler.HeaderEnforcedBy]
		for rowIndex, row := range table.Rows {
			if strings.TrimSpace(strings.ReplaceAll(row[mappingIndex], "`", "")) != "none" {
				t.Errorf("table %q row %d is not test-blind", table.Section, rowIndex+1)
			}
		}
	}
}

func compileSpecSkillTemplate(t *testing.T, source string) (*semanticir.Task, []semanticir.Diagnostic) {
	t.Helper()
	instruction := []byte("Persist valid payloads to writable targets; invalid payload validation takes precedence over read-only rejection.\nRead and write active sessions, reject reads from closed sessions, and closed sessions cannot receive write requests.\n")
	specBytes := []byte(source)
	return speccompiler.Compile(context.Background(), speccompiler.Request{
		TaskID: "spec-skill-template",
		Artifact: semanticir.ArtifactRef{
			ID: "spec", Kind: semanticir.ArtifactSpec, Path: "spec.md", Digest: semanticir.DigestBytes(specBytes),
		},
		Source: specBytes,
		Instruction: semanticir.ArtifactRef{
			ID: "instruction", Kind: semanticir.ArtifactInstruction, Path: "instruction.md", Digest: semanticir.DigestBytes(instruction),
		},
		InstructionSource: instruction,
	})
}

func TestSpecSkillTemplate(t *testing.T) {
	template := readSkillFile(t, "spec", "templates", "spec.md")
	task, diagnostics := compileSpecSkillTemplate(t, template)
	if task == nil || semanticir.HasErrors(diagnostics) {
		t.Fatalf("corrected strict template does not compile: task=%v diagnostics=%+v", task, diagnostics)
	}
	if len(task.Operations) != 2 || len(task.Requirements) != 7 || len(task.Constraints) != 1 {
		t.Fatalf("compiled full-N-way shape operations=%d requirements=%d constraints=%d", len(task.Operations), len(task.Requirements), len(task.Constraints))
	}
	if !semanticir.ValidDigest(task.SpecIRDigest) || semanticir.HasErrors(semanticir.ValidateSpecIRDigest(task)) {
		t.Fatalf("template did not compile to canonical Spec Semantic IR: digest=%q diagnostics=%+v", task.SpecIRDigest, semanticir.ValidateSpecIRDigest(task))
	}
	if len(task.CodeCases) != 0 || len(task.Tests) != 0 {
		t.Fatalf("spec compilation copied reference/test semantics: code=%d tests=%d", len(task.CodeCases), len(task.Tests))
	}
	constraint := task.Constraints[0]
	if constraint.OperationID != "session_request" || constraint.Conditions["session_state"] != "closed" || constraint.Conditions["request_kind"] != "write" || strings.TrimSpace(constraint.Reason) == "" {
		t.Fatalf("excluded full assignment lost its exact constraint: %+v", constraint)
	}
	requiredEffectTargets := map[string]bool{}
	for _, requirement := range task.Requirements {
		if len(requirement.RequiredOutcomes) == 0 || len(requirement.ForbiddenOutcomes) == 0 || len(requirement.InstructionClauseIDs) == 0 {
			t.Errorf("requirement is not closed/provenanced: %+v", requirement)
		}
		if len(requirement.TestIDs) != 0 {
			t.Errorf("pre-test template compiled test-derived mapping: %+v", requirement)
		}
		for _, effect := range requirement.Effects {
			requiredEffectTargets[effect.Target] = true
		}
	}
	for _, target := range []string{"target", "session"} {
		if !requiredEffectTargets[target] {
			t.Errorf("template does not compile observable effect target %q", target)
		}
	}

	negative := []struct {
		name   string
		source string
		code   semanticir.DiagnosticCode
	}{
		{
			name: "missing-full-n-way-row",
			source: strings.Replace(template,
				"| \"active\" | \"write\" | REQ-session-active-write | session_request | reachable | return \"written\" | return \"value\"; raise StateError containing \"closed session\"; other outcome | write:session=\"updated\" | none | [{\"request\":\"write\",\"session\":\"active\"}] | none | 2 | — |\n",
				"", 1),
			code: semanticir.DiagnosticIncomplete,
		},
		{
			name: "excluded-without-constraint",
			source: strings.Replace(template,
				"| none | none | — | the frozen API cannot issue write requests for a closed session |",
				"| none | none | — | — |", 1),
			code: semanticir.DiagnosticInvalidInput,
		},
		{
			name:   "prose-graded-rule",
			source: "The implementation must always store valid data.\n\n" + template,
			code:   semanticir.DiagnosticProseRequirement,
		},
		{
			name: "open-outcome-partition",
			source: strings.Replace(template,
				"raise PermissionError containing \"read-only target\"; other outcome",
				"raise PermissionError containing \"read-only target\"", 1),
			code: semanticir.DiagnosticIncomplete,
		},
	}
	for _, test := range negative {
		t.Run(test.name, func(t *testing.T) {
			compiled, got := compileSpecSkillTemplate(t, test.source)
			if compiled != nil || !diagnosticsContain(got, test.code) {
				t.Fatalf("invalid template mutation accepted: task=%v diagnostics=%+v", compiled, got)
			}
		})
	}
}

func diagnosticsContain(diagnostics []semanticir.Diagnostic, code semanticir.DiagnosticCode) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func TestTaskSkillFormalPipeline(t *testing.T) {
	skill := readSkillFile(t, "task", "SKILL.md")
	normalized := normalizeSkillText(skill)
	for _, rule := range []string{
		"instruction, issue, or PR description",
		"Exact base repository commit",
		"Complete reference solution or PR diff",
		"Do not open public or hidden tests",
		"every complete full-N-way input/state combination",
		"Update only `Enforced by`",
		"one exact global predicate `T(F)`",
		"False positive",
		"False negative",
		"simplified mathematical oracle",
		"solver `UNKNOWN` blocks the task",
		"exact oracle/reference inputs used by `diff-test`",
		"PICT scenarios must be derived from the frozen Spec parameter domains",
		"Record it as `N/A` only with evidence",
		"EXISTS x,o: C(x,o) AND NOT R(x,o)",
		"EXISTS F: T(F) AND EXISTS x: NOT R(x,F(x))",
		"EXISTS F: (FORALL x: R(x,F(x))) AND NOT T(F)",
		"T(C)",
		"`VERIFIED`",
		"`NOT VERIFIED`",
		"`PROOF BLOCKED`",
	} {
		if !strings.Contains(normalized, rule) {
			t.Errorf("task skill omits frozen-pipeline rule %q", rule)
		}
	}

	mapping := normalizeSkillText(readSkillFile(t, "spec", "templates", "mapping-review.md"))
	for _, field := range []string{
		"Public test artifact paths and SHA-256 values",
		"Hidden test artifact paths and SHA-256 values",
		"Requirement-to-test mapping",
		"Test-only restriction inventory",
		"Global verifier predicate review",
		"False-positive direction",
		"False-negative direction",
		"Exact reference acceptance",
		"Only final `Enforced by` cells changed",
	} {
		if !strings.Contains(mapping, field) {
			t.Errorf("mapping review omits %q", field)
		}
	}
}

func TestSpecSkillRejectsObsoleteArchitecture(t *testing.T) {
	files := []string{
		readSkillFile(t, "spec", "SKILL.md"),
		readSkillFile(t, "spec", "references", "examples.md"),
		readSkillFile(t, "spec", "templates", "spec.md"),
		readSkillFile(t, "spec", "templates", "authoring-record.md"),
		readSkillFile(t, "spec", "templates", "mapping-review.md"),
		readSkillFile(t, "task", "SKILL.md"),
	}
	joined := strings.ToLower(strings.Join(files, "\n"))
	for _, forbidden := range []string{
		"finite-adapter",
		"hyperray-adapter-v1",
		"generated verifier",
		"acceptance-init",
		"phase-a-authoring-record",
		"phase-a-freeze-ledger",
	} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("skill surface retains rejected architecture term %q", forbidden)
		}
	}
	for _, obsolete := range []string{
		"mutation score proves",
		"mutation testing proves",
		"pict proves",
		"sampling proves",
		"good to go",
	} {
		if strings.Contains(joined, obsolete) {
			t.Errorf("skill surface promotes diagnostic evidence via %q", obsolete)
		}
	}
}
