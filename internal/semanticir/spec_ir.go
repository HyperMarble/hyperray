package semanticir

import "fmt"

const SpecIRSchemaV1 = "ray-spec-semantic-ir/v1"

// CanonicalSpecIRDigest binds the frozen spec source and its provenance
// artifacts to the one canonical typed Spec model (D, O, constraints, R, and
// invariants). Test models are deliberately absent.
//
// Instruction and Reference are each optional but at least one must be
// present, because a row anchors into one of them. Benchmark prompts vary in
// style and withhold the rubric by design, so a spec may cite only the
// reference solution. Both are bound so neither can be swapped after freezing.
func CanonicalSpecIRDigest(task *Task) (string, error) {
	if task == nil {
		return "", fmt.Errorf("task is nil")
	}
	if err := validateArtifactRef(task.Spec); err != nil {
		return "", fmt.Errorf("spec: %w", err)
	}
	if task.Instruction.ID == "" && task.Reference.ID == "" {
		return "", fmt.Errorf("no provenance artifact: want an instruction, a reference, or both")
	}
	if task.Instruction.ID != "" {
		if err := validateArtifactRef(task.Instruction); err != nil {
			return "", fmt.Errorf("instruction: %w", err)
		}
	}
	if task.Reference.ID != "" {
		if err := validateArtifactRef(task.Reference); err != nil {
			return "", fmt.Errorf("reference: %w", err)
		}
	}
	semanticsDigest, err := FrozenSpecSemanticsDigest(task)
	if err != nil {
		return "", err
	}
	return Digest(struct {
		Schema          string      `json:"schema"`
		TaskID          string      `json:"task_id"`
		Spec            ArtifactRef `json:"spec"`
		Instruction     ArtifactRef `json:"instruction"`
		Reference       ArtifactRef `json:"reference"`
		SemanticsDigest string      `json:"semantics_digest"`
	}{SpecIRSchemaV1, task.ID, task.Spec, task.Instruction, task.Reference, semanticsDigest})
}

// ValidateSpecIRDigest is the focused stale/tamper check used by proof,
// pipeline, and certificates before consuming Task's compiled Spec fields.
func ValidateSpecIRDigest(task *Task) []Diagnostic {
	if task == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticInvalidInput, "spec IR task is nil", Provenance{})}
	}
	want, err := CanonicalSpecIRDigest(task)
	if err != nil {
		return []Diagnostic{errorDiagnostic(DiagnosticInvalidInput, "rebuild canonical Spec IR: "+err.Error(), task.Provenance)}
	}
	if !ValidDigest(task.SpecIRDigest) || task.SpecIRDigest != want {
		return []Diagnostic{errorDiagnostic(DiagnosticStaleArtifact, "task Spec IR digest differs from canonical compiled D/O/R model", task.Provenance)}
	}
	return nil
}
