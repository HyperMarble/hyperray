package semanticir

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

const maxFiniteAssignments = 100000

// ValidateSpec validates the finite specification portion of a task. It does
// not require code or test translations, so the strict spec compiler can use
// it before independent frontend models have been merged.
func (task *Task) ValidateSpec() []Diagnostic {
	if task == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticInvalidInput, "task is nil", Provenance{})}
	}

	var diagnostics []Diagnostic
	if strings.TrimSpace(task.ID) == "" {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "task ID is empty", task.Provenance))
	}
	if err := validateArtifactRef(task.Instruction); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "instruction: "+err.Error(), task.Provenance))
	}
	if err := validateArtifactRef(task.Spec); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, err.Error(), task.Provenance))
	}
	if err := validateProvenance(task.Provenance); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, err.Error(), task.Provenance))
	}
	diagnostics = append(diagnostics, ValidateSpecIRDigest(task)...)
	coverageArtifacts := map[string]bool{}
	for _, coverage := range task.Coverage {
		diagnostics = append(diagnostics, validateCoverage(coverage)...)
		if coverage.Status == TranslationComplete && coverage.TranslatedConstructs == coverage.TotalConstructs {
			coverageArtifacts[coverage.Provenance.ArtifactID] = true
		}
	}
	for _, artifact := range []ArtifactRef{task.Spec, task.Instruction} {
		if !coverageArtifacts[artifact.ID] {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("task has no complete translation coverage for %s artifact %q", artifact.Kind, artifact.ID), task.Provenance))
		}
	}
	if task.InstructionModel.Artifact != task.Instruction {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "instruction model artifact differs from task instruction", task.Provenance))
	}
	diagnostics = append(diagnostics, validateCoverage(task.InstructionModel.Coverage)...)
	clauseIDs := map[string]InstructionClause{}
	for _, clause := range task.InstructionModel.Clauses {
		if clause.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "instruction clause ID is empty", clause.Provenance))
		} else if _, exists := clauseIDs[clause.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("duplicate instruction clause ID %q", clause.ID), clause.Provenance))
		}
		clauseIDs[clause.ID] = clause
		if !ValidDigest(clause.SliceDigest) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("instruction clause %q has invalid slice digest", clause.ID), clause.Provenance))
		}
		if err := validateProvenance(clause.Provenance); err != nil || clause.Provenance.ArtifactID != task.Instruction.ID || clause.Provenance.ArtifactDigest != task.Instruction.Digest {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("instruction clause %q is not anchored to the frozen instruction", clause.ID), clause.Provenance))
		}
	}

	domainValues := make(map[string]map[string]struct{}, len(task.Domains))
	for _, domain := range task.Domains {
		if err := validateFactSource(domain.Provenance, task.Spec); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "domain: "+err.Error(), domain.Provenance))
		}
		if domain.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticMissingDomain, "domain ID is empty", domain.Provenance))
			continue
		}
		if _, exists := domainValues[domain.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("duplicate domain ID %q", domain.ID), domain.Provenance))
			continue
		}
		if len(domain.Values) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticNonFinite, fmt.Sprintf("domain %q is empty", domain.ID), domain.Provenance))
			continue
		}
		if !ValidValueType(domain.Type) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("domain %q has invalid type %q", domain.ID, domain.Type), domain.Provenance))
		}
		values := make(map[string]struct{}, len(domain.Values))
		for _, value := range domain.Values {
			if err := validateFactSource(value.Provenance, task.Spec); err != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "domain value: "+err.Error(), value.Provenance))
			}
			if value.ID == "" {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticNonFinite, fmt.Sprintf("domain %q contains an empty value", domain.ID), value.Provenance))
				continue
			}
			if _, exists := values[value.ID]; exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("domain %q contains duplicate value %q", domain.ID, value.ID), value.Provenance))
			}
			if value.Value != nil && value.Value.Type != domain.Type {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("domain %q value %q has type %q, want %q", domain.ID, value.ID, value.Value.Type, domain.Type), value.Provenance))
			} else if value.Value != nil {
				if err := ValidateLiteral(*value.Value); err != nil {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("domain %q value %q: %v", domain.ID, value.ID, err), value.Provenance))
				}
			}
			values[value.ID] = struct{}{}
		}
		domainValues[domain.ID] = values
	}
	outcomes := map[string]ObservableOutcome{}
	for _, outcome := range task.Outcomes {
		if err := validateFactSource(outcome.Provenance, task.Spec); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "outcome: "+err.Error(), outcome.Provenance))
		}
		if canonicalID := OutcomeID(outcome); outcome.ID != "" && outcome.ID != canonicalID {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("outcome %q canonical ID is %q", outcome.ID, canonicalID), outcome.Provenance))
		}
		diagnostics = append(diagnostics, validateOutcome(outcome, task.Spec, fmt.Sprintf("outcome %q", outcome.ID))...)
		if outcome.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "outcome ID is empty", outcome.Provenance))
			continue
		}
		if _, exists := outcomes[outcome.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("duplicate outcome ID %q", outcome.ID), outcome.Provenance))
		}
		outcomes[outcome.ID] = outcome
	}
	if len(outcomes) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "task declares no observable outcomes", task.Provenance))
	}

	operations := map[string]Operation{}
	for _, operation := range task.Operations {
		if err := validateFactSource(operation.Provenance, task.Spec); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "operation: "+err.Error(), operation.Provenance))
		}
		if operation.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "operation ID is empty", operation.Provenance))
			continue
		}
		if _, exists := operations[operation.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("duplicate operation ID %q", operation.ID), operation.Provenance))
		}
		operations[operation.ID] = operation
		switch operation.Kind {
		case OperationCallable, OperationFunction, OperationMethod:
		default:
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("spec operation %q has invalid kind %q", operation.ID, operation.Kind), operation.Provenance))
		}
		seenDomains := map[string]struct{}{}
		for _, domainID := range operation.DomainIDs {
			if _, exists := domainValues[domainID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("operation %q refers to unknown domain %q", operation.ID, domainID), operation.Provenance))
			}
			if _, exists := seenDomains[domainID]; exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("operation %q repeats domain %q", operation.ID, domainID), operation.Provenance))
			}
			seenDomains[domainID] = struct{}{}
		}
		if len(operation.Inputs) == 0 && len(operation.DomainIDs) > 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("operation %q has semantic domains but no typed concrete inputs", operation.ID), operation.Provenance))
		}
		seenInputs := map[string]struct{}{}
		for _, input := range operation.Inputs {
			if err := validateFactSource(input.Provenance, task.Spec); err != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("operation %q input: %v", operation.ID, err), input.Provenance))
			}
			if input.Name == "" || !ValidValueType(input.Type) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("operation %q input is unnamed or untyped", operation.ID), input.Provenance))
			}
			if input.DomainID != "" {
				domain, exists := domainByID(task.Domains, input.DomainID)
				if !exists || input.Type != domain.Type {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("operation %q input %q has invalid optional direct-domain binding", operation.ID, input.Name), input.Provenance))
				}
			}
			seenUniverse := map[string]struct{}{}
			for _, literal := range input.Universe {
				if err := ValidateLiteral(literal); err != nil || literal.Type != input.Type {
					if err == nil {
						err = fmt.Errorf("type %q differs from input type %q", literal.Type, input.Type)
					}
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("operation %q input %q Universe: %v", operation.ID, input.Name, err), input.Provenance))
					continue
				}
				digest, _ := Digest(literal)
				if _, duplicate := seenUniverse[digest]; duplicate {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("operation %q input %q Universe repeats a value", operation.ID, input.Name), input.Provenance))
				}
				seenUniverse[digest] = struct{}{}
			}
			if _, exists := seenInputs[input.Name]; exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("operation %q repeats input name %q", operation.ID, input.Name), input.Provenance))
			}
			seenInputs[input.Name] = struct{}{}
		}
		if len(operation.OutcomeIDs) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("operation %q declares no observable outcomes", operation.ID), operation.Provenance))
		}
		seenOutcomes := map[string]struct{}{}
		otherID := OtherOutcome(operation.ID, operation.Provenance).ID
		otherCount := 0
		for _, outcomeID := range operation.OutcomeIDs {
			if _, exists := outcomes[outcomeID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("operation %q refers to unknown outcome %q", operation.ID, outcomeID), operation.Provenance))
			}
			if _, exists := seenOutcomes[outcomeID]; exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("operation %q repeats outcome %q", operation.ID, outcomeID), operation.Provenance))
			}
			seenOutcomes[outcomeID] = struct{}{}
			if outcomeID == otherID {
				otherCount++
			}
		}
		if otherCount != 1 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("operation %q must contain exactly one canonical other outcome complement", operation.ID), operation.Provenance))
		}
	}
	for _, outcome := range task.Outcomes {
		if outcome.Kind == OutcomeOther {
			operation, exists := operations[outcome.OperationID]
			if !exists || !containsString(operation.OutcomeIDs, outcome.ID) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "canonical other outcome is orphaned or outside its operation", outcome.Provenance))
			}
		}
	}
	if len(operations) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "task declares no operations", task.Provenance))
	}
	diagnostics = append(diagnostics, validateGroundingAxioms(task, operations)...)
	groundingDiagnostics, groundingsByID, groundingsByBehavior := validateAssignmentGroundings(task, operations, domainValues)
	diagnostics = append(diagnostics, groundingDiagnostics...)
	total := 0
	for _, operation := range operations {
		operationTotal := 1
		for _, domainID := range operation.DomainIDs {
			factor := len(domainValues[domainID])
			if factor == 0 || operationTotal > maxFiniteAssignments/factor {
				operationTotal = maxFiniteAssignments + 1
				break
			}
			operationTotal *= factor
		}
		if operationTotal > maxFiniteAssignments || total > maxFiniteAssignments-operationTotal {
			total = maxFiniteAssignments + 1
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticNonFinite, fmt.Sprintf("operation-scoped finite universe exceeds %d behaviors", maxFiniteAssignments), task.Provenance))
			break
		}
		total += operationTotal
	}

	invariants := map[string]struct{}{}
	for _, invariant := range task.Invariants {
		if err := validateFactSource(invariant.Provenance, task.Spec); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "invariant: "+err.Error(), invariant.Provenance))
		}
		if invariant.Predicate.Type != TypeBool {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("invariant %q predicate is not boolean", invariant.ID), invariant.Provenance))
		}
		diagnostics = append(diagnostics, validateExpression(invariant.Predicate, task.Spec, fmt.Sprintf("invariant %q predicate", invariant.ID))...)
		boundVariables := map[string]struct{}{}
		for _, binding := range invariant.Bindings {
			if err := validateFactSource(binding.Provenance, task.Spec); err != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("invariant %q binding: %v", invariant.ID, err), binding.Provenance))
			}
			if binding.Variable == "" {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("invariant %q has a binding with no variable", invariant.ID), binding.Provenance))
			}
			if _, exists := boundVariables[binding.Variable]; exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("invariant %q repeats binding variable %q", invariant.ID, binding.Variable), binding.Provenance))
			}
			boundVariables[binding.Variable] = struct{}{}
			switch binding.Kind {
			case BindDomainValue:
				if _, exists := domainValues[binding.DomainID]; !exists || binding.EffectKind != "" || binding.EffectTarget != "" {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("invariant %q has an invalid domain-value binding", invariant.ID), binding.Provenance))
				}
			case BindOutcomeValue:
				if binding.DomainID != "" || binding.EffectKind != "" || binding.EffectTarget != "" {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("invariant %q outcome-value binding has selector fields", invariant.ID), binding.Provenance))
				}
			case BindEffectValue:
				if binding.DomainID != "" || binding.EffectTarget == "" {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("invariant %q has an invalid effect-value binding", invariant.ID), binding.Provenance))
				}
				switch binding.EffectKind {
				case EffectRead, EffectWrite, EffectCall, EffectOutput:
				default:
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("invariant %q binding has invalid effect kind %q", invariant.ID, binding.EffectKind), binding.Provenance))
				}
			default:
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("invariant %q has invalid binding kind %q", invariant.ID, binding.Kind), binding.Provenance))
			}
		}
		for variable := range expressionVariables(invariant.Predicate) {
			if _, exists := boundVariables[variable]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("invariant %q predicate variable %q has no binding", invariant.ID, variable), invariant.Provenance))
			}
		}
		if invariant.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "invariant ID is empty", invariant.Provenance))
			continue
		}
		if _, exists := invariants[invariant.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("duplicate invariant ID %q", invariant.ID), invariant.Provenance))
		}
		invariants[invariant.ID] = struct{}{}
	}

	requirementIDs := map[string]struct{}{}
	usedGroundings := map[string]struct{}{}
	covered := map[string]string{}
	requirementsByBehavior := map[string][]RequirementCase{}
	for _, requirement := range task.Requirements {
		if err := validateFactSource(requirement.Provenance, task.Spec); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "requirement: "+err.Error(), requirement.Provenance))
		}
		diagnostics = append(diagnostics, validateEffects(requirement.Effects, task.Spec, fmt.Sprintf("requirement %q", requirement.ID))...)
		if requirement.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "requirement ID is empty", requirement.Provenance))
		} else if _, exists := requirementIDs[requirement.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("duplicate requirement ID %q", requirement.ID), requirement.Provenance))
		}
		requirementIDs[requirement.ID] = struct{}{}
		operation, operationExists := operations[requirement.OperationID]
		if !operationExists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("requirement %q refers to unknown operation %q", requirement.ID, requirement.OperationID), requirement.Provenance))
		}
		if err := validateAssignment(requirement.Conditions, operationDomainValues(operation, domainValues)); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("requirement %q: %v", requirement.ID, err), requirement.Provenance))
		} else {
			key := behaviorKey(requirement.OperationID, requirement.Conditions)
			if _, exists := covered[key]; !exists {
				covered[key] = fmt.Sprintf("requirement %q", requirement.ID)
			}
			requirementsByBehavior[key] = append(requirementsByBehavior[key], requirement)
		}
		grounding, groundingExists := groundingsByID[requirement.GroundingID]
		behavior := behaviorKey(requirement.OperationID, requirement.Conditions)
		if !groundingExists || requirement.GroundingID == "" || grounding.OperationID != requirement.OperationID || !reflect.DeepEqual(grounding.Conditions, requirement.Conditions) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("requirement %q does not reference its exact outcome-free assignment grounding", requirement.ID), requirement.Provenance))
		} else if registered, exists := groundingsByBehavior[behavior]; !exists || registered.ID != grounding.ID {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("requirement %q grounding is not the unique witness for behavior %s", requirement.ID, behavior), requirement.Provenance))
		} else {
			usedGroundings[grounding.ID] = struct{}{}
		}
		diagnostics = append(diagnostics, validateOutcomePartition(requirement, operation, outcomes)...)
		for _, invariantID := range requirement.InvariantIDs {
			if _, exists := invariants[invariantID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("requirement %q refers to unknown invariant %q", requirement.ID, invariantID), requirement.Provenance))
			}
		}
		for _, outcomeID := range requirement.RequiredOutcomes {
			outcome, exists := outcomes[outcomeID]
			if exists && !effectsSatisfied(outcome.Effects, requirement.Effects) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("requirement %q required outcome %q omits required effects", requirement.ID, outcomeID), requirement.Provenance))
			}
		}
		if len(requirement.InstructionSources) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("requirement %q has no instruction source", requirement.ID), requirement.Provenance))
		}
		// A clause link exists only for prompt-anchored rows. A row anchored
		// into the reference solution has no prompt text to link, and
		// requiring one would forbid specifying behavior the deliberately
		// incomplete prompt never stated.
		if len(requirement.InstructionClauseIDs) == 0 && anchoredToInstruction(requirement, task.Instruction) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("requirement %q has no instruction clause link", requirement.ID), requirement.Provenance))
		}
		for _, clauseID := range requirement.InstructionClauseIDs {
			clause, exists := clauseIDs[clauseID]
			if !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("requirement %q refers to unknown instruction clause %q", requirement.ID, clauseID), requirement.Provenance))
				continue
			}
			matchedSource := false
			for _, source := range requirement.InstructionSources {
				if source.Location == clause.Span {
					matchedSource = true
					break
				}
			}
			if !matchedSource {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("requirement %q clause %q does not match an instruction source", requirement.ID, clauseID), requirement.Provenance))
			}
		}
		for _, source := range requirement.InstructionSources {
			if err := validateProvenance(source); err != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("requirement %q instruction source: %v", requirement.ID, err), source))
			} else if !anchoredTo(source, task.Instruction) && !anchoredTo(source, task.Reference) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("requirement %q source is anchored to neither frozen instruction %q nor frozen reference %q", requirement.ID, task.Instruction.ID, task.Reference.ID), source))
			}
		}
		if len(requirement.Evidence) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("requirement %q has no evidence", requirement.ID), requirement.Provenance))
		}
		for _, evidence := range requirement.Evidence {
			if err := validateProvenance(evidence); err != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("requirement %q evidence: %v", requirement.ID, err), evidence))
			}
		}
		if !containsProvenance(requirement.Evidence, requirement.Provenance) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("requirement %q evidence omits its spec row", requirement.ID), requirement.Provenance))
		}
		for _, source := range requirement.InstructionSources {
			if !containsProvenance(requirement.Evidence, source) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("requirement %q evidence omits an instruction source", requirement.ID), source))
			}
		}
	}
	for groundingID, grounding := range groundingsByID {
		if _, used := usedGroundings[groundingID]; !used {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("assignment grounding %q does not witness a reachable requirement", groundingID), grounding.Provenance))
		}
	}
	for key, requirements := range requirementsByBehavior {
		if len(requirements) < 2 {
			continue
		}
		allowed := stringSet(requirements[0].RequiredOutcomes)
		for _, requirement := range requirements[1:] {
			allowed = intersectSet(allowed, stringSet(requirement.RequiredOutcomes))
		}
		if len(allowed) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("requirements for behavior %s have an empty allowed-outcome intersection", key), requirements[len(requirements)-1].Provenance))
		}
	}

	constraintIDs := map[string]struct{}{}
	for _, constraint := range task.Constraints {
		if err := validateFactSource(constraint.Provenance, task.Spec); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "constraint: "+err.Error(), constraint.Provenance))
		}
		if constraint.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "constraint ID is empty", constraint.Provenance))
		} else if _, exists := constraintIDs[constraint.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("duplicate constraint ID %q", constraint.ID), constraint.Provenance))
		}
		constraintIDs[constraint.ID] = struct{}{}
		operation, operationExists := operations[constraint.OperationID]
		if !operationExists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("constraint %q refers to unknown operation %q", constraint.ID, constraint.OperationID), constraint.Provenance))
		}
		if strings.TrimSpace(constraint.Reason) == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("constraint %q has no reason", constraint.ID), constraint.Provenance))
		}
		if err := validateAssignment(constraint.Conditions, operationDomainValues(operation, domainValues)); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("constraint %q: %v", constraint.ID, err), constraint.Provenance))
		} else {
			key := behaviorKey(constraint.OperationID, constraint.Conditions)
			if previous, exists := covered[key]; exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("assignment %s is covered by both %s and constraint %q", key, previous, constraint.ID), constraint.Provenance))
			} else {
				covered[key] = fmt.Sprintf("constraint %q", constraint.ID)
			}
		}
	}

	if total <= maxFiniteAssignments {
		for operationID, operation := range operations {
			for _, assignment := range EnumerateAssignments(selectDomains(task.Domains, operation.DomainIDs)) {
				key := behaviorKey(operationID, assignment)
				if _, exists := covered[key]; !exists {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("reachable/excluded partition omits behavior %s", key), task.Provenance))
				}
			}
		}
	}
	_, pointDiagnostics := ConcreteBehaviorPoints(task)
	diagnostics = append(diagnostics, pointDiagnostics...)
	return diagnostics
}

