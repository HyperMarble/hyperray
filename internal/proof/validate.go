package proof

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

const enumerationMethod = "exhaustive-finite-enumeration"

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type caseKey struct {
	operation  string
	assignment string
	inputs     string
}

type finiteCase struct {
	operation    string
	conditions   semanticir.Assignment
	inputs       map[string]semanticir.Literal
	requirements []semanticir.RequirementCase
	allowed      []string
	rejectedBy   map[string]string
	code         semanticir.BehaviorCase
}

type finiteModel struct {
	operationDomains         map[string][]string
	operationOutcomes        map[string][]string
	outcomeIDs               []string
	outcomes                 map[string]semanticir.ObservableOutcome
	reachable                map[string][]semanticir.Assignment
	reachableCount           uint64
	cases                    []finiteCase
	tests                    []semanticir.TestModel
	totalAssignments         uint64
	excluded                 uint64
	specIRDigest             string
	referenceIRDigest        string
	testIRDigest             string
	environmentIRDigest      string
	compilerEvidence         []semanticir.CompilerEvidence
	compilerEvidenceSHA256   string
	derivationReplays        []DerivationReplayBinding
	derivationReplaysSHA256  string
	scopeClosures            []semanticir.ScopeClosureEvidence
	scopeClosuresSHA256      string
	exhaustiveEvidence       []semanticir.ExhaustiveExecutionEvidence
	exhaustiveEvidenceSHA256 string
	testSuite                *semanticir.TestSuiteModel
	testSuiteSHA256          string
}

type validator struct {
	ctx               context.Context
	task              *semanticir.Task
	artifacts         map[string]semanticir.ArtifactRef
	covered           map[string]bool
	domains           map[string]map[string]bool
	domainIDs         []string
	operations        map[string]semanticir.Operation
	operationDomains  map[string][]string
	operationOutcomes map[string][]string
	outcomes          map[string]semanticir.ObservableOutcome
	invariants        map[string]semanticir.Invariant
	derivationReplays []DerivationReplayBinding
	blockers          []Blocker
}

func validate(ctx context.Context, task *semanticir.Task) (*finiteModel, []Blocker) {
	if task == nil {
		return nil, []Blocker{{Code: "nil-task", Message: "semantic IR task is nil"}}
	}
	if err := ctx.Err(); err != nil {
		return nil, []Blocker{{Code: "context-cancelled", Message: err.Error()}}
	}

	v := &validator{
		ctx:               ctx,
		task:              task,
		artifacts:         make(map[string]semanticir.ArtifactRef),
		covered:           make(map[string]bool),
		domains:           make(map[string]map[string]bool),
		operations:        make(map[string]semanticir.Operation),
		operationDomains:  make(map[string][]string),
		operationOutcomes: make(map[string][]string),
		outcomes:          make(map[string]semanticir.ObservableOutcome),
		invariants:        make(map[string]semanticir.Invariant),
	}
	v.validateArtifacts()
	v.validateCoverage()
	v.validateEvidence()
	v.validateInstructionEnvironment()
	v.validateFlattenedModels()
	v.validateSpecIR()
	if len(v.blockers) != 0 {
		return nil, normalizedBlockers(v.blockers)
	}

	v.validateDomains()
	v.validateOperations()
	v.validateGroundingAxioms()
	v.validateOutcomes()
	v.validateInvariants()
	if len(v.blockers) != 0 {
		return nil, normalizedBlockers(v.blockers)
	}

	constraintKeys := v.validateConstraints()
	reachable, total, excluded := v.enumerateAssignments(ctx, constraintKeys)
	v.validateAssignmentGroundings(reachable)
	if err := ctx.Err(); err != nil {
		v.add("context-cancelled", err.Error(), nil)
	}
	if len(v.blockers) != 0 {
		return nil, normalizedBlockers(v.blockers)
	}
	if len(v.blockers) != 0 {
		return nil, normalizedBlockers(v.blockers)
	}

	model := &finiteModel{
		operationDomains:   v.operationDomains,
		operationOutcomes:  v.operationOutcomes,
		outcomes:           v.outcomes,
		reachable:          reachable,
		totalAssignments:   total,
		excluded:           excluded,
		specIRDigest:       v.task.SpecIRDigest,
		compilerEvidence:   v.compilerEvidenceTranscript(),
		derivationReplays:  append([]DerivationReplayBinding(nil), v.derivationReplays...),
		scopeClosures:      v.scopeClosureTranscript(),
		exhaustiveEvidence: v.exhaustiveEvidenceTranscript(),
	}
	v.validateIndependentIR(model)
	if len(v.blockers) != 0 {
		return nil, normalizedBlockers(v.blockers)
	}
	model.compilerEvidenceSHA256 = compilerEvidenceDigest(model.compilerEvidence)
	model.derivationReplaysSHA256 = proofEvidenceDigest(model.derivationReplays)
	model.scopeClosuresSHA256 = proofEvidenceDigest(model.scopeClosures)
	model.exhaustiveEvidenceSHA256 = proofEvidenceDigest(model.exhaustiveEvidence)
	for _, assignments := range reachable {
		model.reachableCount += uint64(len(assignments))
	}
	for id := range v.outcomes {
		model.outcomeIDs = append(model.outcomeIDs, id)
	}
	sort.Strings(model.outcomeIDs)
	v.validateCases(model, constraintKeys)
	if len(v.blockers) != 0 {
		return nil, normalizedBlockers(v.blockers)
	}
	return model, nil
}

func (v *validator) validateGroundingAxioms() {
	domains := make(map[string]semanticir.Domain, len(v.task.Domains))
	for _, domain := range v.task.Domains {
		domains[domain.ID] = domain
	}
	for _, operation := range v.task.Operations {
		if operation.Kind == semanticir.OperationTest {
			continue
		}
		for _, domainID := range operation.DomainIDs {
			domain, exists := domains[domainID]
			if !exists {
				continue
			}
			for _, value := range domain.Values {
				grounding, ok := value.GroundingFor(operation.ID)
				if !ok || grounding.Kind != semanticir.GroundingMembership || grounding.Membership == nil {
					v.add("missing-domain-grounding", fmt.Sprintf("operation %q domain %q label %q has no unique closed membership grounding", operation.ID, domainID, value.ID), &value.Provenance)
					continue
				}
				v.expressionProvenance(grounding.Membership, fmt.Sprintf("operation %q domain %q label %q membership", operation.ID, domainID, value.ID))
				if grounding.Membership.Type != semanticir.TypeBool {
					v.add("invalid-domain-grounding", fmt.Sprintf("operation %q domain %q label %q membership is not boolean", operation.ID, domainID, value.ID), &grounding.Provenance)
				}
				if len(grounding.ConcreteWitness) != len(operation.Inputs) {
					v.add("incomplete-domain-grounding", fmt.Sprintf("operation %q domain %q label %q witness assigns %d inputs; want %d", operation.ID, domainID, value.ID, len(grounding.ConcreteWitness), len(operation.Inputs)), &grounding.Provenance)
					continue
				}
				passed, err := semanticir.EvaluateGroundingMembership(*grounding.Membership, grounding.ConcreteWitness)
				if err != nil || !passed {
					v.add("invalid-domain-grounding", fmt.Sprintf("operation %q domain %q label %q witness does not satisfy its membership: %v", operation.ID, domainID, value.ID, err), &grounding.Provenance)
				}
			}
		}
	}
}

func (v *validator) validateAssignmentGroundings(reachable map[string][]semanticir.Assignment) {
	wanted := make(map[caseKey]bool)
	for operationID, assignments := range reachable {
		for _, assignment := range assignments {
			wanted[caseKey{operation: operationID, assignment: canonicalAssignment(v.operationDomains[operationID], assignment)}] = true
		}
	}
	seen := make(map[caseKey]bool)
	for _, grounding := range v.task.Groundings {
		operation, exists := v.operations[grounding.OperationID]
		key := caseKey{operation: grounding.OperationID, assignment: canonicalAssignment(v.operationDomains[grounding.OperationID], grounding.Conditions)}
		if !exists || !wanted[key] || seen[key] || grounding.ID != semanticir.AssignmentGroundingID(grounding.OperationID, grounding.Conditions) {
			v.add("invalid-assignment-grounding", fmt.Sprintf("assignment grounding %q is unknown, unreachable, duplicated, or noncanonical", grounding.ID), &grounding.Provenance)
			continue
		}
		seen[key] = true
		conjunction, err := semanticir.GroundingConjunction(operation, v.task.Domains, grounding.Conditions, grounding.Provenance)
		if err != nil {
			v.add("invalid-assignment-grounding", fmt.Sprintf("assignment grounding %q: %v", grounding.ID, err), &grounding.Provenance)
			continue
		}
		passed, evaluationErr := semanticir.EvaluateGroundingMembership(conjunction, grounding.Inputs)
		if evaluationErr != nil || !passed {
			v.add("invalid-assignment-grounding", fmt.Sprintf("assignment grounding %q concrete witness does not satisfy all selected labels: %v", grounding.ID, evaluationErr), &grounding.Provenance)
		}
	}
	for key := range wanted {
		if !seen[key] {
			v.add("missing-assignment-grounding", fmt.Sprintf("reachable behavior %q %s has no outcome-free concrete reachability witness", key.operation, key.assignment), nil)
		}
	}
}

