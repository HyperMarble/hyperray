package proof

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"

	"github.com/HyperMarble/ray/internal/semanticir"
)

// validateTestSuite validates the frozen, independently translated static
// TestsPass predicate. Per-artifact TestModel values are proof-critical
// inputs: the suite predicate must be their canonical conjunction. Optional
// execution cross-checks are audit evidence only.
func (v *validator) validateTestSuite(model *finiteModel, constraints map[string]map[string]semanticir.Constraint, targetOperations map[string]bool) {
	if v.task.TestSuite == nil {
		v.add("missing-test-suite", "task has no authoritative exhaustive test-suite model", nil)
		return
	}
	suite := v.task.TestSuite
	// A TestPredicate over one BehaviorRef represents an entire semantic
	// category, not merely its concrete witness. Structural TestProjection in
	// v0.1 has no universal membership-quantification record, so it is sound
	// only when each selected-label conjunction uniquely fixes every concrete
	// operation input. Category witnesses alone must never collapse x>=0 to
	// the one tested point x=0.
	// model.cases is already expanded to exact concrete points. Test leaves
	// below must name one of those points; unobserved points remain independent
	// variables and are therefore visible to soundness/fairness search.
	v.suitePredicateProvenance(&suite.Predicate, "authoritative test-suite predicate")
	if suite.Predicate.Kind == "" {
		v.add("incomplete-test-suite", "authoritative test suite has no global predicate", &suite.Coverage.Provenance)
	} else {
		v.validatePredicate(suite.Predicate, "authoritative test-suite predicate", constraints, targetOperations)
		v.validateConcretePredicateRefs(model, suite.Predicate)
	}
	expectedPredicate := semanticir.StaticTestPredicate(v.task.Tests, suite.Predicate.Provenance)
	if !reflect.DeepEqual(expectedPredicate, suite.Predicate) {
		v.add("stale-test-suite-predicate", "authoritative test-suite predicate differs from the canonical conjunction of independently translated test models", &suite.Predicate.Provenance)
	}

	testArtifacts := make(map[string]semanticir.ArtifactModel)
	wantedSources := make(map[string]semanticir.ArtifactRef)
	for _, artifact := range v.task.Artifacts {
		if artifact.Kind == semanticir.ArtifactTests {
			v.validateTestArtifactEvidence(model, &artifact)
			testArtifacts[artifact.Artifact.ID] = artifact
			wantedSources[artifact.Artifact.ID] = artifact.Artifact
			// A test frontend may select a dedicated frozen runner
			// configuration rather than the task's general configuration.  It
			// is proof input only through the independently validated runner
			// selection record, never merely because the suite listed it.
			if artifact.RunnerSelection != nil {
				configuration := artifact.RunnerSelection.Configuration
				wantedSources[configuration.ID] = configuration
			}
		}
	}
	if v.task.Environment != nil {
		configuration := environmentConfiguration(v.task.Environment)
		wantedSources[configuration.ID] = configuration
		for _, source := range v.task.Environment.SourceArtifacts {
			wantedSources[source.ID] = source
		}
	}
	seenSources := make(map[string]bool)
	for index, source := range suite.SourceArtifacts {
		wanted, exists := wantedSources[source.ID]
		if !exists || wanted != source || (source.Kind != semanticir.ArtifactTests && source.Kind != semanticir.ArtifactEnvironment && source.Kind != semanticir.ArtifactConfiguration) || seenSources[source.ID] {
			v.add("invalid-test-suite-source", fmt.Sprintf("authoritative test suite has missing, stale, invalid-kind, or duplicate source artifact %q", source.ID), &suite.Coverage.Provenance)
		}
		seenSources[source.ID] = true
		if index > 0 {
			previous := suite.SourceArtifacts[index-1]
			if previous.ID > source.ID || (previous.ID == source.ID && previous.Path > source.Path) {
				v.add("noncanonical-test-suite", "authoritative test-suite source artifacts are not canonically sorted", &suite.Coverage.Provenance)
			}
		}
	}
	if len(seenSources) != len(wantedSources) {
		v.add("incomplete-test-suite-source", fmt.Sprintf("authoritative test suite binds %d source artifacts; want %d tests+environment sources", len(seenSources), len(wantedSources)), &suite.Coverage.Provenance)
	}
	seenModels := make(map[string]bool)
	for _, binding := range suite.SourceModels {
		artifact, exists := testArtifacts[binding.ArtifactID]
		digest, err := semanticir.Digest(artifact)
		if !exists || err != nil || !digestPattern.MatchString(binding.Digest) || binding.Digest != digest || seenModels[binding.ArtifactID] {
			v.add("stale-test-suite-model", fmt.Sprintf("authoritative test suite model digest for artifact %q is missing, duplicate, or stale", binding.ArtifactID), &suite.Coverage.Provenance)
		}
		seenModels[binding.ArtifactID] = true
	}
	if len(seenModels) != len(testArtifacts) {
		v.add("incomplete-test-suite-source", fmt.Sprintf("authoritative test suite binds %d source models; want %d", len(seenModels), len(testArtifacts)), &suite.Coverage.Provenance)
	}

	v.validateTool(suite.Verifier, "authoritative test-suite verifier")
	if !v.environmentHasTool(suite.Verifier) {
		v.add("missing-tool-evidence", fmt.Sprintf("test-suite verifier %q is not frozen in the environment", suite.Verifier.Name), &suite.Coverage.Provenance)
	}
	if !v.environmentHasCommand(suite.Execution) {
		v.add("stale-test-suite-execution", "authoritative test-suite execution does not match a frozen workspace command", &suite.Execution.Provenance)
	}
	if suite.Execution.State != semanticir.WorkspaceSolutionNewTests || !suite.Execution.ExpectedPass || !suite.Execution.ObservedPass || !containsToolRef(suite.Execution.Tools, suite.Verifier) {
		v.add("invalid-test-suite-execution", "authoritative test-suite execution is not a passing solution+new-tests command bound to its verifier", &suite.Execution.Provenance)
	}
	if source, exists := wantedSources[suite.Execution.Provenance.ArtifactID]; !exists || source.Digest != suite.Execution.Provenance.ArtifactDigest {
		v.add("invalid-test-suite-execution", "authoritative test-suite execution is not anchored to a declared source", &suite.Execution.Provenance)
	}

	vectorCount, countErr := finiteVectorCount(model)
	if countErr != nil {
		v.add("test-suite-count-overflow", countErr.Error(), &suite.Coverage.Provenance)
	} else if suite.VectorCount != vectorCount {
		v.add("incomplete-test-suite", fmt.Sprintf("test suite declares mathematical universe %d; want %d", suite.VectorCount, vectorCount), &suite.Coverage.Provenance)
	}
	if len(suite.Vectors) != 0 || suite.AcceptedVectorCount != 0 || suite.AcceptedVectorsDigest != "" || suite.VectorEvidenceDigest != "" || suite.Repetitions != 0 || len(suite.RunDigests) != 0 {
		v.add("obsolete-test-suite-vectors", "test suite contains obsolete inline vector evidence; use optional cross_check", &suite.Coverage.Provenance)
	}
	if suite.Coverage.TotalConstructs <= 0 {
		v.add("incomplete-test-suite", "authoritative test-suite coverage has zero constructs", &suite.Coverage.Provenance)
	}
	if len(suite.Evidence) == 0 {
		v.add("missing-test-suite-evidence", "authoritative test suite has no source/execution evidence", &suite.Coverage.Provenance)
	}
	coveredSources := make(map[string]bool)
	for i := range suite.Evidence {
		v.provenance(suite.Evidence[i], fmt.Sprintf("authoritative test-suite evidence %d", i))
		if source, exists := wantedSources[suite.Evidence[i].ArtifactID]; exists && source.Digest == suite.Evidence[i].ArtifactDigest {
			coveredSources[source.ID] = true
		} else {
			v.add("invalid-test-suite-source", "authoritative test-suite evidence is not anchored to a declared source", &suite.Evidence[i])
		}
	}
	for sourceID := range wantedSources {
		if !coveredSources[sourceID] {
			v.add("missing-test-suite-evidence", fmt.Sprintf("authoritative test suite has no evidence for source artifact %q", sourceID), &suite.Coverage.Provenance)
		}
	}

	model.testSuite = cloneTestSuite(suite)
	if digest, err := semanticir.Digest(suite); err != nil {
		v.add("invalid-test-suite-digest", fmt.Sprintf("cannot canonically digest authoritative test suite: %v", err), &suite.Coverage.Provenance)
	} else {
		model.testSuiteSHA256 = digest
	}
	model.tests = []semanticir.TestModel{{ID: "authoritative-global-verifier", Predicate: suite.Predicate, Provenance: suite.Coverage.Provenance}}
	if suite.CrossCheck != nil {
		v.verifyTestCrossCheck(model, suite.CrossCheck, suite.VectorCount, suite.Coverage.Provenance)
	}
	v.validateTestObservationCompleteness(model, suite)
}

