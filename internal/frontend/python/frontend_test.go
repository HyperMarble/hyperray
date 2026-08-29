package python

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/HyperMarble/hyperray/internal/executor"
	"github.com/HyperMarble/hyperray/internal/semanticir"
)

func TestBoundCPythonExhaustiveGroundedSingletons(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mainSource := []byte("def decide(value):\n    return value == 0\n")
	mainArtifact := testArtifact("code", semanticir.ArtifactCode, "main.py", mainSource)
	if err := os.WriteFile(filepath.Join(root, "main.py"), mainSource, 0o600); err != nil {
		t.Fatal(err)
	}
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	pythonPath, err = filepath.Abs(pythonPath)
	if err != nil {
		t.Fatal(err)
	}
	toolBytes, err := os.ReadFile(pythonPath)
	if err != nil {
		t.Fatal(err)
	}
	version, err := exec.Command(pythonPath, "--version").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	trueLiteral := semanticir.Literal{Type: semanticir.TypeBool, Bool: true}
	falseLiteral := semanticir.Literal{Type: semanticir.TypeBool}
	outcomeProvenance := semanticir.NewProvenance(mainArtifact, semanticir.SourceLocation{Path: mainArtifact.Path, StartLine: 1, StartColumn: 1, EndLine: 2, EndColumn: 22}, semanticir.TranslationTranslated)
	trueOutcome := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &trueLiteral, OperationID: "decide", Provenance: outcomeProvenance}
	trueOutcome.ID = semanticir.OutcomeID(trueOutcome)
	falseOutcome := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &falseLiteral, OperationID: "decide", Provenance: outcomeProvenance}
	falseOutcome.ID = semanticir.OutcomeID(falseOutcome)
	aValue := semanticir.Literal{Type: semanticir.TypeInteger, Integer: 0}
	bValue := semanticir.Literal{Type: semanticir.TypeInteger, Integer: 1}
	grounding := func(literal semanticir.Literal) []semanticir.GroundingAxiom {
		membership := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{
			{Kind: semanticir.ExprVariable, Type: semanticir.TypeInteger, Name: "value"},
			{Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &literal},
		}}
		return []semanticir.GroundingAxiom{{
			OperationID: "decide", Kind: semanticir.GroundingMembership, Membership: &membership,
			ConcreteWitness: map[string]semanticir.Literal{"value": literal},
		}}
	}
	domain := semanticir.Domain{ID: "decide.value", Type: semanticir.TypeInteger, Values: []semanticir.DomainValue{
		{ID: "zero-label", Groundings: grounding(aValue)},
		{ID: "one-label", Groundings: grounding(bValue)},
	}}
	operation := semanticir.Operation{ID: "decide", Kind: semanticir.OperationFunction, DomainIDs: []string{domain.ID}, OutcomeIDs: []string{trueOutcome.ID, falseOutcome.ID}, Inputs: []semanticir.Variable{{Name: "value", Type: semanticir.TypeInteger, DomainID: domain.ID}}}
	assignmentGroundings := []semanticir.AssignmentGrounding{
		{OperationID: operation.ID, Conditions: semanticir.Assignment{domain.ID: "zero-label"}, Inputs: map[string]semanticir.Literal{"value": aValue}},
		{OperationID: operation.ID, Conditions: semanticir.Assignment{domain.ID: "one-label"}, Inputs: map[string]semanticir.Literal{"value": bValue}},
	}
	for index := range assignmentGroundings {
		assignmentGroundings[index].ID = semanticir.AssignmentGroundingID(operation.ID, assignmentGroundings[index].Conditions)
	}
	entries := []semanticir.WorkspaceEntry{{Path: "main.py", Artifact: mainArtifact}}
	treeDigest, err := executor.WorkspaceDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	toolSum := sha256.Sum256(toolBytes)
	environment, environmentDigest := testPythonEnvironment(t)
	request := semanticir.FrontendRequest{
		TaskID: "bound-python", Artifact: mainArtifact, Language: semanticir.LanguagePython, Kind: semanticir.ArtifactCode,
		Source: mainSource, EntryPoints: []string{"decide"}, FiniteDomains: []semanticir.Domain{domain}, Groundings: assignmentGroundings, Operations: []semanticir.Operation{operation}, Outcomes: []semanticir.ObservableOutcome{trueOutcome, falseOutcome},
		Options:    testBoundOptions(t, map[string]string{"python.execution": "bound-cpython", "python.module": "main", "python.package_root": root}),
		Translator: semanticir.ToolRef{Name: "cpython", Path: pythonPath, Digest: "sha256:" + hex.EncodeToString(toolSum[:]), Version: strings.TrimSpace(string(version))},
		Prover:     testProver(t),
		Workspace: semanticir.WorkspaceRef{
			ID: "workspace", State: semanticir.WorkspaceSolutionNewTests, Root: root, WorkingDirectory: ".", TreeDigest: treeDigest,
			BuildCommand: "python verification", Environment: environment, EnvironmentDigest: environmentDigest, ClearEnvironment: true, KillProcessGroup: true, Entries: entries,
		},
		FocusArtifacts: []semanticir.ArtifactRef{mainArtifact},
		ChangedRanges:  []semanticir.ChangedSourceRange{testChangedRange(mainArtifact, mainSource)},
	}
	model, diagnostics := Translate(context.Background(), request)
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("Translate errors: %+v", diagnostics)
	}
	if model.Coverage.Status != semanticir.TranslationComplete || len(model.Cases) != 2 {
		t.Fatalf("model coverage/cases = %+v / %+v", model.Coverage, model.Cases)
	}
	if !reflect.DeepEqual(model.ExhaustiveEvidence[0].Replay, semanticir.ExhaustiveReplayEvidence{}) {
		t.Fatal("frontend supplied caller-authored exhaustive replay evidence")
	}
	replayed := executor.ReplayExhaustive(context.Background(), executor.ExhaustiveReplayPlan{
		ID: "test-central-replay", Workspace: executor.ProbeWorkspace{
			ID: request.Workspace.ID, Root: request.Workspace.Root, State: request.Workspace.State, TreeSHA256: request.Workspace.TreeDigest,
		},
		SourceArtifacts: append([]semanticir.ArtifactRef(nil), request.FocusArtifacts...),
		Operations:      append([]semanticir.Operation(nil), request.Operations...),
		Evidence:        model.ExhaustiveEvidence[0],
	})
	if replayed.Status != executor.StatusConfirmed {
		t.Fatalf("central exhaustive replay = %+v", replayed)
	}
	semanticReplay, err := executor.SemanticReplay(replayed)
	if err != nil {
		t.Fatal(err)
	}
	model.ExhaustiveEvidence[0].Replay = semanticReplay
	if diagnostics := semanticir.ValidateArtifactModel(model); semanticir.HasErrors(diagnostics) {
		t.Fatalf("artifact model invalid: %+v", diagnostics)
	}
	if diagnostics := semanticir.ValidateArtifactScope(request, model); semanticir.HasErrors(diagnostics) {
		t.Fatalf("artifact scope invalid: %+v", diagnostics)
	}
	for _, run := range model.ExhaustiveEvidence[0].Runs {
		for _, observation := range run.Observations {
			var raw map[string]any
			if err := json.Unmarshal(observation.SignalValue, &raw); err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"id", "operation_id", "provenance"} {
				if _, exists := raw[forbidden]; exists {
					t.Fatalf("raw harness signal self-asserts semantic field %q: %s", forbidden, observation.SignalValue)
				}
			}
			raw["id"] = "forged"
			forged, err := json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			decoder := json.NewDecoder(bytes.NewReader(forged))
			decoder.DisallowUnknownFields()
			var trace semanticir.RawOutcomeTrace
			if err := decoder.Decode(&trace); err == nil {
				t.Fatal("forged semantic identity field was accepted as a raw runtime trace")
			}
		}
	}
	choices := make([]semanticir.BehaviorChoice, 0, len(model.Cases))
	observed := make([]string, 0, len(model.Cases))
	for _, behaviorCase := range model.Cases {
		var inputs map[string]semanticir.Literal
		for _, grounding := range model.Groundings {
			if grounding.OperationID == behaviorCase.OperationID && reflect.DeepEqual(grounding.Conditions, behaviorCase.Conditions) {
				inputs = cloneLiteralMap(grounding.Inputs)
				break
			}
		}
		if inputs == nil {
			t.Fatalf("case has no exact assignment grounding: %+v", behaviorCase)
		}
		choices = append(choices, semanticir.BehaviorChoice{
			Behavior:  semanticir.BehaviorRef{OperationID: behaviorCase.OperationID, Conditions: cloneAssignment(behaviorCase.Conditions), Inputs: inputs, Provenance: behaviorCase.Provenance},
			OutcomeID: behaviorCase.OutcomeIDs[0],
		})
		observed = append(observed, behaviorCase.OutcomeIDs[0])
	}
	witness := semanticir.Counterexample{
		ID: "reference-direct-probe", Obligation: semanticir.ObligationReferenceCorrectness,
		OperationID: operation.ID, Conditions: cloneAssignment(model.Cases[0].Conditions),
		ObservedOutcomes: observed, ExpectedOutcomes: append([]string(nil), observed...), Choices: choices,
		Provenance: model.Cases[0].Provenance,
	}
	plan, probeDiagnostics := GenerateProbe(context.Background(), semanticir.MaterializationRequest{
		Frontend: request, Task: &semanticir.Task{Operations: []semanticir.Operation{operation}, Outcomes: model.Outcomes, CodeCases: model.Cases}, Model: model, Counterexample: witness,
	})
	if semanticir.HasErrors(probeDiagnostics) {
		t.Fatalf("GenerateProbe errors: %+v", probeDiagnostics)
	}
	report := executor.ConfirmProbes(context.Background(), executor.TaskEnvironment{
		Command: []string{pythonPath, "-I", "-B", "-c", "pass"}, WorkspaceRoot: root, WorkDir: root,
		Timeout: 10 * time.Second, Environment: testEnvironmentStrings(environment), ExactEnvironment: true, PassSignal: executor.ExitCodeSignal(0),
	}, []executor.ProbePlan{plan})
	if report.Status != executor.StatusConfirmed || len(report.Confirmations) != 1 || report.Confirmations[0].Probe == nil || !report.Confirmations[0].Probe.SemanticsMatch {
		t.Fatalf("direct probe confirmation = %+v", report)
	}
	materializedWitness := witness
	materializedWitness.ID = "materialize-complete-vector"
	for index := range materializedWitness.Choices {
		materializedWitness.Choices[index].OutcomeID = falseOutcome.ID
	}
	materializedWitness.ObservedOutcomes = []string{falseOutcome.ID, falseOutcome.ID}
	materializedWitness.ExpectedOutcomes = []string{falseOutcome.ID, falseOutcome.ID}
	editPlan, editDiagnostics := Materialize(context.Background(), semanticir.MaterializationRequest{
		Frontend: request, Task: &semanticir.Task{Operations: []semanticir.Operation{operation}, Outcomes: model.Outcomes, CodeCases: model.Cases}, Model: model, Counterexample: materializedWitness,
	})
	if semanticir.HasErrors(editDiagnostics) || len(editPlan.Edits) != 1 {
		t.Fatalf("exact full-vector Materialize = %+v / %+v", editPlan, editDiagnostics)
	}
	categoryGrounded := request
	zero := semanticir.Literal{Type: semanticir.TypeInteger}
	membership := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpGE, Operands: []semanticir.Expression{
		{Kind: semanticir.ExprVariable, Type: semanticir.TypeInteger, Name: "value"},
		{Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &zero},
	}}
	categoryGrounded.FiniteDomains = []semanticir.Domain{{
		ID: domain.ID, Type: semanticir.TypeInteger,
		Values: []semanticir.DomainValue{
			{ID: "category-a", Groundings: []semanticir.GroundingAxiom{{OperationID: "decide", Kind: semanticir.GroundingMembership, Membership: &membership, ConcreteWitness: map[string]semanticir.Literal{"value": aValue}}}},
			{ID: "category-b", Groundings: []semanticir.GroundingAxiom{{OperationID: "decide", Kind: semanticir.GroundingMembership, Membership: &membership, ConcreteWitness: map[string]semanticir.Literal{"value": bValue}}}},
		},
	}}
	blockedModel, blockedDiagnostics := Translate(context.Background(), categoryGrounded)
	if !semanticir.HasErrors(blockedDiagnostics) || blockedModel.Coverage.Status != semanticir.TranslationBlocked {
		t.Fatalf("non-singleton semantic categories did not block exact CPython execution: %+v / %+v", blockedModel, blockedDiagnostics)
	}
}

