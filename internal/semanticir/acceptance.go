package semanticir

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

const SpecAuthoringRecordSchemaV1 = "hyperray.spec-authoring-record/v1"

// FrozenSpecSemanticsDigest hashes the Phase-A-owned semantics while omitting
// Phase-B TestIDs and spec-file provenance/digests. It is therefore stable
// exactly when final spec.md differs from spec.pretest.md only in Enforced by.
func FrozenSpecSemanticsDigest(task *Task) (string, error) {
	if task == nil {
		return "", fmt.Errorf("task is nil")
	}
	type groundingView struct {
		OperationID     string             `json:"operation_id"`
		Kind            GroundingKind      `json:"kind"`
		Membership      any                `json:"membership"`
		ConcreteWitness map[string]Literal `json:"concrete_witness"`
	}
	type valueView struct {
		ID         string          `json:"id"`
		Value      *Literal        `json:"value,omitempty"`
		Groundings []groundingView `json:"groundings"`
	}
	type domainView struct {
		ID     string      `json:"id"`
		Type   ValueType   `json:"type"`
		Values []valueView `json:"values"`
	}
	type requirementView struct {
		ID                   string       `json:"id"`
		Conditions           Assignment   `json:"conditions"`
		OperationID          string       `json:"operation_id"`
		RequiredOutcomes     []string     `json:"required_outcomes"`
		ForbiddenOutcomes    []string     `json:"forbidden_outcomes"`
		Effects              []any        `json:"effects"`
		InvariantIDs         []string     `json:"invariant_ids"`
		GroundingID          string       `json:"grounding_id"`
		InstructionClauseIDs []string     `json:"instruction_clause_ids"`
		InstructionSources   []Provenance `json:"instruction_sources"`
	}
	type assignmentGroundingView struct {
		ID          string             `json:"id"`
		OperationID string             `json:"operation_id"`
		Conditions  Assignment         `json:"conditions"`
		Inputs      map[string]Literal `json:"inputs"`
	}
	type variableView struct {
		Name     string    `json:"name"`
		Type     ValueType `json:"type"`
		DomainID string    `json:"domain_id"`
	}
	type operationView struct {
		ID         string         `json:"id"`
		Kind       OperationKind  `json:"kind"`
		DomainIDs  []string       `json:"domain_ids"`
		OutcomeIDs []string       `json:"outcome_ids"`
		Inputs     []variableView `json:"inputs"`
	}
	type invariantBindingView struct {
		Variable     string               `json:"variable"`
		Kind         InvariantBindingKind `json:"kind"`
		DomainID     string               `json:"domain_id"`
		EffectKind   EffectKind           `json:"effect_kind"`
		EffectTarget string               `json:"effect_target"`
	}
	type invariantView struct {
		ID        string                 `json:"id"`
		Predicate any                    `json:"predicate"`
		Bindings  []invariantBindingView `json:"bindings"`
	}
	stripEffect := func(effect Effect) any {
		return struct {
			Kind   EffectKind `json:"kind"`
			Target string     `json:"target"`
			Value  any        `json:"value,omitempty"`
		}{effect.Kind, effect.Target, expressionSemanticsOf(effect.Value)}
	}
	domains := make([]domainView, 0, len(task.Domains))
	for _, domain := range task.Domains {
		values := make([]valueView, 0, len(domain.Values))
		for _, value := range domain.Values {
			groundings := make([]groundingView, 0, len(value.Groundings))
			for _, grounding := range value.Groundings {
				groundings = append(groundings, groundingView{grounding.OperationID, grounding.Kind, expressionSemanticsOf(grounding.Membership), grounding.ConcreteWitness})
			}
			values = append(values, valueView{value.ID, value.Value, groundings})
		}
		domains = append(domains, domainView{domain.ID, domain.Type, values})
	}
	operations := make([]operationView, 0, len(task.Operations))
	for _, operation := range task.Operations {
		inputs := make([]variableView, 0, len(operation.Inputs))
		for _, input := range operation.Inputs {
			inputs = append(inputs, variableView{input.Name, input.Type, input.DomainID})
		}
		operations = append(operations, operationView{operation.ID, operation.Kind, append([]string(nil), operation.DomainIDs...), append([]string(nil), operation.OutcomeIDs...), inputs})
	}
	requirements := make([]requirementView, 0, len(task.Requirements))
	for _, requirement := range task.Requirements {
		effects := make([]any, 0, len(requirement.Effects))
		for _, effect := range requirement.Effects {
			effects = append(effects, stripEffect(effect))
		}
		sources := append([]Provenance(nil), requirement.InstructionSources...)
		for index := range sources {
			// Instruction provenance is immutable across phases and remains part
			// of the semantic freeze. Only the spec-row provenance is omitted.
			sources[index].Translation = TranslationTranslated
		}
		requirements = append(requirements, requirementView{
			requirement.ID, requirement.Conditions, requirement.OperationID,
			append([]string(nil), requirement.RequiredOutcomes...), append([]string(nil), requirement.ForbiddenOutcomes...), effects,
			append([]string(nil), requirement.InvariantIDs...), requirement.GroundingID, append([]string(nil), requirement.InstructionClauseIDs...), sources,
		})
	}
	groundings := make([]assignmentGroundingView, 0, len(task.Groundings))
	for _, grounding := range task.Groundings {
		groundings = append(groundings, assignmentGroundingView{grounding.ID, grounding.OperationID, grounding.Conditions, grounding.Inputs})
	}
	outcomes := make([]any, 0, len(task.Outcomes))
	for _, outcome := range task.Outcomes {
		effects := make([]any, 0, len(outcome.Effects))
		for _, effect := range outcome.Effects {
			effects = append(effects, stripEffect(effect))
		}
		outcomes = append(outcomes, struct {
			ID            string      `json:"id"`
			Kind          OutcomeKind `json:"kind"`
			Value         *Literal    `json:"value,omitempty"`
			ExceptionType string      `json:"exception_type"`
			Message       string      `json:"message"`
			OperationID   string      `json:"operation_id"`
			Effects       []any       `json:"effects"`
		}{outcome.ID, outcome.Kind, outcome.Value, outcome.ExceptionType, outcome.Message, outcome.OperationID, effects})
	}
	invariants := make([]invariantView, 0, len(task.Invariants))
	for _, invariant := range task.Invariants {
		bindings := make([]invariantBindingView, 0, len(invariant.Bindings))
		for _, binding := range invariant.Bindings {
			bindings = append(bindings, invariantBindingView{binding.Variable, binding.Kind, binding.DomainID, binding.EffectKind, binding.EffectTarget})
		}
		invariants = append(invariants, invariantView{invariant.ID, expressionSemanticsOf(&invariant.Predicate), bindings})
	}
	return Digest(struct {
		TaskID       string                    `json:"task_id"`
		Instruction  ArtifactRef               `json:"instruction"`
		Domains      []domainView              `json:"domains"`
		Groundings   []assignmentGroundingView `json:"groundings"`
		Constraints  []Constraint              `json:"constraints"`
		Operations   []operationView           `json:"operations"`
		Outcomes     []any                     `json:"outcomes"`
		Invariants   []invariantView           `json:"invariants"`
		Requirements []requirementView         `json:"requirements"`
	}{task.ID, task.Instruction, domains, groundings, stripConstraintProvenance(task.Constraints), operations, outcomes, invariants, requirements})
}

