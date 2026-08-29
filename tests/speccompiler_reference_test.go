package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/internal/semanticir"
	"github.com/HyperMarble/hyperray/internal/speccompiler"
)

// A `reference:` Evidence anchor cites the frozen reference solution instead
// of the instruction (evidence-rule.md). spec-lint accepted these while the
// production pipeline never passed the reference artifact, so every anchored
// spec compiled at authoring time and failed inside hyperray check. The two paths
// share speccompiler.Compile; these tests pin the contract at that seam.

const referenceAnchoredSpec = `# Reference anchors

## Operation

Universe: act.mode = values ["on"].
Inputs: act(mode: string).
Grounding: act.mode."on" = when mode == "on"; witness {"mode":"on"}.

Parameters: ` + "`mode`" + ` ("on").

| mode | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Input witnesses | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|---|
| "on" | REQ-act-on | act | reachable | return unit | other outcome | none | none | [{"mode":"on"}] | none | reference:1 | — |
`

func compileWithReference(t *testing.T, withReference bool) []semanticir.Diagnostic {
	t.Helper()
	spec := []byte(referenceAnchoredSpec)
	instruction := []byte("Do the thing.\n")
	referenceSource := []byte("func act() {}\n")
	request := speccompiler.Request{
		TaskID: "reference-anchor-check",
		Artifact: semanticir.ArtifactRef{
			ID: "spec", Kind: semanticir.ArtifactSpec, Path: "spec.md",
			Digest: semanticir.DigestBytes(spec),
		},
		Source: spec,
		Instruction: semanticir.ArtifactRef{
			ID: "instruction", Kind: semanticir.ArtifactInstruction, Path: "instruction.md",
			Digest: semanticir.DigestBytes(instruction),
		},
		InstructionSource: instruction,
	}
	if withReference {
		request.Reference = semanticir.ArtifactRef{
			ID: "reference", Kind: semanticir.ArtifactCode, Path: "solution.patch",
			Digest: semanticir.DigestBytes(referenceSource),
		}
		request.ReferenceSource = referenceSource
	}
	task, diagnostics := speccompiler.Compile(context.Background(), request)
	if withReference && task == nil {
		t.Fatalf("reference-anchored spec did not compile with the reference frozen: %v", diagnostics)
	}
	return diagnostics
}

func TestReferenceAnchorCompilesWithFrozenReference(t *testing.T) {
	if diagnostics := compileWithReference(t, true); semanticir.HasErrors(diagnostics) {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestReferenceAnchorRejectedWithoutFrozenReference(t *testing.T) {
	diagnostics := compileWithReference(t, false)
	if !semanticir.HasErrors(diagnostics) {
		t.Fatal("a reference: anchor compiled with no reference artifact frozen; the anchor would cite bytes nothing verified")
	}
	found := false
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "no reference artifact was frozen") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the missing-reference explanation, got: %v", diagnostics)
	}
}
