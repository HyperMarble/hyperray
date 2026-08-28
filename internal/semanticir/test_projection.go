package semanticir

import (
	"fmt"
	"reflect"
	"sort"
)

func validateTestProjection(model ArtifactModel) []Diagnostic {
	if model.Kind != ArtifactTests {
		if model.TestProjection != nil || model.RunnerSelection != nil {
			return []Diagnostic{errorDiagnostic(DiagnosticInvalidInput, "non-test artifact declares test projection/runner evidence", model.Coverage.Provenance)}
		}
		return nil
	}
	if model.TestProjection == nil || model.RunnerSelection == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticIncomplete, "test artifact lacks compiler-derived observation projection or runner selection", model.Coverage.Provenance)}
	}
	projection, runner := model.TestProjection, model.RunnerSelection
	var diagnostics []Diagnostic
	testIDs := make([]string, 0, len(model.Tests))
	for _, test := range model.Tests {
		testIDs = append(testIDs, test.ID)
	}
	sort.Strings(testIDs)
	gotIDs := append([]string(nil), projection.TestIDs...)
	sort.Strings(gotIDs)
	predicate := StaticTestPredicate(model.Tests, projection.Provenance)
	predicateDigest, _ := Digest(predicate)
	if projection.Source != model.Artifact || !reflect.DeepEqual(gotIDs, testIDs) || hasDuplicateStrings(gotIDs) || projection.PredicateDigest != predicateDigest || validateFactSource(projection.Provenance, model.Artifact) != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test projection source/tests/static predicate binding is incomplete", projection.Provenance))
	}
	wantKind := map[Language]CompilerIRKind{LanguagePython: CompilerIRCPythonBytecode, LanguageRust: CompilerIRRustMIR, LanguageCPP: CompilerIRLLVM}[model.Language]
	derivation := projection.Derivation
	modelDigest, _ := Digest(model.Tests)
	if derivation.SourceDigest != model.Artifact.Digest || derivation.Tool != model.Translator || derivation.IRKind != wantKind || !ValidDigest(derivation.WorkspaceTreeDigest) || !ValidDigest(derivation.IRDigest) || derivation.OutputDigest != DigestBytes(derivation.Output) || derivation.DecodedModelDigest != modelDigest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test projection compiler derivation is incomplete or stale", projection.Provenance))
	}
	diagnostics = append(diagnostics, ValidateProbeSteps(derivation.Steps, projection.Provenance)...)
	outputBound := false
	for _, step := range derivation.Steps {
		outputBound = outputBound || (step.Kind == ProbeStepRun && step.ExpectedStdoutDigest == derivation.OutputDigest)
	}
	if !outputBound {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test derivation output is not the exact output of a direct replay step", projection.Provenance))
	}
	constructs := map[string]TestConstructEvidence{}
	for _, construct := range projection.Constructs {
		if construct.ArtifactID != model.Artifact.ID || !ValidDigest(construct.Digest) || construct.IRKind != wantKind || construct.IRDigest != derivation.IRDigest || construct.Tool != model.Translator || len(construct.CompilerNodeIDs) == 0 || validateFactSource(construct.Provenance, model.Artifact) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test construct is not bound to compiler/interpreter IR", construct.Provenance))
		}
		if _, exists := constructs[construct.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "test projection repeats construct "+construct.ID, construct.Provenance))
		}
		constructs[construct.ID] = construct
	}
	if len(constructs) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test projection has no pass-influencing constructs", projection.Provenance))
	}
	if len(constructs) != model.Coverage.TotalConstructs {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test projection construct graph does not exactly match complete source/compiler coverage", projection.Provenance))
	}
	derived, graphDiagnostics := deriveTestProjection(model, projection, constructs)
	diagnostics = append(diagnostics, graphDiagnostics...)
	if !reflect.DeepEqual(derived, StaticTestPredicate(model.Tests, projection.Provenance)) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "test predicates differ from the compiler-derived pass dependency graph", projection.Provenance))
	}
	dependencyConstructs := map[string]struct{}{}
	for _, dependency := range projection.Dependencies {
		if _, exists := constructs[dependency.ConstructID]; !exists || (dependency.Kind != TestDependencyCall && dependency.Kind != TestDependencyRead && dependency.Kind != TestDependencyEffect) || len(dependency.CompilerNodeIDs) == 0 || dependency.Behavior.OperationID == "" || dependency.Behavior.Inputs == nil || !reflect.DeepEqual(dependency.Inputs, dependency.Behavior.Inputs) || validateFactSource(dependency.Provenance, model.Artifact) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test dependency is not a compiler-grounded BehaviorRef projection", dependency.Provenance))
		}
		dependencyConstructs[dependency.ConstructID] = struct{}{}
	}
	_ = dependencyConstructs // exact leaf/dependency agreement is checked by graph traversal below.
	runnerIDs := append([]string(nil), runner.TestIDs...)
	sort.Strings(runnerIDs)
	if !runner.ConjunctivePass || !reflect.DeepEqual(runnerIDs, testIDs) || runner.PredicateDigest != predicateDigest || runner.Configuration.Kind != ArtifactConfiguration || validateArtifactRef(runner.Configuration) != nil || validateToolRef(runner.Verifier) != nil || validateFactSource(runner.Provenance, runner.Configuration) != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test runner does not select exactly translated tests with conjunctive pass semantics", runner.Provenance))
	}
	command := runner.Command
	if command.Command == "" || command.WorkingDirectory == "" || command.TimeoutMillis <= 0 || !command.ClearEnvironment || !command.KillProcessGroup || validateExactEnvironment(command.Environment, command.EnvironmentDigest) != nil || !ValidDigest(command.TreeDigest) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test runner command is not exact and hermetic", runner.Provenance))
	}
	toolFound := false
	for _, tool := range command.Tools {
		toolFound = toolFound || tool == runner.Verifier
	}
	if !toolFound {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("test runner command omits verifier %q", runner.Verifier.Name), runner.Provenance))
	}
	return diagnostics
}