func stripConstraintProvenance(values []Constraint) []Constraint {
	result := make([]Constraint, 0, len(values))
	for _, value := range values {
		result = append(result, Constraint{ID: value.ID, Conditions: value.Conditions, OperationID: value.OperationID, Reason: value.Reason})
	}
	return result
}

func validateSpecAcceptance(task *Task) []Diagnostic {
	if task.SpecAcceptance == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticIncomplete, "task has no frozen Phase-A spec acceptance evidence", task.Provenance)}
	}
	acceptance := task.SpecAcceptance
	provenance := task.Provenance
	var diagnostics []Diagnostic
	for label, artifact := range map[string]ArtifactRef{"authoring record": acceptance.AuthoringRecord, "detached ledger": acceptance.DetachedLedger, "Phase-A spec": acceptance.PhaseASpec, "Phase-A environment": acceptance.PhaseAEnvironment} {
		if err := validateArtifactRef(artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+": "+err.Error(), provenance))
		}
	}
	if acceptance.AuthoringRecord.Kind != ArtifactSpecAuthoringRecord || acceptance.DetachedLedger.Kind != ArtifactSpecLedger || acceptance.PhaseASpec.Kind != ArtifactSpec || acceptance.PhaseAEnvironment.Kind != ArtifactEnvironment {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "spec acceptance artifacts have incorrect kinds", provenance))
	}
	if acceptance.Schema != SpecAuthoringRecordSchemaV1 || acceptance.TaskID != task.ID || acceptance.FinalSpec != task.Spec || acceptance.Instruction != task.Instruction {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "spec acceptance schema/task/spec/instruction binding differs from task", provenance))
	}
	if len(acceptance.OperationIDs) != 0 || len(acceptance.ConstraintIDs) != 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "spec acceptance uses ambiguous legacy inventory fields", provenance))
	}
	if task.Environment == nil || acceptance.Environment != acceptance.PhaseAEnvironment || acceptance.EnvironmentConfigDigest != task.Environment.ConfigDigest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "spec acceptance environment binding differs from task", provenance))
	}
	phaseEnvironment := acceptance.PhaseAEnvironmentModel
	phaseEnvironmentDigest, _ := Digest(phaseEnvironment)
	if phaseEnvironment.Schema != PhaseAEnvironmentSchemaV1 || !phaseEnvironment.Complete || phaseEnvironment.Identity != task.Environment.Identity || phaseEnvironment.ConfigurationDigest != task.Environment.ConfigDigest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "Phase-A environment identity/configuration differs from final frozen environment", provenance))
	}
	if phaseEnvironmentDigest != acceptance.PhaseAEnvironment.Digest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticStaleArtifact, "Phase-A environment canonical model digest differs from frozen artifact", provenance))
	}
	for _, tool := range phaseEnvironment.Tools {
		found := false
		for _, finalTool := range task.Environment.Tools {
			found = found || tool == finalTool
		}
		if !found || validateToolRef(tool) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "Phase-A environment uses a tool absent from final environment", provenance))
		}
	}
	for _, source := range phaseEnvironment.SourceArtifacts {
		found := false
		for _, finalSource := range task.Environment.SourceArtifacts {
			found = found || source == finalSource
		}
		if !found || source.Kind != ArtifactEnvironment || strings.Contains(strings.ToLower(source.ID+" "+source.Path), "test") {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "Phase-A environment has a test-derived or non-final source", provenance))
		}
	}
	if len(phaseEnvironment.Tools) == 0 || len(phaseEnvironment.SourceArtifacts) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "Phase-A environment omits tools or environment sources", provenance))
	}
	wantFrozen, err := FrozenSpecSemanticsDigest(task)
	if err != nil || acceptance.FrozenSemanticsDigest != wantFrozen || !ValidDigest(acceptance.PhaseASpecIRDigest) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticStaleArtifact, "spec acceptance semantic/Phase-A IR digest is missing or stale", provenance))
	}
	if !acceptance.Complete || acceptance.Decision != SpecAcceptanceAccepted || acceptance.ExpandedTableReview != SpecAcceptanceAccepted || acceptance.ExpectedGroundingReview != SpecAcceptanceAccepted || acceptance.TestAccess != "not-accessed" {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "spec acceptance is incomplete, rejected, or test-contaminated", provenance))
	}
	if strings.TrimSpace(acceptance.AuthorIdentity) == "" || strings.TrimSpace(acceptance.IndependentReviewer) == "" || acceptance.AuthorIdentity == acceptance.IndependentReviewer {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "spec acceptance requires distinct author and independent reviewer identities", provenance))
	}
	completed, timestampErr := time.Parse(time.RFC3339, acceptance.CompletedAtUTC)
	_, offset := completed.Zone()
	if timestampErr != nil || offset != 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "spec acceptance completion timestamp is not RFC3339 UTC", provenance))
	}
	if acceptance.SnapshotPath != acceptance.PhaseASpec.Path || acceptance.FinalPath != task.Spec.Path || acceptance.LedgerPath != acceptance.DetachedLedger.Path || strings.TrimSpace(acceptance.LintCommand) == "" {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "spec acceptance paths/lint command are incomplete or inconsistent", provenance))
	}
	wantOperations := make([]string, 0, len(task.Operations))
	for _, operation := range task.Operations {
		wantOperations = append(wantOperations, operation.ID)
	}
	sort.Strings(wantOperations)
	gotOperations := make([]string, 0, len(acceptance.Operations))
	for _, binding := range acceptance.Operations {
		gotOperations = append(gotOperations, binding.OperationID)
		if binding.OperationID == "" || strings.TrimSpace(binding.EntryPoint) == "" || strings.TrimSpace(binding.PhaseAEvidence) == "" || strings.TrimSpace(binding.ObservableBoundary) == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "spec acceptance operation inventory has an incomplete row", provenance))
		}
		if binding.Decision != SpecAcceptanceAccepted || len(binding.InstructionClauseIDs) == 0 || len(binding.Evidence) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "spec acceptance operation lacks typed instruction/evidence review", provenance))
		}
	}
	sort.Strings(gotOperations)
	if !sameStringSet(gotOperations, wantOperations) || hasDuplicateStrings(gotOperations) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "spec acceptance operation inventory differs from compiled spec", provenance))
	}
	wantDomains := acceptanceDomains(task)
	gotDomains := append([]AcceptanceDomainBinding(nil), acceptance.Domains...)
	sortAcceptanceDomains(gotDomains)
	if len(gotDomains) != len(wantDomains) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "spec acceptance domain/label inventory differs from compiled spec", provenance))
	} else {
		for index, got := range gotDomains {
			want := wantDomains[index]
			labelIDs := []string{}
			for _, label := range got.Labels {
				labelIDs = append(labelIDs, label.ValueID)
				if label.ValueID == "" || len(label.DefinitionEvidence) == 0 || strings.TrimSpace(label.ExpectedCompilerPath) == "" || strings.TrimSpace(label.ExpectedReachableWitness) == "" {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "spec acceptance label lacks definition/path/witness review", provenance))
				}
			}
			sort.Strings(labelIDs)
			values := append([]string(nil), got.ValueIDs...)
			sort.Strings(values)
			if got.OperationID != want.OperationID || got.DomainID != want.DomainID || !sameStringSet(values, want.ValueIDs) || !sameStringSet(labelIDs, want.ValueIDs) || hasDuplicateStrings(values) || hasDuplicateStrings(labelIDs) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "spec acceptance domain/label inventory differs from compiled spec", provenance))
			}
		}
	}
	wantConstraints := make([]AcceptanceConstraintBinding, 0, len(task.Constraints))
	for _, constraint := range task.Constraints {
		wantConstraints = append(wantConstraints, AcceptanceConstraintBinding{ID: constraint.ID, OperationID: constraint.OperationID, Conditions: constraint.Conditions, Reason: constraint.Reason})
	}
	gotConstraints := append([]AcceptanceConstraintBinding(nil), acceptance.Constraints...)
	sort.Slice(wantConstraints, func(i, j int) bool { return wantConstraints[i].ID < wantConstraints[j].ID })
	sort.Slice(gotConstraints, func(i, j int) bool { return gotConstraints[i].ID < gotConstraints[j].ID })
	constraintCore := func(values []AcceptanceConstraintBinding) []AcceptanceConstraintBinding {
		result := make([]AcceptanceConstraintBinding, 0, len(values))
		for _, value := range values {
			result = append(result, AcceptanceConstraintBinding{ID: value.ID, OperationID: value.OperationID, Conditions: value.Conditions, Reason: value.Reason})
		}
		return result
	}
	if len(gotConstraints) != len(wantConstraints) || (len(gotConstraints) > 0 && !reflect.DeepEqual(constraintCore(gotConstraints), wantConstraints)) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "spec acceptance constraint inventory differs from compiled spec", provenance))
	}
	for _, constraint := range gotConstraints {
		if strings.TrimSpace(constraint.NoPathEvidence) == "" || len(constraint.Evidence) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "spec acceptance constraint lacks typed no-path evidence", provenance))
		}
	}
	reviewedRequirements := map[string]struct{}{}
	for _, review := range acceptance.Reviews {
		if review.ID == "" || review.Decision != SpecAcceptanceAccepted || len(review.InstructionClauseIDs) == 0 || len(review.Evidence) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "spec acceptance clause/row review is incomplete", provenance))
		}
		for _, id := range review.RequirementIDs {
			reviewedRequirements[id] = struct{}{}
		}
	}
	for _, requirement := range task.Requirements {
		if _, ok := reviewedRequirements[requirement.ID]; !ok {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "spec acceptance review omits requirement "+requirement.ID, provenance))
		}
	}
	if acceptance.NoDisagreements && len(acceptance.Resolutions) != 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "spec acceptance claims no disagreements but records resolutions", provenance))
	}
	if !acceptance.NoDisagreements && len(acceptance.Resolutions) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "spec acceptance omits source disagreement resolution", provenance))
	}
	for _, resolution := range acceptance.Resolutions {
		if resolution.ID == "" || len(resolution.SourceRoles) < 2 || strings.TrimSpace(resolution.Disagreement) == "" || strings.TrimSpace(resolution.Resolution) == "" || resolution.Decision != SpecAcceptanceAccepted || len(resolution.Evidence) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "spec acceptance source resolution is incomplete", provenance))
		}
	}
	diagnostics = append(diagnostics, validateAcceptanceManifest(task, acceptance)...)
	return diagnostics
}

