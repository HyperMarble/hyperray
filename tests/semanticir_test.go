package tests

import (
	"reflect"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/internal/semanticir"
	"github.com/HyperMarble/hyperray/internal/testir"
)

func TestSemanticIRRepresentsProofContract(t *testing.T) {
	instructionBytes := []byte("Choose a finite result.\n")
	specBytes := []byte("frozen strict spec")
	codeBytes := []byte("def choose(mode): ...")
	testBytes := []byte("def test_choose(): ...")
	environmentBytes := []byte("environment")
	configurationBytes := []byte("configuration")
	toolBytes := []byte("translator")
	instruction := frozenRef("instruction", semanticir.ArtifactInstruction, "instruction.md", instructionBytes)
	spec := frozenRef("spec", semanticir.ArtifactSpec, "spec.md", specBytes)
	code := frozenRef("code", semanticir.ArtifactCode, "choose.py", codeBytes)
	testsArtifact := frozenRef("tests", semanticir.ArtifactTests, "test_choose.py", testBytes)
	environmentArtifact := frozenRef("environment", semanticir.ArtifactEnvironment, "environment.json", environmentBytes)
	configurationArtifact := frozenRef("configuration", semanticir.ArtifactConfiguration, "hyperray.toml", configurationBytes)
	prov := semanticir.NewProvenance(spec, semanticir.SourceLocation{Path: spec.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	instructionProv := semanticir.NewProvenance(instruction, semanticir.SourceLocation{Path: instruction.Path, StartLine: 1, StartColumn: 1, EndLine: 1}, semanticir.TranslationTranslated)
	codeProv := semanticir.NewProvenance(code, semanticir.SourceLocation{Path: code.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	testProv := semanticir.NewProvenance(testsArtifact, semanticir.SourceLocation{Path: testsArtifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	environmentProv := semanticir.NewProvenance(environmentArtifact, semanticir.SourceLocation{Path: environmentArtifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	configurationProv := semanticir.NewProvenance(configurationArtifact, semanticir.SourceLocation{Path: configurationArtifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	domain := semanticir.Domain{ID: "mode", Type: semanticir.TypeString, Provenance: prov, Values: []semanticir.DomainValue{{ID: "zero", Provenance: prov}, {ID: "one", Provenance: prov}}}
	writeEffect := semanticir.Effect{ID: "write-result", Kind: semanticir.EffectWrite, Target: "result", Provenance: prov}
	zero := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 0}, Effects: []semanticir.Effect{writeEffect}, Provenance: prov}
	zero.ID = semanticir.OutcomeID(zero)
	one := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 1}, Provenance: prov}
	one.ID = semanticir.OutcomeID(one)
	other := semanticir.OtherOutcome("choose", prov)
	operation := semanticir.Operation{ID: "choose", Kind: semanticir.OperationCallable, DomainIDs: []string{"mode"}, OutcomeIDs: []string{zero.ID, one.ID, other.ID}, Inputs: []semanticir.Variable{{Name: "mode", Type: semanticir.TypeString, DomainID: "mode", Provenance: prov}}, Provenance: prov}
	for index := range domain.Values {
		literal := semanticir.Literal{Type: semanticir.TypeString, String: domain.Values[index].ID}
		domain.Values[index].Groundings = []semanticir.GroundingAxiom{{OperationID: operation.ID, Kind: semanticir.GroundingMembership, Membership: &semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{{Kind: semanticir.ExprVariable, Type: semanticir.TypeString, Name: "mode", Provenance: prov}, {Kind: semanticir.ExprLiteral, Type: semanticir.TypeString, Literal: &literal, Provenance: prov}}, Provenance: prov}, ConcreteWitness: map[string]semanticir.Literal{"mode": literal}, Provenance: prov}}
	}
	zeroConditions := semanticir.Assignment{"mode": "zero"}
	oneConditions := semanticir.Assignment{"mode": "one"}
	zeroGroundingID := semanticir.AssignmentGroundingID(operation.ID, zeroConditions)
	oneGroundingID := semanticir.AssignmentGroundingID(operation.ID, oneConditions)
	clause := semanticir.InstructionClause{ID: "clause", Span: instructionProv.Location, SliceDigest: semanticir.DigestBytes(instructionBytes[:len(instructionBytes)-1]), Provenance: instructionProv}
	completeSpecCoverage := semanticir.TranslationCoverage{Status: semanticir.TranslationComplete, TotalConstructs: 1, TranslatedConstructs: 1, Provenance: prov}
	completeInstructionCoverage := semanticir.TranslationCoverage{Status: semanticir.TranslationComplete, TotalConstructs: 1, TranslatedConstructs: 1, Provenance: instructionProv}
	task := &semanticir.Task{
		ID:               "task",
		Instruction:      instruction,
		InstructionModel: semanticir.InstructionModel{Artifact: instruction, Clauses: []semanticir.InstructionClause{clause}, Coverage: completeInstructionCoverage},
		Spec:             spec,
		Domains:          []semanticir.Domain{domain},
		Groundings: []semanticir.AssignmentGrounding{
			{ID: zeroGroundingID, OperationID: operation.ID, Conditions: zeroConditions, Inputs: map[string]semanticir.Literal{"mode": {Type: semanticir.TypeString, String: "zero"}}, Provenance: prov},
			{ID: oneGroundingID, OperationID: operation.ID, Conditions: oneConditions, Inputs: map[string]semanticir.Literal{"mode": {Type: semanticir.TypeString, String: "one"}}, Provenance: prov},
		},
		Operations: []semanticir.Operation{operation},
		Outcomes:   []semanticir.ObservableOutcome{zero, one, other},
		Requirements: []semanticir.RequirementCase{
			{ID: "zero", Conditions: zeroConditions, OperationID: "choose", RequiredOutcomes: []string{zero.ID}, ForbiddenOutcomes: []string{one.ID, other.ID}, Effects: []semanticir.Effect{writeEffect}, GroundingID: zeroGroundingID, InstructionClauseIDs: []string{clause.ID}, InstructionSources: []semanticir.Provenance{instructionProv}, Evidence: []semanticir.Provenance{prov, instructionProv}, Provenance: prov},
			{ID: "one", Conditions: oneConditions, OperationID: "choose", RequiredOutcomes: []string{one.ID}, ForbiddenOutcomes: []string{zero.ID, other.ID}, GroundingID: oneGroundingID, InstructionClauseIDs: []string{clause.ID}, InstructionSources: []semanticir.Provenance{instructionProv}, Evidence: []semanticir.Provenance{prov, instructionProv}, Provenance: prov},
		},
		Coverage:   []semanticir.TranslationCoverage{completeSpecCoverage, completeInstructionCoverage},
		Provenance: prov,
	}
	tool := semanticir.ToolRef{Name: "frontend", Path: "/tools/frontend", Digest: semanticir.DigestBytes(toolBytes), Version: "1.0.0"}
	codeCoverage := semanticir.TranslationCoverage{Status: semanticir.TranslationComplete, TotalConstructs: 2, TranslatedConstructs: 2, Provenance: codeProv}
	codeCases := []semanticir.BehaviorCase{{ID: "code-zero", Conditions: semanticir.Assignment{"mode": "zero"}, OperationID: "choose", Inputs: map[string]semanticir.Literal{"mode": {Type: semanticir.TypeString, String: "zero"}}, OutcomeIDs: []string{zero.ID}, Provenance: codeProv}, {ID: "code-one", Conditions: semanticir.Assignment{"mode": "one"}, OperationID: "choose", Inputs: map[string]semanticir.Literal{"mode": {Type: semanticir.TypeString, String: "one"}}, OutcomeIDs: []string{one.ID}, Provenance: codeProv}}
	codeModel := semanticir.ArtifactModel{
		Artifact: code, Kind: semanticir.ArtifactCode, Language: semanticir.LanguagePython,
		Domains: []semanticir.Domain{domain}, Operations: []semanticir.Operation{{ID: "choose", Kind: semanticir.OperationFunction, DomainIDs: []string{"mode"}, OutcomeIDs: []string{zero.ID, one.ID, other.ID}, Inputs: []semanticir.Variable{{Name: "mode", Type: semanticir.TypeString, DomainID: "mode", Provenance: codeProv}}, Provenance: codeProv}},
		Outcomes:         []semanticir.ObservableOutcome{{ID: zero.ID, Kind: zero.Kind, Value: zero.Value, Effects: []semanticir.Effect{{ID: writeEffect.ID, Kind: writeEffect.Kind, Target: writeEffect.Target, Provenance: codeProv}}, Provenance: codeProv}, {ID: one.ID, Kind: one.Kind, Value: one.Value, Provenance: codeProv}, semanticir.OtherOutcome("choose", codeProv)},
		Cases:            codeCases,
		CompilerEvidence: []semanticir.CompilerEvidence{compilerEvidenceFor(code, codeProv, tool, operation, domain, codeCases)}, ScopeClosure: scopeClosureFor(code, codeProv, tool, operation), Coverage: codeCoverage, Translator: tool,
	}
	validEvidence := codeModel.CompilerEvidence[0]
	if diagnostics := semanticir.ValidateCompilerSemanticGraph(codeModel, validEvidence); semanticir.HasErrors(diagnostics) {
		t.Fatalf("valid compiler semantic graph rejected: %+v", diagnostics)
	}
	assertGraphRejected := func(name string, mutate func(*semanticir.CompilerSemanticGraph)) {
		t.Helper()
		evidence := validEvidence
		graph := *validEvidence.SemanticGraph
		graph.Nodes = append([]semanticir.CompilerSemanticNode(nil), graph.Nodes...)
		graph.Blocks = append([]semanticir.CompilerSemanticBlock(nil), graph.Blocks...)
		graph.Edges = append([]semanticir.CompilerControlEdge(nil), graph.Edges...)
		graph.Operations = append([]semanticir.CompilerOperationGraph(nil), graph.Operations...)
		graph.Numeric = append([]semanticir.CompilerNumericSemantics(nil), graph.Numeric...)
		mutate(&graph)
		evidence.SemanticGraph = &graph
		if diagnostics := semanticir.ValidateCompilerSemanticGraph(codeModel, evidence); !semanticir.HasErrors(diagnostics) {
			t.Fatalf("%s compiler graph adversary was accepted", name)
		}
	}
	assertGraphRejected("partial-branch", func(graph *semanticir.CompilerSemanticGraph) { graph.Edges = graph.Edges[:1] })
	assertGraphRejected("omitted-node", func(graph *semanticir.CompilerSemanticGraph) { graph.Nodes = graph.Nodes[1:] })
	assertGraphRejected("cross-operation", func(graph *semanticir.CompilerSemanticGraph) {
		graph.Operations = append(graph.Operations, semanticir.CompilerOperationGraph{OperationID: "other-operation", EntryBlockID: "entry", TerminalNodeIDs: []string{"return-one"}, Provenance: codeProv})
	})
	assertGraphRejected("non-dominating-operand", func(graph *semanticir.CompilerSemanticGraph) {
		graph.Blocks[1].NodeIDs = []string{"write-result", "return-zero"}
		graph.Blocks[2].NodeIDs = append([]string{"integer-zero"}, graph.Blocks[2].NodeIDs...)
	})
	assertGraphRejected("mixed-numeric", func(graph *semanticir.CompilerSemanticGraph) {
		graph.Numeric = append(graph.Numeric, semanticir.CompilerNumericSemantics{ID: "i8", Kind: semanticir.CompilerNumericBitVector, Width: 8, Signed: true, Overflow: semanticir.CompilerOverflowWrap, Range: semanticir.CompilerRangeAll})
		for index := range graph.Nodes {
			if graph.Nodes[index].ID == "return-one" {
				graph.Nodes[index].NumericID = "i8"
			}
		}
	})
	if diagnostics := task.AddArtifact(codeModel); semanticir.HasErrors(diagnostics) {
		t.Fatalf("AddArtifact(code): %+v", diagnostics)
	}
	testCoverage := semanticir.TranslationCoverage{Status: semanticir.TranslationComplete, TotalConstructs: 2, TranslatedConstructs: 2, Provenance: testProv}
	testModels := []semanticir.TestModel{}
	for _, item := range []struct{ value, outcome string }{{"zero", zero.ID}, {"one", one.ID}} {
		inputs := map[string]semanticir.Literal{"mode": {Type: semanticir.TypeString, String: item.value}}
		behavior := semanticir.BehaviorRef{OperationID: "choose", Conditions: semanticir.Assignment{"mode": item.value}, Inputs: inputs, Provenance: testProv}
		observation := semanticir.Observation{Kind: semanticir.ObserveOutcome, Behavior: behavior, OutcomeIDs: []string{item.outcome}, Provenance: testProv}
		testModels = append(testModels, semanticir.TestModel{ID: "test-" + item.value, Conditions: behavior.Conditions, OperationID: "choose", AcceptedOutcomes: []string{item.outcome}, Predicate: semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeIn, Observe: &observation, Provenance: testProv}, Provenance: testProv})
	}
	testPredicate := semanticir.StaticTestPredicate(testModels, testProv)
	testPredicateDigest := mustDigest(testPredicate)
	testIRDigest := semanticir.DigestBytes([]byte("test-observation-ir"))
	testConstructs := []semanticir.TestConstructEvidence{{ID: "assert-one", ArtifactID: testsArtifact.ID, Kind: semanticir.TestConstructAssertion, Digest: semanticir.DigestBytes([]byte("assert-one")), IRKind: semanticir.CompilerIRCPythonBytecode, IRDigest: testIRDigest, Tool: tool, CompilerNodeIDs: []string{"assert-node-one"}, Provenance: testProv}, {ID: "assert-zero", ArtifactID: testsArtifact.ID, Kind: semanticir.TestConstructAssertion, Digest: semanticir.DigestBytes([]byte("assert-zero")), IRKind: semanticir.CompilerIRCPythonBytecode, IRDigest: testIRDigest, Tool: tool, CompilerNodeIDs: []string{"assert-node-zero"}, Provenance: testProv}}
	dependencies := []semanticir.TestBehaviorDependency{{ConstructID: "assert-zero", Kind: semanticir.TestDependencyCall, Behavior: testModels[0].Predicate.Observe.Behavior, Inputs: testModels[0].Predicate.Observe.Behavior.Inputs, CompilerNodeIDs: []string{"call-zero"}, Provenance: testProv}, {ConstructID: "assert-one", Kind: semanticir.TestDependencyCall, Behavior: testModels[1].Predicate.Observe.Behavior, Inputs: testModels[1].Predicate.Observe.Behavior.Inputs, CompilerNodeIDs: []string{"call-one"}, Provenance: testProv}}
	quantification := make([]semanticir.TestQuantificationEvidence, 0, len(dependencies))
	for _, dependency := range dependencies {
		concrete := []map[string]semanticir.Literal{dependency.Behavior.Inputs}
		digest, err := semanticir.TestConcreteInputsDigest(concrete)
		if err != nil {
			t.Fatal(err)
		}
		quantification = append(quantification, semanticir.TestQuantificationEvidence{Behavior: semanticir.BehaviorRef{OperationID: dependency.Behavior.OperationID, Conditions: dependency.Behavior.Conditions, Provenance: testProv}, Kind: semanticir.TestQuantificationSingleton, ConcreteInputs: concrete, ConcreteInputsDigest: digest, Result: semanticir.ProofProved, Provenance: testProv})
	}
	nodes := []semanticir.TestProjectionNode{{ID: "pass-zero", Kind: semanticir.PredicateOutcomeIn, Observe: testModels[0].Predicate.Observe, CompilerNodeIDs: []string{"pass-node-zero"}, ConstructIDs: []string{"assert-zero"}, Provenance: testProv}, {ID: "pass-one", Kind: semanticir.PredicateOutcomeIn, Observe: testModels[1].Predicate.Observe, CompilerNodeIDs: []string{"pass-node-one"}, ConstructIDs: []string{"assert-one"}, Provenance: testProv}}
	passRoots := []semanticir.TestPassRoot{{TestID: "test-zero", NodeID: "pass-zero", CompilerNodeIDs: []string{"root-zero"}}, {TestID: "test-one", NodeID: "pass-one", CompilerNodeIDs: []string{"root-one"}}}
	derivationOutput, _ := semanticir.CanonicalJSON(testModels)
	stepEnvironment := []semanticir.EnvironmentVariable{{Name: "PATH", Value: "/tools"}}
	derivationStep := semanticir.ProbeStep{ID: "derive-tests", Kind: semanticir.ProbeStepRun, Tool: tool, Argv: []string{"--derive-tests"}, StdinDigest: semanticir.DigestBytes(nil), WorkingDirectory: "/workspace", Environment: stepEnvironment, EnvironmentDigest: mustDigest(stepEnvironment), ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 1000, ExpectedStdoutDigest: semanticir.DigestBytes(derivationOutput), ExpectedStderrDigest: semanticir.DigestBytes(nil), ExpectedSignalDigest: semanticir.DigestBytes(nil), SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone}, Provenance: testProv}
	suiteExecution := workspaceEvidence("suite", semanticir.WorkspaceSolutionNewTests, true, environmentProv, environmentBytes, tool)
	extScope := compilerPredicate(tool, testIRDigest, "artifact-test-scope", "true")
	extEqualities := []semanticir.TestBehaviorEquality{}
	extPredicates := []semanticir.CompilerPredicate{}
	for index, dependency := range dependencies {
		predicate := compilerPredicate(tool, testIRDigest, "artifact-equality", "(= ray_mode ray_mode)")
		predicate.CompilerNodeIDs = []string{"artifact-equality-node-" + string(rune('0'+index))}
		extEqualities = append(extEqualities, semanticir.TestBehaviorEquality{Behavior: dependency.Behavior, Predicate: predicate})
		extPredicates = append(extPredicates, predicate)
	}
	extLeft := compilerPredicate(tool, testIRDigest, "artifact-left-pass", "true")
	extRight := compilerPredicate(tool, testIRDigest, "artifact-right-pass", "true")
	extHarness := semanticir.DigestBytes([]byte("artifact-test-harness"))
	extContext := semanticir.CompilerProofContext{SourceDigest: testsArtifact.Digest, WorkspaceTreeDigest: suiteExecution.TreeDigest, EmittedIRDigest: testIRDigest, HarnessDigest: extHarness, Compiler: tool}
	extClaim := semanticir.NewTestObservationCompletenessClaim(extContext, extScope, extPredicates, extLeft, extRight)
	extProof := replayableProof(tool, semanticir.SolverUNSAT, extClaim)
	extensionality := semanticir.TestExtensionalityEvidence{ObservationIRDigest: testIRDigest, HarnessDigest: extHarness, Prover: tool, BehaviorEqualities: extEqualities, LeftPass: extLeft, RightPass: extRight, Result: semanticir.ProofProved, ProofDigest: extProof.QueryDigest, Proof: extProof, Provenance: testProv}
	projection := &semanticir.TestObservationProjection{Source: testsArtifact, TestIDs: []string{"test-one", "test-zero"}, PredicateDigest: testPredicateDigest, Constructs: testConstructs, Dependencies: dependencies, Nodes: nodes, PassRoots: passRoots, Quantification: quantification, Derivation: semanticir.CompilerDerivationEvidence{SourceDigest: testsArtifact.Digest, WorkspaceTreeDigest: suiteExecution.TreeDigest, Tool: tool, IRKind: semanticir.CompilerIRCPythonBytecode, IRDigest: testIRDigest, Steps: []semanticir.ProbeStep{derivationStep}, Output: derivationOutput, OutputDigest: semanticir.DigestBytes(derivationOutput), DecodedModelDigest: mustDigest(testModels), Complete: true}, Extensionality: extensionality, Complete: true, Provenance: testProv}
	runner := &semanticir.RunnerSelectionEvidence{TestIDs: []string{"test-one", "test-zero"}, PredicateDigest: testPredicateDigest, Configuration: configurationArtifact, Verifier: tool, Command: suiteExecution, ConjunctivePass: true, Complete: true, Provenance: configurationProv}
	testModel := semanticir.ArtifactModel{Artifact: testsArtifact, Kind: semanticir.ArtifactTests, Language: semanticir.LanguagePython, Operations: []semanticir.Operation{{ID: "test_choose", Kind: semanticir.OperationTest, Provenance: testProv}}, Tests: testModels, TestProjection: projection, RunnerSelection: runner, Coverage: testCoverage, Translator: tool}
	if diagnostics := task.AddArtifact(testModel); semanticir.HasErrors(diagnostics) {
		t.Fatalf("AddArtifact(tests): %+v", diagnostics)
	}
	task.Environment = &semanticir.EnvironmentModel{
		Artifact: configurationArtifact, Configuration: configurationArtifact, SourceArtifacts: []semanticir.ArtifactRef{environmentArtifact}, Identity: "linux-amd64", ConfigDigest: configurationArtifact.Digest, Tools: []semanticir.ToolRef{tool}, Provenance: configurationProv,
		Coverage: semanticir.TranslationCoverage{Status: semanticir.TranslationComplete, TotalConstructs: 1, TranslatedConstructs: 1, Provenance: configurationProv},
		Commands: []semanticir.WorkspaceCommand{
			workspaceEvidence("base-old", semanticir.WorkspaceBaseOldTests, true, environmentProv, environmentBytes, tool),
			workspaceEvidence("base-new", semanticir.WorkspaceBaseNewTests, false, environmentProv, environmentBytes, tool),
			workspaceEvidence("solution", semanticir.WorkspaceSolutionNewTests, true, environmentProv, environmentBytes, tool),
		},
	}
	phaseASpec := frozenRef("phase-a-spec", semanticir.ArtifactSpec, "spec.pretest.md", []byte("phase a spec"))
	phaseEnvironmentModel := semanticir.PhaseAEnvironmentModel{Schema: semanticir.PhaseAEnvironmentSchemaV1, Identity: task.Environment.Identity, ConfigurationDigest: task.Environment.ConfigDigest, Tools: []semanticir.ToolRef{tool}, SourceArtifacts: []semanticir.ArtifactRef{environmentArtifact}, Complete: true}
	phaseEnvironmentBytes, _ := semanticir.CanonicalJSON(phaseEnvironmentModel)
	phaseEnvironmentArtifact := frozenRef("phase-a-environment", semanticir.ArtifactEnvironment, "environment.phase-a.json", phaseEnvironmentBytes)
	authoringRecord := frozenRef("phase-a-record", semanticir.ArtifactSpecAuthoringRecord, "spec-authoring-phase-a.md", []byte("accepted record"))
	ledger := frozenRef("phase-a-ledger", semanticir.ArtifactSpecLedger, "spec-pretest.sha256", []byte("ledger"))
	frozenSemantics, err := semanticir.FrozenSpecSemanticsDigest(task)
	if err != nil {
		t.Fatal(err)
	}
	task.SpecAcceptance = &semanticir.SpecAcceptanceEvidence{
		Schema: semanticir.SpecAuthoringRecordSchemaV1, AuthoringRecord: authoringRecord, DetachedLedger: ledger, PhaseASpec: phaseASpec, FinalSpec: spec,
		Instruction: instruction, Environment: phaseEnvironmentArtifact, PhaseAEnvironment: phaseEnvironmentArtifact, PhaseAEnvironmentModel: phaseEnvironmentModel, TaskID: task.ID, PhaseASpecIRDigest: semanticir.DigestBytes([]byte("phase-a-ir")), FrozenSemanticsDigest: frozenSemantics, EnvironmentConfigDigest: task.Environment.ConfigDigest,
		Manifest:   []semanticir.AcceptanceSourceBinding{{Role: "instruction", Path: instruction.Path, Digest: instruction.Digest, Relevant: "line 1"}, {Role: "environment", Path: phaseEnvironmentArtifact.Path, Digest: phaseEnvironmentArtifact.Digest, Relevant: "frozen authoring environment"}},
		Operations: []semanticir.AcceptanceOperationBinding{{OperationID: operation.ID, EntryPoint: operation.ID, PhaseAEvidence: "instruction line 1", ObservableBoundary: "return and effects", InstructionClauseIDs: []string{clause.ID}, Evidence: []semanticir.Provenance{instructionProv}, Decision: semanticir.SpecAcceptanceAccepted}}, Domains: []semanticir.AcceptanceDomainBinding{{OperationID: operation.ID, DomainID: domain.ID, ValueIDs: []string{"zero", "one"}, Labels: []semanticir.AcceptanceLabelBinding{{ValueID: "zero", DefinitionEvidence: []semanticir.Provenance{instructionProv}, ExpectedCompilerPath: "mode zero path", ExpectedReachableWitness: "mode=zero"}, {ValueID: "one", DefinitionEvidence: []semanticir.Provenance{instructionProv}, ExpectedCompilerPath: "mode one path", ExpectedReachableWitness: "mode=one"}}}}, Reviews: []semanticir.AcceptanceReviewBinding{{ID: "review-all", RequirementIDs: []string{"zero", "one"}, InstructionClauseIDs: []string{clause.ID}, Decision: semanticir.SpecAcceptanceAccepted, Evidence: []semanticir.Provenance{instructionProv}}}, NoDisagreements: true, LintCommand: "hyperray spec-lint spec.pretest.md",
		TestAccess: "not-accessed", Decision: semanticir.SpecAcceptanceAccepted, ExpandedTableReview: semanticir.SpecAcceptanceAccepted, ExpectedGroundingReview: semanticir.SpecAcceptanceAccepted,
		AuthorIdentity: "author@example", IndependentReviewer: "reviewer@example", CompletedAtUTC: "2026-08-27T12:00:00Z", SnapshotPath: phaseASpec.Path, FinalPath: spec.Path, LedgerPath: ledger.Path, Complete: true,
		Evidence: []semanticir.Provenance{
			semanticir.NewProvenance(authoringRecord, semanticir.SourceLocation{Path: authoringRecord.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated),
			semanticir.NewProvenance(ledger, semanticir.SourceLocation{Path: ledger.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated),
			semanticir.NewProvenance(phaseASpec, semanticir.SourceLocation{Path: phaseASpec.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated),
			semanticir.NewProvenance(phaseEnvironmentArtifact, semanticir.SourceLocation{Path: phaseEnvironmentArtifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated),
		},
	}
	modelDigest := mustDigest(testModel)
	vectorResults := []semanticir.TestVectorResult{}
	for _, left := range []string{zero.ID, one.ID} {
		for _, right := range []string{zero.ID, one.ID} {
			vectorResults = append(vectorResults, semanticir.TestVectorResult{Choices: []semanticir.BehaviorChoice{{Behavior: testModels[0].Predicate.Observe.Behavior, OutcomeID: left}, {Behavior: testModels[1].Predicate.Observe.Behavior, OutcomeID: right}}, Accepted: left == zero.ID && right == one.ID})
		}
	}
	vectorDigest, acceptedDigest, err := semanticir.TestVectorDigests(vectorResults)
	if err != nil {
		t.Fatal(err)
	}
	suiteSources := []semanticir.ArtifactRef{testsArtifact, environmentArtifact, configurationArtifact}
	suiteSourceDigest, err := semanticir.TestSuiteSourceDigest(suiteSources)
	if err != nil {
		t.Fatal(err)
	}
	projectionGraphDigest, _ := semanticir.TestProjectionGraphDigest(*projection)
	projectionComponents := []semanticir.ArtifactModelDigest{{ArtifactID: testsArtifact.ID, Digest: projectionGraphDigest}}
	observationIRDigest := mustDigest(projectionComponents)
	observationHarnessDigest := mustDigest(suiteExecution)
	observationContext := semanticir.CompilerProofContext{SourceDigest: suiteSourceDigest, WorkspaceTreeDigest: suiteExecution.TreeDigest, EmittedIRDigest: observationIRDigest, HarnessDigest: observationHarnessDigest, Compiler: tool}
	observationScope := compilerPredicate(tool, observationIRDigest, "test-scope", "true")
	behaviorEqualities := []semanticir.TestBehaviorEquality{}
	var equalityPredicates []semanticir.CompilerPredicate
	for index, testModel := range testModels {
		predicate := compilerPredicate(tool, observationIRDigest, "behavior-equality", "(= ray_mode ray_mode)")
		predicate.CompilerNodeIDs = []string{"equality-node-" + string(rune('0'+index))}
		behaviorEqualities = append(behaviorEqualities, semanticir.TestBehaviorEquality{Behavior: testModel.Predicate.Observe.Behavior, Predicate: predicate})
		equalityPredicates = append(equalityPredicates, predicate)
	}
	leftPass := compilerPredicate(tool, observationIRDigest, "left-pass", "true")
	rightPass := compilerPredicate(tool, observationIRDigest, "right-pass", "true")
	observationClaim := semanticir.NewTestObservationCompletenessClaim(observationContext, observationScope, equalityPredicates, leftPass, rightPass)
	_ = replayableProof(tool, semanticir.SolverUNSAT, observationClaim) // exercises canonical aggregate claim construction; structural composition is authoritative
	suitePredicate := semanticir.StaticTestPredicate(task.Tests, testProv)
	suitePredicateDigest := mustDigest(suitePredicate)
	constructs := testConstructs
	task.TestSuite = &semanticir.TestSuiteModel{
		SourceArtifacts: suiteSources, SourceModels: []semanticir.ArtifactModelDigest{{ArtifactID: testsArtifact.ID, Digest: modelDigest}},
		Predicate: suitePredicate, Verifier: tool,
		Execution: suiteExecution, VectorCount: 9, CrossCheck: &semanticir.TestCrossCheckEvidence{Vectors: vectorResults, AcceptedVectorCount: 1, AcceptedVectorsDigest: acceptedDigest, VectorEvidenceDigest: vectorDigest, Repetitions: 2, RunDigests: []string{vectorDigest, vectorDigest}, Provenance: testProv},
		ObservationCompleteness: semanticir.TestObservationCompleteness{ProjectionComponents: projectionComponents, SourceModels: []semanticir.ArtifactModelDigest{{ArtifactID: testsArtifact.ID, Digest: modelDigest}}, StaticPredicateDigest: suitePredicateDigest, IRKind: semanticir.CompilerIRCPythonBytecode, Constructs: constructs, ObservationIRDigest: observationIRDigest, HarnessDigest: observationHarnessDigest, Result: semanticir.ProofProved, ProofDigest: observationIRDigest, Provenance: testProv},
		Coverage:                semanticir.TranslationCoverage{Status: semanticir.TranslationComplete, TotalConstructs: 4, TranslatedConstructs: 4, Provenance: testProv}, Evidence: []semanticir.Provenance{testProv, environmentProv, configurationProv},
	}
	task.TestSuite.RunnerComposition, err = testir.ComposeRunner([]semanticir.ArtifactModel{testModel}, suiteSources, tool, suiteExecution, configurationProv)
	if err != nil {
		t.Fatal(err)
	}
	task.SpecIRDigest, err = semanticir.CanonicalSpecIRDigest(task)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics := task.Validate(); semanticir.HasErrors(diagnostics) {
		t.Fatalf("Validate: %+v", diagnostics)
	}
	if task.Requirements[0].Effects[0].Kind != semanticir.EffectWrite || task.CodeCases[0].OutcomeIDs[0] != zero.ID || task.Tests[0].Predicate.Kind != semanticir.PredicateOutcomeIn {
		t.Fatal("typed requirements/effects/code/tests were not preserved")
	}
	unsupportedProv := testProv
	unsupportedProv.Translation = semanticir.TranslationUnsupported
	advisoryCoverage := semanticir.TranslationCoverage{Status: semanticir.TranslationPartial, TotalConstructs: 3, TranslatedConstructs: 2, Unsupported: []semanticir.UnsupportedConstruct{{Kind: "fixture", Reason: "advisory source extraction is not executable suite truth", Provenance: unsupportedProv}}, Provenance: testProv}
	for index := range task.Artifacts {
		if task.Artifacts[index].Artifact.ID == testsArtifact.ID {
			task.Artifacts[index].Coverage = advisoryCoverage
			modelDigest = mustDigest(task.Artifacts[index])
		}
	}
	for index := range task.Coverage {
		if task.Coverage[index].Provenance.ArtifactID == testsArtifact.ID {
			task.Coverage[index] = advisoryCoverage
		}
	}
	task.TestSuite.SourceModels[0].Digest = modelDigest
	if diagnostics := task.Validate(); !semanticir.HasErrors(diagnostics) {
		t.Fatal("partial independent test translation was accepted despite static TestSuite authority")
	}
	witness := semanticir.Counterexample{ID: "witness", Obligation: semanticir.ObligationTestsSound, Choices: []semanticir.BehaviorChoice{{Behavior: semanticir.BehaviorRef{OperationID: "choose", Conditions: semanticir.Assignment{"mode": "zero"}, Inputs: map[string]semanticir.Literal{"mode": {Type: semanticir.TypeString, String: "zero"}}, Provenance: testProv}, OutcomeID: zero.ID}, {Behavior: semanticir.BehaviorRef{OperationID: "choose", Conditions: semanticir.Assignment{"mode": "one"}, Inputs: map[string]semanticir.Literal{"mode": {Type: semanticir.TypeString, String: "one"}}, Provenance: testProv}, OutcomeID: one.ID}}, Provenance: testProv}
	if diagnostics := semanticir.ValidateCounterexample(task, witness); semanticir.HasErrors(diagnostics) {
		t.Fatalf("ValidateCounterexample: %+v", diagnostics)
	}
}

func scopeClosureFor(artifact semanticir.ArtifactRef, provenance semanticir.Provenance, tool semanticir.ToolRef, operation semanticir.Operation) *semanticir.ScopeClosureEvidence {
	irDigest := semanticir.DigestBytes([]byte("scope-ir"))
	changed := semanticir.ChangedSourceRange{ArtifactID: artifact.ID, Path: artifact.Path, StartLine: 1, EndLine: 1, SliceDigest: artifact.Digest, Provenance: provenance}
	declaration := semanticir.CompilerDeclaration{ID: "decl-choose", QualifiedName: operation.ID, Artifact: artifact, Location: provenance.Location, CompilerNodeIDs: []string{"decl-node"}, Changed: true, Provenance: provenance}
	record := &semanticir.ScopeClosureEvidence{SourceArtifacts: []semanticir.ArtifactRef{artifact}, WorkspaceTreeDigest: semanticir.DigestBytes([]byte("workspace")), Compiler: tool, Prover: tool, CompilerIRDigest: irDigest, ChangedRanges: []semanticir.ChangedSourceRange{changed}, Declarations: []semanticir.CompilerDeclaration{declaration}, ImpactedDeclarationIDs: []string{declaration.ID}, OperationOwners: []semanticir.OperationOwner{{OperationID: operation.ID, DeclarationID: declaration.ID}}, Completeness: semanticir.ProofProved, Complete: true, Provenance: provenance}
	graphDigest, _ := semanticir.ScopeClosureGraphDigest(*record)
	sourceDigest, _ := semanticir.Digest(record.SourceArtifacts)
	context := semanticir.CompilerProofContext{SourceDigest: sourceDigest, WorkspaceTreeDigest: record.WorkspaceTreeDigest, EmittedIRDigest: irDigest, HarnessDigest: graphDigest, Compiler: tool}
	scope := compilerPredicate(tool, irDigest, "omitted-scope", "false")
	claim := semanticir.NewProofClaim(semanticir.ClaimScopeClosure, context, scope, nil, nil)
	record.CompletenessProof = replayableProof(tool, semanticir.SolverUNSAT, claim)
	return record
}

func compilerEvidenceFor(artifact semanticir.ArtifactRef, provenance semanticir.Provenance, tool semanticir.ToolRef, operation semanticir.Operation, domain semanticir.Domain, cases []semanticir.BehaviorCase) semanticir.CompilerEvidence {
	emittedIR := []byte("bytecode")
	emittedDigest := semanticir.DigestBytes(emittedIR)
	harnessDigest := semanticir.DigestBytes([]byte("harness"))
	workspaceDigest := semanticir.DigestBytes([]byte("workspace"))
	context := semanticir.CompilerProofContext{SourceDigest: artifact.Digest, WorkspaceTreeDigest: workspaceDigest, EmittedIRDigest: emittedDigest, HarnessDigest: harnessDigest, Compiler: tool}
	scope := compilerPredicate(tool, emittedDigest, "scope", "true")
	labels := make([]semanticir.LabelPathEvidence, 0, len(domain.Values))
	for _, value := range domain.Values {
		membership := compilerPredicate(tool, emittedDigest, "membership-"+value.ID, "(= ray_mode \""+value.ID+"\")")
		claim := semanticir.NewProofClaim(semanticir.ClaimReachability, context, scope, []semanticir.CompilerPredicate{membership}, nil)
		proof := replayableProof(tool, semanticir.SolverSAT, claim)
		witness := semanticir.Literal{Type: semanticir.TypeString, String: value.ID}
		labels = append(labels, semanticir.LabelPathEvidence{
			ValueID: value.ID, PredicateDigest: membership.FormulaDigest, MembershipPredicate: membership, CompilerNodeIDs: membership.CompilerNodeIDs,
			Reachability: semanticir.ProofProved, ReachabilityProofDigest: proof.QueryDigest, ReachabilityProof: proof, ConcreteWitness: &witness, WitnessDigest: mustDigest(witness), Provenance: provenance,
		})
	}
	memberships := make([]semanticir.CompilerPredicate, 0, len(labels))
	for _, label := range labels {
		memberships = append(memberships, label.MembershipPredicate)
	}
	totalityProof := replayableProof(tool, semanticir.SolverUNSAT, semanticir.NewProofClaim(semanticir.ClaimTotality, context, scope, memberships, nil))
	disjointnessProof := replayableProof(tool, semanticir.SolverUNSAT, semanticir.NewProofClaim(semanticir.ClaimDisjointness, context, scope, memberships, nil))
	partition := semanticir.DomainPartitionEvidence{
		OperationID: operation.ID, DomainID: domain.ID, ScopePredicateDigest: scope.FormulaDigest, ScopePredicate: scope, Labels: labels,
		Totality: semanticir.ProofProved, TotalityProofDigest: totalityProof.QueryDigest, TotalityProof: totalityProof,
		Disjointness: semanticir.ProofProved, DisjointnessProofDigest: disjointnessProof.QueryDigest, DisjointnessProof: disjointnessProof, Provenance: provenance,
	}
	var behaviorProofs []semanticir.BehaviorRealizationEvidence
	for _, behaviorCase := range cases {
		predicateDigest := ""
		for _, label := range labels {
			if label.ValueID == behaviorCase.Conditions[domain.ID] {
				predicateDigest = label.PredicateDigest
			}
		}
		var claimMemberships []semanticir.CompilerPredicate
		for _, label := range labels {
			if label.PredicateDigest == predicateDigest {
				claimMemberships = append(claimMemberships, label.MembershipPredicate)
			}
		}
		var outcomePredicates []semanticir.CompilerOutcomePredicate
		for _, outcomeID := range behaviorCase.OutcomeIDs {
			predicate := compilerPredicate(tool, emittedDigest, "outcome-"+outcomeID, "(= ray_outcome \""+outcomeID+"\")")
			outcomePredicates = append(outcomePredicates, semanticir.CompilerOutcomePredicate{OutcomeID: outcomeID, Predicate: predicate})
		}
		proof := replayableProof(tool, semanticir.SolverUNSAT, semanticir.NewProofClaim(semanticir.ClaimRealization, context, scope, claimMemberships, outcomePredicates))
		behaviorProofs = append(behaviorProofs, semanticir.BehaviorRealizationEvidence{BehaviorCaseID: behaviorCase.ID, Behavior: semanticir.BehaviorRef{OperationID: behaviorCase.OperationID, Conditions: behaviorCase.Conditions, Inputs: behaviorCase.Inputs, Provenance: provenance}, OutcomeIDs: behaviorCase.OutcomeIDs, CategoryPredicateDigests: []string{predicateDigest}, RealizationProof: proof, Provenance: provenance})
	}
	var declared []semanticir.CompilerOutcomePredicate
	var complementID string
	for _, outcomeID := range operation.OutcomeIDs {
		predicate := compilerPredicate(tool, emittedDigest, "closure-"+outcomeID, "(= ray_outcome \""+outcomeID+"\")")
		if outcomeID == semanticir.OtherOutcome(operation.ID, provenance).ID {
			complementID = outcomeID
		} else {
			declared = append(declared, semanticir.CompilerOutcomePredicate{OutcomeID: outcomeID, Predicate: predicate})
		}
	}
	complementPredicate := compilerPredicate(tool, emittedDigest, "closure-other", "(and (not (= ray_outcome \""+operation.OutcomeIDs[0]+"\")) (not (= ray_outcome \""+operation.OutcomeIDs[1]+"\")))")
	complement := semanticir.OutcomeComplement{ID: complementID, Kind: semanticir.OutcomeComplementEffects, Description: "all other exact traces", Predicate: semanticir.CompilerOutcomePredicate{OutcomeID: complementID, Predicate: complementPredicate}}
	closureMemberships := []semanticir.CompilerPredicate{declared[0].Predicate, declared[1].Predicate, complementPredicate}
	closureTotal := replayableProof(tool, semanticir.SolverUNSAT, semanticir.NewProofClaim(semanticir.ClaimTotality, context, scope, closureMemberships, nil))
	closureDisjoint := replayableProof(tool, semanticir.SolverUNSAT, semanticir.NewProofClaim(semanticir.ClaimDisjointness, context, scope, closureMemberships, nil))
	closure := semanticir.OutcomeClosureEvidence{OperationID: operation.ID, BoundaryDigest: semanticir.DigestBytes([]byte("terminal+effects")), Declared: declared, Complements: []semanticir.OutcomeComplement{complement}, Totality: semanticir.ProofProved, TotalityProof: closureTotal, Disjointness: semanticir.ProofProved, DisjointnessProof: closureDisjoint, Provenance: provenance}
	environmentVariables := []semanticir.EnvironmentVariable{{Name: "PATH", Value: "/tools"}}
	derivationStep := semanticir.ProbeStep{ID: "compile-ir", Kind: semanticir.ProbeStepRun, Tool: tool, Argv: []string{"--emit-ir", artifact.Path}, StdinDigest: semanticir.DigestBytes(nil), WorkingDirectory: "/workspace", Environment: environmentVariables, EnvironmentDigest: mustDigest(environmentVariables), ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 1000, ExpectedStdoutDigest: emittedDigest, ExpectedStderrDigest: semanticir.DigestBytes(nil), ExpectedSignalDigest: semanticir.DigestBytes(nil), SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone}, Provenance: provenance}
	compilerNodeID := "compiler-root"
	numeric := semanticir.CompilerNumericSemantics{ID: "python-int", Kind: semanticir.CompilerNumericUnbounded, Signed: true, Overflow: semanticir.CompilerOverflowUnbounded, Range: semanticir.CompilerRangeAll}
	zeroLiteral := semanticir.Literal{Type: semanticir.TypeString, String: "zero"}
	integerZero := semanticir.Literal{Type: semanticir.TypeInteger, Integer: 0}
	integerOne := semanticir.Literal{Type: semanticir.TypeInteger, Integer: 1}
	graphNodes := []semanticir.CompilerSemanticNode{
		{ID: "input-mode", Kind: semanticir.CompilerNodeInput, Type: semanticir.TypeString, InputName: "mode", CompilerNodeIDs: []string{compilerNodeID}, Provenance: provenance},
		{ID: "label-zero", Kind: semanticir.CompilerNodeConstant, Type: semanticir.TypeString, Literal: &zeroLiteral, CompilerNodeIDs: []string{compilerNodeID}, Provenance: provenance},
		{ID: "is-zero", Kind: semanticir.CompilerNodeEQ, Type: semanticir.TypeBool, Operands: []string{"input-mode", "label-zero"}, CompilerNodeIDs: []string{compilerNodeID}, Provenance: provenance},
		{ID: "integer-zero", Kind: semanticir.CompilerNodeConstant, Type: semanticir.TypeInteger, NumericID: numeric.ID, Literal: &integerZero, CompilerNodeIDs: []string{compilerNodeID}, Provenance: provenance},
		{ID: "write-result", Kind: semanticir.CompilerNodeEffect, Type: semanticir.TypeUnit, EffectKind: semanticir.EffectWrite, EffectTarget: "result", CompilerNodeIDs: []string{compilerNodeID}, Provenance: provenance},
		{ID: "return-zero", Kind: semanticir.CompilerNodeReturn, Type: semanticir.TypeInteger, NumericID: numeric.ID, Operands: []string{"integer-zero"}, EffectNodeIDs: []string{"write-result"}, CompilerNodeIDs: []string{compilerNodeID}, Provenance: provenance},
		{ID: "integer-one", Kind: semanticir.CompilerNodeConstant, Type: semanticir.TypeInteger, NumericID: numeric.ID, Literal: &integerOne, CompilerNodeIDs: []string{compilerNodeID}, Provenance: provenance},
		{ID: "return-one", Kind: semanticir.CompilerNodeReturn, Type: semanticir.TypeInteger, NumericID: numeric.ID, Operands: []string{"integer-one"}, CompilerNodeIDs: []string{compilerNodeID}, Provenance: provenance},
	}
	graphBlocks := []semanticir.CompilerSemanticBlock{
		{ID: "entry", NodeIDs: []string{"input-mode", "label-zero", "is-zero"}, CompilerNodeIDs: []string{compilerNodeID}, Provenance: provenance},
		{ID: "zero", NodeIDs: []string{"integer-zero", "write-result", "return-zero"}, CompilerNodeIDs: []string{compilerNodeID}, Provenance: provenance},
		{ID: "one", NodeIDs: []string{"integer-one", "return-one"}, CompilerNodeIDs: []string{compilerNodeID}, Provenance: provenance},
	}
	graphEdges := []semanticir.CompilerControlEdge{
		{ID: "zero-edge", FromBlockID: "entry", ToBlockID: "zero", GuardNodeID: "is-zero", GuardValue: true, CompilerNodeIDs: []string{compilerNodeID}, Provenance: provenance},
		{ID: "one-edge", FromBlockID: "entry", ToBlockID: "one", GuardNodeID: "is-zero", GuardValue: false, CompilerNodeIDs: []string{compilerNodeID}, Provenance: provenance},
	}
	semanticNodeIDs := []string{"input-mode", "label-zero", "is-zero", "integer-zero", "write-result", "return-zero", "integer-one", "return-one"}
	constructs := []semanticir.CompilerConstructBinding{{ID: compilerNodeID, Kind: semanticir.CompilerConstructControl, Opcode: "branch", SemanticNodeIDs: semanticNodeIDs, BlockIDs: []string{"entry", "zero", "one"}, EdgeIDs: []string{"zero-edge", "one-edge"}, Provenance: provenance}}
	graph := &semanticir.CompilerSemanticGraph{SourceDigest: artifact.Digest, WorkspaceTreeDigest: workspaceDigest, Tool: tool, IRKind: semanticir.CompilerIRCPythonBytecode, IR: emittedIR, IRDigest: emittedDigest, Environment: environmentVariables, EnvironmentDigest: mustDigest(environmentVariables), DerivationSteps: []semanticir.ProbeStep{derivationStep}, Numeric: []semanticir.CompilerNumericSemantics{numeric}, Nodes: graphNodes, Blocks: graphBlocks, Edges: graphEdges, Operations: []semanticir.CompilerOperationGraph{{OperationID: operation.ID, EntryBlockID: "entry", Inputs: []semanticir.CompilerInputNode{{InputName: "mode", NodeID: "input-mode"}}, TerminalNodeIDs: []string{"return-zero", "return-one"}, Provenance: provenance}}, Constructs: constructs, Provenance: provenance}
	decoderOutput, _ := semanticir.CanonicalCompilerDecoderOutput(graph)
	decoderStep := semanticir.ProbeStep{ID: "decode-ir", Kind: semanticir.ProbeStepRun, Tool: tool, Argv: []string{"--decode-ir", artifact.Path}, StdinDigest: semanticir.DigestBytes(nil), WorkingDirectory: "/workspace", Environment: environmentVariables, EnvironmentDigest: mustDigest(environmentVariables), ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 1000, ExpectedStdoutDigest: semanticir.DigestBytes(decoderOutput), ExpectedStderrDigest: semanticir.DigestBytes(nil), ExpectedSignalDigest: semanticir.DigestBytes(nil), SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone}, Provenance: provenance}
	graph.DecoderSteps = []semanticir.ProbeStep{decoderStep}
	graph.DecoderOutput = decoderOutput
	graph.DecoderOutputDigest = semanticir.DigestBytes(decoderOutput)
	graphDigest, _ := semanticir.CompilerSemanticGraphDigest(graph)
	return semanticir.CompilerEvidence{
		ID: "compiler-evidence", Method: semanticir.CompilerEvidenceModelChecker, FormulaDerivationDigest: graphDigest, Tool: tool, Prover: tool, SourceDigest: artifact.Digest, WorkspaceTreeDigest: workspaceDigest, Argv: []string{tool.Path, artifact.Path},
		EnvironmentDigest: mustDigest(environmentVariables), IRKind: semanticir.CompilerIRCPythonBytecode, EmittedIRDigest: emittedDigest, HarnessDigest: harnessDigest, SemanticGraph: graph,
		TotalConstructs: 1, TranslatedConstructs: 1, OperationScopes: []semanticir.OperationScopeEvidence{{OperationID: operation.ID, ScopePredicateDigest: scope.FormulaDigest, ScopePredicate: scope, Provenance: provenance}}, Partitions: []semanticir.DomainPartitionEvidence{partition}, BehaviorProofs: behaviorProofs, OutcomeClosures: []semanticir.OutcomeClosureEvidence{closure}, Provenance: provenance,
	}
}

func compilerPredicate(tool semanticir.ToolRef, irDigest, label, expression string) semanticir.CompilerPredicate {
	declarations := []byte("(declare-const ray_mode String)\n(declare-const ray_outcome String)")
	formula := []byte(expression)
	return semanticir.CompilerPredicate{Logic: semanticir.ProofLogicSMTLIB2, Declarations: declarations, DeclarationsDigest: semanticir.DigestBytes(declarations), Formula: formula, FormulaDigest: semanticir.DigestBytes(formula), Tool: tool, IRDigest: irDigest, CompilerNodeIDs: []string{"node-" + label}}
}

func replayableProof(tool semanticir.ToolRef, result semanticir.SolverResult, claim semanticir.ProofClaim) semanticir.ReplayableProof {
	query, err := semanticir.CanonicalProofQuery(claim)
	if err != nil {
		panic(err)
	}
	output := []byte(string(result) + "\n")
	environment := []semanticir.EnvironmentVariable{{Name: "PATH", Value: "/tools"}}
	return semanticir.ReplayableProof{Claim: claim, Logic: semanticir.ProofLogicSMTLIB2, Query: query, QueryDigest: semanticir.DigestBytes(query), Prover: tool, Argv: []string{"--in"}, WorkingDirectory: "/workspace", Environment: environment, EnvironmentDigest: mustDigest(environment), ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 1000, SolverOutput: output, SolverOutputDigest: semanticir.DigestBytes(output), Result: result, SubjectDigests: semanticir.ProofClaimSubjectDigests(claim)}
}

func mustDigest(value any) string {
	digest, err := semanticir.Digest(value)
	if err != nil {
		panic(err)
	}
	return digest
}

func workspaceEvidence(id string, state semanticir.WorkspaceState, passed bool, provenance semanticir.Provenance, environment []byte, tool semanticir.ToolRef) semanticir.WorkspaceCommand {
	variables := []semanticir.EnvironmentVariable{{Name: "RAY_ENV", Value: string(environment)}}
	return semanticir.WorkspaceCommand{ID: id, WorkspaceID: id, State: state, TreeDigest: semanticir.DigestBytes([]byte("tree-" + id)), Command: "pytest", WorkingDirectory: "/workspace", Environment: variables, EnvironmentDigest: mustDigest(variables), ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 1000, PassSignal: semanticir.PassSignal{Kind: semanticir.PassSignalExitCode, Expected: "0", Provenance: provenance}, ExpectedPass: passed, ObservedPass: passed, ExitCode: 0, StdoutDigest: semanticir.DigestBytes(nil), StderrDigest: semanticir.DigestBytes(nil), SignalValueDigest: semanticir.DigestBytes([]byte("0")), Tools: []semanticir.ToolRef{tool}, Provenance: provenance}
}

func TestSemanticIRCanonicalDeterministic(t *testing.T) {
	left := map[string]any{"z": []string{"x", "y"}, "a": map[string]string{"b": "2", "a": "1"}}
	right := map[string]any{}
	right["a"] = map[string]string{"a": "1", "b": "2"}
	right["z"] = []string{"x", "y"}
	leftJSON, err := semanticir.CanonicalJSON(left)
	if err != nil {
		t.Fatal(err)
	}
	rightJSON, err := semanticir.CanonicalJSON(right)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(leftJSON, rightJSON) {
		t.Fatalf("canonical encodings differ:\n%s\n%s", leftJSON, rightJSON)
	}
	leftDigest, _ := semanticir.Digest(left)
	rightDigest, _ := semanticir.Digest(right)
	if leftDigest != rightDigest || !semanticir.ValidDigest(leftDigest) {
		t.Fatalf("digests differ or invalid: %q %q", leftDigest, rightDigest)
	}
}

func TestSemanticIRExhaustiveRejectsHarnessReportedEffects(t *testing.T) {
	trace := semanticir.RawOutcomeTrace{
		Kind:    semanticir.OutcomeSuccess,
		Effects: []semanticir.RawEffectTrace{{Kind: semanticir.EffectWrite, Target: "state", Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 1}}},
	}
	if err := semanticir.ValidateRawOutcomeTrace(trace); err != nil {
		t.Fatalf("raw effect protocol should be structurally decodable: %v", err)
	}
	if err := semanticir.ValidateExhaustiveRawOutcomeTrace(trace); err == nil {
		t.Fatal("harness-reported effect was accepted as exhaustive code-semantics evidence")
	}
}

func TestSemanticIRCanonicalProofQueryRejectsAssertFalseSubstitution(t *testing.T) {
	artifact := frozenRef("proof-source", semanticir.ArtifactCode, "proof.py", []byte("pass\n"))
	provenance := semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	tool := semanticir.ToolRef{Name: "compiler", Path: "/tools/compiler", Digest: semanticir.DigestBytes([]byte("compiler")), Version: "1"}
	irDigest := semanticir.DigestBytes([]byte("ir"))
	context := semanticir.CompilerProofContext{SourceDigest: artifact.Digest, WorkspaceTreeDigest: semanticir.DigestBytes([]byte("tree")), EmittedIRDigest: irDigest, HarnessDigest: semanticir.DigestBytes([]byte("harness")), Compiler: tool}
	scope := compilerPredicate(tool, irDigest, "scope", "true")
	membership := compilerPredicate(tool, irDigest, "member", "(= ray_mode \"only\")")

	singleton := semanticir.NewProofClaim(semanticir.ClaimDisjointness, context, scope, []semanticir.CompilerPredicate{membership}, nil)
	singletonQuery, err := semanticir.CanonicalProofQuery(singleton)
	if err != nil || !strings.Contains(string(singletonQuery), "(assert false)") {
		t.Fatalf("singleton disjointness is not vacuously false: query=%q err=%v", singletonQuery, err)
	}

	outcomeID := semanticir.OutcomeID(semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeUnit}})
	outcome := semanticir.CompilerOutcomePredicate{OutcomeID: outcomeID, Predicate: compilerPredicate(tool, irDigest, "outcome", "(= ray_outcome \""+outcomeID+"\")")}
	zeroArgument := semanticir.NewProofClaim(semanticir.ClaimRealization, context, scope, nil, []semanticir.CompilerOutcomePredicate{outcome})
	good := replayableProof(tool, semanticir.SolverUNSAT, zeroArgument)
	if diagnostics := semanticir.ValidateReplayableProof(good, semanticir.SolverUNSAT, provenance); semanticir.HasErrors(diagnostics) {
		t.Fatalf("canonical zero-domain realization proof rejected: %+v", diagnostics)
	}

	tampered := good
	tampered.Query = []byte("(set-logic ALL)\n(assert false)\n(check-sat)\n")
	tampered.QueryDigest = semanticir.DigestBytes(tampered.Query)
	if diagnostics := semanticir.ValidateReplayableProof(tampered, semanticir.SolverUNSAT, provenance); !semanticir.HasErrors(diagnostics) {
		t.Fatal("unrelated assert-false replay query was accepted")
	}
	tampered = good
	tampered.Environment[0].Value = "/ambient"
	if diagnostics := semanticir.ValidateReplayableProof(tampered, semanticir.SolverUNSAT, provenance); !semanticir.HasErrors(diagnostics) {
		t.Fatal("replay proof accepted environment bytes that differ from its digest")
	}
}

func TestSemanticIRCompositeValuesAndEffectIdentity(t *testing.T) {
	sequence := semanticir.Literal{Type: semanticir.TypeSequence, Elements: &semanticir.LiteralElements{Values: []semanticir.Literal{{Type: semanticir.TypeInteger, Integer: 1}, {Type: semanticir.TypeOptional, Null: true}}}}
	tuple := semanticir.Literal{Type: semanticir.TypeTuple, Elements: &semanticir.LiteralElements{Values: []semanticir.Literal{{Type: semanticir.TypeString, String: "x"}, {Type: semanticir.TypeBool, Bool: true}}}}
	record := semanticir.Literal{Type: semanticir.TypeRecord, Fields: &semanticir.LiteralFields{Values: map[string]semanticir.Literal{"items": sequence, "pair": tuple}}}
	optional := semanticir.Literal{Type: semanticir.TypeOptional, Elements: &semanticir.LiteralElements{Values: []semanticir.Literal{record}}}
	for _, literal := range []semanticir.Literal{sequence, tuple, record, optional, {Type: semanticir.TypeOptional, Null: true}} {
		if err := semanticir.ValidateLiteral(literal); err != nil {
			t.Fatalf("valid composite literal rejected: %v", err)
		}
	}
	if err := semanticir.ValidateLiteral(semanticir.Literal{Type: semanticir.TypeOptional}); err == nil {
		t.Fatal("malformed optional literal accepted")
	}

	artifact := frozenRef("effect-source", semanticir.ArtifactSpec, "spec.md", []byte("effect"))
	provenance := semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	effectValue := func(value int64) *semanticir.Expression {
		literal := semanticir.Literal{Type: semanticir.TypeInteger, Integer: value}
		return &semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &literal, Provenance: provenance}
	}
	left := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeUnit}, Effects: []semanticir.Effect{{ID: "write", Kind: semanticir.EffectWrite, Target: "state", Value: effectValue(1), Provenance: provenance}}, Provenance: provenance}
	right := left
	right.Effects = []semanticir.Effect{{ID: "write", Kind: semanticir.EffectWrite, Target: "state", Value: effectValue(2), Provenance: provenance}}
	if semanticir.OutcomeID(left) == semanticir.OutcomeID(right) {
		t.Fatal("effect values did not distinguish observable outcome identities")
	}

	recordExpression := semanticir.Expression{Kind: semanticir.ExprRecord, Type: semanticir.TypeRecord, Operands: []semanticir.Expression{
		{Kind: semanticir.ExprLiteral, Type: semanticir.TypeString, Literal: &semanticir.Literal{Type: semanticir.TypeString, String: "field"}, Provenance: provenance},
		{Kind: semanticir.ExprSequence, Type: semanticir.TypeSequence, Operands: []semanticir.Expression{{Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 1}, Provenance: provenance}}, Provenance: provenance},
	}, Provenance: provenance}
	statements := []semanticir.Statement{
		{Kind: semanticir.StmtAssign, Target: "items", Value: &recordExpression, Provenance: provenance},
		{Kind: semanticir.StmtLoop, Target: "item", Iterator: &recordExpression.Operands[1], Then: []semanticir.Statement{{Kind: semanticir.StmtContinue, Provenance: provenance}}, Provenance: provenance},
		{Kind: semanticir.StmtTry, Then: []semanticir.Statement{{Kind: semanticir.StmtRaise, ExceptionType: "ValueError", Provenance: provenance}}, Catches: []semanticir.CatchClause{{ExceptionType: "ValueError", Body: []semanticir.Statement{{Kind: semanticir.StmtReturn, Value: effectValue(1), Provenance: provenance}}, Provenance: provenance}}, Provenance: provenance},
	}
	if _, err := semanticir.CanonicalJSON(statements); err != nil {
		t.Fatalf("extended finite nodes are not canonically encodable: %v", err)
	}
}