// validateTestObservationCompleteness closes the quotient boundary between
// concrete verifier executions and the finite BehaviorChoice vector. Merely
// running one representative for each vector is insufficient: the replayed
// UNSAT obligation must establish that equal modeled vectors cannot produce
// different pass signals.
func (v *validator) validateTestObservationCompleteness(model *finiteModel, suite *semanticir.TestSuiteModel) {
	record := &suite.ObservationCompleteness
	label := "test observation completeness"
	v.provenance(record.Provenance, label)
	if record.Result != semanticir.ProofProved || !digestPattern.MatchString(record.ObservationIRDigest) || !digestPattern.MatchString(record.HarnessDigest) || !digestPattern.MatchString(record.ProofDigest) {
		v.add("missing-test-observation-completeness", "authoritative test suite has no proved verifier-observation completeness evidence", &record.Provenance)
		return
	}
	predicateDigest, predicateErr := semanticir.Digest(suite.Predicate)
	wantModels := append([]semanticir.ArtifactModelDigest(nil), suite.SourceModels...)
	gotModels := append([]semanticir.ArtifactModelDigest(nil), record.SourceModels...)
	sort.Slice(wantModels, func(i, j int) bool { return wantModels[i].ArtifactID < wantModels[j].ArtifactID })
	sort.Slice(gotModels, func(i, j int) bool { return gotModels[i].ArtifactID < gotModels[j].ArtifactID })
	if predicateErr != nil || record.StaticPredicateDigest != predicateDigest || !reflect.DeepEqual(gotModels, wantModels) {
		v.add("stale-test-observation-completeness", "aggregate observation completeness differs from the exact static predicate/source models", &record.Provenance)
	}
	wantConstructs := make(map[string]semanticir.TestConstructEvidence)
	wantProjectionComponents := make(map[string]string)
	for _, artifact := range v.task.Artifacts {
		if artifact.Kind != semanticir.ArtifactTests || artifact.TestProjection == nil {
			continue
		}
		for _, construct := range artifact.TestProjection.Constructs {
			wantConstructs[construct.ID] = construct
		}
		digest, err := semanticir.TestProjectionGraphDigest(*artifact.TestProjection)
		if err != nil {
			v.add("invalid-test-observation-completeness", fmt.Sprintf("cannot digest test projection %q: %v", artifact.Artifact.ID, err), &record.Provenance)
		} else {
			wantProjectionComponents[artifact.Artifact.ID] = digest
		}
	}
	seenConstructs := make(map[string]bool)
	for _, construct := range record.Constructs {
		if wanted, exists := wantConstructs[construct.ID]; !exists || !reflect.DeepEqual(wanted, construct) || seenConstructs[construct.ID] {
			v.add("stale-test-observation-completeness", "aggregate observation construct is missing, duplicated, or differs from per-artifact evidence", &construct.Provenance)
		}
		seenConstructs[construct.ID] = true
	}
	if len(seenConstructs) != len(wantConstructs) {
		v.add("incomplete-test-observation-completeness", fmt.Sprintf("aggregate observation completeness binds %d constructs; want %d", len(seenConstructs), len(wantConstructs)), &record.Provenance)
	}
	if len(record.ProjectionComponents) != 0 {
		components := append([]semanticir.ArtifactModelDigest(nil), record.ProjectionComponents...)
		sort.Slice(components, func(i, j int) bool { return components[i].ArtifactID < components[j].ArtifactID })
		seen := make(map[string]bool)
		for _, component := range components {
			if seen[component.ArtifactID] || wantProjectionComponents[component.ArtifactID] != component.Digest {
				v.add("stale-test-observation-completeness", "aggregate structural projection component is stale or duplicated", &record.Provenance)
			}
			seen[component.ArtifactID] = true
		}
		compositionDigest, digestErr := semanticir.Digest(components)
		harnessDigest, harnessErr := semanticir.Digest(suite.Execution)
		if len(seen) != len(wantProjectionComponents) || digestErr != nil || harnessErr != nil || record.ProofDigest != compositionDigest || record.ObservationIRDigest != compositionDigest || record.HarnessDigest != harnessDigest {
			v.add("stale-test-observation-completeness", "aggregate structural projection composition is incomplete or stale", &record.Provenance)
		}
		// Each component's compiler-derived dependency graph was independently
		// validated above. The aggregate is a canonical composition digest, not
		// another frontend-authored formula or a legacy extensionality claim.
		return
	}
	// A prover is relevant only to the rejected legacy monolithic SMT form.
	// Structural TestProjection composition is checked through the exact graph
	// component digests above and deliberately carries no synthetic prover.
	v.add("missing-test-observation-completeness", "test suite has no structural observation-projection components", &record.Provenance)
	return

	/* Legacy monolithic SMT observation proofs are intentionally unreachable.
	They allowed a frontend-authored quotient formula to replace the exact
	compiler-derived dependency graph now required above.
	sourceDigest, err := semanticir.TestSuiteSourceDigest(suite.SourceArtifacts)
	if err != nil {
		v.add("invalid-test-observation-completeness", fmt.Sprintf("cannot digest authoritative test-suite sources: %v", err), &record.Provenance)
		return
	}
	context := semanticir.CompilerProofContext{
		SourceDigest: sourceDigest, WorkspaceTreeDigest: suite.Execution.TreeDigest,
		EmittedIRDigest: record.ObservationIRDigest, HarnessDigest: record.HarnessDigest,
		Compiler: suite.Verifier,
	}
	reachable := make(map[caseKey]bool, len(model.cases))
	for _, finiteCase := range model.cases {
		reachable[finiteCaseKey(model, finiteCase)] = true
	}
	seen := make(map[caseKey]bool, len(record.BehaviorEqualities))
	memberships := make([]semanticir.CompilerPredicate, 0, len(record.BehaviorEqualities))
	for index := range record.BehaviorEqualities {
		equality := &record.BehaviorEqualities[index]
		key := concreteCaseKey(equality.Behavior.OperationID, model.operationDomains[equality.Behavior.OperationID], equality.Behavior.Conditions, equality.Behavior.Inputs)
		if !reachable[key] || seen[key] {
			v.add("invalid-test-observation-completeness", fmt.Sprintf("behavior equality %d is unreachable or duplicated", index), &equality.Behavior.Provenance)
		}
		seen[key] = true
		v.validateCompilerPredicate(equality.Predicate, suite.Verifier, record.ObservationIRDigest, fmt.Sprintf("%s behavior equality %d", label, index), &equality.Behavior.Provenance)
		memberships = append(memberships, equality.Predicate)
	}
	if len(seen) != len(reachable) {
		v.add("incomplete-test-observation-completeness", fmt.Sprintf("observation proof binds %d reachable behaviors; want %d", len(seen), len(reachable)), &record.Provenance)
	}
	v.validateCompilerPredicate(record.LeftPass, suite.Verifier, record.ObservationIRDigest, label+" left pass", &record.Provenance)
	v.validateCompilerPredicate(record.RightPass, suite.Verifier, record.ObservationIRDigest, label+" right pass", &record.Provenance)
	wantClaim := semanticir.NewTestObservationCompletenessClaim(context, record.Proof.Claim.Scope, memberships, record.LeftPass, record.RightPass)
	if record.ProofDigest != record.Proof.QueryDigest || record.Proof.Prover != record.Prover || !reflect.DeepEqual(record.Proof.Claim, wantClaim) {
		v.add("noncanonical-test-observation-proof", "test observation proof differs from the exact source/vector/pass obligation", &record.Provenance)
	}
	if len(v.blockers) != startBlockers {
		return
	}
	canonical, canonicalErr := semanticir.CanonicalProofQuery(wantClaim)
	if canonicalErr != nil || !reflect.DeepEqual(record.Proof.Query, canonical) {
		v.add("noncanonical-test-observation-proof", "test observation query is not centrally reconstructed from its typed obligation", &record.Provenance)
		return
	}
	if err := Replay(v.ctx, record.Proof, semanticir.SolverUNSAT, v.task.Environment); err != nil {
		v.add("test-observation-proof-replay-failed", err.Error(), &record.Provenance)
	}
	*/
}