func (v *validator) validateArtifacts() {
	if strings.TrimSpace(v.task.ID) == "" {
		v.add("missing-task-id", "task ID is empty", nil)
	}
	v.registerArtifact(v.task.Spec, "task spec")
	if v.task.Spec.Kind != semanticir.ArtifactSpec {
		v.add("invalid-spec-artifact", "task Spec must have artifact kind spec", nil)
	}
	v.registerArtifact(v.task.Instruction, "task instruction")
	if v.task.Instruction.Kind != semanticir.ArtifactInstruction {
		v.add("invalid-instruction-artifact", "task Instruction must have artifact kind instruction", nil)
	}
	v.registerArtifact(v.task.InstructionModel.Artifact, "instruction model")
	if v.task.InstructionModel.Artifact != v.task.Instruction {
		v.add("stale-evidence", "instruction model is not bound to Task.Instruction", nil)
	}
	if v.task.Environment == nil {
		v.add("missing-environment-model", "task has no typed frozen environment model", nil)
	} else {
		configuration := environmentConfiguration(v.task.Environment)
		v.registerArtifact(configuration, "environment configuration")
		if configuration.Kind != semanticir.ArtifactConfiguration {
			v.add("invalid-environment-model", "environment configuration does not have configuration kind", &v.task.Environment.Provenance)
		}
		seenSources := map[string]bool{}
		for _, source := range v.task.Environment.SourceArtifacts {
			v.registerArtifact(source, "environment source")
			if source.Kind != semanticir.ArtifactEnvironment || seenSources[source.ID] {
				v.add("invalid-environment-model", "environment model has an invalid or duplicate environment source", &v.task.Environment.Provenance)
			}
			seenSources[source.ID] = true
		}
		if len(seenSources) == 0 {
			v.add("missing-environment-source", "environment model has no frozen environment source artifacts", &v.task.Environment.Provenance)
		}
	}
	for i := range v.task.Artifacts {
		model := &v.task.Artifacts[i]
		v.registerArtifact(model.Artifact, fmt.Sprintf("artifact model %d", i))
		if model.Kind == semanticir.ArtifactTests && model.RunnerSelection != nil {
			v.registerArtifact(model.RunnerSelection.Configuration, fmt.Sprintf("test runner configuration for artifact model %q", model.Artifact.ID))
		}
		if model.Kind != model.Artifact.Kind {
			v.add("artifact-kind-mismatch", fmt.Sprintf("artifact model %q kind %q differs from artifact kind %q", model.Artifact.ID, model.Kind, model.Artifact.Kind), nil)
		}
	}
	for _, required := range []semanticir.ArtifactKind{semanticir.ArtifactInstruction, semanticir.ArtifactSpec, semanticir.ArtifactCode, semanticir.ArtifactTests} {
		found := false
		for _, artifact := range v.artifacts {
			if artifact.Kind == required {
				found = true
				break
			}
		}
		if !found {
			v.add("missing-artifact", fmt.Sprintf("no frozen %s artifact is registered", required), nil)
		}
	}
	if v.task.Environment != nil && environmentConfiguration(v.task.Environment).Kind != semanticir.ArtifactConfiguration {
		v.add("missing-artifact", "no frozen configuration source artifact is registered", nil)
	}
}

func (v *validator) registerArtifact(artifact semanticir.ArtifactRef, label string) {
	if strings.TrimSpace(artifact.ID) == "" {
		v.add("invalid-artifact", label+" has an empty ID", nil)
		return
	}
	if artifact.Kind == "" || strings.TrimSpace(artifact.Path) == "" || !digestPattern.MatchString(artifact.Digest) {
		v.add("invalid-artifact", fmt.Sprintf("%s %q is missing kind/path or a canonical SHA-256 digest", label, artifact.ID), nil)
	}
	if previous, ok := v.artifacts[artifact.ID]; ok {
		if previous != artifact {
			v.add("stale-evidence", fmt.Sprintf("artifact ID %q has conflicting frozen references", artifact.ID), nil)
		}
		return
	}
	v.artifacts[artifact.ID] = artifact
}

func environmentConfiguration(environment *semanticir.EnvironmentModel) semanticir.ArtifactRef {
	if environment == nil {
		return semanticir.ArtifactRef{}
	}
	if environment.Configuration != (semanticir.ArtifactRef{}) {
		return environment.Configuration
	}
	if environment.Artifact.Kind == semanticir.ArtifactConfiguration {
		return environment.Artifact
	}
	return semanticir.ArtifactRef{}
}

func (v *validator) validateCoverage() {
	v.checkCoverage(&v.task.InstructionModel.Coverage, "instruction model coverage")
	if v.task.Environment != nil {
		v.checkCoverage(&v.task.Environment.Coverage, "environment model coverage")
		if v.task.Environment.Coverage.Status == semanticir.TranslationComplete && v.task.Environment.Coverage.TotalConstructs > 0 && v.task.Environment.Coverage.TranslatedConstructs == v.task.Environment.Coverage.TotalConstructs && len(v.task.Environment.Coverage.Unsupported) == 0 {
			for _, source := range v.task.Environment.SourceArtifacts {
				v.covered[source.ID] = true
			}
		}
	}
	if v.task.TestSuite != nil {
		v.checkCoverage(&v.task.TestSuite.Coverage, "authoritative test-suite coverage")
	}
	for i := range v.task.Coverage {
		v.checkCoverage(&v.task.Coverage[i], fmt.Sprintf("task coverage %d", i))
	}
	for i := range v.task.Artifacts {
		model := &v.task.Artifacts[i]
		if model.Kind == semanticir.ArtifactTests {
			v.checkAdvisoryCoverage(&model.Coverage, fmt.Sprintf("advisory test artifact %q coverage", model.Artifact.ID))
			if model.RunnerSelection != nil && testSuiteHasExactSource(v.task.TestSuite, model.RunnerSelection.Configuration) {
				// Runner configuration is an immutable input selected by the
				// independently validated test frontend. It is not itself a
				// translated source construct, so its exact suite binding closes
				// provenance without inventing a translation coverage record.
				v.covered[model.RunnerSelection.Configuration.ID] = true
			}
		} else {
			v.checkCoverage(&model.Coverage, fmt.Sprintf("artifact %q coverage", model.Artifact.ID))
		}
	}
	for id, artifact := range v.artifacts {
		if artifact.Kind != semanticir.ArtifactInstruction && artifact.Kind != semanticir.ArtifactSpec && artifact.Kind != semanticir.ArtifactCode && artifact.Kind != semanticir.ArtifactEnvironment && artifact.Kind != semanticir.ArtifactConfiguration {
			continue
		}
		if !v.covered[id] {
			v.add("missing-translation-coverage", fmt.Sprintf("artifact %q has no complete translation coverage", id), nil)
		}
	}
}

func testSuiteHasExactSource(suite *semanticir.TestSuiteModel, wanted semanticir.ArtifactRef) bool {
	if suite == nil {
		return false
	}
	for _, source := range suite.SourceArtifacts {
		if source == wanted {
			return true
		}
	}
	return false
}

func (v *validator) checkAdvisoryCoverage(coverage *semanticir.TranslationCoverage, label string) {
	if !v.rawProvenance(coverage.Provenance, label, true) {
		return
	}
	// This only authorizes provenance resolution for an immutable advisory
	// source model. TestsPass still comes exclusively from the independently
	// exhaustive TestSuite and never from this coverage record.
	v.covered[coverage.Provenance.ArtifactID] = true
	if coverage.TotalConstructs < 0 || coverage.TranslatedConstructs < 0 || coverage.TranslatedConstructs > coverage.TotalConstructs {
		v.add("invalid-advisory-translation", fmt.Sprintf("%s has invalid construct counts %d/%d", label, coverage.TranslatedConstructs, coverage.TotalConstructs), &coverage.Provenance)
	}
	if coverage.Status == semanticir.TranslationComplete && (coverage.TotalConstructs <= 0 || coverage.TranslatedConstructs != coverage.TotalConstructs || len(coverage.Unsupported) != 0) {
		v.add("invalid-advisory-translation", label+" claims complete coverage with incomplete or unsupported constructs", &coverage.Provenance)
	}
}

func (v *validator) checkCoverage(coverage *semanticir.TranslationCoverage, label string) {
	if coverage.Status != semanticir.TranslationComplete || coverage.TotalConstructs <= 0 || coverage.TranslatedConstructs <= 0 || coverage.TranslatedConstructs != coverage.TotalConstructs || len(coverage.Unsupported) != 0 {
		v.add("incomplete-translation", fmt.Sprintf("%s is not complete (%d/%d translated, %d unsupported)", label, coverage.TranslatedConstructs, coverage.TotalConstructs, len(coverage.Unsupported)), &coverage.Provenance)
	}
	if !v.rawProvenance(coverage.Provenance, label, true) {
		return
	}
	if coverage.Status == semanticir.TranslationComplete && coverage.TranslatedConstructs == coverage.TotalConstructs && len(coverage.Unsupported) == 0 {
		v.covered[coverage.Provenance.ArtifactID] = true
	}
}

