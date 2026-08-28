package semanticir

import (
	"fmt"
	"reflect"
	"sort"
)

// TestSuiteSourceDigest canonically binds the complete frozen verifier source
// set irrespective of declaration order.
func TestSuiteSourceDigest(sources []ArtifactRef) (string, error) {
	values := append([]ArtifactRef(nil), sources...)
	sort.Slice(values, func(i, j int) bool {
		return values[i].ID+"\x00"+values[i].Digest < values[j].ID+"\x00"+values[j].Digest
	})
	return Digest(values)
}

// StaticTestPredicate deterministically AND-composes all independently
// translated test predicates. Empty is true only for an explicitly frozen
// empty verifier universe.
func StaticTestPredicate(tests []TestModel, aggregate Provenance) TestPredicate {
	values := append([]TestModel(nil), tests...)
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	if len(values) == 0 {
		return TestPredicate{Kind: PredicateTrue, Provenance: aggregate}
	}
	if len(values) == 1 {
		return values[0].Predicate
	}
	children := make([]TestPredicate, 0, len(values))
	for _, model := range values {
		children = append(children, model.Predicate)
	}
	return TestPredicate{Kind: PredicateAnd, Children: children, Provenance: aggregate}
}

func validateTestSuite(task *Task, reachable map[string]struct{}, outcomes map[string]map[string]struct{}) []Diagnostic {
	if task.TestSuite == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticIncomplete, "task has no authoritative exhaustive test-suite model", task.Provenance)}
	}
	suite := task.TestSuite
	var diagnostics []Diagnostic
	if err := validateToolRef(suite.Verifier); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test-suite verifier: "+err.Error(), suite.Coverage.Provenance))
	}
	diagnostics = append(diagnostics, validateCoverage(suite.Coverage)...)
	if suite.Coverage.TotalConstructs == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test-suite coverage has zero constructs", suite.Coverage.Provenance))
	}
	diagnostics = append(diagnostics, validateTestPredicate(suite.Predicate, reachable, outcomes)...)
	if expected := StaticTestPredicate(task.Tests, suite.Predicate.Provenance); !reflect.DeepEqual(expected, suite.Predicate) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "authoritative test-suite predicate differs from deterministic conjunction of independent test models", suite.Predicate.Provenance))
	}

	sourceByID := map[string]ArtifactRef{}
	for _, source := range suite.SourceArtifacts {
		if err := validateArtifactRef(source); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test-suite source: "+err.Error(), suite.Coverage.Provenance))
		}
		if source.Kind != ArtifactTests && source.Kind != ArtifactSource && source.Kind != ArtifactEnvironment && source.Kind != ArtifactConfiguration {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("test-suite source %q has non-test/environment/configuration kind %q", source.ID, source.Kind), suite.Coverage.Provenance))
		}
		if _, exists := sourceByID[source.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("test-suite repeats source artifact %q", source.ID), suite.Coverage.Provenance))
		}
		sourceByID[source.ID] = source
	}
	if task.Environment == nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test-suite has no task environment to bind", suite.Coverage.Provenance))
	} else {
		environmentArtifact := task.Environment.Configuration
		if environmentArtifact == (ArtifactRef{}) {
			environmentArtifact = task.Environment.Artifact
		}
		if source, exists := sourceByID[environmentArtifact.ID]; !exists || source != environmentArtifact {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test-suite sources omit or alter the frozen environment artifact", suite.Coverage.Provenance))
		}
		toolFound := false
		for _, tool := range task.Environment.Tools {
			if tool == suite.Verifier {
				toolFound = true
				break
			}
		}
		if !toolFound {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test-suite verifier is not frozen in task environment", suite.Coverage.Provenance))
		}
	}

	wantModels := map[string]string{}
	for _, artifact := range task.Artifacts {
		if artifact.Kind != ArtifactTests {
			continue
		}
		if source, exists := sourceByID[artifact.Artifact.ID]; !exists || source != artifact.Artifact {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("test-suite sources omit or alter test artifact %q", artifact.Artifact.ID), suite.Coverage.Provenance))
		}
		digest, err := Digest(artifact)
		if err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "digest test artifact model: "+err.Error(), artifact.Coverage.Provenance))
		} else {
			wantModels[artifact.Artifact.ID] = digest
		}
		if artifact.RunnerSelection == nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test-suite artifact has no independent compiler-derived runner selection", artifact.Coverage.Provenance))
		}
	}
	seenModels := map[string]struct{}{}
	for _, binding := range suite.SourceModels {
		if !ValidDigest(binding.Digest) || wantModels[binding.ArtifactID] != binding.Digest {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("test-suite source model %q digest does not match attached artifact model", binding.ArtifactID), suite.Coverage.Provenance))
		}
		if _, exists := seenModels[binding.ArtifactID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("test-suite repeats source model %q", binding.ArtifactID), suite.Coverage.Provenance))
		}
		seenModels[binding.ArtifactID] = struct{}{}
	}
	if len(seenModels) != len(wantModels) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("test-suite binds %d source models, want %d", len(seenModels), len(wantModels)), suite.Coverage.Provenance))
	}
	for sourceID, source := range sourceByID {
		if source.Kind == ArtifactTests || source.Kind == ArtifactSource {
			if _, exists := wantModels[sourceID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test-suite source has no attached independently translated model "+sourceID, suite.Coverage.Provenance))
			}
		}
	}

	if len(suite.Evidence) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "test-suite has no aggregate source evidence", suite.Coverage.Provenance))
	}
	coveredSources := map[string]struct{}{}
	for _, provenance := range suite.Evidence {
		if err := validateProvenance(provenance); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "test-suite evidence: "+err.Error(), provenance))
			continue
		}
		if source, exists := sourceByID[provenance.ArtifactID]; !exists || source.Digest != provenance.ArtifactDigest {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "test-suite evidence is not anchored to a declared source artifact", provenance))
			continue
		}
		coveredSources[provenance.ArtifactID] = struct{}{}
	}
	for sourceID := range sourceByID {
		if _, exists := coveredSources[sourceID]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, fmt.Sprintf("test-suite has no evidence for source artifact %q", sourceID), suite.Coverage.Provenance))
		}
	}

	diagnostics = append(diagnostics, validateSuiteExecution(suite.Execution, suite.Verifier, sourceByID)...)
	diagnostics = append(diagnostics, ValidateRunnerComposition(task, suite)...)
	points, pointDiagnostics := ConcreteBehaviorPoints(task)
	diagnostics = append(diagnostics, pointDiagnostics...)
	pointSet := make(map[string]struct{}, len(points))
	for _, point := range points {
		pointSet[BehaviorRefKey(point)] = struct{}{}
	}
	wantVectors, overflow := candidateVectorCount(task, points)
	if overflow {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticNonFinite, "candidate behavior-vector count overflows uint64", suite.Coverage.Provenance))
	} else if suite.VectorCount != wantVectors {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("test-suite declares mathematical universe %d, want %d", suite.VectorCount, wantVectors), suite.Coverage.Provenance))
	}
	if len(suite.Vectors) != 0 || suite.AcceptedVectorCount != 0 || suite.AcceptedVectorsDigest != "" || suite.VectorEvidenceDigest != "" || suite.Repetitions != 0 || len(suite.RunDigests) != 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test-suite uses obsolete inline vector evidence; use optional cross_check", suite.Coverage.Provenance))
	}
	if suite.CrossCheck != nil {
		diagnostics = append(diagnostics, validateTestCrossCheck(suite.CrossCheck, suite.VectorCount, pointSet, outcomes)...)
	}
	diagnostics = append(diagnostics, validateTestObservationCompleteness(task, suite, reachable)...)
	return diagnostics
}

