package proof

import (
	"fmt"

	"github.com/HyperMarble/ray/internal/semanticir"
)

// validateIndependentIR establishes the three non-property authorities used
// by the theorem: the independently translated reference C, the one global
// test predicate T, and the frozen environment. None of these canonical
// digests includes RequirementCase/R.
func (v *validator) validateIndependentIR(model *finiteModel) {
	checks := []struct {
		code        string
		validate    func(*semanticir.Task) []semanticir.Diagnostic
		digest      func(*semanticir.Task) (string, error)
		destination *string
	}{
		{"invalid-reference-ir", semanticir.ValidateReferenceIR, semanticir.CanonicalReferenceIRDigest, &model.referenceIRDigest},
		{"invalid-test-ir", semanticir.ValidateTestIR, semanticir.CanonicalTestIRDigest, &model.testIRDigest},
		{"invalid-environment-ir", semanticir.ValidateEnvironmentIR, semanticir.CanonicalEnvironmentIRDigest, &model.environmentIRDigest},
	}
	for _, check := range checks {
		for _, diagnostic := range check.validate(v.task) {
			if diagnostic.Severity != semanticir.SeverityError {
				continue
			}
			provenance := diagnostic.Provenance
			v.add(check.code, string(diagnostic.Code)+": "+diagnostic.Message, &provenance)
		}
		digest, err := check.digest(v.task)
		if err != nil {
			v.add(check.code, fmt.Sprintf("cannot reconstruct canonical digest: %v", err), nil)
			continue
		}
		*check.destination = digest
	}
}