// TestProjectionGraphDigest binds the complete compiler-derived pass graph.
func TestProjectionGraphDigest(projection TestObservationProjection) (string, error) {
	return Digest(struct {
		Source          ArtifactRef                  `json:"source"`
		TestIDs         []string                     `json:"test_ids"`
		PredicateDigest string                       `json:"predicate_digest"`
		Constructs      []TestConstructEvidence      `json:"constructs"`
		Dependencies    []TestBehaviorDependency     `json:"dependencies"`
		Nodes           []TestProjectionNode         `json:"nodes"`
		PassRoots       []TestPassRoot               `json:"pass_roots"`
		Quantification  []TestQuantificationEvidence `json:"quantification"`
		Derivation      CompilerDerivationEvidence   `json:"derivation"`
	}{projection.Source, projection.TestIDs, projection.PredicateDigest, projection.Constructs, projection.Dependencies, projection.Nodes, projection.PassRoots, projection.Quantification, projection.Derivation})
}

func deriveTestProjection(model ArtifactModel, projection *TestObservationProjection, constructs map[string]TestConstructEvidence) (TestPredicate, []Diagnostic) {
	var diagnostics []Diagnostic
	nodes := map[string]TestProjectionNode{}
	for _, node := range projection.Nodes {
		if node.ID == "" || len(node.CompilerNodeIDs) == 0 || validateFactSource(node.Provenance, model.Artifact) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test projection node lacks compiler IR identity", node.Provenance))
		}
		if _, dup := nodes[node.ID]; dup {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "test projection repeats node "+node.ID, node.Provenance))
		}
		nodes[node.ID] = node
		for _, id := range node.ConstructIDs {
			if _, ok := constructs[id]; !ok {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test projection node refers to unknown construct "+id, node.Provenance))
			}
		}
	}
	state := map[string]int{}
	reachable := map[string]struct{}{}
	dependencyKeys := map[string]struct{}{}
	for _, dependency := range projection.Dependencies {
		dependencyKeys[dependency.ConstructID+"\x00"+BehaviorRefKey(dependency.Behavior)] = struct{}{}
	}
	usedDependencies := map[string]struct{}{}
	var visit func(string) (TestPredicate, bool)
	visit = func(id string) (TestPredicate, bool) {
		node, exists := nodes[id]
		if !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test pass graph refers to missing node "+id, projection.Provenance))
			return TestPredicate{}, false
		}
		if state[id] == 1 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "test pass graph contains a cycle at "+id, node.Provenance))
			return TestPredicate{}, false
		}
		if state[id] == 2 {
			return nodePredicate(nodes, id), true
		}
		state[id] = 1
		reachable[id] = struct{}{}
		predicate := TestPredicate{Kind: node.Kind, Observe: node.Observe, Left: node.Left, Right: node.Right, Provenance: node.Provenance}
		switch node.Kind {
		case PredicateTrue, PredicateFalse:
			if len(node.Children) != 0 || node.Observe != nil || node.Left != nil || node.Right != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "constant test graph node has operands", node.Provenance))
			}
		case PredicateAnd, PredicateOr:
			if len(node.Children) == 0 {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test graph boolean node has no children", node.Provenance))
			}
			for _, child := range node.Children {
				value, ok := visit(child)
				if ok {
					predicate.Children = append(predicate.Children, value)
				}
			}
		case PredicateNot:
			if len(node.Children) != 1 {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test graph not node needs one child", node.Provenance))
			} else {
				value, ok := visit(node.Children[0])
				if ok {
					predicate.Children = []TestPredicate{value}
				}
			}
		case PredicateOutcomeIn, PredicateRaises, PredicateHasEffect:
			if len(node.Children) != 0 || node.Observe == nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test graph observation leaf is malformed", node.Provenance))
			}
			if node.Observe != nil {
				keySuffix := BehaviorRefKey(node.Observe.Behavior)
				for _, constructID := range node.ConstructIDs {
					key := constructID + "\x00" + keySuffix
					if _, ok := dependencyKeys[key]; !ok {
						diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "BehaviorRef leaf has no exact compiler dependency edge", node.Provenance))
					} else {
						usedDependencies[key] = struct{}{}
					}
				}
			}
		case PredicateOutcomeEqual:
			if len(node.Children) != 0 || node.Left == nil || node.Right == nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test graph equality leaf is malformed", node.Provenance))
			}
			for _, reference := range []*BehaviorRef{node.Left, node.Right} {
				if reference != nil {
					keySuffix := BehaviorRefKey(*reference)
					for _, constructID := range node.ConstructIDs {
						key := constructID + "\x00" + keySuffix
						if _, ok := dependencyKeys[key]; !ok {
							diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "BehaviorRef equality leaf has no exact compiler dependency edge", node.Provenance))
						} else {
							usedDependencies[key] = struct{}{}
						}
					}
				}
			}
		default:
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, "test graph node has unsupported kind", node.Provenance))
		}
		state[id] = 2
		nodes[id] = node
		_ = predicate
		return predicate, true
	}
	models := map[string]TestModel{}
	for _, model := range model.Tests {
		models[model.ID] = model
	}
	rootSeen := map[string]struct{}{}
	derivedModels := []TestModel{}
	for _, root := range projection.PassRoots {
		test, exists := models[root.TestID]
		if !exists || root.NodeID == "" || len(root.CompilerNodeIDs) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test pass root is missing its translated test/compiler node", projection.Provenance))
			continue
		}
		if _, dup := rootSeen[root.TestID]; dup {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "test projection repeats pass root "+root.TestID, projection.Provenance))
		}
		rootSeen[root.TestID] = struct{}{}
		predicate, ok := visit(root.NodeID)
		if ok {
			test.Predicate = predicate
			derivedModels = append(derivedModels, test)
		}
	}
	if len(rootSeen) != len(models) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test pass graph does not root every translated test", projection.Provenance))
	}
	if len(reachable) != len(nodes) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test pass graph contains unreachable/unclassified nodes", projection.Provenance))
	}
	constructUse := map[string]struct{}{}
	for id := range reachable {
		for _, constructID := range nodes[id].ConstructIDs {
			constructUse[constructID] = struct{}{}
		}
	}
	for id := range constructs {
		if _, ok := constructUse[id]; !ok {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "pass-influencing construct is absent from reachable pass graph "+id, projection.Provenance))
		}
	}
	if len(usedDependencies) != len(dependencyKeys) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "test projection has dependency edges not consumed by reachable BehaviorRef leaves", projection.Provenance))
	}
	return StaticTestPredicate(derivedModels, projection.Provenance), diagnostics
}