func (v *validator) verifyTestCrossCheck(model *finiteModel, cross *semanticir.TestCrossCheckEvidence, universe uint64, provenance semanticir.Provenance) {
	if cross == nil {
		return
	}
	if len(cross.Vectors) == 0 || cross.Repetitions < 2 || len(cross.RunDigests) != cross.Repetitions {
		v.add("incomplete-test-cross-check", "optional test cross-check lacks vectors or repeated deterministic runs", &cross.Provenance)
	}
	if cross.Full && uint64(len(cross.Vectors)) != universe {
		v.add("incomplete-test-cross-check", fmt.Sprintf("full test cross-check embeds %d vectors; want %d", len(cross.Vectors), universe), &cross.Provenance)
	}
	seen := make(map[string]bool, len(cross.Vectors))
	accepted := uint64(0)
	for vectorIndex, testVector := range cross.Vectors {
		if len(testVector.Choices) != len(model.cases) {
			v.add("incomplete-test-cross-check", fmt.Sprintf("test vector %d has %d choices; want %d", vectorIndex, len(testVector.Choices), len(model.cases)), &cross.Provenance)
			continue
		}
		vector := make(behaviorVector, len(model.cases))
		valid := true
		for _, choice := range testVector.Choices {
			domainIDs, operationExists := model.operationDomains[choice.Behavior.OperationID]
			if !operationExists || len(choice.Behavior.Conditions) != len(domainIDs) {
				v.add("invalid-test-cross-check", fmt.Sprintf("test vector %d has an invalid behavior reference", vectorIndex), &cross.Provenance)
				valid = false
				continue
			}
			key := concreteCaseKey(choice.Behavior.OperationID, domainIDs, choice.Behavior.Conditions, choice.Behavior.Inputs)
			caseExists := false
			for _, finiteCase := range model.cases {
				candidate := finiteCaseKey(model, finiteCase)
				if candidate == key {
					caseExists = true
					break
				}
			}
			if !caseExists || !containsString(model.operationOutcomes[choice.Behavior.OperationID], choice.OutcomeID) {
				v.add("invalid-test-cross-check", fmt.Sprintf("test vector %d selects an unknown case/outcome", vectorIndex), &cross.Provenance)
				valid = false
				continue
			}
			if _, duplicate := vector[key]; duplicate {
				v.add("duplicate-test-cross-check-choice", fmt.Sprintf("test vector %d repeats a behavior choice", vectorIndex), &cross.Provenance)
				valid = false
				continue
			}
			vector[key] = choice.OutcomeID
		}
		if !valid || len(vector) != len(model.cases) {
			continue
		}
		keys := make([]string, 0, len(vector))
		for key, outcomeID := range vector {
			keys = append(keys, key.operation+"\x00"+key.assignment+"\x00"+outcomeID)
		}
		sort.Strings(keys)
		vectorKey := strings.Join(keys, "\x01")
		if seen[vectorKey] {
			v.add("duplicate-test-cross-check-vector", fmt.Sprintf("test cross-check repeats vector %d", vectorIndex), &cross.Provenance)
		}
		seen[vectorKey] = true
		passes, _, err := evaluateSuite(model, vector)
		if err != nil {
			v.add("test-cross-check-replay-blocked", fmt.Sprintf("cannot evaluate test vector %d: %v", vectorIndex, err), &cross.Provenance)
			continue
		}
		if passes != testVector.Accepted {
			v.add("stale-test-cross-check", fmt.Sprintf("test vector %d accepted=%t, predicate evaluates to %t", vectorIndex, testVector.Accepted, passes), &cross.Provenance)
		}
		if testVector.Accepted {
			accepted++
		}
	}
	if len(seen) != len(cross.Vectors) {
		v.add("duplicate-test-cross-check-vector", "test cross-check does not contain unique vectors", &cross.Provenance)
	}
	vectorDigest, acceptedDigest, err := semanticir.TestVectorDigests(cross.Vectors)
	if err != nil {
		v.add("invalid-test-cross-check-digest", fmt.Sprintf("cannot digest test cross-check vectors: %v", err), &cross.Provenance)
	} else if vectorDigest != cross.VectorEvidenceDigest || acceptedDigest != cross.AcceptedVectorsDigest {
		v.add("stale-test-cross-check-digest", "test cross-check vectors do not reproduce their declared digests", &cross.Provenance)
	}
	if accepted != cross.AcceptedVectorCount {
		v.add("stale-test-cross-check-count", fmt.Sprintf("test cross-check accepts %d cases; want %d", accepted, cross.AcceptedVectorCount), &cross.Provenance)
	}
	for _, digest := range cross.RunDigests {
		if digest != vectorDigest {
			v.add("nondeterministic-test-cross-check", "test cross-check repetitions do not reproduce identical vector evidence", &cross.Provenance)
			break
		}
	}
	_ = provenance
}

