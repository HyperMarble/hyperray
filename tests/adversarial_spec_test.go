package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/speccompiler"
)

// Adversarial battery: every case is a spec that tries to get a defect past
// strict compilation. The gate's worth is measured here -- a mutation that
// compiles is a hole an author's mistake (or a shortcut) walks through. The
// battery found one escape on its first run: an Observe declaration for a
// label no row uses was accepted silently, which is exactly the shape of a
// typo'd observer.

const adversarialBase = `# T

## Op

Universe: act.mode = values ["on","off"].
Scope: act = the frozen witnesses.
Classify: act = command c.sh.
Observe: act."yes" = command o1.sh.
Observe: act."no" = command o2.sh.
Inputs: act(mode: string).
Grounding: act.mode."on" = when mode == "on"; witness {"mode":"on"}.
Grounding: act.mode."off" = when mode == "off"; witness {"mode":"off"}.

Parameters: ` + "`mode`" + ` ("on" / "off").

| mode | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Input witnesses | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|---|
| "on" | REQ-on | act | reachable | return "yes" | return "no"; other outcome | none | none | [{"mode":"on"}] | none | 1 | — |
| "off" | REQ-off | act | reachable | return "no" | return "yes"; other outcome | none | none | [{"mode":"off"}] | none | 1 | — |
`

const (
	adversarialRowOn  = `| "on" | REQ-on | act | reachable | return "yes" | return "no"; other outcome | none | none | [{"mode":"on"}] | none | 1 | — |`
	adversarialRowOff = `| "off" | REQ-off | act | reachable | return "no" | return "yes"; other outcome | none | none | [{"mode":"off"}] | none | 1 | — |`
)

func TestAdversarialSpecs(t *testing.T) {
	replace := func(old, new string) string { return strings.Replace(adversarialBase, old, new, 1) }
	cases := []struct {
		name   string
		spec   string
		accept bool
	}{
		{"control-valid", adversarialBase, true},
		{"wildcard-overlap", replace(adversarialRowOff, strings.Replace(adversarialRowOff, `| "off" |`, `| any |`, 1)), false},
		{"missing-combination", replace(adversarialRowOff+"\n", ""), false},
		{"required-in-forbidden", replace(`| return "yes" | return "no"; other outcome |`, `| return "yes" | return "yes"; other outcome |`), false},
		{"duplicate-row-id", replace("REQ-off", "REQ-on"), false},
		{"witness-contradicts-row", replace(`[{"mode":"on"}]`, `[{"mode":"off"}]`), false},
		{"universe-duplicate-value", replace(`values ["on","off"]`, `values ["on","on"]`), false},
		{"grounding-outside-universe", replace(`when mode == "off"; witness {"mode":"off"}`, `when mode == "blah"; witness {"mode":"blah"}`), false},
		{"undeclared-value-in-row", replace(adversarialRowOff, strings.Replace(adversarialRowOff, `| "off" |`, `| "maybe" |`, 1)), false},
		{"empty-forbidden-set", replace(`| return "yes" | return "no"; other outcome |`, `| return "yes" | none |`), false},
		{"evidence-span-past-eof", replace(adversarialRowOn, strings.Replace(adversarialRowOn, `| none | 1 |`, `| none | 99 |`, 1)), false},
		{"noncompact-witness", replace(`[{"mode":"on"}]`, `[{"mode": "on"}]`), false},
		{"orphan-observer", replace(`Observe: act."no" = command o2.sh.`, "Observe: act.\"no\" = command o2.sh.\nObserve: act.\"typo-label\" = command o3.sh."), false},
		{"alphabet-not-closed", replace(`| return "yes" | return "no"; other outcome |`, `| return "yes" | return "no" |`), false},
		{"blank-scope-text", replace("Scope: act = the frozen witnesses.", "Scope: act = ."), false},
		{"reference-anchor-unfrozen", replace(`| none | 1 |`, `| none | reference:1 |`), false},
	}
	instruction := []byte("Do the thing correctly.\n")
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			spec := []byte(testCase.spec)
			task, diagnostics := speccompiler.Compile(context.Background(), speccompiler.Request{
				TaskID: "adversarial",
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
			})
			accepted := task != nil && !semanticir.HasErrors(diagnostics)
			if accepted && !testCase.accept {
				t.Errorf("the gate accepted this defect; a spec author's mistake of this shape now compiles")
			}
			if !accepted && testCase.accept {
				t.Errorf("the gate rejected the valid control: %v", diagnostics)
			}
		})
	}
}