// Validate checks the complete proof input, including frontend coverage and
// exact code/test cases.
func (task *Task) Validate() []Diagnostic {
	diagnostics := task.ValidateSpec()
	if task == nil {
		return diagnostics
	}
	outcomes := make(map[string]struct{}, len(task.Outcomes))
	for _, outcome := range task.Outcomes {
		outcomes[outcome.ID] = struct{}{}
	}
	operationOutcomes := make(map[string]map[string]struct{}, len(task.Operations))
	for _, operation := range task.Operations {
		operationOutcomes[operation.ID] = stringSet(operation.OutcomeIDs)
	}
	reachable := make(map[string]struct{}, len(task.Requirements))
	for _, requirement := range task.Requirements {
		reachable[behaviorKey(requirement.OperationID, requirement.Conditions)] = struct{}{}
	}
	caseIDs := map[string]struct{}{}
	caseRefs := map[string]struct{}{}
	for _, behaviorCase := range task.CodeCases {
		if err := validateProvenance(behaviorCase.Provenance); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("code behavior case %q: %v", behaviorCase.ID, err), behaviorCase.Provenance))
		}
		if behaviorCase.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "code behavior case ID is empty", behaviorCase.Provenance))
		} else if _, exists := caseIDs[behaviorCase.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("duplicate code behavior case ID %q", behaviorCase.ID), behaviorCase.Provenance))
		}
		caseIDs[behaviorCase.ID] = struct{}{}
		categoryKey := behaviorKey(behaviorCase.OperationID, behaviorCase.Conditions)
		key := BehaviorCaseKey(behaviorCase)
		if _, exists := reachable[categoryKey]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnreachable, fmt.Sprintf("code behavior case %q refers to constrained or undeclared behavior %s", behaviorCase.ID, categoryKey), behaviorCase.Provenance))
		}
		if _, exists := caseRefs[key]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("multiple code cases define behavior %s", key), behaviorCase.Provenance))
		}
		caseRefs[key] = struct{}{}
		if len(behaviorCase.OutcomeIDs) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("code behavior case %q has no outcomes", behaviorCase.ID), behaviorCase.Provenance))
		}
		for _, outcomeID := range behaviorCase.OutcomeIDs {
			if _, exists := outcomes[outcomeID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("code behavior case %q refers to unknown outcome %q", behaviorCase.ID, outcomeID), behaviorCase.Provenance))
			} else if _, exists := operationOutcomes[behaviorCase.OperationID][outcomeID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("code behavior case %q uses outcome %q outside operation %q", behaviorCase.ID, outcomeID, behaviorCase.OperationID), behaviorCase.Provenance))
			}
		}
	}
	points, pointDiagnostics := ConcreteBehaviorPoints(task)
	diagnostics = append(diagnostics, pointDiagnostics...)
	reachablePoints := map[string]struct{}{}
	for _, point := range points {
		reachablePoints[BehaviorRefKey(point)] = struct{}{}
	}
	for key := range reachablePoints {
		if _, exists := caseRefs[key]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("code behavior omits reachable concrete point %s", key), task.Provenance))
		}
	}
	for key := range caseRefs {
		if _, exists := reachablePoints[key]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("code behavior invents concrete point %s", key), task.Provenance))
		}
	}

	testIDs := map[string]struct{}{}
	for _, test := range task.Tests {
		if err := validateProvenance(test.Provenance); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("test model %q: %v", test.ID, err), test.Provenance))
		}
		if test.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test model ID is empty", test.Provenance))
		} else if _, exists := testIDs[test.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("duplicate test model ID %q", test.ID), test.Provenance))
		}
		testIDs[test.ID] = struct{}{}
		if test.Predicate.Kind != "" {
			diagnostics = append(diagnostics, validateTestPredicate(test.Predicate, reachable, operationOutcomes)...)
		}
		if len(test.AcceptedOutcomes) > 0 {
			if test.Predicate.Kind != PredicateOutcomeIn || test.Predicate.Observe == nil || test.Predicate.Observe.Kind != ObserveOutcome ||
				behaviorKey(test.OperationID, test.Conditions) != behaviorKey(test.Predicate.Observe.Behavior.OperationID, test.Predicate.Observe.Behavior.Conditions) ||
				!sameStringSet(test.AcceptedOutcomes, test.Predicate.Observe.OutcomeIDs) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("test %q AcceptedOutcomes is ambiguous with its global predicate", test.ID), test.Provenance))
			}
		}
	}
	diagnostics = append(diagnostics, validateTestSuite(task, reachable, operationOutcomes)...)
	for _, artifact := range task.Artifacts {
		diagnostics = append(diagnostics, ValidateArtifactModel(artifact)...)
	}
	var derivedCode []BehaviorCase
	var derivedTests []TestModel
	codeArtifacts := 0
	testArtifacts := 0
	operationOwners := map[string]int{}
	for _, artifact := range task.Artifacts {
		switch artifact.Kind {
		case ArtifactCode:
			codeArtifacts++
			derivedCode = append(derivedCode, artifact.Cases...)
			var scopedOperations []Operation
			for _, operation := range artifact.Operations {
				if operation.Kind != OperationTest {
					scopedOperations = append(scopedOperations, operation)
					operationOwners[operation.ID]++
				}
			}
			if len(artifact.CompilerEvidence) > 0 {
				diagnostics = append(diagnostics, validateCompilerScope(artifact.Domains, artifact.Constraints, scopedOperations, artifact.CompilerEvidence, artifact.Coverage.Provenance)...)
			}
		case ArtifactTests:
			testArtifacts++
			derivedTests = append(derivedTests, artifact.Tests...)
			diagnostics = append(diagnostics, validateTestQuantification(task, artifact)...)
		}
	}
	if codeArtifacts == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "task has no code artifact model", task.Provenance))
	}
	for _, operation := range task.Operations {
		if operationOwners[operation.ID] != 1 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("operation %q has %d code artifact owners, want exactly 1", operation.ID, operationOwners[operation.ID]), operation.Provenance))
		}
	}
	if !reflect.DeepEqual(task.CodeCases, derivedCode) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "flattened task code cases disagree with attached artifact models", task.Provenance))
	}
	if testArtifacts == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "task has no independently translated test artifact model", task.Provenance))
	}
	if !reflect.DeepEqual(task.Tests, derivedTests) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "flattened task tests disagree with attached artifact models", task.Provenance))
	}
	if task.Environment != nil {
		environmentTools := map[string]struct{}{}
		for _, tool := range task.Environment.Tools {
			environmentTools[toolKey(tool)] = struct{}{}
		}
		for _, artifact := range task.Artifacts {
			if _, exists := environmentTools[toolKey(artifact.Translator)]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("artifact %q translator is not frozen in the environment", artifact.Artifact.ID), artifact.Coverage.Provenance))
			}
			for _, evidence := range artifact.CompilerEvidence {
				for _, tool := range []ToolRef{evidence.Tool, evidence.Prover} {
					if _, exists := environmentTools[toolKey(tool)]; !exists {
						diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("artifact %q compiler/prover tool %q is not frozen in the environment", artifact.Artifact.ID, tool.Name), evidence.Provenance))
					}
				}
			}
		}
	}
	diagnostics = append(diagnostics, validateEnvironment(task.Environment)...)
	return diagnostics
}