func (v *validator) validateEvidence() {
	v.provenance(v.task.Provenance, "task")
	for i := range v.task.InstructionModel.Clauses {
		v.provenance(v.task.InstructionModel.Clauses[i].Provenance, fmt.Sprintf("instruction clause %q", v.task.InstructionModel.Clauses[i].ID))
	}
	if v.task.Environment != nil {
		v.provenance(v.task.Environment.Provenance, "environment model")
		for i := range v.task.Environment.Commands {
			command := &v.task.Environment.Commands[i]
			v.provenance(command.Provenance, fmt.Sprintf("environment command %q", command.ID))
			v.provenance(command.PassSignal.Provenance, fmt.Sprintf("environment command %q pass signal", command.ID))
		}
	}
	for i := range v.task.Domains {
		domain := &v.task.Domains[i]
		v.provenance(domain.Provenance, fmt.Sprintf("domain %q", domain.ID))
		v.requireProvenanceKind(domain.Provenance, semanticir.ArtifactSpec, fmt.Sprintf("domain %q", domain.ID))
		for j := range domain.Values {
			v.provenance(domain.Values[j].Provenance, fmt.Sprintf("domain %q value %q", domain.ID, domain.Values[j].ID))
			v.requireProvenanceKind(domain.Values[j].Provenance, semanticir.ArtifactSpec, fmt.Sprintf("domain %q value %q", domain.ID, domain.Values[j].ID))
		}
	}
	for i := range v.task.Constraints {
		v.provenance(v.task.Constraints[i].Provenance, fmt.Sprintf("constraint %q", v.task.Constraints[i].ID))
		v.requireProvenanceKind(v.task.Constraints[i].Provenance, semanticir.ArtifactSpec, fmt.Sprintf("constraint %q", v.task.Constraints[i].ID))
	}
	for i := range v.task.Operations {
		operation := &v.task.Operations[i]
		v.provenance(operation.Provenance, fmt.Sprintf("operation %q", operation.ID))
		v.requireProvenanceKind(operation.Provenance, semanticir.ArtifactSpec, fmt.Sprintf("operation %q", operation.ID))
		for j := range operation.Inputs {
			v.provenance(operation.Inputs[j].Provenance, fmt.Sprintf("operation %q input %q", operation.ID, operation.Inputs[j].Name))
			v.requireProvenanceKind(operation.Inputs[j].Provenance, semanticir.ArtifactSpec, fmt.Sprintf("operation %q input %q", operation.ID, operation.Inputs[j].Name))
		}
	}
	for i := range v.task.Outcomes {
		outcome := &v.task.Outcomes[i]
		v.provenance(outcome.Provenance, fmt.Sprintf("outcome %q", outcome.ID))
		v.requireProvenanceKind(outcome.Provenance, semanticir.ArtifactSpec, fmt.Sprintf("outcome %q", outcome.ID))
		for j := range outcome.Effects {
			v.provenance(outcome.Effects[j].Provenance, fmt.Sprintf("outcome %q effect %q", outcome.ID, outcome.Effects[j].ID))
			v.requireProvenanceKind(outcome.Effects[j].Provenance, semanticir.ArtifactSpec, fmt.Sprintf("outcome %q effect %q", outcome.ID, outcome.Effects[j].ID))
			if outcome.Effects[j].Value != nil {
				v.expressionProvenance(outcome.Effects[j].Value, fmt.Sprintf("outcome %q effect %q value", outcome.ID, outcome.Effects[j].ID))
				v.requireExpressionKind(outcome.Effects[j].Value, semanticir.ArtifactSpec, fmt.Sprintf("outcome %q effect %q value", outcome.ID, outcome.Effects[j].ID))
			}
		}
	}
	for i := range v.task.Requirements {
		requirement := &v.task.Requirements[i]
		v.provenance(requirement.Provenance, fmt.Sprintf("requirement %q", requirement.ID))
		v.requireProvenanceKind(requirement.Provenance, semanticir.ArtifactSpec, fmt.Sprintf("requirement %q", requirement.ID))
		for j := range requirement.InstructionSources {
			v.provenance(requirement.InstructionSources[j], fmt.Sprintf("requirement %q instruction source %d", requirement.ID, j))
		}
		for j := range requirement.Evidence {
			v.provenance(requirement.Evidence[j], fmt.Sprintf("requirement %q evidence %d", requirement.ID, j))
		}
		for j := range requirement.Effects {
			v.provenance(requirement.Effects[j].Provenance, fmt.Sprintf("requirement %q effect %q", requirement.ID, requirement.Effects[j].ID))
			v.requireProvenanceKind(requirement.Effects[j].Provenance, semanticir.ArtifactSpec, fmt.Sprintf("requirement %q effect %q", requirement.ID, requirement.Effects[j].ID))
			if requirement.Effects[j].Value != nil {
				v.expressionProvenance(requirement.Effects[j].Value, fmt.Sprintf("requirement %q effect %q value", requirement.ID, requirement.Effects[j].ID))
				v.requireExpressionKind(requirement.Effects[j].Value, semanticir.ArtifactSpec, fmt.Sprintf("requirement %q effect %q value", requirement.ID, requirement.Effects[j].ID))
			}
		}
	}
	for i := range v.task.Invariants {
		invariant := &v.task.Invariants[i]
		v.provenance(invariant.Provenance, fmt.Sprintf("invariant %q", invariant.ID))
		v.requireProvenanceKind(invariant.Provenance, semanticir.ArtifactSpec, fmt.Sprintf("invariant %q", invariant.ID))
		v.expressionProvenance(&invariant.Predicate, fmt.Sprintf("invariant %q predicate", invariant.ID))
		v.requireExpressionKind(&invariant.Predicate, semanticir.ArtifactSpec, fmt.Sprintf("invariant %q predicate", invariant.ID))
		for j := range invariant.Bindings {
			v.provenance(invariant.Bindings[j].Provenance, fmt.Sprintf("invariant %q binding %q", invariant.ID, invariant.Bindings[j].Variable))
			v.requireProvenanceKind(invariant.Bindings[j].Provenance, semanticir.ArtifactSpec, fmt.Sprintf("invariant %q binding %q", invariant.ID, invariant.Bindings[j].Variable))
		}
	}
	for i := range v.task.CodeCases {
		v.provenance(v.task.CodeCases[i].Provenance, fmt.Sprintf("code case %q", v.task.CodeCases[i].ID))
		v.requireProvenanceKind(v.task.CodeCases[i].Provenance, semanticir.ArtifactCode, fmt.Sprintf("code case %q", v.task.CodeCases[i].ID))
	}
	for i := range v.task.Tests {
		test := &v.task.Tests[i]
		v.provenance(test.Provenance, fmt.Sprintf("test model %q", test.ID))
		v.requireProvenanceKind(test.Provenance, semanticir.ArtifactTests, fmt.Sprintf("test model %q", test.ID))
		// Assertions and parsed predicates are advisory audit material. Their
		// unsupported subexpressions cannot weaken or override the authoritative
		// exhaustive TestSuite predicate, so they are deliberately not evaluated.
	}
}

func (v *validator) requireProvenanceKind(provenance semanticir.Provenance, kind semanticir.ArtifactKind, label string) {
	artifact, exists := v.artifacts[provenance.ArtifactID]
	if !exists || artifact.Kind != kind {
		v.add("invalid-evidence-source", fmt.Sprintf("%s is not derived from a frozen %s artifact", label, kind), &provenance)
	}
}

func (v *validator) expressionProvenance(expression *semanticir.Expression, label string) {
	if expression == nil {
		return
	}
	v.provenance(expression.Provenance, label)
	for i := range expression.Operands {
		v.expressionProvenance(&expression.Operands[i], fmt.Sprintf("%s operand %d", label, i))
	}
}

func (v *validator) requireExpressionKind(expression *semanticir.Expression, kind semanticir.ArtifactKind, label string) {
	if expression == nil {
		return
	}
	v.requireProvenanceKind(expression.Provenance, kind, label)
	for i := range expression.Operands {
		v.requireExpressionKind(&expression.Operands[i], kind, fmt.Sprintf("%s operand %d", label, i))
	}
}