func TestSemanticIRTestVectorDigestsDeterministic(t *testing.T) {
	left := semanticir.TestVectorResult{Accepted: true, Choices: []semanticir.BehaviorChoice{
		{Behavior: semanticir.BehaviorRef{OperationID: "g", Conditions: semanticir.Assignment{"y": "1"}, Inputs: map[string]semanticir.Literal{"y": {Type: semanticir.TypeInteger, Integer: 1}}}, OutcomeID: "B"},
		{Behavior: semanticir.BehaviorRef{OperationID: "f", Conditions: semanticir.Assignment{"x": "0"}, Inputs: map[string]semanticir.Literal{"x": {Type: semanticir.TypeInteger, Integer: 0}}}, OutcomeID: "A"},
	}}
	right := semanticir.TestVectorResult{Accepted: true, Choices: []semanticir.BehaviorChoice{left.Choices[1], left.Choices[0]}}
	leftAll, leftAccepted, err := semanticir.TestVectorDigests([]semanticir.TestVectorResult{left})
	if err != nil {
		t.Fatal(err)
	}
	rightAll, rightAccepted, err := semanticir.TestVectorDigests([]semanticir.TestVectorResult{right})
	if err != nil {
		t.Fatal(err)
	}
	if leftAll != rightAll || leftAccepted != rightAccepted {
		t.Fatal("test vector digests depend on scheduling/order")
	}
}