func validateEnvironment(environment *EnvironmentModel) []Diagnostic {
	if environment == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticInvalidInput, "task has no environment model", Provenance{})}
	}
	var diagnostics []Diagnostic
	configuration := environment.Configuration
	if configuration == (ArtifactRef{}) && environment.Artifact.Kind == ArtifactConfiguration {
		configuration = environment.Artifact
	}
	if err := validateArtifactRef(configuration); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "environment: "+err.Error(), environment.Provenance))
	}
	if configuration.Kind != ArtifactConfiguration {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("environment configuration has kind %q, want configuration", configuration.Kind), environment.Provenance))
	}
	if err := validateFactSource(environment.Provenance, configuration); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "environment: "+err.Error(), environment.Provenance))
	}
	sources := map[string]ArtifactRef{}
	for _, source := range environment.SourceArtifacts {
		if validateArtifactRef(source) != nil || source.Kind != ArtifactEnvironment {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "environment model has an invalid non-environment source", environment.Provenance))
		}
		if _, exists := sources[source.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "environment model repeats source "+source.ID, environment.Provenance))
		}
		sources[source.ID] = source
	}
	if len(sources) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "environment model has no frozen environment sources", environment.Provenance))
	}
	if environment.Identity == "" || !ValidDigest(environment.ConfigDigest) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "environment identity and normalized config digest are required", environment.Provenance))
	}
	if len(environment.Commands) > 0 && environment.Coverage.TotalConstructs == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "non-empty environment model claims zero translation constructs", environment.Coverage.Provenance))
	}
	diagnostics = append(diagnostics, validateCoverage(environment.Coverage)...)
	if environment.Coverage.Provenance.ArtifactID != configuration.ID || environment.Coverage.Provenance.ArtifactDigest != configuration.Digest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "environment coverage is not anchored to the environment artifact", environment.Coverage.Provenance))
	}
	toolKeys := map[string]struct{}{}
	for _, tool := range environment.Tools {
		if err := validateToolRef(tool); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, err.Error(), environment.Provenance))
		}
		key := toolKey(tool)
		if _, exists := toolKeys[key]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("environment repeats tool %q", tool.Name), environment.Provenance))
		}
		toolKeys[key] = struct{}{}
	}
	if len(environment.Commands) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "environment has no workspace command evidence", environment.Provenance))
	}
	states := map[WorkspaceState]int{}
	for _, command := range environment.Commands {
		states[command.State]++
		commandSource, exists := sources[command.Provenance.ArtifactID]
		if !exists {
			commandSource = configuration
		}
		if err := validateFactSource(command.Provenance, commandSource); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("workspace command %q: %v", command.ID, err), command.Provenance))
		}
		if command.ID == "" || command.WorkspaceID == "" || command.Command == "" || command.WorkingDirectory == "" || command.TimeoutMillis <= 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "workspace command has missing identity/command/workdir/timeout", command.Provenance))
		}
		if !command.ClearEnvironment || !command.KillProcessGroup {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "workspace command lacks clear-environment/process-group policy", command.Provenance))
		}
		if err := validateExactEnvironment(command.Environment, command.EnvironmentDigest); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("workspace command %q environment: %v", command.ID, err), command.Provenance))
		}
		for _, digest := range []string{command.TreeDigest, command.EnvironmentDigest, command.StdoutDigest, command.StderrDigest, command.SignalValueDigest} {
			if !ValidDigest(digest) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("workspace command %q has invalid evidence digest", command.ID), command.Provenance))
				break
			}
		}
		if command.PassSignal.Kind != PassSignalExitCode && command.PassSignal.Kind != PassSignalFile {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("workspace command %q has invalid pass signal kind", command.ID), command.Provenance))
		}
		if command.PassSignal.Expected == "" || (command.PassSignal.Kind == PassSignalFile && command.PassSignal.Path == "") {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("workspace command %q has incomplete pass signal", command.ID), command.PassSignal.Provenance))
		}
		if command.ObservedPass != command.ExpectedPass {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("workspace command %q observed pass=%t, want %t", command.ID, command.ObservedPass, command.ExpectedPass), command.Provenance))
		}
		for _, tool := range command.Tools {
			key := toolKey(tool)
			if _, exists := toolKeys[key]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("workspace command %q uses undeclared tool %q", command.ID, tool.Name), command.Provenance))
			}
		}
	}
	for _, state := range []WorkspaceState{WorkspaceBaseOldTests, WorkspaceBaseNewTests, WorkspaceSolutionNewTests} {
		if states[state] != 1 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("environment has %d command evidence records for workspace state %q, want 1", states[state], state), environment.Provenance))
		}
	}
	return diagnostics
}