func (v *validator) suitePredicateProvenance(predicate *semanticir.TestPredicate, label string) {
	if predicate == nil || predicate.Kind == "" {
		return
	}
	v.provenance(predicate.Provenance, label)
	v.requireProvenanceKind(predicate.Provenance, semanticir.ArtifactTests, label)
	if predicate.Observe != nil {
		v.provenance(predicate.Observe.Provenance, label+" observation")
		v.requireProvenanceKind(predicate.Observe.Provenance, semanticir.ArtifactTests, label+" observation")
		v.provenance(predicate.Observe.Behavior.Provenance, label+" observation behavior")
		v.requireProvenanceKind(predicate.Observe.Behavior.Provenance, semanticir.ArtifactTests, label+" observation behavior")
		if predicate.Observe.EffectValue != nil {
			v.expressionProvenance(predicate.Observe.EffectValue, label+" effect value")
			v.requireExpressionKind(predicate.Observe.EffectValue, semanticir.ArtifactTests, label+" effect value")
		}
	}
	if predicate.Left != nil {
		v.provenance(predicate.Left.Provenance, label+" left behavior")
		v.requireProvenanceKind(predicate.Left.Provenance, semanticir.ArtifactTests, label+" left behavior")
	}
	if predicate.Right != nil {
		v.provenance(predicate.Right.Provenance, label+" right behavior")
		v.requireProvenanceKind(predicate.Right.Provenance, semanticir.ArtifactTests, label+" right behavior")
	}
	for i := range predicate.Children {
		v.suitePredicateProvenance(&predicate.Children[i], fmt.Sprintf("%s child %d", label, i))
	}
}