func TestSemanticIRConcreteBehaviorPointsDoNotCollapseCategory(t *testing.T) {
	spec := frozenRef("point-spec", semanticir.ArtifactSpec, "spec.md", []byte("points"))
	code := frozenRef("point-code", semanticir.ArtifactCode, "point.py", []byte("code"))
	testArtifact := frozenRef("point-tests", semanticir.ArtifactTests, "test_point.py", []byte("tests"))
	specProv := semanticir.NewProvenance(spec, semanticir.SourceLocation{Path: spec.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	testProv := semanticir.NewProvenance(testArtifact, semanticir.SourceLocation{Path: testArtifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	input := semanticir.Expression{Kind: semanticir.ExprVariable, Type: semanticir.TypeInteger, Name: "x", Provenance: specProv}
	zeroLiteral := semanticir.Literal{Type: semanticir.TypeInteger, Integer: 0}
	membership := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpGE, Operands: []semanticir.Expression{input, {Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &zeroLiteral, Provenance: specProv}}, Provenance: specProv}
	domain := semanticir.Domain{ID: "sign", Type: semanticir.TypeString, Values: []semanticir.DomainValue{{ID: "nonnegative", Groundings: []semanticir.GroundingAxiom{{OperationID: "f", Kind: semanticir.GroundingMembership, Membership: &membership, ConcreteWitness: map[string]semanticir.Literal{"x": zeroLiteral}, Provenance: specProv}}, Provenance: specProv}}, Provenance: specProv}
	lower := semanticir.Literal{Type: semanticir.TypeInteger, Integer: 0}
	upper := semanticir.Literal{Type: semanticir.TypeInteger, Integer: 1}
	operation := semanticir.Operation{ID: "f", Kind: semanticir.OperationFunction, Inputs: []semanticir.Variable{{Name: "x", Type: semanticir.TypeInteger, Universe: []semanticir.Literal{lower, upper}, Provenance: specProv}}, DomainIDs: []string{"sign"}, OutcomeIDs: []string{"A", "other:f"}, Provenance: specProv}
	graph := &semanticir.CompilerSemanticGraph{
		Numeric:    []semanticir.CompilerNumericSemantics{{ID: "math", Kind: semanticir.CompilerNumericUnbounded, Signed: true, Overflow: semanticir.CompilerOverflowUnbounded, Range: semanticir.CompilerRangeBounded, LowerBound: &lower, UpperBound: &upper}},
		Nodes:      []semanticir.CompilerSemanticNode{{ID: "x", Kind: semanticir.CompilerNodeInput, Type: semanticir.TypeInteger, NumericID: "math", InputName: "x"}},
		Operations: []semanticir.CompilerOperationGraph{{OperationID: "f", Inputs: []semanticir.CompilerInputNode{{InputName: "x", NodeID: "x"}}}},
	}
	conditions := semanticir.Assignment{"sign": "nonnegative"}
	task := &semanticir.Task{Domains: []semanticir.Domain{domain}, Operations: []semanticir.Operation{operation}, Requirements: []semanticir.RequirementCase{{ID: "req", OperationID: "f", Conditions: conditions, Provenance: specProv}}, Artifacts: []semanticir.ArtifactModel{{Artifact: code, Kind: semanticir.ArtifactCode, CompilerEvidence: []semanticir.CompilerEvidence{{SemanticGraph: graph}}}}}
	points, diagnostics := semanticir.ConcreteBehaviorPoints(task)
	if semanticir.HasErrors(diagnostics) || len(points) != 2 || semanticir.BehaviorRefKey(points[0]) == semanticir.BehaviorRefKey(points[1]) {
		t.Fatalf("finite non-singleton category was not expanded into distinct points: points=%+v diagnostics=%+v", points, diagnostics)
	}
	concrete := []map[string]semanticir.Literal{{"x": lower}, {"x": upper}}
	concreteDigest, err := semanticir.TestConcreteInputsDigest(concrete)
	if err != nil {
		t.Fatal(err)
	}
	graphDigest, err := semanticir.CompilerSemanticGraphDigest(graph)
	if err != nil {
		t.Fatal(err)
	}
	category := semanticir.BehaviorRef{OperationID: "f", Conditions: conditions, Provenance: testProv}
	calledPoint := semanticir.BehaviorRef{OperationID: "f", Conditions: conditions, Inputs: map[string]semanticir.Literal{"x": lower}, Provenance: testProv}
	model := semanticir.ArtifactModel{Artifact: testArtifact, Kind: semanticir.ArtifactTests, TestProjection: &semanticir.TestObservationProjection{
		Dependencies:   []semanticir.TestBehaviorDependency{{ConstructID: "call-zero", Kind: semanticir.TestDependencyCall, Behavior: calledPoint, Inputs: calledPoint.Inputs, CompilerNodeIDs: []string{"call-zero"}, Provenance: testProv}},
		Quantification: []semanticir.TestQuantificationEvidence{{Behavior: category, Kind: semanticir.TestQuantificationFiniteExhaustive, ConcreteInputs: concrete, ConcreteInputsDigest: concreteDigest, CodeGraphDigest: graphDigest, Result: semanticir.ProofProved, Provenance: testProv}},
	}}
	if diagnostics := semanticir.ValidateTestObservationQuantification(task, model); semanticir.HasErrors(diagnostics) {
		t.Fatalf("finite point universe with one observed point was rejected: %+v", diagnostics)
	}
	model.TestProjection.Quantification[0].Kind = semanticir.TestQuantificationSingleton
	model.TestProjection.Quantification[0].ConcreteInputs = []map[string]semanticir.Literal{{"x": lower}}
	model.TestProjection.Quantification[0].ConcreteInputsDigest, _ = semanticir.TestConcreteInputsDigest(model.TestProjection.Quantification[0].ConcreteInputs)
	model.TestProjection.Quantification[0].CodeGraphDigest = ""
	if diagnostics := semanticir.ValidateTestObservationQuantification(task, model); !semanticir.HasErrors(diagnostics) {
		t.Fatal("x >= 0 category was unsoundly collapsed to the test call at x=0")
	}
}

func TestSemanticIRArtifactTranslationDigestNormalizesOnlyReplayTime(t *testing.T) {
	model := semanticir.ArtifactModel{ExhaustiveEvidence: []semanticir.ExhaustiveExecutionEvidence{{Runs: []semanticir.ExecutionRunEvidence{{StartedAtUTC: "2026-08-27T00:00:00Z"}}, Replay: semanticir.ExhaustiveReplayEvidence{CoreDigest: semanticir.DigestBytes([]byte("replay"))}}}}
	left, err := semanticir.ArtifactModelTranslationDigest(model)
	if err != nil {
		t.Fatal(err)
	}
	model.ExhaustiveEvidence[0].Runs[0].StartedAtUTC = "2026-08-27T01:00:00Z"
	model.ExhaustiveEvidence[0].Replay.CoreDigest = semanticir.DigestBytes([]byte("fresh-replay"))
	right, err := semanticir.ArtifactModelTranslationDigest(model)
	if err != nil || left != right {
		t.Fatalf("central replay/time perturbed translation digest: %q %q %v", left, right, err)
	}
	model.ExhaustiveEvidence[0].HarnessDigest = semanticir.DigestBytes([]byte("changed-translation"))
	changed, _ := semanticir.ArtifactModelTranslationDigest(model)
	if changed == left {
		t.Fatal("frontend-authored translation change was omitted from translation digest")
	}
}