// ValidateCounterexample validates a complete behavior-vector proof witness.
func ValidateCounterexample(task *Task, counterexample Counterexample) []Diagnostic {
	if task == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticInvalidInput, "counterexample task is nil", counterexample.Provenance)}
	}
	var diagnostics []Diagnostic
	if counterexample.ID == "" {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "counterexample ID is empty", counterexample.Provenance))
	}
	switch counterexample.Obligation {
	case ObligationReferenceCorrectness, ObligationReferenceAcceptance, ObligationTestsSound, ObligationTestsComplete:
	default:
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("counterexample has invalid obligation %q", counterexample.Obligation), counterexample.Provenance))
	}
	if err := validateProvenance(counterexample.Provenance); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "counterexample: "+err.Error(), counterexample.Provenance))
	}
	reachable := map[string]struct{}{}
	for _, requirement := range task.Requirements {
		reachable[behaviorKey(requirement.OperationID, requirement.Conditions)] = struct{}{}
	}
	outcomes := map[string]struct{}{}
	for _, outcome := range task.Outcomes {
		outcomes[outcome.ID] = struct{}{}
	}
	operationOutcomes := map[string]map[string]struct{}{}
	for _, operation := range task.Operations {
		operationOutcomes[operation.ID] = stringSet(operation.OutcomeIDs)
	}
	points, pointDiagnostics := ConcreteBehaviorPoints(task)
	diagnostics = append(diagnostics, pointDiagnostics...)
	reachablePoints := map[string]struct{}{}
	for _, point := range points {
		reachablePoints[BehaviorRefKey(point)] = struct{}{}
	}
	choices := map[string]struct{}{}
	for _, choice := range counterexample.Choices {
		if err := validateProvenance(choice.Behavior.Provenance); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "counterexample choice: "+err.Error(), choice.Behavior.Provenance))
		}
		categoryKey := behaviorKey(choice.Behavior.OperationID, choice.Behavior.Conditions)
		key := BehaviorRefKey(choice.Behavior)
		if _, exists := reachable[categoryKey]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnreachable, fmt.Sprintf("counterexample choice refers to constrained or undeclared behavior %s", categoryKey), choice.Behavior.Provenance))
		} else if _, exists := reachablePoints[key]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("counterexample choice refers to an undeclared concrete point %s", key), choice.Behavior.Provenance))
		}
		if _, exists := choices[key]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("counterexample repeats behavior %s", key), choice.Behavior.Provenance))
		}
		choices[key] = struct{}{}
		if _, exists := outcomes[choice.OutcomeID]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("counterexample choice refers to unknown outcome %q", choice.OutcomeID), choice.Behavior.Provenance))
		} else if _, exists := operationOutcomes[choice.Behavior.OperationID][choice.OutcomeID]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("counterexample choice uses outcome %q outside operation %q", choice.OutcomeID, choice.Behavior.OperationID), choice.Behavior.Provenance))
		}
	}
	for key := range reachablePoints {
		if _, exists := choices[key]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("counterexample behavior vector omits %s", key), counterexample.Provenance))
		}
	}
	for _, outcomeID := range append(append([]string(nil), counterexample.ObservedOutcomes...), counterexample.ExpectedOutcomes...) {
		if _, exists := outcomes[outcomeID]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("counterexample refers to unknown outcome %q", outcomeID), counterexample.Provenance))
		}
	}
	if counterexample.OperationID != "" {
		categoryKey := behaviorKey(counterexample.OperationID, counterexample.Conditions)
		found := false
		for _, choice := range counterexample.Choices {
			found = found || behaviorKey(choice.Behavior.OperationID, choice.Behavior.Conditions) == categoryKey
		}
		if !found {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("counterexample violating category %s is absent from its choices", categoryKey), counterexample.Provenance))
		}
	}
	if counterexample.RequirementID != "" {
		found := false
		for _, requirement := range task.Requirements {
			if requirement.ID == counterexample.RequirementID {
				found = true
				break
			}
		}
		if !found {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("counterexample refers to unknown requirement %q", counterexample.RequirementID), counterexample.Provenance))
		}
	}
	return diagnostics
}