func nodePredicate(nodes map[string]TestProjectionNode, id string) TestPredicate {
	node := nodes[id]
	predicate := TestPredicate{Kind: node.Kind, Observe: node.Observe, Left: node.Left, Right: node.Right, Provenance: node.Provenance}
	for _, child := range node.Children {
		predicate.Children = append(predicate.Children, nodePredicate(nodes, child))
	}
	return predicate
}

func validateArtifactExtensionality(model ArtifactModel, projection *TestObservationProjection) []Diagnostic {
	record := projection.Extensionality
	var diagnostics []Diagnostic
	if record.Result != ProofProved || !ValidDigest(record.ObservationIRDigest) || !ValidDigest(record.HarnessDigest) || record.ObservationIRDigest != projection.Derivation.IRDigest || validateToolRef(record.Prover) != nil || validateFactSource(record.Provenance, model.Artifact) != nil {
		return []Diagnostic{errorDiagnostic(DiagnosticIncomplete, "test artifact has no compiler-derived extensionality proof", record.Provenance)}
	}
	wantBehaviors := map[string]BehaviorRef{}
	for _, dependency := range projection.Dependencies {
		key := BehaviorRefKey(dependency.Behavior)
		wantBehaviors[key] = dependency.Behavior
	}
	seen := map[string]struct{}{}
	predicates := make([]CompilerPredicate, 0, len(record.BehaviorEqualities))
	for _, equality := range record.BehaviorEqualities {
		key := BehaviorRefKey(equality.Behavior)
		if _, exists := wantBehaviors[key]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test extensionality equality is not a projected dependency", equality.Behavior.Provenance))
		}
		if _, dup := seen[key]; dup {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "test extensionality repeats behavior "+key, equality.Behavior.Provenance))
		}
		seen[key] = struct{}{}
		predicates = append(predicates, equality.Predicate)
		diagnostics = append(diagnostics, validateCompilerPredicate(equality.Predicate, model.Translator, record.ObservationIRDigest, equality.Behavior.Provenance)...)
	}
	if len(seen) != len(wantBehaviors) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test extensionality does not cover every projected BehaviorRef", record.Provenance))
	}
	diagnostics = append(diagnostics, validateCompilerPredicate(record.LeftPass, model.Translator, record.ObservationIRDigest, record.Provenance)...)
	diagnostics = append(diagnostics, validateCompilerPredicate(record.RightPass, model.Translator, record.ObservationIRDigest, record.Provenance)...)
	context := CompilerProofContext{SourceDigest: model.Artifact.Digest, WorkspaceTreeDigest: projection.Derivation.WorkspaceTreeDigest, EmittedIRDigest: record.ObservationIRDigest, HarnessDigest: record.HarnessDigest, Compiler: model.Translator}
	claim := NewTestObservationCompletenessClaim(context, record.Proof.Claim.Scope, predicates, record.LeftPass, record.RightPass)
	if !reflect.DeepEqual(record.Proof.Claim, claim) || record.Proof.Prover != record.Prover || record.ProofDigest != record.Proof.QueryDigest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test extensionality proof differs from exact compiler/dependency/pass binding", record.Provenance))
	}
	diagnostics = append(diagnostics, ValidateReplayableProof(record.Proof, SolverUNSAT, record.Provenance)...)
	return diagnostics
}