func testArtifact(id string, kind semanticir.ArtifactKind, path string, content []byte) semanticir.ArtifactRef {
	sum := sha256.Sum256(content)
	return semanticir.ArtifactRef{ID: id, Kind: kind, Path: path, Digest: "sha256:" + hex.EncodeToString(sum[:])}
}

func diagnosticsContain(diagnostics []semanticir.Diagnostic, fragment string) bool {
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, fragment) {
			return true
		}
	}
	return false
}

func testChangedRange(artifact semanticir.ArtifactRef, content []byte) semanticir.ChangedSourceRange {
	endLine := strings.Count(string(content), "\n")
	if !bytes.HasSuffix(content, []byte("\n")) {
		endLine++
	}
	return semanticir.ChangedSourceRange{
		ArtifactID: artifact.ID, Path: artifact.Path, StartLine: 1, EndLine: endLine, SliceDigest: semanticir.DigestBytes(content),
		Provenance: semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1, EndLine: endLine, EndColumn: 1}, semanticir.TranslationTranslated),
	}
}

func testBoundOptions(t *testing.T, options map[string]string) map[string]string {
	t.Helper()
	return options
}

func testProver(t *testing.T) semanticir.ToolRef {
	t.Helper()
	path, err := exec.LookPath("z3")
	if err != nil {
		t.Skip("z3 is required for frozen proof evidence")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	version, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	return semanticir.ToolRef{Name: "z3", Path: path, Digest: semanticir.DigestBytes(content), Version: strings.TrimSpace(string(version))}
}

func testPythonEnvironment(t *testing.T) ([]semanticir.EnvironmentVariable, string) {
	t.Helper()
	environment := []semanticir.EnvironmentVariable{
		{Name: "LANG", Value: "C.UTF-8"},
		{Name: "LC_ALL", Value: "C.UTF-8"},
		{Name: "PYTHONHASHSEED", Value: "0"},
		{Name: "TZ", Value: "UTC"},
	}
	digest, err := semanticir.Digest(environment)
	if err != nil {
		t.Fatal(err)
	}
	return environment, digest
}

func testEnvironmentStrings(environment []semanticir.EnvironmentVariable) []string {
	result := make([]string, 0, len(environment))
	for _, variable := range environment {
		result = append(result, variable.Name+"="+variable.Value)
	}
	return result
}