func finiteVectorCount(model *finiteModel) (uint64, error) {
	count := uint64(1)
	for _, finiteCase := range model.cases {
		outcomes := uint64(len(model.operationOutcomes[finiteCase.operation]))
		if outcomes == 0 || count > math.MaxUint64/outcomes {
			return 0, fmt.Errorf("finite behavior-vector count exceeds proof accounting capacity")
		}
		count *= outcomes
	}
	return count, nil
}

func (v *validator) environmentHasCommand(wanted semanticir.WorkspaceCommand) bool {
	if v.task.Environment == nil {
		return false
	}
	for _, command := range v.task.Environment.Commands {
		if reflect.DeepEqual(command, wanted) {
			return true
		}
	}
	return false
}

func containsToolRef(tools []semanticir.ToolRef, wanted semanticir.ToolRef) bool {
	for _, tool := range tools {
		if tool == wanted {
			return true
		}
	}
	return false
}

func (v *validator) validateTestArtifactEvidence(model *finiteModel, artifact *semanticir.ArtifactModel) {
	if artifact == nil {
		v.add("missing-test-projection", "nil test artifact model", nil)
		return
	}
	// Validate only the test-owned frontend record here. Task registries copied
	// onto an ArtifactModel are cross-reference vocabulary and retain their
	// spec provenance; they are validated separately by the proof task passes.
	audit := *artifact
	audit.Domains = nil
	audit.Constraints = nil
	audit.Operations = nil
	audit.Outcomes = nil
	audit.Cases = nil
	audit.Invariants = nil
	audit.CompilerEvidence = nil
	audit.ExhaustiveEvidence = nil
	audit.ScopeClosure = nil
	for _, diagnostic := range semanticir.ValidateArtifactModel(audit) {
		if diagnostic.Severity != semanticir.SeverityError {
			continue
		}
		provenance := diagnostic.Provenance
		v.add("invalid-test-projection", diagnostic.Message, &provenance)
	}
	for _, diagnostic := range semanticir.ValidateTestObservationQuantification(v.task, *artifact) {
		if diagnostic.Severity != semanticir.SeverityError {
			continue
		}
		provenance := diagnostic.Provenance
		v.add("invalid-test-quantification", diagnostic.Message, &provenance)
	}
	if artifact.TestProjection == nil {
		return
	}
	_ = model
}

