package semanticir

import (
	"fmt"
	"reflect"
)

// AddArtifactScope validates the exact request/model boundary, then
// transactionally merges the independently translated artifact.
func (task *Task) AddArtifactScope(request FrontendRequest, model ArtifactModel) []Diagnostic {
	diagnostics := ValidateArtifactScope(request, model)
	if HasErrors(diagnostics) {
		return diagnostics
	}
	return task.AddArtifact(model)
}

// AddArtifact validates and transactionally merges an independent frontend
// model. The frozen spec owns all vocabulary; a model may own an exact subset
// selected by its frontend request. Task.Validate later requires those subsets
// to cover every spec operation exactly once. On any diagnostic, task is
// unchanged.
func (task *Task) AddArtifact(model ArtifactModel) []Diagnostic {
	if task == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticInvalidInput, "cannot add an artifact to a nil task", Provenance{})}
	}
	diagnostics := ValidateArtifactModel(model)
	for _, existing := range task.Artifacts {
		if existing.Artifact.ID == model.Artifact.ID && existing.Artifact != model.Artifact {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticStaleArtifact, fmt.Sprintf("artifact %q is attached with different frozen identity", model.Artifact.ID), model.Coverage.Provenance))
		} else if existing.Artifact.ID == model.Artifact.ID && existing.Kind == model.Kind {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("artifact %q already has a %q translation", model.Artifact.ID, model.Kind), model.Coverage.Provenance))
		} else if existing.Artifact.ID == model.Artifact.ID && model.Artifact.Kind != ArtifactSource {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("artifact %q cannot carry multiple semantic roles without neutral source kind", model.Artifact.ID), model.Coverage.Provenance))
		}
	}
	if !domainVocabularySubset(task.Domains, model.Domains) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "artifact finite domains are not an exact subset of the frozen spec", model.Coverage.Provenance))
	}
	if !constraintSubset(task.Constraints, model.Constraints) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "artifact constraints are not an exact subset of the frozen spec", model.Coverage.Provenance))
	}

	operations := map[string]Operation{}
	for _, operation := range task.Operations {
		operations[operation.ID] = operation
	}
	for _, operation := range model.Operations {
		if operation.Kind == OperationTest {
			continue
		}
		declared, exists := operations[operation.ID]
		if !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("artifact refers to undeclared operation %q", operation.ID), operation.Provenance))
			continue
		}
		if declared.Kind != OperationCallable && declared.Kind != operation.Kind {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("artifact operation %q kind %q differs from spec kind %q", operation.ID, operation.Kind, declared.Kind), operation.Provenance))
		}
		if !reflect.DeepEqual(declared.DomainIDs, operation.DomainIDs) || !sameOperationInputs(declared.Inputs, operation.Inputs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("artifact operation %q domain scope differs from the frozen spec", operation.ID), operation.Provenance))
		}
		if !sameStringSet(declared.OutcomeIDs, operation.OutcomeIDs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("artifact operation %q outcome universe differs from the frozen spec", operation.ID), operation.Provenance))
		}
	}

	outcomes := map[string]ObservableOutcome{}
	for _, outcome := range task.Outcomes {
		outcomes[outcome.ID] = outcome
	}
	for _, outcome := range model.Outcomes {
		declared, exists := outcomes[outcome.ID]
		if !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("artifact introduces outcome %q outside the frozen spec universe", outcome.ID), outcome.Provenance))
			continue
		}
		if !sameOutcomeSemantics(declared, outcome) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("artifact outcome %q semantics differ from the frozen spec", outcome.ID), outcome.Provenance))
		}
	}
	if !outcomeVocabularySubset(task.Outcomes, model.Outcomes) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "artifact outcome vocabulary is not an exact subset of the frozen spec", model.Coverage.Provenance))
	}
	if model.Kind == ArtifactCode {
		var scopedOperations []Operation
		for _, operation := range model.Operations {
			if operation.Kind != OperationTest {
				scopedOperations = append(scopedOperations, operation)
			}
		}
		if len(model.CompilerEvidence) > 0 {
			diagnostics = append(diagnostics, validateCompilerScope(model.Domains, model.Constraints, scopedOperations, model.CompilerEvidence, model.Coverage.Provenance)...)
		}
	}
	for _, behaviorCase := range model.Cases {
		if _, exists := operations[behaviorCase.OperationID]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("behavior case %q refers to undeclared operation %q", behaviorCase.ID, behaviorCase.OperationID), behaviorCase.Provenance))
		}
		for _, outcomeID := range behaviorCase.OutcomeIDs {
			if _, exists := outcomes[outcomeID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("behavior case %q refers to undeclared outcome %q", behaviorCase.ID, outcomeID), behaviorCase.Provenance))
			}
		}
	}
	for _, test := range model.Tests {
		if _, exists := operations[test.OperationID]; !exists && (test.OperationID != "" || test.Predicate.Kind == "") {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("test %q refers to undeclared operation %q", test.ID, test.OperationID), test.Provenance))
		}
		for _, outcomeID := range test.AcceptedOutcomes {
			if _, exists := outcomes[outcomeID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("test %q accepts undeclared outcome %q", test.ID, outcomeID), test.Provenance))
			}
		}
	}
	if HasErrors(diagnostics) {
		return diagnostics
	}

	task.Artifacts = append(task.Artifacts, model)
	task.Coverage = append(task.Coverage, model.Coverage)
	switch model.Kind {
	case ArtifactCode:
		task.CodeCases = append(task.CodeCases, model.Cases...)
	case ArtifactTests:
		task.Tests = append(task.Tests, model.Tests...)
	default:
		// Instruction/environment models contribute evidence and coverage but
		// have no direct CodeBehaviour or TestsPass cases.
	}
	return nil
}