// ValidateSpecAcceptance exposes the single strict validator used by the
// pipeline, proof engine, and certificate builder.
func ValidateSpecAcceptance(task *Task) []Diagnostic { return validateSpecAcceptance(task) }

func acceptanceDomains(task *Task) []AcceptanceDomainBinding {
	byID := map[string]Domain{}
	for _, domain := range task.Domains {
		byID[domain.ID] = domain
	}
	var result []AcceptanceDomainBinding
	for _, operation := range task.Operations {
		for _, domainID := range operation.DomainIDs {
			binding := AcceptanceDomainBinding{OperationID: operation.ID, DomainID: domainID}
			for _, value := range byID[domainID].Values {
				binding.ValueIDs = append(binding.ValueIDs, value.ID)
			}
			result = append(result, binding)
		}
	}
	sortAcceptanceDomains(result)
	return result
}

func sortAcceptanceDomains(values []AcceptanceDomainBinding) {
	for index := range values {
		values[index].ValueIDs = append([]string(nil), values[index].ValueIDs...)
		sort.Strings(values[index].ValueIDs)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].OperationID+"\x00"+values[i].DomainID < values[j].OperationID+"\x00"+values[j].DomainID
	})
}

func hasDuplicateStrings(values []string) bool {
	for index := 1; index < len(values); index++ {
		if values[index] == values[index-1] {
			return true
		}
	}
	return false
}