func (v *validator) validateInstructionEnvironment() {
	clauseIDs := make(map[string]bool)
	if len(v.task.InstructionModel.Clauses) == 0 {
		v.add("missing-instruction-evidence", "instruction model has no reviewed clauses", nil)
	}
	for _, clause := range v.task.InstructionModel.Clauses {
		if strings.TrimSpace(clause.ID) == "" || clauseIDs[clause.ID] || !digestPattern.MatchString(clause.SliceDigest) {
			v.add("invalid-instruction-evidence", fmt.Sprintf("instruction clause %q has an empty/duplicate ID or invalid slice digest", clause.ID), &clause.Provenance)
		}
		clauseIDs[clause.ID] = true
		if clause.Provenance.ArtifactID != v.task.Instruction.ID || clause.Span != clause.Provenance.Location {
			v.add("invalid-instruction-evidence", fmt.Sprintf("instruction clause %q is not bound to its frozen instruction span", clause.ID), &clause.Provenance)
		}
	}
	if environment := v.task.Environment; environment != nil {
		if strings.TrimSpace(environment.Identity) == "" || !digestPattern.MatchString(environment.ConfigDigest) {
			v.add("invalid-environment-model", "environment identity or configuration digest is missing", &environment.Provenance)
		}
		if len(environment.Tools) == 0 {
			v.add("missing-tool-evidence", "environment model has no immutable tool references", &environment.Provenance)
		}
		toolSet := make(map[semanticir.ToolRef]bool)
		for _, tool := range environment.Tools {
			v.validateTool(tool, "environment")
			if toolSet[tool] {
				v.add("duplicate-tool-evidence", fmt.Sprintf("environment repeats tool %q", tool.Name), &environment.Provenance)
			}
			toolSet[tool] = true
		}
		commandIDs := make(map[string]bool)
		workspaceStates := make(map[semanticir.WorkspaceState]int)
		for _, command := range environment.Commands {
			if strings.TrimSpace(command.ID) == "" || commandIDs[command.ID] || strings.TrimSpace(command.WorkspaceID) == "" || strings.TrimSpace(command.Command) == "" || strings.TrimSpace(command.WorkingDirectory) == "" || command.TimeoutMillis <= 0 {
				v.add("invalid-environment-command", fmt.Sprintf("environment command %q is incomplete or duplicated", command.ID), &command.Provenance)
			}
			if !command.ClearEnvironment || !command.KillProcessGroup {
				v.add("invalid-environment-command", fmt.Sprintf("environment command %q may inherit ambient state or leak child processes", command.ID), &command.Provenance)
			}
			environmentDigest, environmentErr := semanticir.Digest(command.Environment)
			if environmentErr != nil || environmentDigest != command.EnvironmentDigest {
				v.add("invalid-environment-command", fmt.Sprintf("environment command %q environment entries do not match their digest", command.ID), &command.Provenance)
			}
			previousVariable := ""
			for variableIndex, variable := range command.Environment {
				if strings.TrimSpace(variable.Name) == "" || strings.Contains(variable.Name, "=") || strings.ContainsRune(variable.Name, '\x00') || strings.ContainsRune(variable.Value, '\x00') || (variableIndex > 0 && variable.Name <= previousVariable) {
					v.add("invalid-environment-command", fmt.Sprintf("environment command %q variables are not a sorted unique NUL-free name/value list", command.ID), &command.Provenance)
					break
				}
				previousVariable = variable.Name
			}
			commandIDs[command.ID] = true
			workspaceStates[command.State]++
			for _, digest := range []string{command.TreeDigest, command.EnvironmentDigest, command.StdoutDigest, command.StderrDigest, command.SignalValueDigest} {
				if !digestPattern.MatchString(digest) {
					v.add("invalid-environment-command", fmt.Sprintf("environment command %q has an invalid evidence digest", command.ID), &command.Provenance)
					break
				}
			}
			if command.ObservedPass != command.ExpectedPass {
				v.add("stale-environment-evidence", fmt.Sprintf("environment command %q observed pass does not match its expected pass", command.ID), &command.Provenance)
			}
			switch command.PassSignal.Kind {
			case semanticir.PassSignalExitCode:
				if strings.TrimSpace(command.PassSignal.Expected) == "" {
					v.add("invalid-pass-signal", fmt.Sprintf("command %q exit-code signal has no expected value", command.ID), &command.PassSignal.Provenance)
				}
			case semanticir.PassSignalFile:
				if strings.TrimSpace(command.PassSignal.Path) == "" || strings.TrimSpace(command.PassSignal.Expected) == "" {
					v.add("invalid-pass-signal", fmt.Sprintf("command %q file signal is incomplete", command.ID), &command.PassSignal.Provenance)
				}
			default:
				v.add("invalid-pass-signal", fmt.Sprintf("command %q has unknown pass signal kind %q", command.ID, command.PassSignal.Kind), &command.PassSignal.Provenance)
			}
			for _, tool := range command.Tools {
				v.validateTool(tool, "command "+strconv.Quote(command.ID))
				if !toolSet[tool] {
					v.add("missing-tool-evidence", fmt.Sprintf("command %q uses tool %q absent from the environment", command.ID, tool.Name), &command.Provenance)
				}
			}
		}
		for _, state := range []semanticir.WorkspaceState{semanticir.WorkspaceBaseOldTests, semanticir.WorkspaceBaseNewTests, semanticir.WorkspaceSolutionNewTests} {
			if workspaceStates[state] != 1 {
				v.add("missing-workspace-evidence", fmt.Sprintf("environment model has %d records for workspace state %q; want exactly one", workspaceStates[state], state), &environment.Provenance)
			}
		}
	}
	for _, artifact := range v.task.Artifacts {
		v.validateTool(artifact.Translator, "artifact "+strconv.Quote(artifact.Artifact.ID)+" translator")
		found := false
		if v.task.Environment != nil {
			for _, tool := range v.task.Environment.Tools {
				if tool == artifact.Translator {
					found = true
					break
				}
			}
		}
		if !found {
			v.add("missing-tool-evidence", fmt.Sprintf("artifact %q translator is not frozen in the environment model", artifact.Artifact.ID), &artifact.Coverage.Provenance)
		}
	}
	for _, requirement := range v.task.Requirements {
		if len(requirement.InstructionClauseIDs) == 0 {
			v.add("missing-instruction-evidence", fmt.Sprintf("requirement %q cites no instruction clause", requirement.ID), &requirement.Provenance)
		}
		seen := make(map[string]bool)
		for _, clauseID := range requirement.InstructionClauseIDs {
			if seen[clauseID] || !clauseIDs[clauseID] {
				v.add("invalid-instruction-evidence", fmt.Sprintf("requirement %q cites missing or duplicate instruction clause %q", requirement.ID, clauseID), &requirement.Provenance)
			}
			seen[clauseID] = true
		}
	}
}

func (v *validator) validateTool(tool semanticir.ToolRef, label string) {
	if strings.TrimSpace(tool.Name) == "" || !filepath.IsAbs(tool.Path) || strings.TrimSpace(tool.Version) == "" || !digestPattern.MatchString(tool.Digest) {
		v.add("invalid-tool-evidence", fmt.Sprintf("%s has an incomplete immutable tool reference %q", label, tool.Name), nil)
	}
}

func (v *validator) validateFlattenedModels() {
	var translatedCode []semanticir.BehaviorCase
	var translatedTests []semanticir.TestModel
	codeOperations := make(map[string]bool)
	for _, artifact := range v.task.Artifacts {
		for _, operation := range artifact.Operations {
			// Test helper operations are frontend-local. semanticir.AddArtifact
			// deliberately excludes them from the frozen behavior vocabulary;
			// TestsPass is represented by the global TestPredicate instead.
			if operation.Kind == semanticir.OperationTest {
				continue
			}
			authoritative, exists := findOperation(v.task.Operations, operation.ID)
			if !exists || !operationsSemanticallyEqual(authoritative, operation) {
				v.add("conflicting-operation-model", fmt.Sprintf("artifact %q operation %q does not match the authoritative operation vocabulary", artifact.Artifact.ID, operation.ID), &operation.Provenance)
			}
			if artifact.Kind == semanticir.ArtifactCode && operation.Kind != semanticir.OperationTest {
				codeOperations[operation.ID] = true
			}
		}
		for _, outcome := range artifact.Outcomes {
			authoritative, exists := findOutcome(v.task.Outcomes, outcome.ID)
			if !exists || !outcomesSemanticallyEqual(authoritative, outcome) {
				v.add("conflicting-outcome-model", fmt.Sprintf("artifact %q outcome %q does not match the authoritative outcome vocabulary", artifact.Artifact.ID, outcome.ID), &outcome.Provenance)
			}
		}
		if artifact.Kind == semanticir.ArtifactCode && !outcomeVocabulariesSemanticallyEqual(v.task.Outcomes, artifact.Outcomes) {
			v.add("conflicting-outcome-model", fmt.Sprintf("artifact %q outcome vocabulary does not exactly match the authoritative vocabulary", artifact.Artifact.ID), &artifact.Coverage.Provenance)
		}
		switch artifact.Kind {
		case semanticir.ArtifactCode:
			translatedCode = append(translatedCode, artifact.Cases...)
		case semanticir.ArtifactTests:
			translatedTests = append(translatedTests, artifact.Tests...)
		}
	}
	code := append([]semanticir.BehaviorCase(nil), v.task.CodeCases...)
	sort.Slice(translatedCode, func(i, j int) bool { return translatedCode[i].ID < translatedCode[j].ID })
	sort.Slice(code, func(i, j int) bool { return code[i].ID < code[j].ID })
	if !reflect.DeepEqual(translatedCode, code) {
		v.add("conflicting-code-model", "flattened Task.CodeCases differs from frozen code ArtifactModel cases", nil)
	}
	if len(v.task.Tests) != 0 {
		tests := append([]semanticir.TestModel(nil), v.task.Tests...)
		sort.Slice(translatedTests, func(i, j int) bool { return translatedTests[i].ID < translatedTests[j].ID })
		sort.Slice(tests, func(i, j int) bool { return tests[i].ID < tests[j].ID })
		if !reflect.DeepEqual(translatedTests, tests) {
			v.add("conflicting-test-model", "non-empty flattened Task.Tests differs from frozen advisory test ArtifactModel predicates", nil)
		}
	}
	for _, operation := range v.task.Operations {
		if operation.Kind != semanticir.OperationTest && !codeOperations[operation.ID] {
			v.add("missing-code-operation", fmt.Sprintf("operation %q has no matching frozen code translation", operation.ID), &operation.Provenance)
		}
	}
}

func findOperation(operations []semanticir.Operation, id string) (semanticir.Operation, bool) {
	for _, operation := range operations {
		if operation.ID == id {
			return operation, true
		}
	}
	return semanticir.Operation{}, false
}

func findOutcome(outcomes []semanticir.ObservableOutcome, id string) (semanticir.ObservableOutcome, bool) {
	for _, outcome := range outcomes {
		if outcome.ID == id {
			return outcome, true
		}
	}
	return semanticir.ObservableOutcome{}, false
}

func operationsSemanticallyEqual(left, right semanticir.Operation) bool {
	// The frozen spec owns the operation's semantic scope. Frontends may add
	// typed Inputs while translating a callable; those are implementation
	// evidence, not a second declaration of the spec vocabulary. Keep this
	// check aligned with semanticir.AddArtifact.
	kindMatches := left.Kind == semanticir.OperationCallable || left.Kind == right.Kind
	return left.ID == right.ID && kindMatches && reflect.DeepEqual(left.DomainIDs, right.DomainIDs) && stringSetsEqual(left.OutcomeIDs, right.OutcomeIDs)
}

func outcomesSemanticallyEqual(left, right semanticir.ObservableOutcome) bool {
	if left.ID != right.ID || left.Kind != right.Kind || left.OperationID != right.OperationID || left.ExceptionType != right.ExceptionType || left.Message != right.Message || !literalPointersEqual(left.Value, right.Value) || len(left.Effects) != len(right.Effects) {
		return false
	}
	// Effects are an observable trace, so their order is semantic. This mirrors
	// semanticir.AddArtifact rather than treating effects as a set.
	for i := range left.Effects {
		if left.Effects[i].ID != right.Effects[i].ID || left.Effects[i].Kind != right.Effects[i].Kind || left.Effects[i].Target != right.Effects[i].Target || !expressionPointersEqual(left.Effects[i].Value, right.Effects[i].Value) {
			return false
		}
	}
	return true
}