func validateTestObservationCompleteness(task *Task, suite *TestSuiteModel, reachable map[string]struct{}) []Diagnostic {
	record := suite.ObservationCompleteness
	var diagnostics []Diagnostic
	if record.Result != ProofProved || !ValidDigest(record.ObservationIRDigest) || !ValidDigest(record.HarnessDigest) || !ValidDigest(record.ProofDigest) {
		return []Diagnostic{errorDiagnostic(DiagnosticIncomplete, "test-suite has no proved observation-completeness evidence", record.Provenance)}
	}
	if len(record.ProjectionComponents) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test observation completeness has no per-artifact structural projection components", record.Provenance))
	}
	if len(record.ProjectionComponents) == 0 && validateToolRef(record.Prover) != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test observation prover is invalid", record.Provenance))
	}
	if validateProvenance(record.Provenance) != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "test observation completeness has invalid provenance", record.Provenance))
	}
	wantModels := append([]ArtifactModelDigest(nil), suite.SourceModels...)
	gotModels := append([]ArtifactModelDigest(nil), record.SourceModels...)
	sort.Slice(wantModels, func(i, j int) bool { return wantModels[i].ArtifactID < wantModels[j].ArtifactID })
	sort.Slice(gotModels, func(i, j int) bool { return gotModels[i].ArtifactID < gotModels[j].ArtifactID })
	predicateDigest, predicateErr := Digest(suite.Predicate)
	if predicateErr != nil || record.StaticPredicateDigest != predicateDigest || !reflect.DeepEqual(gotModels, wantModels) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test observation completeness differs from exact static predicate/source models", record.Provenance))
	}
	validKinds := map[TestConstructKind]bool{TestConstructFixture: true, TestConstructParameter: true, TestConstructControl: true, TestConstructAssertion: true, TestConstructMock: true, TestConstructEffect: true, TestConstructCall: true}
	seenConstructs := map[string]struct{}{}
	coveredTests := map[string]struct{}{}
	for _, construct := range record.Constructs {
		source, exists := func() (ArtifactRef, bool) {
			for _, item := range suite.SourceArtifacts {
				if item.ID == construct.ArtifactID {
					return item, true
				}
			}
			return ArtifactRef{}, false
		}()
		if construct.ID == "" || !validKinds[construct.Kind] || !ValidDigest(construct.Digest) || !ValidDigest(construct.IRDigest) || len(construct.CompilerNodeIDs) == 0 || validateToolRef(construct.Tool) != nil || !exists || (source.Kind != ArtifactTests && source.Kind != ArtifactSource) || validateFactSource(construct.Provenance, source) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test construct lacks frozen compiler/interpreter IR grounding", construct.Provenance))
		}
		if _, duplicate := seenConstructs[construct.ID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "test observation repeats construct "+construct.ID, construct.Provenance))
		}
		seenConstructs[construct.ID] = struct{}{}
		coveredTests[construct.ArtifactID] = struct{}{}
	}
	for _, source := range suite.SourceArtifacts {
		if source.Kind == ArtifactTests || source.Kind == ArtifactSource {
			if _, exists := coveredTests[source.ID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test observation completeness omits compiler-grounded constructs for "+source.ID, record.Provenance))
			}
		}
	}
	if len(record.Constructs) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test observation completeness has no compiler-grounded constructs", record.Provenance))
	}
	wantConstructs := map[string]TestConstructEvidence{}
	for _, artifact := range task.Artifacts {
		if artifact.Kind == ArtifactTests && artifact.TestProjection != nil {
			for _, construct := range artifact.TestProjection.Constructs {
				wantConstructs[construct.ID] = construct
			}
		}
	}
	if len(record.Constructs) != len(wantConstructs) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "aggregate observation completeness does not contain every per-artifact compiler construct", record.Provenance))
	} else {
		for _, construct := range record.Constructs {
			if !reflect.DeepEqual(construct, wantConstructs[construct.ID]) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "aggregate observation construct differs from per-artifact compiler evidence "+construct.ID, construct.Provenance))
			}
		}
	}
	wantProjectionComponents := map[string]string{}
	for _, artifact := range task.Artifacts {
		if artifact.Kind == ArtifactTests && artifact.TestProjection != nil {
			digest, _ := TestProjectionGraphDigest(*artifact.TestProjection)
			wantProjectionComponents[artifact.Artifact.ID] = digest
		}
	}
	projectionComponents := append([]ArtifactModelDigest(nil), record.ProjectionComponents...)
	sort.Slice(projectionComponents, func(i, j int) bool { return projectionComponents[i].ArtifactID < projectionComponents[j].ArtifactID })
	seenProjectionComponents := map[string]struct{}{}
	for _, component := range projectionComponents {
		if wantProjectionComponents[component.ArtifactID] != component.Digest || !ValidDigest(component.Digest) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "aggregate structural projection component is stale "+component.ArtifactID, record.Provenance))
		}
		if _, dup := seenProjectionComponents[component.ArtifactID]; dup {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "aggregate structural projection repeats component "+component.ArtifactID, record.Provenance))
		}
		seenProjectionComponents[component.ArtifactID] = struct{}{}
	}
	projectionCompositionDigest, _ := Digest(projectionComponents)
	projectionHarnessDigest, _ := Digest(suite.Execution)
	if len(seenProjectionComponents) != len(wantProjectionComponents) || record.ProofDigest != projectionCompositionDigest || record.ObservationIRDigest != projectionCompositionDigest || record.HarnessDigest != projectionHarnessDigest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "aggregate structural projection composition digest/source set is stale", record.Provenance))
	}
	if len(record.ProjectionComponents) > 0 {
		return diagnostics
	}
	wantComponents := map[string]TestExtensionalityEvidence{}
	for _, artifact := range task.Artifacts {
		if artifact.Kind == ArtifactTests && artifact.TestProjection != nil {
			wantComponents[artifact.Artifact.ID] = artifact.TestProjection.Extensionality
		}
	}
	components := append([]TestExtensionalityComponent(nil), record.Components...)
	sort.Slice(components, func(i, j int) bool { return components[i].ArtifactID < components[j].ArtifactID })
	seenComponents := map[string]struct{}{}
	for _, component := range components {
		want, exists := wantComponents[component.ArtifactID]
		digest, _ := Digest(want)
		if !exists || component.Digest != digest || !reflect.DeepEqual(component.Evidence, want) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "aggregate extensionality component differs from per-artifact proof "+component.ArtifactID, record.Provenance))
		}
		if _, dup := seenComponents[component.ArtifactID]; dup {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "aggregate extensionality repeats component "+component.ArtifactID, record.Provenance))
		}
		seenComponents[component.ArtifactID] = struct{}{}
	}
	compositionDigest, _ := Digest(components)
	harnessDigest, _ := Digest(suite.Execution)
	if len(seenComponents) != len(wantComponents) || record.ProofDigest != compositionDigest || record.ObservationIRDigest != compositionDigest || record.HarnessDigest != harnessDigest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "aggregate extensionality composition digest/source set is stale", record.Provenance))
	}
	if len(record.Components) > 0 {
		return diagnostics
	}
	sourceDigest, err := TestSuiteSourceDigest(suite.SourceArtifacts)
	if err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "digest test-suite sources: "+err.Error(), record.Provenance))
	}
	context := CompilerProofContext{SourceDigest: sourceDigest, WorkspaceTreeDigest: suite.Execution.TreeDigest, EmittedIRDigest: record.ObservationIRDigest, HarnessDigest: record.HarnessDigest, Compiler: suite.Verifier}
	seen := map[string]struct{}{}
	memberships := make([]CompilerPredicate, 0, len(record.BehaviorEqualities))
	for _, equality := range record.BehaviorEqualities {
		key := BehaviorRefKey(equality.Behavior)
		if _, exists := reachable[key]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnreachable, "test observation equality refers to undeclared/constrained behavior "+key, equality.Behavior.Provenance))
		}
		if _, exists := seen[key]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "test observation completeness repeats behavior "+key, equality.Behavior.Provenance))
		}
		seen[key] = struct{}{}
		diagnostics = append(diagnostics, validateCompilerPredicate(equality.Predicate, suite.Verifier, record.ObservationIRDigest, equality.Behavior.Provenance)...)
		memberships = append(memberships, equality.Predicate)
	}
	if len(seen) != len(reachable) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("test observation completeness binds %d behaviors, want %d", len(seen), len(reachable)), record.Provenance))
	}
	diagnostics = append(diagnostics, validateCompilerPredicate(record.LeftPass, suite.Verifier, record.ObservationIRDigest, record.Provenance)...)
	diagnostics = append(diagnostics, validateCompilerPredicate(record.RightPass, suite.Verifier, record.ObservationIRDigest, record.Provenance)...)
	claim := NewTestObservationCompletenessClaim(context, record.Proof.Claim.Scope, memberships, record.LeftPass, record.RightPass)
	if record.ProofDigest != record.Proof.QueryDigest || record.Proof.Prover != record.Prover || !reflect.DeepEqual(record.Proof.Claim, claim) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test observation proof claim differs from exact source/vector/pass binding", record.Provenance))
	}
	diagnostics = append(diagnostics, ValidateReplayableProof(record.Proof, SolverUNSAT, record.Provenance)...)
	toolFound := false
	if task.Environment != nil {
		for _, tool := range task.Environment.Tools {
			toolFound = toolFound || tool == record.Prover
		}
	}
	if !toolFound {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test observation prover is absent from frozen environment", record.Provenance))
	}
	return diagnostics
}