// ValidateArtifactModel checks the fail-closed portions of a frontend result.
func ValidateArtifactModel(model ArtifactModel) []Diagnostic {
	var diagnostics []Diagnostic
	if err := validateArtifactRef(model.Artifact); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, err.Error(), model.Coverage.Provenance))
	}
	if !artifactSupportsRole(model.Artifact.Kind, model.Kind) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("artifact model role %q is incompatible with artifact kind %q", model.Kind, model.Artifact.Kind), model.Coverage.Provenance))
	}
	diagnostics = append(diagnostics, validateModelSourceRoleRanges(model)...)
	if model.Kind == ArtifactCode || model.Kind == ArtifactTests {
		switch model.Language {
		case LanguagePython, LanguageRust, LanguageCPP:
		default:
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("artifact model has unsupported language %q", model.Language), model.Coverage.Provenance))
		}
	}
	constructs := len(model.Operations) + len(model.Outcomes) + len(model.RawReferenceCases) + len(model.Cases) + len(model.Invariants) + len(model.Tests) + len(model.CompilerEvidence) + len(model.ExhaustiveEvidence)
	if constructs > 0 && model.Coverage.TotalConstructs == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "non-empty artifact model claims zero translation constructs", model.Coverage.Provenance))
	}
	if err := validateToolRef(model.Translator); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, err.Error(), model.Coverage.Provenance))
	}
	diagnostics = append(diagnostics, validateCompilerEvidence(model)...)
	diagnostics = append(diagnostics, validateExhaustiveExecutionEvidence(model)...)
	diagnostics = append(diagnostics, validateScopeClosure(model)...)
	diagnostics = append(diagnostics, validateTestProjection(model)...)
	diagnostics = append(diagnostics, validateCoverage(model.Coverage)...)
	if model.Coverage.Provenance.ArtifactID != model.Artifact.ID || model.Coverage.Provenance.ArtifactDigest != model.Artifact.Digest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "artifact coverage is not anchored to the modeled artifact", model.Coverage.Provenance))
	}
	for _, operation := range model.Operations {
		if err := validateFactSource(operation.Provenance, model.Artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("operation %q: %v", operation.ID, err), operation.Provenance))
		}
		diagnostics = append(diagnostics, validateOperation(operation, model.Artifact)...)
	}
	for _, outcome := range model.Outcomes {
		if err := validateProvenance(outcome.Provenance); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("outcome %q: %v", outcome.ID, err), outcome.Provenance))
		}
		if outcome.ID != OutcomeID(outcome) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("outcome %q is not canonically identified", outcome.ID), outcome.Provenance))
		}
		outcomeArtifact := ArtifactRef{ID: outcome.Provenance.ArtifactID, Digest: outcome.Provenance.ArtifactDigest, Path: outcome.Provenance.Location.Path, Kind: model.Artifact.Kind}
		diagnostics = append(diagnostics, validateOutcome(outcome, outcomeArtifact, fmt.Sprintf("outcome %q", outcome.ID))...)
	}
	modelOperations := map[string]Operation{}
	for _, operation := range model.Operations {
		modelOperations[operation.ID] = operation
	}
	rawPoints := map[string]struct{}{}
	for _, raw := range model.RawReferenceCases {
		if raw.ID == "" || raw.OperationID == "" || raw.Inputs == nil || len(raw.Outcomes) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "raw reference case lacks ID/operation/inputs/outcomes", raw.Provenance))
		}
		if err := validateFactSource(raw.Provenance, model.Artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "raw reference case: "+err.Error(), raw.Provenance))
		}
		for _, trace := range raw.Outcomes {
			if err := ValidateRawOutcomeTrace(trace); err != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "raw reference outcome: "+err.Error(), raw.Provenance))
			}
		}
		key := BehaviorRefKey(BehaviorRef{OperationID: raw.OperationID, Conditions: raw.Conditions, Inputs: raw.Inputs})
		if _, duplicate := rawPoints[key]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "raw reference model repeats concrete point "+key, raw.Provenance))
		}
		rawPoints[key] = struct{}{}
	}
	casePoints := map[string]struct{}{}
	for _, behaviorCase := range model.Cases {
		if err := validateFactSource(behaviorCase.Provenance, model.Artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("behavior case %q: %v", behaviorCase.ID, err), behaviorCase.Provenance))
		}
		if behaviorCase.ID == "" || behaviorCase.OperationID == "" || behaviorCase.Inputs == nil || len(behaviorCase.OutcomeIDs) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "artifact behavior case lacks ID/operation/outcomes", behaviorCase.Provenance))
		}
		if operation, exists := modelOperations[behaviorCase.OperationID]; exists && behaviorCase.Inputs != nil {
			declaredInputs := inputsByName(operation.Inputs)
			if len(behaviorCase.Inputs) != len(declaredInputs) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "artifact behavior case does not bind every operation input exactly once", behaviorCase.Provenance))
			}
			for name, literal := range behaviorCase.Inputs {
				input, declared := declaredInputs[name]
				if !declared || ValidateLiteral(literal) != nil || literal.Type != input.Type {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "artifact behavior case has an unknown or ill-typed input "+name, behaviorCase.Provenance))
				}
			}
			membership, err := GroundingConjunction(operation, model.Domains, behaviorCase.Conditions, behaviorCase.Provenance)
			matches, evaluationErr := EvaluateGroundingMembership(membership, behaviorCase.Inputs)
			if err != nil || evaluationErr != nil || !matches {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "artifact behavior case inputs do not satisfy its semantic category", behaviorCase.Provenance))
			}
		}
		pointKey := BehaviorCaseKey(behaviorCase)
		if _, duplicate := casePoints[pointKey]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "artifact repeats concrete behavior point "+pointKey, behaviorCase.Provenance))
		}
		casePoints[pointKey] = struct{}{}
	}
	for _, test := range model.Tests {
		if err := validateFactSource(test.Provenance, model.Artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("test %q: %v", test.ID, err), test.Provenance))
		}
	}
	for _, invariant := range model.Invariants {
		if err := validateFactSource(invariant.Provenance, model.Artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("invariant %q: %v", invariant.ID, err), invariant.Provenance))
		}
		if invariant.ID == "" || invariant.Predicate.Type != TypeBool {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "artifact invariant lacks ID/boolean predicate", invariant.Provenance))
		}
		diagnostics = append(diagnostics, validateExpression(invariant.Predicate, model.Artifact, fmt.Sprintf("invariant %q predicate", invariant.ID))...)
	}
	return diagnostics
}