func sameDomainVocabulary(left, right []Domain) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].ID != right[index].ID || left[index].Type != right[index].Type || len(left[index].Values) != len(right[index].Values) {
			return false
		}
		for valueIndex := range left[index].Values {
			if left[index].Values[valueIndex].ID != right[index].Values[valueIndex].ID || !reflect.DeepEqual(left[index].Values[valueIndex].Value, right[index].Values[valueIndex].Value) {
				return false
			}
		}
	}
	return true
}

func domainVocabularySubset(registry, subset []Domain) bool {
	byID := map[string]Domain{}
	for _, domain := range registry {
		byID[domain.ID] = domain
	}
	seen := map[string]struct{}{}
	for _, domain := range subset {
		declared, exists := byID[domain.ID]
		if !exists || !sameDomainVocabulary([]Domain{declared}, []Domain{domain}) {
			return false
		}
		if _, exists := seen[domain.ID]; exists {
			return false
		}
		seen[domain.ID] = struct{}{}
	}
	return true
}

func sameConstraints(left, right []Constraint) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].ID != right[index].ID || left[index].OperationID != right[index].OperationID || left[index].Reason != right[index].Reason || !reflect.DeepEqual(left[index].Conditions, right[index].Conditions) {
			return false
		}
	}
	return true
}

func sameAssignmentGroundings(left, right []AssignmentGrounding) bool {
	if len(left) != len(right) {
		return false
	}
	byID := map[string]AssignmentGrounding{}
	for _, grounding := range left {
		byID[grounding.ID] = grounding
	}
	for _, grounding := range right {
		declared, exists := byID[grounding.ID]
		if !exists || declared.OperationID != grounding.OperationID || !reflect.DeepEqual(declared.Conditions, grounding.Conditions) || !reflect.DeepEqual(declared.Inputs, grounding.Inputs) {
			return false
		}
	}
	return true
}

func constraintSubset(registry, subset []Constraint) bool {
	byID := map[string]Constraint{}
	for _, constraint := range registry {
		byID[constraint.ID] = constraint
	}
	seen := map[string]struct{}{}
	for _, constraint := range subset {
		declared, exists := byID[constraint.ID]
		if !exists || !sameConstraints([]Constraint{declared}, []Constraint{constraint}) {
			return false
		}
		if _, exists := seen[constraint.ID]; exists {
			return false
		}
		seen[constraint.ID] = struct{}{}
	}
	return true
}

func outcomeVocabularySubset(registry, subset []ObservableOutcome) bool {
	byID := map[string]ObservableOutcome{}
	for _, outcome := range registry {
		byID[outcome.ID] = outcome
	}
	seen := map[string]struct{}{}
	for _, outcome := range subset {
		declared, exists := byID[outcome.ID]
		if !exists || !sameOutcomeSemantics(declared, outcome) {
			return false
		}
		if _, exists := seen[outcome.ID]; exists {
			return false
		}
		seen[outcome.ID] = struct{}{}
	}
	return true
}

func sameOutcomeSemantics(left, right ObservableOutcome) bool {
	return left.ID == right.ID &&
		left.Kind == right.Kind &&
		reflect.DeepEqual(left.Value, right.Value) &&
		left.ExceptionType == right.ExceptionType &&
		left.Message == right.Message &&
		sameEffectSemantics(left.Effects, right.Effects)
}

func sameEffectSemantics(left, right []Effect) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].ID != right[index].ID || left[index].Kind != right[index].Kind || left[index].Target != right[index].Target || !reflect.DeepEqual(expressionSemanticsOf(left[index].Value), expressionSemanticsOf(right[index].Value)) {
			return false
		}
	}
	return true
}