func validateAcceptanceManifest(task *Task, acceptance *SpecAcceptanceEvidence) []Diagnostic {
	var diagnostics []Diagnostic
	seen := map[string]struct{}{}
	foundInstruction, foundEnvironment := false, false
	for _, binding := range acceptance.Manifest {
		key := binding.Role + "\x00" + binding.Path
		lower := strings.ToLower(binding.Role + " " + binding.Path)
		if binding.Role == "" || binding.Path == "" || !ValidDigest(binding.Digest) || binding.Relevant == "" || strings.Contains(lower, "test") {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "spec acceptance manifest has invalid or test-derived source", task.Provenance))
		}
		if _, exists := seen[key]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "spec acceptance manifest repeats a source", task.Provenance))
		}
		seen[key] = struct{}{}
		foundInstruction = foundInstruction || binding.Digest == task.Instruction.Digest
		if task.Environment != nil {
			foundEnvironment = foundEnvironment || binding.Digest == acceptance.PhaseAEnvironment.Digest
		}
	}
	if len(acceptance.Manifest) == 0 || !foundInstruction || !foundEnvironment {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "spec acceptance manifest omits instruction or environment evidence", task.Provenance))
	}
	wantEvidence := map[string]ArtifactRef{
		acceptance.AuthoringRecord.ID:   acceptance.AuthoringRecord,
		acceptance.DetachedLedger.ID:    acceptance.DetachedLedger,
		acceptance.PhaseASpec.ID:        acceptance.PhaseASpec,
		acceptance.PhaseAEnvironment.ID: acceptance.PhaseAEnvironment,
	}
	seenEvidence := map[string]struct{}{}
	for _, evidence := range acceptance.Evidence {
		artifact, exists := wantEvidence[evidence.ArtifactID]
		if !exists || validateFactSource(evidence, artifact) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "spec acceptance evidence is not anchored to a frozen acceptance artifact", evidence))
			continue
		}
		seenEvidence[evidence.ArtifactID] = struct{}{}
	}
	if len(seenEvidence) != len(wantEvidence) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "spec acceptance does not cite record, ledger, and Phase-A snapshot", task.Provenance))
	}
	return diagnostics
}