func validateOperation(operation Operation, artifact ArtifactRef) []Diagnostic {
	var diagnostics []Diagnostic
	if operation.ID == "" {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "operation ID is empty", operation.Provenance))
	}
	switch operation.Kind {
	case OperationCallable, OperationFunction, OperationMethod, OperationTest:
	default:
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("operation %q has invalid kind %q", operation.ID, operation.Kind), operation.Provenance))
	}
	seenInputs := map[string]struct{}{}
	for _, input := range operation.Inputs {
		if err := validateFactSource(input.Provenance, artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("operation %q input: %v", operation.ID, err), input.Provenance))
		}
		if input.Name == "" || !ValidValueType(input.Type) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("operation %q has an unnamed or untyped input", operation.ID), input.Provenance))
		}
		if _, exists := seenInputs[input.Name]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("operation %q repeats input %q", operation.ID, input.Name), input.Provenance))
		}
		seenInputs[input.Name] = struct{}{}
	}
	if len(operation.Body) > 0 {
		diagnostics = append(diagnostics, validateStatements(operation.Body, artifact, fmt.Sprintf("operation %q", operation.ID))...)
	}
	return diagnostics
}

func validateCoverage(coverage TranslationCoverage) []Diagnostic {
	diagnostics := validateCoverageShape(coverage)
	if coverage.Status != TranslationComplete || coverage.TranslatedConstructs != coverage.TotalConstructs || len(coverage.Unsupported) != 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, "translation coverage is not complete", coverage.Provenance))
	}
	return diagnostics
}

func validateCoverageShape(coverage TranslationCoverage) []Diagnostic {
	var diagnostics []Diagnostic
	if err := validateCoverageProvenance(coverage); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, err.Error(), coverage.Provenance))
	}
	if coverage.TotalConstructs < 0 || coverage.TranslatedConstructs < 0 || coverage.TranslatedConstructs > coverage.TotalConstructs {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "translation coverage counts are inconsistent", coverage.Provenance))
	}
	switch coverage.Status {
	case TranslationComplete, TranslationPartial, TranslationBlocked:
	default:
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("translation coverage has invalid aggregate status %q", coverage.Status), coverage.Provenance))
	}
	for _, unsupported := range coverage.Unsupported {
		provenance := unsupported.Provenance
		validSource := provenance.ArtifactID != "" && ValidDigest(provenance.ArtifactDigest) && provenance.Location.Path != "" && provenance.Location.StartLine > 0 && provenance.Location.StartColumn > 0
		if unsupported.Kind == "" || unsupported.Reason == "" || !validSource || provenance.Translation != TranslationUnsupported {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "translation coverage has malformed unsupported-construct evidence", unsupported.Provenance))
		}
	}
	return diagnostics
}

func validateTestPredicate(predicate TestPredicate, reachable map[string]struct{}, outcomes map[string]map[string]struct{}) []Diagnostic {
	var diagnostics []Diagnostic
	if predicate.Kind == "" {
		return []Diagnostic{errorDiagnostic(DiagnosticUnsupported, "test has no global predicate", predicate.Provenance)}
	}
	if err := validateProvenance(predicate.Provenance); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "test predicate: "+err.Error(), predicate.Provenance))
	}
	leafFieldsEmpty := predicate.Observe == nil && predicate.Left == nil && predicate.Right == nil
	switch predicate.Kind {
	case PredicateTrue, PredicateFalse:
		if len(predicate.Children) != 0 || !leafFieldsEmpty {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "true predicate has operands", predicate.Provenance))
		}
	case PredicateAnd, PredicateOr:
		if len(predicate.Children) < 2 || !leafFieldsEmpty {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("%s predicate requires at least two child predicates and no leaf fields", predicate.Kind), predicate.Provenance))
		}
	case PredicateNot:
		if len(predicate.Children) != 1 || !leafFieldsEmpty {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "not predicate requires exactly one child and no leaf fields", predicate.Provenance))
		}
	case PredicateOutcomeIn, PredicateRaises, PredicateHasEffect:
		if len(predicate.Children) != 0 || predicate.Observe == nil || predicate.Left != nil || predicate.Right != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("%s predicate requires exactly one observation", predicate.Kind), predicate.Provenance))
			break
		}
		wantKind := map[TestPredicateKind]ObservationKind{
			PredicateOutcomeIn: ObserveOutcome,
			PredicateRaises:    ObserveRaise,
			PredicateHasEffect: ObserveEffect,
		}[predicate.Kind]
		if predicate.Observe.Kind != wantKind {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("%s predicate uses %s observation", predicate.Kind, predicate.Observe.Kind), predicate.Observe.Provenance))
		}
		diagnostics = append(diagnostics, validateObservation(*predicate.Observe, reachable, outcomes)...)
	case PredicateOutcomeEqual:
		if len(predicate.Children) != 0 || predicate.Observe != nil || predicate.Left == nil || predicate.Right == nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "outcome-equal predicate requires left and right behavior references", predicate.Provenance))
			break
		}
		diagnostics = append(diagnostics, validateBehaviorRef(*predicate.Left, reachable)...)
		diagnostics = append(diagnostics, validateBehaviorRef(*predicate.Right, reachable)...)
	default:
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("unsupported test predicate kind %q", predicate.Kind), predicate.Provenance))
	}
	for _, child := range predicate.Children {
		diagnostics = append(diagnostics, validateTestPredicate(child, reachable, outcomes)...)
	}
	return diagnostics
}