func validateTestCrossCheck(cross *TestCrossCheckEvidence, universe uint64, reachable map[string]struct{}, outcomes map[string]map[string]struct{}) []Diagnostic {
	var diagnostics []Diagnostic
	if cross.Repetitions < 2 || len(cross.RunDigests) != cross.Repetitions || len(cross.Vectors) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test cross-check lacks vectors or repeated deterministic runs", cross.Provenance))
	}
	if cross.Full && uint64(len(cross.Vectors)) != universe {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "full test cross-check does not contain the complete vector universe", cross.Provenance))
	}
	seen := map[string]struct{}{}
	accepted := uint64(0)
	for index, vector := range cross.Vectors {
		canonical, err := canonicalizeTestVector(vector, index)
		if err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, err.Error(), cross.Provenance))
			continue
		}
		if len(canonical.Choices) != len(reachable) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test cross-check vector is incomplete", cross.Provenance))
		}
		for _, choice := range canonical.Choices {
			key := BehaviorRefKey(BehaviorRef{OperationID: choice.OperationID, Conditions: choice.Conditions, Inputs: choice.Inputs})
			if _, ok := reachable[key]; !ok {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnreachable, "test cross-check uses unreachable behavior "+key, cross.Provenance))
			}
			if _, ok := outcomes[choice.OperationID][choice.OutcomeID]; !ok {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test cross-check uses an outcome outside its operation", cross.Provenance))
			}
		}
		keyBytes, _ := CanonicalJSON(canonical.Choices)
		key := string(keyBytes)
		if _, exists := seen[key]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "test cross-check repeats a vector", cross.Provenance))
		}
		seen[key] = struct{}{}
		if vector.Accepted {
			accepted++
		}
	}
	allDigest, acceptedDigest, err := TestVectorDigests(cross.Vectors)
	if err != nil || allDigest != cross.VectorEvidenceDigest || acceptedDigest != cross.AcceptedVectorsDigest || accepted != cross.AcceptedVectorCount {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test cross-check vector evidence digest/count mismatch", cross.Provenance))
	}
	for _, digest := range cross.RunDigests {
		if digest != allDigest {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "test cross-check repetitions are nondeterministic", cross.Provenance))
		}
	}
	return diagnostics
}

