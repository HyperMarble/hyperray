package semanticir

import (
	"fmt"
	"reflect"
	"sort"
)

// NormalizeReferenceCases is the only Spec-vocabulary join performed for an
// independently translated reference model. The request contains D/O scope
// but never RequirementCase rows. Each raw runtime trace is classified by its
// exact terminal/value/exception/effect semantics; an unmatched trace maps to
// the operation-local Other outcome.
func NormalizeReferenceCases(request FrontendRequest, rawCases []RawReferenceCase) ([]BehaviorCase, []Diagnostic) {
	operations := make(map[string]Operation, len(request.Operations))
	for _, operation := range request.Operations {
		operations[operation.ID] = operation
	}
	domainValues := domainValueRegistry(request.FiniteDomains)
	groundings := map[string]AssignmentGrounding{}
	for _, grounding := range request.Groundings {
		groundings[behaviorKey(grounding.OperationID, grounding.Conditions)] = grounding
	}

	var diagnostics []Diagnostic
	seen := map[string]struct{}{}
	normalized := make([]BehaviorCase, 0, len(rawCases))
	for _, raw := range rawCases {
		operation, exists := operations[raw.OperationID]
		if raw.ID == "" || !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "raw reference case has no ID or refers to an unknown operation", raw.Provenance))
			continue
		}
		if err := validateFactSource(raw.Provenance, request.Artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "raw reference case: "+err.Error(), raw.Provenance))
		}
		if err := validateAssignment(raw.Conditions, operationDomainValues(operation, domainValues)); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("raw reference case %q: %v", raw.ID, err), raw.Provenance))
			continue
		}
		declaredInputs := inputsByName(operation.Inputs)
		if raw.Inputs == nil || len(raw.Inputs) != len(declaredInputs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("raw reference case %q does not bind every operation input", raw.ID), raw.Provenance))
			continue
		}
		validInputs := true
		for name, literal := range raw.Inputs {
			input, declared := declaredInputs[name]
			if !declared || literal.Type != input.Type || ValidateLiteral(literal) != nil {
				validInputs = false
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("raw reference case %q has unknown or ill-typed input %q", raw.ID, name), raw.Provenance))
			}
		}
		grounding, grounded := groundings[behaviorKey(raw.OperationID, raw.Conditions)]
		if !validInputs || !grounded || !reflect.DeepEqual(grounding.Inputs, raw.Inputs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("raw reference case %q differs from the frozen exact input/state point", raw.ID), raw.Provenance))
			continue
		}
		if len(raw.Outcomes) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("raw reference case %q has no observed outcome", raw.ID), raw.Provenance))
			continue
		}
		outcomeSet := map[string]struct{}{}
		for _, trace := range raw.Outcomes {
			if err := ValidateRawOutcomeTrace(trace); err != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("raw reference case %q: %v", raw.ID, err), raw.Provenance))
				continue
			}
			outcomeID, err := ClassifyRawOutcome(operation, trace, raw.Provenance)
			if err != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("raw reference case %q: %v", raw.ID, err), raw.Provenance))
				continue
			}
			if _, declared := stringSet(operation.OutcomeIDs)[outcomeID]; !declared {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("operation %q observable alphabet has no exact/complement mapping for raw reference outcome", operation.ID), raw.Provenance))
				continue
			}
			outcomeSet[outcomeID] = struct{}{}
		}
		outcomeIDs := make([]string, 0, len(outcomeSet))
		for outcomeID := range outcomeSet {
			outcomeIDs = append(outcomeIDs, outcomeID)
		}
		sort.Strings(outcomeIDs)
		if len(outcomeIDs) == 0 {
			continue
		}
		behavior := BehaviorRef{OperationID: raw.OperationID, Conditions: raw.Conditions, Inputs: raw.Inputs}
		key := BehaviorRefKey(behavior)
		if _, duplicate := seen[key]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "raw reference translation repeats concrete point "+key, raw.Provenance))
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, BehaviorCase{ID: raw.ID, Conditions: raw.Conditions, OperationID: raw.OperationID, Inputs: raw.Inputs, OutcomeIDs: outcomeIDs, Provenance: raw.Provenance})
	}
	sort.Slice(normalized, func(i, j int) bool {
		left, right := BehaviorCaseKey(normalized[i]), BehaviorCaseKey(normalized[j])
		if left == right {
			return normalized[i].ID < normalized[j].ID
		}
		return left < right
	})
	return normalized, diagnostics
}