func (v *validator) validateConcretePredicateRefs(model *finiteModel, predicate semanticir.TestPredicate) {
	points := make(map[caseKey]bool, len(model.cases))
	for _, finiteCase := range model.cases {
		points[finiteCaseKey(model, finiteCase)] = true
	}
	for _, ref := range predicateBehaviorRefs(predicate) {
		if ref.Inputs == nil {
			v.add("category-test-reference", fmt.Sprintf("test predicate behavior %q %s names a category instead of one exact typed input point", ref.OperationID, canonicalAssignment(model.operationDomains[ref.OperationID], ref.Conditions)), &ref.Provenance)
			continue
		}
		operation, exists := v.operations[ref.OperationID]
		if !exists || len(ref.Inputs) != len(operation.Inputs) {
			v.add("invalid-test-point", fmt.Sprintf("test predicate behavior %q does not assign every operation input", ref.OperationID), &ref.Provenance)
			continue
		}
		validInputs := true
		for _, input := range operation.Inputs {
			literal, exists := ref.Inputs[input.Name]
			if !exists || literal.Type != input.Type || semanticir.ValidateLiteral(literal) != nil {
				validInputs = false
				break
			}
		}
		key := concreteCaseKey(ref.OperationID, model.operationDomains[ref.OperationID], ref.Conditions, ref.Inputs)
		if !validInputs || !points[key] {
			v.add("invalid-test-point", fmt.Sprintf("test predicate behavior %q is outside the complete concrete category universe", semanticir.BehaviorRefKey(ref)), &ref.Provenance)
		}
	}
}

func cloneTestSuite(suite *semanticir.TestSuiteModel) *semanticir.TestSuiteModel {
	if suite == nil {
		return nil
	}
	encoded, err := semanticir.CanonicalJSON(suite)
	if err != nil {
		return nil
	}
	var result semanticir.TestSuiteModel
	if err := json.Unmarshal(encoded, &result); err != nil {
		return nil
	}
	return &result
}