func outcomeVocabulariesSemanticallyEqual(left, right []semanticir.ObservableOutcome) bool {
	if len(left) != len(right) {
		return false
	}
	byID := make(map[string]semanticir.ObservableOutcome, len(left))
	for _, outcome := range left {
		if _, exists := byID[outcome.ID]; exists {
			return false
		}
		byID[outcome.ID] = outcome
	}
	seen := make(map[string]bool, len(right))
	for _, outcome := range right {
		declared, exists := byID[outcome.ID]
		if !exists || seen[outcome.ID] || !outcomesSemanticallyEqual(declared, outcome) {
			return false
		}
		seen[outcome.ID] = true
	}
	return true
}

func literalPointersEqual(left, right *semanticir.Literal) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return reflect.DeepEqual(*left, *right)
}

func stringSetsEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := append([]string(nil), left...)
	rightCopy := append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	return reflect.DeepEqual(leftCopy, rightCopy)
}

func (v *validator) provenance(provenance semanticir.Provenance, label string) {
	if !v.rawProvenance(provenance, label, false) {
		return
	}
	if !v.covered[provenance.ArtifactID] {
		v.add("missing-translation-coverage", fmt.Sprintf("%s cites artifact %q without complete coverage", label, provenance.ArtifactID), &provenance)
	}
}

func (v *validator) rawProvenance(provenance semanticir.Provenance, label string, aggregate bool) bool {
	artifact, ok := v.artifacts[provenance.ArtifactID]
	if !ok {
		v.add("missing-evidence", fmt.Sprintf("%s cites unknown artifact %q", label, provenance.ArtifactID), &provenance)
		return false
	}
	if provenance.ArtifactDigest != artifact.Digest {
		v.add("stale-evidence", fmt.Sprintf("%s digest does not match frozen artifact %q", label, artifact.ID), &provenance)
		return false
	}
	translationOK := provenance.Translation == semanticir.TranslationTranslated
	if aggregate {
		translationOK = translationOK || provenance.Translation == semanticir.TranslationComplete
	}
	location := provenance.Location
	// EndColumn zero denotes a line-only anchor. It is valid with either an
	// unknown end (0/0) or a known end line, as emitted by the spec compiler.
	validEnd := (location.EndLine == 0 && location.EndColumn == 0) || (location.EndLine >= location.StartLine && location.EndColumn >= 0)
	if location.EndColumn > 0 && location.EndLine == location.StartLine && location.EndColumn < location.StartColumn {
		validEnd = false
	}
	if !translationOK || location.Path == "" || location.Path != artifact.Path || location.StartLine < 1 || location.StartColumn < 1 || !validEnd {
		v.add("invalid-provenance", fmt.Sprintf("%s has incomplete translation evidence or an invalid source location", label), &provenance)
		return false
	}
	return true
}

func (v *validator) validateDomains() {
	if len(v.task.Domains) == 0 {
		v.add("non-finite-domain", "task declares no finite domains", nil)
		return
	}
	for _, domain := range v.task.Domains {
		if strings.TrimSpace(domain.ID) == "" {
			v.add("invalid-domain", "domain ID is empty", &domain.Provenance)
			continue
		}
		if _, exists := v.domains[domain.ID]; exists {
			v.add("duplicate-domain", fmt.Sprintf("domain ID %q is duplicated", domain.ID), &domain.Provenance)
			continue
		}
		values := make(map[string]bool)
		if !validValueType(domain.Type) {
			v.add("invalid-domain", fmt.Sprintf("domain %q has unknown or unsupported type %q", domain.ID, domain.Type), &domain.Provenance)
		}
		if len(domain.Values) == 0 {
			v.add("non-finite-domain", fmt.Sprintf("domain %q has no explicitly enumerated values", domain.ID), &domain.Provenance)
		}
		for _, value := range domain.Values {
			if strings.TrimSpace(value.ID) == "" {
				v.add("invalid-domain-value", fmt.Sprintf("domain %q contains an empty value ID", domain.ID), &value.Provenance)
				continue
			}
			if values[value.ID] {
				v.add("duplicate-domain-value", fmt.Sprintf("domain %q value %q is duplicated", domain.ID, value.ID), &value.Provenance)
			}
			values[value.ID] = true
			// Domain value IDs are finite semantic labels. A direct literal is
			// optional; concrete inputs are established by the closed
			// per-operation grounding axiom instead.
			if value.Value != nil {
				v.validateLiteral(value.Value, domain.Type, fmt.Sprintf("domain %q value %q", domain.ID, value.ID), &value.Provenance, make(map[any]bool))
			}
		}
		v.domains[domain.ID] = values
		v.domainIDs = append(v.domainIDs, domain.ID)
	}
	sort.Strings(v.domainIDs)
}

func (v *validator) validateOperations() {
	for _, operation := range v.task.Operations {
		if strings.TrimSpace(operation.ID) == "" {
			v.add("invalid-operation", "operation ID is empty", &operation.Provenance)
			continue
		}
		if _, exists := v.operations[operation.ID]; exists {
			v.add("duplicate-operation", fmt.Sprintf("operation ID %q is duplicated", operation.ID), &operation.Provenance)
			continue
		}
		switch operation.Kind {
		case semanticir.OperationCallable, semanticir.OperationFunction, semanticir.OperationMethod, semanticir.OperationTest:
		default:
			v.add("invalid-operation", fmt.Sprintf("operation %q has unknown kind %q", operation.ID, operation.Kind), &operation.Provenance)
		}
		domainSeen := make(map[string]bool)
		for _, domainID := range operation.DomainIDs {
			if _, exists := v.domains[domainID]; !exists || domainSeen[domainID] {
				v.add("invalid-operation-domain", fmt.Sprintf("operation %q cites missing or duplicate domain %q", operation.ID, domainID), &operation.Provenance)
			}
			domainSeen[domainID] = true
		}
		domainIDs := append([]string(nil), operation.DomainIDs...)
		sort.Strings(domainIDs)
		v.operationDomains[operation.ID] = domainIDs
		inputNames := make(map[string]bool)
		for _, input := range operation.Inputs {
			directDomainValid := true
			if input.DomainID != "" {
				domainType, domainExists := v.domainType(input.DomainID)
				directDomainValid = domainSeen[input.DomainID] && domainExists && input.Type == domainType
			}
			// Operation inputs describe the concrete implementation signature. Their
			// DomainID is only an optional direct binding: semantic category
			// domains may instead be related to raw inputs by the exact frozen
			// groundings (for example a bool input classified by string labels).
			if strings.TrimSpace(input.Name) == "" || inputNames[input.Name] || !validValueType(input.Type) || !directDomainValid {
				v.add("invalid-operation-input", fmt.Sprintf("operation %q has invalid input %q for domain %q", operation.ID, input.Name, input.DomainID), &input.Provenance)
			}
			inputNames[input.Name] = true
		}
		v.operations[operation.ID] = operation
	}
}

func (v *validator) domainType(domainID string) (semanticir.ValueType, bool) {
	for _, domain := range v.task.Domains {
		if domain.ID == domainID {
			return domain.Type, true
		}
	}
	return semanticir.TypeUnknown, false
}

func (v *validator) validateOutcomes() {
	if len(v.task.Outcomes) == 0 {
		v.add("non-finite-outcomes", "task declares no finite observable outcomes", nil)
	}
	for _, outcome := range v.task.Outcomes {
		if strings.TrimSpace(outcome.ID) == "" {
			v.add("invalid-outcome", "observable outcome ID is empty", &outcome.Provenance)
			continue
		}
		if _, exists := v.outcomes[outcome.ID]; exists {
			v.add("duplicate-outcome", fmt.Sprintf("observable outcome ID %q is duplicated", outcome.ID), &outcome.Provenance)
			continue
		}
		switch outcome.Kind {
		case semanticir.OutcomeReturn, semanticir.OutcomeRaise, semanticir.OutcomeSuccess, semanticir.OutcomeOther:
		default:
			v.add("invalid-outcome", fmt.Sprintf("outcome %q has unknown kind %q", outcome.ID, outcome.Kind), &outcome.Provenance)
		}
		if outcome.Kind == semanticir.OutcomeReturn && outcome.Value == nil {
			v.add("invalid-outcome", fmt.Sprintf("return outcome %q has no typed value", outcome.ID), &outcome.Provenance)
		}
		if outcome.Value != nil {
			v.validateLiteral(outcome.Value, outcome.Value.Type, "outcome "+strconv.Quote(outcome.ID)+" value", &outcome.Provenance, make(map[any]bool))
		}
		if outcome.Kind == semanticir.OutcomeRaise && strings.TrimSpace(outcome.ExceptionType) == "" {
			v.add("invalid-outcome", fmt.Sprintf("raise outcome %q has no exception type", outcome.ID), &outcome.Provenance)
		}
		if outcome.Kind == semanticir.OutcomeOther && (strings.TrimSpace(outcome.OperationID) == "" || outcome.ID != semanticir.OtherOutcome(outcome.OperationID, outcome.Provenance).ID || outcome.Value != nil || outcome.ExceptionType != "" || outcome.Message != "" || len(outcome.Effects) != 0) {
			v.add("invalid-outcome", fmt.Sprintf("other outcome %q is not the canonical operation-scoped complement", outcome.ID), &outcome.Provenance)
		}
		v.validateEffects(outcome.Effects, "outcome "+strconv.Quote(outcome.ID))
		v.outcomes[outcome.ID] = outcome
	}
	for _, operation := range v.task.Operations {
		if operation.Kind == semanticir.OperationTest {
			continue
		}
		outcomeIDs := v.validateReferencedOutcomes(operation.OutcomeIDs, "operation "+strconv.Quote(operation.ID)+" outcome universe", false, &operation.Provenance)
		if len(outcomeIDs) == 0 {
			v.add("non-finite-outcomes", fmt.Sprintf("operation %q has no finite outcome universe", operation.ID), &operation.Provenance)
		}
		ordered := make([]string, 0, len(outcomeIDs))
		otherCount := 0
		for id := range outcomeIDs {
			ordered = append(ordered, id)
			outcome := v.outcomes[id]
			if outcome.Kind == semanticir.OutcomeOther {
				otherCount++
				if outcome.OperationID != operation.ID {
					v.add("invalid-operation-outcome", fmt.Sprintf("operation %q includes complement outcome for operation %q", operation.ID, outcome.OperationID), &operation.Provenance)
				}
			}
		}
		if otherCount != 1 {
			v.add("incomplete-outcome-universe", fmt.Sprintf("operation %q has %d explicit other-outcome complements; want exactly one", operation.ID, otherCount), &operation.Provenance)
		}
		sort.Strings(ordered)
		v.operationOutcomes[operation.ID] = ordered
	}
	for outcomeID, outcome := range v.outcomes {
		used := false
		for _, ids := range v.operationOutcomes {
			if containsString(ids, outcomeID) {
				used = true
				break
			}
		}
		if !used {
			v.add("unscoped-outcome", fmt.Sprintf("outcome %q belongs to no behavior operation", outcomeID), &outcome.Provenance)
		}
	}
}