func validateObservation(observation Observation, reachable map[string]struct{}, outcomes map[string]map[string]struct{}) []Diagnostic {
	var diagnostics []Diagnostic
	if err := validateProvenance(observation.Provenance); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "test observation: "+err.Error(), observation.Provenance))
	}
	diagnostics = append(diagnostics, validateBehaviorRef(observation.Behavior, reachable)...)
	switch observation.Kind {
	case ObserveOutcome:
		if len(observation.OutcomeIDs) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "outcome observation has an empty accepted set", observation.Provenance))
		}
		seen := map[string]struct{}{}
		for _, outcomeID := range observation.OutcomeIDs {
			if _, exists := outcomes[observation.Behavior.OperationID][outcomeID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("outcome observation refers to unknown outcome %q", outcomeID), observation.Provenance))
			}
			if _, exists := seen[outcomeID]; exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("outcome observation repeats outcome %q", outcomeID), observation.Provenance))
			}
			seen[outcomeID] = struct{}{}
		}
		if observation.ExceptionType != "" || observation.Message != "" || observation.EffectKind != "" || observation.EffectTarget != "" || observation.EffectValue != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "outcome observation also sets raise/effect fields", observation.Provenance))
		}
	case ObserveRaise:
		if observation.ExceptionType == "" || len(observation.OutcomeIDs) != 0 || observation.EffectKind != "" || observation.EffectTarget != "" || observation.EffectValue != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "raise observation must set only exception type/message", observation.Provenance))
		}
	case ObserveEffect:
		if observation.EffectKind == "" || observation.EffectTarget == "" || len(observation.OutcomeIDs) != 0 || observation.ExceptionType != "" || observation.Message != "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "effect observation must set effect kind/target and optional exact value", observation.Provenance))
		}
		switch observation.EffectKind {
		case EffectRead, EffectWrite, EffectCall, EffectOutput:
		default:
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("effect observation has invalid effect kind %q", observation.EffectKind), observation.Provenance))
		}
		if observation.EffectValue != nil {
			artifact := ArtifactRef{ID: observation.Provenance.ArtifactID, Digest: observation.Provenance.ArtifactDigest, Path: observation.Provenance.Location.Path, Kind: ArtifactTests}
			diagnostics = append(diagnostics, validateExpression(*observation.EffectValue, artifact, "effect observation value")...)
		}
	default:
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("unsupported observation kind %q", observation.Kind), observation.Provenance))
	}
	return diagnostics
}

func validateBehaviorRef(reference BehaviorRef, reachable map[string]struct{}) []Diagnostic {
	var diagnostics []Diagnostic
	if err := validateProvenance(reference.Provenance); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "behavior reference: "+err.Error(), reference.Provenance))
	}
	key := behaviorKey(reference.OperationID, reference.Conditions)
	if _, exists := reachable[key]; !exists {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnreachable, fmt.Sprintf("behavior reference points to constrained or undeclared behavior %s", key), reference.Provenance))
	}
	if reference.Inputs == nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test observation behavior reference is a category, not one exact concrete point", reference.Provenance))
	} else {
		for name, literal := range reference.Inputs {
			if name == "" || ValidateLiteral(literal) != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test observation behavior reference has an invalid typed input", reference.Provenance))
			}
		}
	}
	return diagnostics
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, value := range left {
		counts[value]++
	}
	for _, value := range right {
		counts[value]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func intersectSet(left, right map[string]struct{}) map[string]struct{} {
	result := map[string]struct{}{}
	for value := range left {
		if _, exists := right[value]; exists {
			result[value] = struct{}{}
		}
	}
	return result
}

func containsProvenance(values []Provenance, target Provenance) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validateOutcomePartition(requirement RequirementCase, operation Operation, outcomes map[string]ObservableOutcome) []Diagnostic {
	var diagnostics []Diagnostic
	if len(requirement.RequiredOutcomes) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("requirement %q has no required outcomes", requirement.ID), requirement.Provenance))
	}
	seen := map[string]string{}
	operationOutcomes := stringSet(operation.OutcomeIDs)
	for _, item := range []struct {
		name string
		ids  []string
	}{{"required", requirement.RequiredOutcomes}, {"forbidden", requirement.ForbiddenOutcomes}} {
		for _, outcomeID := range item.ids {
			if _, exists := outcomes[outcomeID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("requirement %q refers to unknown outcome %q", requirement.ID, outcomeID), requirement.Provenance))
			}
			if _, exists := operationOutcomes[outcomeID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("requirement %q classifies outcome %q outside operation %q", requirement.ID, outcomeID, operation.ID), requirement.Provenance))
			}
			if previous, exists := seen[outcomeID]; exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("requirement %q outcome %q is both %s and %s", requirement.ID, outcomeID, previous, item.name), requirement.Provenance))
			}
			seen[outcomeID] = item.name
		}
	}
	for _, outcomeID := range operation.OutcomeIDs {
		if _, exists := seen[outcomeID]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("requirement %q does not classify outcome %q", requirement.ID, outcomeID), requirement.Provenance))
		}
	}
	return diagnostics
}

func operationDomainValues(operation Operation, registry map[string]map[string]struct{}) map[string]map[string]struct{} {
	result := make(map[string]map[string]struct{}, len(operation.DomainIDs))
	for _, domainID := range operation.DomainIDs {
		if values, exists := registry[domainID]; exists {
			result[domainID] = values
		}
	}
	return result
}

func selectDomains(registry []Domain, ids []string) []Domain {
	byID := make(map[string]Domain, len(registry))
	for _, domain := range registry {
		byID[domain.ID] = domain
	}
	result := make([]Domain, 0, len(ids))
	for _, id := range ids {
		if domain, exists := byID[id]; exists {
			result = append(result, domain)
		}
	}
	return result
}

func validateAssignment(assignment Assignment, domains map[string]map[string]struct{}) error {
	if len(assignment) != len(domains) {
		return fmt.Errorf("assignment fixes %d domains, want %d", len(assignment), len(domains))
	}
	for domain, values := range domains {
		value, exists := assignment[domain]
		if !exists {
			return fmt.Errorf("assignment omits domain %q", domain)
		}
		if _, exists := values[value]; !exists {
			return fmt.Errorf("assignment uses undeclared %s=%q", domain, value)
		}
	}
	for domain := range assignment {
		if _, exists := domains[domain]; !exists {
			return fmt.Errorf("assignment includes unknown domain %q", domain)
		}
	}
	return nil
}

func validateArtifactRef(artifact ArtifactRef) error {
	if artifact.ID == "" {
		return fmt.Errorf("artifact ID is empty")
	}
	if artifact.Kind == "" {
		return fmt.Errorf("artifact %q kind is empty", artifact.ID)
	}
	if artifact.Path == "" {
		return fmt.Errorf("artifact %q path is empty", artifact.ID)
	}
	if !ValidDigest(artifact.Digest) {
		return fmt.Errorf("artifact %q digest is not normalized SHA-256", artifact.ID)
	}
	return nil
}