func validateSuiteExecution(execution WorkspaceCommand, verifier ToolRef, sources map[string]ArtifactRef) []Diagnostic {
	var diagnostics []Diagnostic
	if execution.ID == "" || execution.WorkspaceID == "" || execution.Command == "" || execution.WorkingDirectory == "" || execution.TimeoutMillis <= 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test-suite execution has incomplete command/workspace/timeout", execution.Provenance))
	}
	if !execution.ClearEnvironment || !execution.KillProcessGroup {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test-suite execution lacks clear-environment/process-group policy", execution.Provenance))
	}
	if err := validateExactEnvironment(execution.Environment, execution.EnvironmentDigest); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test-suite execution environment: "+err.Error(), execution.Provenance))
	}
	for _, digest := range []string{execution.TreeDigest, execution.EnvironmentDigest, execution.StdoutDigest, execution.StderrDigest, execution.SignalValueDigest} {
		if !ValidDigest(digest) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test-suite execution has an invalid digest", execution.Provenance))
			break
		}
	}
	if execution.PassSignal.Kind != PassSignalExitCode && execution.PassSignal.Kind != PassSignalFile {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test-suite execution has invalid pass-signal kind", execution.PassSignal.Provenance))
	}
	if execution.PassSignal.Expected == "" || (execution.PassSignal.Kind == PassSignalFile && execution.PassSignal.Path == "") {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test-suite execution has incomplete pass signal", execution.PassSignal.Provenance))
	}
	if !execution.ExpectedPass || !execution.ObservedPass {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test-suite enumeration command did not pass", execution.Provenance))
	}
	toolFound := false
	for _, tool := range execution.Tools {
		if tool == verifier {
			toolFound = true
		}
	}
	if !toolFound {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test-suite execution does not bind its verifier tool", execution.Provenance))
	}
	if err := validateProvenance(execution.Provenance); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "test-suite execution: "+err.Error(), execution.Provenance))
	} else if source, exists := sources[execution.Provenance.ArtifactID]; !exists || source.Digest != execution.Provenance.ArtifactDigest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "test-suite execution is not anchored to a declared source", execution.Provenance))
	}
	if err := validateProvenance(execution.PassSignal.Provenance); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "test-suite pass signal: "+err.Error(), execution.PassSignal.Provenance))
	}
	return diagnostics
}

func candidateVectorCount(task *Task, points []BehaviorRef) (uint64, bool) {
	outcomeCounts := map[string]uint64{}
	for _, operation := range task.Operations {
		outcomeCounts[operation.ID] = uint64(len(operation.OutcomeIDs))
	}
	count := uint64(1)
	for _, point := range points {
		factor := outcomeCounts[point.OperationID]
		if factor == 0 || count > ^uint64(0)/factor {
			return 0, true
		}
		count *= factor
	}
	return count, false
}