func (v *validator) validateInvariants() {
	for _, invariant := range v.task.Invariants {
		if strings.TrimSpace(invariant.ID) == "" {
			v.add("invalid-invariant", "invariant ID is empty", &invariant.Provenance)
			continue
		}
		if _, exists := v.invariants[invariant.ID]; exists {
			v.add("duplicate-invariant", fmt.Sprintf("invariant ID %q is duplicated", invariant.ID), &invariant.Provenance)
			continue
		}
		if invariant.Predicate.Type != semanticir.TypeBool {
			v.add("invalid-invariant", fmt.Sprintf("invariant %q predicate is not boolean", invariant.ID), &invariant.Provenance)
		}
		bindingNames := make(map[string]bool)
		for _, binding := range invariant.Bindings {
			if strings.TrimSpace(binding.Variable) == "" || bindingNames[binding.Variable] {
				v.add("invalid-invariant", fmt.Sprintf("invariant %q has empty or duplicate binding %q", invariant.ID, binding.Variable), &binding.Provenance)
			}
			bindingNames[binding.Variable] = true
			switch binding.Kind {
			case semanticir.BindDomainValue:
				if _, exists := v.domains[binding.DomainID]; !exists {
					v.add("invalid-invariant", fmt.Sprintf("invariant %q binding %q cites unknown domain %q", invariant.ID, binding.Variable, binding.DomainID), &binding.Provenance)
				}
			case semanticir.BindOutcomeValue:
			case semanticir.BindEffectValue:
				if !validEffectKind(binding.EffectKind) || strings.TrimSpace(binding.EffectTarget) == "" {
					v.add("invalid-invariant", fmt.Sprintf("invariant %q effect binding %q is incomplete", invariant.ID, binding.Variable), &binding.Provenance)
				}
			default:
				v.add("invalid-invariant", fmt.Sprintf("invariant %q binding %q has unsupported kind %q", invariant.ID, binding.Variable, binding.Kind), &binding.Provenance)
			}
		}
		v.invariants[invariant.ID] = invariant
	}
}

func (v *validator) validateConstraints() map[string]map[string]semanticir.Constraint {
	keys := make(map[string]map[string]semanticir.Constraint)
	for operationID, operation := range v.operations {
		if operation.Kind != semanticir.OperationTest {
			keys[operationID] = make(map[string]semanticir.Constraint)
		}
	}
	ids := make(map[string]bool)
	for _, constraint := range v.task.Constraints {
		if strings.TrimSpace(constraint.ID) == "" || ids[constraint.ID] {
			v.add("duplicate-constraint", fmt.Sprintf("constraint ID %q is empty or duplicated", constraint.ID), &constraint.Provenance)
		}
		ids[constraint.ID] = true
		if strings.TrimSpace(constraint.Reason) == "" {
			v.add("missing-constraint-reason", fmt.Sprintf("constraint %q has no exclusion reason", constraint.ID), &constraint.Provenance)
		}
		operation, exists := v.operations[constraint.OperationID]
		if !exists || operation.Kind == semanticir.OperationTest {
			v.add("invalid-constraint", fmt.Sprintf("constraint %q cites missing or non-behavior operation %q", constraint.ID, constraint.OperationID), &constraint.Provenance)
			continue
		}
		key, ok := v.assignmentKeyFor(v.operationDomains[constraint.OperationID], constraint.Conditions, "constraint "+strconv.Quote(constraint.ID), &constraint.Provenance)
		if !ok {
			continue
		}
		if previous, duplicate := keys[constraint.OperationID][key]; duplicate {
			v.add("contradictory-universe", fmt.Sprintf("constraints %q and %q both exclude operation %q assignment", previous.ID, constraint.ID, constraint.OperationID), &constraint.Provenance)
			continue
		}
		keys[constraint.OperationID][key] = constraint
	}
	return keys
}

func (v *validator) enumerateAssignments(ctx context.Context, constraints map[string]map[string]semanticir.Constraint) (map[string][]semanticir.Assignment, uint64, uint64) {
	reachable := make(map[string][]semanticir.Assignment)
	var total, excluded uint64
	operationIDs := make([]string, 0, len(v.operations))
	for operationID, operation := range v.operations {
		if operation.Kind != semanticir.OperationTest {
			operationIDs = append(operationIDs, operationID)
		}
	}
	sort.Strings(operationIDs)
	for _, operationID := range operationIDs {
		domainIDs := v.operationDomains[operationID]
		values := make([][]string, len(domainIDs))
		for i, id := range domainIDs {
			for value := range v.domains[id] {
				values[i] = append(values[i], value)
			}
			sort.Strings(values[i])
		}
		assignment := make(semanticir.Assignment, len(domainIDs))
		var localTotal, localExcluded uint64
		var walk func(int) bool
		walk = func(index int) bool {
			if index == len(domainIDs) {
				total++
				localTotal++
				if total&1023 == 0 && ctx.Err() != nil {
					return false
				}
				copyAssignment := cloneAssignment(assignment)
				if _, ok := constraints[operationID][canonicalAssignment(domainIDs, copyAssignment)]; ok {
					excluded++
					localExcluded++
				} else {
					reachable[operationID] = append(reachable[operationID], copyAssignment)
				}
				return true
			}
			for _, value := range values[index] {
				assignment[domainIDs[index]] = value
				if !walk(index + 1) {
					return false
				}
			}
			return true
		}
		if !walk(0) {
			return reachable, total, excluded
		}
		if localTotal == localExcluded {
			operation := v.operations[operationID]
			v.add("contradictory-universe", fmt.Sprintf("constraints exclude every assignment for operation %q", operationID), &operation.Provenance)
		}
	}
	return reachable, total, excluded
}