func validateToolRef(tool ToolRef) error {
	if tool.Name == "" || tool.Path == "" || tool.Version == "" {
		return fmt.Errorf("translator tool name, path, and version are required")
	}
	if !ValidDigest(tool.Digest) {
		return fmt.Errorf("translator tool %q digest is not normalized SHA-256", tool.Name)
	}
	return nil
}

func toolKey(tool ToolRef) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s", tool.Name, tool.Path, tool.Digest, tool.Version)
}

func validateFactSource(provenance Provenance, artifact ArtifactRef) error {
	if err := validateProvenance(provenance); err != nil {
		return err
	}
	if provenance.ArtifactID != artifact.ID || provenance.ArtifactDigest != artifact.Digest {
		return fmt.Errorf("provenance is not anchored to frozen artifact %q", artifact.ID)
	}
	return nil
}

func validateCoverageProvenance(coverage TranslationCoverage) error {
	provenance := coverage.Provenance
	if provenance.ArtifactID == "" || !ValidDigest(provenance.ArtifactDigest) || provenance.Location.Path == "" || provenance.Location.StartLine < 1 || provenance.Location.StartColumn < 1 {
		return fmt.Errorf("coverage has invalid source provenance")
	}
	if provenance.Translation != coverage.Status && provenance.Translation != TranslationTranslated {
		return fmt.Errorf("coverage provenance status %q differs from coverage status %q", provenance.Translation, coverage.Status)
	}
	return nil
}

func validateEffects(effects []Effect, artifact ArtifactRef, label string) []Diagnostic {
	var diagnostics []Diagnostic
	seen := map[string]struct{}{}
	for _, effect := range effects {
		if effect.ID == "" || effect.Target == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" has an effect with empty ID/target", effect.Provenance))
		}
		if _, exists := seen[effect.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("%s repeats effect ID %q", label, effect.ID), effect.Provenance))
		}
		seen[effect.ID] = struct{}{}
		switch effect.Kind {
		case EffectRead, EffectWrite, EffectCall, EffectOutput:
		default:
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("%s has unsupported effect kind %q", label, effect.Kind), effect.Provenance))
		}
		if err := validateFactSource(effect.Provenance, artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, label+" effect: "+err.Error(), effect.Provenance))
		}
		if effect.Value != nil {
			diagnostics = append(diagnostics, validateExpression(*effect.Value, artifact, label+" effect value")...)
		}
	}
	return diagnostics
}

func validateOutcome(outcome ObservableOutcome, artifact ArtifactRef, label string) []Diagnostic {
	var diagnostics []Diagnostic
	if outcome.Value != nil {
		if err := ValidateLiteral(*outcome.Value); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+": "+err.Error(), outcome.Provenance))
		}
	}
	switch outcome.Kind {
	case OutcomeReturn:
		if outcome.Value == nil || outcome.ExceptionType != "" || outcome.Message != "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" return requires a value and no raise fields", outcome.Provenance))
		}
	case OutcomeRaise:
		if outcome.Value != nil || outcome.ExceptionType == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" raise requires an exception type and no return value", outcome.Provenance))
		}
	case OutcomeSuccess:
		if outcome.Value != nil || outcome.ExceptionType != "" || outcome.Message != "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" success has return/raise fields", outcome.Provenance))
		}
	case OutcomeTimeout:
		if outcome.Value != nil || outcome.ExceptionType != "" || outcome.Message != "" || len(outcome.Effects) != 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" timeout has return/raise/effect fields", outcome.Provenance))
		}
	case OutcomeOther:
		if outcome.OperationID == "" || outcome.Value != nil || outcome.ExceptionType != "" || outcome.Message != "" || len(outcome.Effects) != 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" other complement requires only an operation ID", outcome.Provenance))
		}
	default:
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("%s has unsupported kind %q", label, outcome.Kind), outcome.Provenance))
	}
	diagnostics = append(diagnostics, validateEffects(outcome.Effects, artifact, label)...)
	return diagnostics
}

func effectsSatisfied(actual, required []Effect) bool {
	used := make([]bool, len(actual))
	for _, want := range required {
		matched := false
		for index, got := range actual {
			if !used[index] && want.Kind == got.Kind && want.Target == got.Target && reflect.DeepEqual(expressionSemanticsOf(want.Value), expressionSemanticsOf(got.Value)) {
				used[index] = true
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func domainByID(domains []Domain, id string) (Domain, bool) {
	for _, domain := range domains {
		if domain.ID == id {
			return domain, true
		}
	}
	return Domain{}, false
}

func validateProvenance(provenance Provenance) error {
	if provenance.ArtifactID == "" {
		return fmt.Errorf("provenance has no artifact ID")
	}
	if !ValidDigest(provenance.ArtifactDigest) {
		return fmt.Errorf("provenance for %q has invalid digest", provenance.ArtifactID)
	}
	if provenance.Location.Path == "" || provenance.Location.StartLine < 1 || provenance.Location.StartColumn < 1 {
		return fmt.Errorf("provenance for %q has invalid source location", provenance.ArtifactID)
	}
	if provenance.Translation != TranslationTranslated {
		return fmt.Errorf("provenance for %q is %q, want translated", provenance.ArtifactID, provenance.Translation)
	}
	return nil
}

func errorDiagnostic(code DiagnosticCode, message string, provenance Provenance) Diagnostic {
	return Diagnostic{Severity: SeverityError, Code: code, Message: message, Provenance: provenance}
}

// EnumerateAssignments returns the full Cartesian product in declaration
// order. The product of zero domains is the one empty assignment, which is
// the behavior key for a zero-argument operation.
func EnumerateAssignments(domains []Domain) []Assignment {
	if len(domains) == 0 {
		return []Assignment{{}}
	}
	assignments := []Assignment{{}}
	for _, domain := range domains {
		var next []Assignment
		for _, assignment := range assignments {
			for _, value := range domain.Values {
				copyAssignment := make(Assignment, len(assignment)+1)
				for key, existing := range assignment {
					copyAssignment[key] = existing
				}
				copyAssignment[domain.ID] = value.ID
				next = append(next, copyAssignment)
			}
		}
		assignments = next
	}
	return assignments
}

func assignmentKey(assignment Assignment) string {
	keys := make([]string, 0, len(assignment))
	for key := range assignment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+assignment[key])
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func behaviorKey(operationID string, assignment Assignment) string {
	return operationID + assignmentKey(assignment)
}

// BehaviorCaseKey is the canonical identity shared by code cases and exact
// test/proof behavior points.
func BehaviorCaseKey(behaviorCase BehaviorCase) string {
	return BehaviorRefKey(BehaviorRef{OperationID: behaviorCase.OperationID, Conditions: behaviorCase.Conditions, Inputs: behaviorCase.Inputs})
}

// BehaviorRefKey is the canonical identity of a concrete behavior point.
// Category references (Inputs == nil) retain the legacy category key.
func BehaviorRefKey(reference BehaviorRef) string {
	key := behaviorKey(reference.OperationID, reference.Conditions)
	if reference.Inputs == nil {
		return key
	}
	digest, err := Digest(reference.Inputs)
	if err != nil {
		return key + "@invalid-inputs"
	}
	return key + "@" + digest
}

// anchoredTo reports whether a provenance cites exactly this frozen artifact.
// An empty ArtifactRef never matches, so a task without a reference keeps
// rejecting reference anchors.
func anchoredTo(source Provenance, artifact ArtifactRef) bool {
	return artifact.ID != "" && source.ArtifactID == artifact.ID && source.ArtifactDigest == artifact.Digest
}

// anchoredToInstruction reports whether any of a requirement's sources cites
// the prompt, which is what makes an instruction clause link expected.
func anchoredToInstruction(requirement RequirementCase, instruction ArtifactRef) bool {
	for _, source := range requirement.InstructionSources {
		if anchoredTo(source, instruction) {
			return true
		}
	}
	return false
}
