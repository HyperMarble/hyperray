package proof

import "github.com/HyperMarble/hyperray/internal/semanticir"

// validateSpecIR establishes the sole formal-property authority consumed by
// proof. The proof engine evaluates the typed Task registries only after their
// canonical compiler digest has been independently reconstructed; it never
// reads or interprets spec.md prose.
func (v *validator) validateSpecIR() {
	for _, diagnostic := range semanticir.ValidateSpecIRDigest(v.task) {
		if diagnostic.Severity != semanticir.SeverityError {
			continue
		}
		provenance := diagnostic.Provenance
		v.add("invalid-spec-ir", string(diagnostic.Code)+": "+diagnostic.Message, &provenance)
	}
}