func (v *validator) validateCases(model *finiteModel, constraints map[string]map[string]semanticir.Constraint) {
	requirements := make(map[caseKey][]semanticir.RequirementCase)
	codeCases := make(map[caseKey]semanticir.BehaviorCase)
	pointsByCategory := make(map[caseKey][]semanticir.BehaviorRef)
	concretePoints, pointDiagnostics := semanticir.ConcreteBehaviorPoints(v.task)
	for _, diagnostic := range pointDiagnostics {
		if diagnostic.Severity == semanticir.SeverityError {
			provenance := diagnostic.Provenance
			v.add("invalid-concrete-point-universe", string(diagnostic.Code)+": "+diagnostic.Message, &provenance)
		}
	}
	for _, point := range concretePoints {
		categoryKey := caseKey{operation: point.OperationID, assignment: canonicalAssignment(v.operationDomains[point.OperationID], point.Conditions)}
		pointsByCategory[categoryKey] = append(pointsByCategory[categoryKey], point)
	}
	targetOperations := make(map[string]bool)
	for operationID, operation := range v.operations {
		if operation.Kind != semanticir.OperationTest {
			targetOperations[operationID] = true
		}
	}
	requirementIDs := make(map[string]bool)
	codeIDs := make(map[string]bool)
	testIDs := make(map[string]bool)

	for _, requirement := range v.task.Requirements {
		key, ok := v.semanticCaseKey(requirement.OperationID, requirement.Conditions, "requirement "+strconv.Quote(requirement.ID), &requirement.Provenance, constraints)
		if strings.TrimSpace(requirement.ID) == "" {
			v.add("invalid-requirement", "requirement ID is empty", &requirement.Provenance)
		}
		if requirementIDs[requirement.ID] {
			v.add("duplicate-requirement", fmt.Sprintf("requirement ID %q is duplicated", requirement.ID), &requirement.Provenance)
		}
		requirementIDs[requirement.ID] = true
		if ok {
			requirements[key] = append(requirements[key], requirement)
			targetOperations[requirement.OperationID] = true
		}
		if len(requirement.InstructionSources) == 0 || len(requirement.Evidence) == 0 {
			v.add("missing-evidence", fmt.Sprintf("requirement %q must carry instruction sources and independent evidence", requirement.ID), &requirement.Provenance)
		}
		for _, source := range requirement.InstructionSources {
			if artifact, exists := v.artifacts[source.ArtifactID]; !exists || artifact.Kind != semanticir.ArtifactInstruction {
				v.add("invalid-instruction-evidence", fmt.Sprintf("requirement %q instruction source is not from the frozen instruction", requirement.ID), &source)
			}
		}
		v.validateRequirementOutcomes(requirement)
		v.validateEffects(requirement.Effects, "requirement "+strconv.Quote(requirement.ID))
		for _, invariantID := range requirement.InvariantIDs {
			if _, exists := v.invariants[invariantID]; !exists {
				v.add("missing-invariant", fmt.Sprintf("requirement %q cites unknown invariant %q", requirement.ID, invariantID), &requirement.Provenance)
			}
		}
	}

	for _, code := range v.task.CodeCases {
		categoryKey, ok := v.semanticCaseKey(code.OperationID, code.Conditions, "code case "+strconv.Quote(code.ID), &code.Provenance, constraints)
		if strings.TrimSpace(code.ID) == "" || codeIDs[code.ID] {
			v.add("duplicate-code-case", fmt.Sprintf("code case ID %q is empty or duplicated", code.ID), &code.Provenance)
		}
		codeIDs[code.ID] = true
		v.validateLocalOutcomeIDs(code.OperationID, code.OutcomeIDs, "code case "+strconv.Quote(code.ID), false, &code.Provenance)
		if len(code.OutcomeIDs) != 1 {
			v.add("nondeterministic-code-model", fmt.Sprintf("code case %q fixes %d outcomes; v0.1 requires exactly one outcome per expanded semantic case", code.ID, len(code.OutcomeIDs)), &code.Provenance)
		}
		if code.Inputs == nil {
			v.add("incomplete-code-point", fmt.Sprintf("code case %q names a category instead of one exact concrete point", code.ID), &code.Provenance)
			ok = false
		}
		key := concreteCaseKey(code.OperationID, v.operationDomains[code.OperationID], code.Conditions, code.Inputs)
		pointKnown := false
		for _, point := range pointsByCategory[categoryKey] {
			if inputPointKey(point.Inputs) == key.inputs {
				pointKnown = true
				break
			}
		}
		if ok && !pointKnown {
			v.add("invalid-code-point", fmt.Sprintf("code case %q is outside the complete concrete point universe", code.ID), &code.Provenance)
			ok = false
		}
		if ok {
			if previous, exists := codeCases[key]; exists {
				v.add("overlapping-code", fmt.Sprintf("code cases %q and %q describe the same reachable case", previous.ID, code.ID), &code.Provenance)
			} else {
				codeCases[key] = code
				targetOperations[code.OperationID] = true
			}
		}
	}

	for _, test := range v.task.Tests {
		if strings.TrimSpace(test.ID) == "" {
			continue
		}
		if testIDs[test.ID] {
			continue
		}
		testIDs[test.ID] = true
	}

	if len(targetOperations) == 0 {
		v.add("empty-behavior-universe", "task has no semantic operation cases", nil)
		return
	}
	operationIDs := make([]string, 0, len(targetOperations))
	for operationID := range targetOperations {
		operationIDs = append(operationIDs, operationID)
	}
	sort.Strings(operationIDs)
	for _, operationID := range operationIDs {
		for _, assignment := range model.reachable[operationID] {
			key := caseKey{operation: operationID, assignment: canonicalAssignment(v.operationDomains[operationID], assignment)}
			caseRequirements := requirements[key]
			hasRequirement := len(caseRequirements) != 0
			if !hasRequirement {
				v.add("incomplete-spec", fmt.Sprintf("reachable case %s has no requirement", v.formatCase(operationID, assignment)), nil)
			}
			if hasRequirement {
				sort.Slice(caseRequirements, func(i, j int) bool { return caseRequirements[i].ID < caseRequirements[j].ID })
				allowed, rejectedBy := v.intersectRequirements(caseRequirements, v.operationOutcomes[operationID], assignment)
				if len(allowed) == 0 {
					v.add("contradictory-spec", fmt.Sprintf("requirements for reachable case %s have no common allowed outcome", v.formatCase(operationID, assignment)), &caseRequirements[0].Provenance)
				} else {
					for _, point := range pointsByCategory[key] {
						pointKey := concreteCaseKey(operationID, v.operationDomains[operationID], assignment, point.Inputs)
						code, hasCode := codeCases[pointKey]
						if !hasCode {
							v.add("incomplete-code-model", fmt.Sprintf("reachable concrete point %s@%s has no code behavior", v.formatCase(operationID, assignment), pointKey.inputs), nil)
							continue
						}
						model.cases = append(model.cases, finiteCase{operation: operationID, conditions: cloneAssignment(assignment), inputs: cloneInputs(point.Inputs), requirements: caseRequirements, allowed: allowed, rejectedBy: rejectedBy, code: code})
					}
				}
			}
		}
	}
	v.validateTestSuite(model, constraints, targetOperations)

	for _, requirement := range v.task.Requirements {
		seen := make(map[string]bool)
		for _, testID := range requirement.TestIDs {
			if strings.TrimSpace(testID) == "" || seen[testID] {
				v.add("duplicate-test-reference", fmt.Sprintf("requirement %q repeats test ID %q", requirement.ID, testID), &requirement.Provenance)
			}
			seen[testID] = true
			// Requirement.TestIDs are author-facing trace labels, not semantic
			// owners of TestsPass. The authoritative TestSuite is independently
			// bound to every frozen test artifact/model and its exhaustive global
			// predicate above; requiring an advisory Task.Tests row with the same
			// label would reintroduce parsed tests as proof truth.
		}
	}
}

func (v *validator) semanticCaseKey(operationID string, assignment semanticir.Assignment, label string, provenance *semanticir.Provenance, constraints map[string]map[string]semanticir.Constraint) (caseKey, bool) {
	operation, exists := v.operations[operationID]
	if !exists || operation.Kind == semanticir.OperationTest {
		v.add("invalid-operation-reference", fmt.Sprintf("%s cites missing or non-behavior operation %q", label, operationID), provenance)
		return caseKey{}, false
	}
	assignmentKey, ok := v.assignmentKeyFor(v.operationDomains[operationID], assignment, label, provenance)
	if !ok {
		return caseKey{}, false
	}
	if constraint, excluded := constraints[operationID][assignmentKey]; excluded {
		v.add("contradictory-universe", fmt.Sprintf("%s describes assignment excluded by constraint %q", label, constraint.ID), provenance)
		return caseKey{}, false
	}
	return caseKey{operation: operationID, assignment: assignmentKey}, true
}

func (v *validator) validateRequirementOutcomes(requirement semanticir.RequirementCase) {
	required := v.validateReferencedOutcomes(requirement.RequiredOutcomes, "requirement "+strconv.Quote(requirement.ID)+" required outcomes", false, &requirement.Provenance)
	forbidden := v.validateReferencedOutcomes(requirement.ForbiddenOutcomes, "requirement "+strconv.Quote(requirement.ID)+" forbidden outcomes", true, &requirement.Provenance)
	localOutcomes := make(map[string]bool)
	for _, id := range v.operationOutcomes[requirement.OperationID] {
		localOutcomes[id] = true
	}
	if len(requirement.RequiredOutcomes) == 0 {
		v.add("incomplete-spec", fmt.Sprintf("requirement %q must allow at least one required outcome", requirement.ID), &requirement.Provenance)
	}
	for id := range required {
		if !localOutcomes[id] {
			v.add("invalid-operation-outcome", fmt.Sprintf("requirement %q requires outcome %q outside operation %q's universe", requirement.ID, id, requirement.OperationID), &requirement.Provenance)
		}
		if forbidden[id] {
			v.add("contradictory-spec", fmt.Sprintf("requirement %q both requires and forbids outcome %q", requirement.ID, id), &requirement.Provenance)
		}
	}
	for id := range forbidden {
		if !localOutcomes[id] {
			v.add("invalid-operation-outcome", fmt.Sprintf("requirement %q forbids unrelated outcome %q outside operation %q's universe", requirement.ID, id, requirement.OperationID), &requirement.Provenance)
		}
	}
	if len(required)+len(forbidden) != len(localOutcomes) {
		v.add("incomplete-spec", fmt.Sprintf("requirement %q does not partition all %d outcomes for operation %q", requirement.ID, len(localOutcomes), requirement.OperationID), &requirement.Provenance)
	}
}

func (v *validator) intersectRequirements(requirements []semanticir.RequirementCase, localOutcomes []string, assignment semanticir.Assignment) ([]string, map[string]string) {
	var allowed []string
	rejectedBy := make(map[string]string)
	for _, outcomeID := range localOutcomes {
		outcome := v.outcomes[outcomeID]
		accepted := true
		for _, requirement := range requirements {
			if !containsString(requirement.RequiredOutcomes, outcomeID) || !outcomeSatisfiesEffects(outcome, requirement.Effects) {
				accepted = false
				rejectedBy[outcomeID] = requirement.ID
				break
			}
			for _, invariantID := range requirement.InvariantIDs {
				invariant := v.invariants[invariantID]
				holds, err := evaluateInvariant(v.task, invariant, assignment, outcome)
				if err != nil {
					v.add("unsupported-invariant-proof", fmt.Sprintf("requirement %q invariant %q cannot be evaluated exactly: %v", requirement.ID, invariantID, err), &invariant.Provenance)
					accepted = false
					rejectedBy[outcomeID] = requirement.ID
					break
				}
				if !holds {
					accepted = false
					rejectedBy[outcomeID] = requirement.ID
					break
				}
			}
			if !accepted {
				break
			}
		}
		if accepted {
			allowed = append(allowed, outcomeID)
		}
	}
	return allowed, rejectedBy
}

func (v *validator) validateReferencedOutcomes(ids []string, label string, emptyAllowed bool, provenance *semanticir.Provenance) map[string]bool {
	result := make(map[string]bool)
	if len(ids) == 0 && !emptyAllowed {
		v.add("missing-outcomes", label+" is empty", provenance)
	}
	for _, id := range ids {
		if _, exists := v.outcomes[id]; !exists {
			v.add("unknown-outcome", fmt.Sprintf("%s cites undeclared outcome %q", label, id), provenance)
		}
		if result[id] {
			v.add("duplicate-outcome-reference", fmt.Sprintf("%s repeats outcome %q", label, id), provenance)
		}
		result[id] = true
	}
	return result
}

func (v *validator) validateLocalOutcomeIDs(operationID string, ids []string, label string, emptyAllowed bool, provenance *semanticir.Provenance) map[string]bool {
	result := v.validateReferencedOutcomes(ids, label, emptyAllowed, provenance)
	local := make(map[string]bool)
	for _, id := range v.operationOutcomes[operationID] {
		local[id] = true
	}
	for id := range result {
		if !local[id] {
			v.add("invalid-operation-outcome", fmt.Sprintf("%s cites outcome %q outside operation %q's universe", label, id, operationID), provenance)
		}
	}
	return result
}

func (v *validator) validateEffects(effects []semanticir.Effect, label string) {
	ids := make(map[string]bool)
	for _, effect := range effects {
		if strings.TrimSpace(effect.ID) == "" || ids[effect.ID] {
			v.add("invalid-effect", fmt.Sprintf("%s has an empty or duplicate effect ID %q", label, effect.ID), &effect.Provenance)
		}
		ids[effect.ID] = true
		if strings.TrimSpace(effect.Target) == "" {
			v.add("invalid-effect", fmt.Sprintf("%s effect %q has an empty target", label, effect.ID), &effect.Provenance)
		}
		switch effect.Kind {
		case semanticir.EffectRead, semanticir.EffectWrite, semanticir.EffectCall, semanticir.EffectOutput:
		default:
			v.add("invalid-effect", fmt.Sprintf("%s effect %q has unknown kind %q", label, effect.ID, effect.Kind), &effect.Provenance)
		}
		if effect.Value != nil {
			value, err := evaluateExpression(*effect.Value, nil)
			if err != nil {
				v.add("unsupported-effect-value", fmt.Sprintf("%s effect %q value is not a concrete finite expression: %v", label, effect.ID, err), &effect.Provenance)
			} else {
				v.validateLiteral(&value, value.Type, fmt.Sprintf("%s effect %q value", label, effect.ID), &effect.Provenance, make(map[any]bool))
			}
		}
	}
}

func (v *validator) assignmentKeyFor(domainIDs []string, assignment semanticir.Assignment, label string, provenance *semanticir.Provenance) (string, bool) {
	if len(assignment) != len(domainIDs) {
		v.add("incomplete-assignment", fmt.Sprintf("%s fixes %d domains; want exactly %d", label, len(assignment), len(domainIDs)), provenance)
		return "", false
	}
	for domainID, valueID := range assignment {
		values, exists := v.domains[domainID]
		if !exists || !values[valueID] {
			v.add("invalid-assignment", fmt.Sprintf("%s assigns undeclared value %q to domain %q", label, valueID, domainID), provenance)
			return "", false
		}
	}
	return canonicalAssignment(domainIDs, assignment), true
}

func (v *validator) formatCase(operation string, assignment semanticir.Assignment) string {
	return strconv.Quote(operation) + " " + canonicalAssignment(v.operationDomains[operation], assignment)
}

func sameDomainSet(domainIDs []string, assignment semanticir.Assignment) bool {
	if len(domainIDs) != len(assignment) {
		return false
	}
	for _, domainID := range domainIDs {
		if _, exists := assignment[domainID]; !exists {
			return false
		}
	}
	return true
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validValueType(valueType semanticir.ValueType) bool {
	switch valueType {
	case semanticir.TypeBool, semanticir.TypeInteger, semanticir.TypeString, semanticir.TypeUnit, semanticir.TypeSequence, semanticir.TypeTuple, semanticir.TypeRecord, semanticir.TypeOptional:
		return true
	default:
		return false
	}
}

func (v *validator) validateLiteral(literal *semanticir.Literal, expected semanticir.ValueType, label string, provenance *semanticir.Provenance, seen map[any]bool) {
	if literal == nil {
		v.add("invalid-literal", label+" is nil", provenance)
		return
	}
	if !validValueType(literal.Type) || literal.Type != expected {
		v.add("invalid-literal", fmt.Sprintf("%s has type %q; want %q", label, literal.Type, expected), provenance)
		return
	}
	switch literal.Type {
	case semanticir.TypeBool, semanticir.TypeInteger, semanticir.TypeString, semanticir.TypeUnit:
		if literal.Null || literal.Elements != nil || literal.Fields != nil {
			v.add("invalid-literal", label+" scalar literal carries composite/null data", provenance)
		}
	case semanticir.TypeSequence, semanticir.TypeTuple:
		if literal.Null || literal.Elements == nil || literal.Fields != nil {
			v.add("invalid-literal", label+" sequence/tuple literal has invalid storage", provenance)
			return
		}
		if seen[literal.Elements] {
			v.add("non-finite-literal", label+" contains a recursive element cycle", provenance)
			return
		}
		seen[literal.Elements] = true
		for i := range literal.Elements.Values {
			child := &literal.Elements.Values[i]
			v.validateLiteral(child, child.Type, fmt.Sprintf("%s element %d", label, i), provenance, seen)
		}
		delete(seen, literal.Elements)
	case semanticir.TypeRecord:
		if literal.Null || literal.Fields == nil || literal.Elements != nil {
			v.add("invalid-literal", label+" record literal has invalid storage", provenance)
			return
		}
		if seen[literal.Fields] {
			v.add("non-finite-literal", label+" contains a recursive field cycle", provenance)
			return
		}
		seen[literal.Fields] = true
		for name, childValue := range literal.Fields.Values {
			if strings.TrimSpace(name) == "" {
				v.add("invalid-literal", label+" contains an empty field name", provenance)
			}
			child := childValue
			v.validateLiteral(&child, child.Type, fmt.Sprintf("%s field %q", label, name), provenance, seen)
		}
		delete(seen, literal.Fields)
	case semanticir.TypeOptional:
		if literal.Fields != nil || (literal.Null && literal.Elements != nil) || (!literal.Null && (literal.Elements == nil || len(literal.Elements.Values) != 1)) {
			v.add("invalid-literal", label+" optional literal must be null or contain exactly one element", provenance)
			return
		}
		if !literal.Null {
			if seen[literal.Elements] {
				v.add("non-finite-literal", label+" contains a recursive optional cycle", provenance)
				return
			}
			seen[literal.Elements] = true
			child := &literal.Elements.Values[0]
			v.validateLiteral(child, child.Type, label+" optional value", provenance, seen)
			delete(seen, literal.Elements)
		}
	}
}

func outcomeSatisfiesEffects(outcome semanticir.ObservableOutcome, required []semanticir.Effect) bool {
	// Requirement effects are the common observable trace that every allowed
	// outcome must contain. Alternative-specific effects may appear between
	// them, but order and multiplicity remain semantic.
	nextActual := 0
	for _, expected := range required {
		matched := false
		for nextActual < len(outcome.Effects) {
			actual := outcome.Effects[nextActual]
			nextActual++
			// Effect IDs are evidence identities, not observable semantics. The
			// ordered kind/target/value trace is what Spec constrains.
			if expected.Kind == actual.Kind && expected.Target == actual.Target && effectValuesEqual(expected.Value, actual.Value) {
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

func effectValuesEqual(left, right *semanticir.Expression) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	leftValue, leftErr := evaluateExpression(*left, nil)
	rightValue, rightErr := evaluateExpression(*right, nil)
	return leftErr == nil && rightErr == nil && reflect.DeepEqual(leftValue, rightValue)
}

func expressionPointersEqual(left, right *semanticir.Expression) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return expressionsEqual(*left, *right)
}

func expressionsEqual(left, right semanticir.Expression) bool {
	if left.Kind != right.Kind || left.Type != right.Type || left.Name != right.Name || left.Operator != right.Operator || len(left.Operands) != len(right.Operands) {
		return false
	}
	if left.Literal == nil || right.Literal == nil {
		if left.Literal != nil || right.Literal != nil {
			return false
		}
	} else if !reflect.DeepEqual(*left.Literal, *right.Literal) {
		return false
	}
	for i := range left.Operands {
		if !expressionsEqual(left.Operands[i], right.Operands[i]) {
			return false
		}
	}
	return true
}

func (v *validator) add(code, message string, provenance *semanticir.Provenance) {
	var copyProvenance *semanticir.Provenance
	if provenance != nil {
		copyValue := *provenance
		copyProvenance = &copyValue
	}
	v.blockers = append(v.blockers, Blocker{Code: code, Message: message, Provenance: copyProvenance})
}

func normalizedBlockers(blockers []Blocker) []Blocker {
	sort.SliceStable(blockers, func(i, j int) bool {
		if blockers[i].Code == blockers[j].Code {
			return blockers[i].Message < blockers[j].Message
		}
		return blockers[i].Code < blockers[j].Code
	})
	result := blockers[:0]
	for _, blocker := range blockers {
		if len(result) != 0 && result[len(result)-1].Code == blocker.Code && result[len(result)-1].Message == blocker.Message {
			continue
		}
		result = append(result, blocker)
	}
	return result
}

func cloneAssignment(assignment semanticir.Assignment) semanticir.Assignment {
	result := make(semanticir.Assignment, len(assignment))
	for key, value := range assignment {
		result[key] = value
	}
	return result
}
