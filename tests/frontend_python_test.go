package tests

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	frontendpython "github.com/HyperMarble/hyperray/internal/frontend/python"
	"github.com/HyperMarble/hyperray/internal/semanticir"
)

func frontendPythonTool(t *testing.T) semanticir.ToolRef {
	t.Helper()
	path := testPython3(t)
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(absolute)
	if err != nil {
		t.Fatal(err)
	}
	version, err := exec.Command(absolute, "--version").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	return semanticir.ToolRef{
		Name: "cpython-ast", Path: absolute,
		Digest: "sha256:" + hex.EncodeToString(digest[:]), Version: strings.TrimSpace(string(version)),
	}
}

func frontendPythonProver(t *testing.T) semanticir.ToolRef {
	t.Helper()
	path, err := exec.LookPath("z3")
	if err != nil {
		t.Skip("z3 is required for exact Python execution evidence")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(absolute)
	if err != nil {
		t.Fatal(err)
	}
	version, err := exec.Command(absolute, "--version").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	return semanticir.ToolRef{
		Name: "z3", Path: absolute, Digest: semanticir.DigestBytes(content),
		Version: strings.TrimSpace(string(version)),
	}
}

func frontendPythonRequest(t *testing.T, kind semanticir.ArtifactKind, source string, domains []semanticir.Domain, entryPoints ...string) semanticir.FrontendRequest {
	t.Helper()
	digest := sha256.Sum256([]byte(source))
	artifact := semanticir.ArtifactRef{
		ID: "python-" + string(kind), Kind: kind, Path: string(kind) + ".py",
		Digest: "sha256:" + hex.EncodeToString(digest[:]),
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, artifact.Path), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	workspaceEntries := []semanticir.WorkspaceEntry{{Path: artifact.Path, Artifact: artifact}}
	focus := []semanticir.ArtifactRef{artifact}
	var configuration *semanticir.ArtifactRef
	operations := make([]semanticir.Operation, 0, len(entryPoints))
	for _, entryPoint := range entryPoints {
		operation := semanticir.Operation{ID: entryPoint, Kind: semanticir.OperationFunction}
		for _, domain := range domains {
			if strings.HasPrefix(domain.ID, entryPoint+".") {
				operation.DomainIDs = append(operation.DomainIDs, domain.ID)
			}
		}
		if len(operation.DomainIDs) == 0 && len(domains) == 1 {
			operation.DomainIDs = []string{domains[0].ID}
		}
		for _, domainID := range operation.DomainIDs {
			name := domainID
			if separator := strings.LastIndex(name, "."); separator >= 0 {
				name = name[separator+1:]
			}
			var valueType semanticir.ValueType
			for _, domain := range domains {
				if domain.ID == domainID {
					valueType = frontendFixtureInputType(domain)
					break
				}
			}
			operation.Inputs = append(operation.Inputs, semanticir.Variable{Name: name, Type: valueType, DomainID: domainID})
		}
		operations = append(operations, operation)
	}
	if kind == semanticir.ArtifactTests && len(entryPoints) != 0 {
		var targetSource strings.Builder
		for _, entryPoint := range entryPoints {
			targetSource.WriteString("def ")
			targetSource.WriteString(entryPoint)
			targetSource.WriteString("():\n    pass\n")
		}
		targetBytes := []byte(targetSource.String())
		targetDigest := sha256.Sum256(targetBytes)
		target := semanticir.ArtifactRef{
			ID: "python-solution", Kind: semanticir.ArtifactCode, Path: "solution.py",
			Digest: "sha256:" + hex.EncodeToString(targetDigest[:]),
		}
		if err := os.WriteFile(filepath.Join(root, target.Path), targetBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		workspaceEntries = append(workspaceEntries, semanticir.WorkspaceEntry{Path: target.Path, Artifact: target})
		focus = append(focus, target)
		emptyConfiguration := semanticir.ArtifactRef{
			ID: "python-pytest-empty-config", Kind: semanticir.ArtifactConfiguration, Path: "pytest-empty.ini",
			Digest: semanticir.DigestBytes(nil),
		}
		if err := os.WriteFile(filepath.Join(root, emptyConfiguration.Path), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		workspaceEntries = append(workspaceEntries, semanticir.WorkspaceEntry{Path: emptyConfiguration.Path, Artifact: emptyConfiguration})
		configuration = &emptyConfiguration
	}
	treeDigest, err := semanticir.Digest(workspaceEntries)
	if err != nil {
		t.Fatal(err)
	}
	environment := []semanticir.EnvironmentVariable{{Name: "PYTHONHASHSEED", Value: "0"}}
	if kind == semanticir.ArtifactTests {
		environment = append(environment, semanticir.EnvironmentVariable{Name: "PYTEST_DISABLE_PLUGIN_AUTOLOAD", Value: "1"})
		sort.Slice(environment, func(i, j int) bool { return environment[i].Name < environment[j].Name })
	}
	environmentDigest, err := semanticir.Digest(environment)
	if err != nil {
		t.Fatal(err)
	}
	request := semanticir.FrontendRequest{
		TaskID: "frontend-python", Artifact: artifact, Language: semanticir.LanguagePython,
		Kind: kind, Source: []byte(source), EntryPoints: entryPoints, FiniteDomains: domains, Operations: operations,
		Translator: frontendPythonTool(t),
		Workspace: semanticir.WorkspaceRef{
			ID: "workspace-python", State: semanticir.WorkspaceSolutionNewTests, Root: root,
			TreeDigest: treeDigest, WorkingDirectory: ".", BuildCommand: "python verification",
			Environment: environment, EnvironmentDigest: environmentDigest, ClearEnvironment: true, KillProcessGroup: true, Entries: workspaceEntries,
		},
		FocusArtifacts: focus,
	}
	if kind == semanticir.ArtifactTests {
		var testIDs []string
		for _, line := range strings.Split(source, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "def test_") {
				continue
			}
			name := strings.TrimPrefix(line, "def ")
			if end := strings.IndexByte(name, '('); end >= 0 {
				testIDs = append(testIDs, name[:end])
			}
		}
		sort.Strings(testIDs)
		var script strings.Builder
		script.WriteString("import pathlib,runpy,sys;assert pathlib.Path(")
		script.WriteString(strconv.Quote(configuration.Path))
		script.WriteString(").read_bytes()==b\"\";sys.path.insert(0,\".\");m=runpy.run_path(")
		script.WriteString(strconv.Quote(artifact.Path))
		script.WriteByte(')')
		for _, testID := range testIDs {
			script.WriteString(";m[")
			script.WriteString(strconv.Quote(testID))
			script.WriteString("]()")
		}
		command := []string{request.Translator.Path, "-P", "-I", "-S", "-c"}
		provenance := semanticir.NewProvenance(*configuration, semanticir.SourceLocation{Path: configuration.Path, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1}, semanticir.TranslationTranslated)
		request.Runner = request.Translator
		request.Configuration = configuration
		request.RunnerCommand = &semanticir.WorkspaceCommand{
			ID: "python-pytest", WorkspaceID: request.Workspace.ID, State: request.Workspace.State, TreeDigest: request.Workspace.TreeDigest,
			Command: strings.Join(command, " ") + " '" + script.String() + "'", WorkingDirectory: request.Workspace.WorkingDirectory,
			Environment: append([]semanticir.EnvironmentVariable(nil), environment...), EnvironmentDigest: environmentDigest,
			ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 30000,
			PassSignal:   semanticir.PassSignal{Kind: semanticir.PassSignalExitCode, Expected: "0", Provenance: provenance},
			ExpectedPass: true, ObservedPass: true, Tools: []semanticir.ToolRef{request.Translator}, Provenance: provenance,
		}
	}
	return request
}

// frontendFixtureInputType is test setup, not frontend inference. These
// fixtures explicitly choose the raw type supplied by their synthetic spec.
func frontendFixtureInputType(domain semanticir.Domain) semanticir.ValueType {
	allBool, allInteger := len(domain.Values) != 0, len(domain.Values) != 0
	for _, value := range domain.Values {
		if value.ID != "true" && value.ID != "false" && value.ID != "True" && value.ID != "False" {
			allBool = false
		}
		if _, err := strconv.ParseInt(value.ID, 10, 64); err != nil {
			allInteger = false
		}
	}
	if allBool {
		return semanticir.TypeBool
	}
	if allInteger {
		return semanticir.TypeInteger
	}
	return semanticir.TypeString
}

func frontendDomain(id string, values ...string) semanticir.Domain {
	domain := semanticir.Domain{ID: id}
	for _, value := range values {
		domain.Values = append(domain.Values, semanticir.DomainValue{ID: value})
	}
	return domain
}

func requireNoFrontendErrors(t *testing.T, diagnostics []semanticir.Diagnostic) {
	t.Helper()
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("unexpected frontend errors: %+v", diagnostics)
	}
}

func requireFrontendCodeExecutionBlocked(t *testing.T, model semanticir.ArtifactModel, diagnostics []semanticir.Diagnostic) {
	t.Helper()
	if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked {
		t.Fatalf("code without exact interpreter evidence did not block: diagnostics=%+v coverage=%+v", diagnostics, model.Coverage)
	}
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "PY_EXECUTION_REQUIRED") || strings.Contains(diagnostic.Message, "code behavior requires") {
			return
		}
	}
	t.Fatalf("missing PY_EXECUTION_REQUIRED diagnostic: %+v", diagnostics)
}

func TestFrontendPython(t *testing.T) {
	t.Run("solution lowers control flow structurally but blocks AST-derived outcomes", func(t *testing.T) {
		source := `def helper(x):
    return x + 1

def classify(x):
    helper(x)
    if x > 0 and x <= 2:
        return helper(x)
    elif x == 0:
        raise ValueError("zero")
    return -1
`
		request := frontendPythonRequest(t, semanticir.ArtifactCode, source,
			[]semanticir.Domain{frontendDomain("x", "-1", "0", "1", "2")}, "helper", "classify")
		model, diagnostics := frontendpython.Translate(context.Background(), request)
		requireFrontendCodeExecutionBlocked(t, model, diagnostics)
		if model.Translator != request.Translator {
			t.Fatalf("translator identity was not preserved: %+v", model.Translator)
		}
		if len(model.Operations) != 2 {
			t.Fatalf("operations = %d, want 2", len(model.Operations))
		}
		var classify semanticir.Operation
		for _, operation := range model.Operations {
			if operation.ID == "classify" {
				classify = operation
			}
		}
		if len(classify.DomainIDs) != 1 || classify.DomainIDs[0] != "x" {
			t.Fatalf("operation domains = %v", classify.DomainIDs)
		}
		if len(model.Cases) != 0 || len(model.Outcomes) != 0 {
			t.Fatalf("AST evaluator leaked runtime outcomes/cases: outcomes=%+v cases=%+v", model.Outcomes, model.Cases)
		}
		if classify.Body[0].Kind != semanticir.StmtCall || classify.Body[1].Kind != semanticir.StmtBranch {
			t.Fatalf("classify body did not lower call/branch: %+v", classify.Body)
		}
		condition := classify.Body[1].Condition
		if condition == nil || condition.Kind != semanticir.ExprBool || condition.Operator != semanticir.OpAnd {
			t.Fatalf("branch condition not lowered as boolean comparisons: %+v", condition)
		}
		if classify.Body[1].Provenance.Location.StartLine != 6 {
			t.Fatalf("branch provenance = %+v", classify.Body[1].Provenance.Location)
		}
	})

	t.Run("assertions become one global relational predicate", func(t *testing.T) {
		source := `from solution import classify

def test_relations():
    assert classify(1) == 2
    assert classify(1) == 2 or classify(1) == 3
    assert classify(1) != classify(2)
`
		request := frontendPythonRequest(t, semanticir.ArtifactTests, source,
			[]semanticir.Domain{frontendDomain("x", "0", "1", "2")}, "classify")
		request.Operations[0].Inputs[0].Type = semanticir.TypeInteger
		request.FiniteDomains[0].Type = semanticir.TypeString
		for index := range request.FiniteDomains[0].Values {
			member := &request.FiniteDomains[0].Values[index]
			value, err := strconv.ParseInt(member.ID, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			literal := semanticir.Literal{Type: semanticir.TypeInteger, Integer: value}
			membership := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{
				{Kind: semanticir.ExprVariable, Type: semanticir.TypeInteger, Name: "x"},
				{Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &literal},
			}}
			member.Groundings = []semanticir.GroundingAxiom{{
				OperationID: "classify", Kind: semanticir.GroundingMembership, Membership: &membership,
				ConcreteWitness: map[string]semanticir.Literal{"x": literal},
			}}
			assignment := semanticir.Assignment{"x": member.ID}
			request.Groundings = append(request.Groundings, semanticir.AssignmentGrounding{
				ID: semanticir.AssignmentGroundingID("classify", assignment), OperationID: "classify",
				Conditions: assignment, Inputs: map[string]semanticir.Literal{"x": literal},
			})
		}
		model, diagnostics := frontendpython.Translate(context.Background(), request)
		requireNoFrontendErrors(t, diagnostics)
		if validation := semanticir.ValidateArtifactModel(model); semanticir.HasErrors(validation) {
			t.Fatalf("relational test artifact model is invalid: %+v", validation)
		}
		if model.Coverage.Status != semanticir.TranslationComplete || len(model.Tests) != 1 {
			t.Fatalf("tests=%d coverage=%q unsupported=%+v", len(model.Tests), model.Coverage.Status, model.Coverage.Unsupported)
		}
		if model.TestProjection == nil || len(model.TestProjection.Quantification) != 2 {
			t.Fatalf("exact singleton test categories lack quantification evidence: %+v", model.TestProjection)
		}
		if validation := semanticir.ValidateTestObservationQuantification(&semanticir.Task{
			Domains: request.FiniteDomains, Groundings: request.Groundings, Operations: request.Operations,
		}, model); semanticir.HasErrors(validation) {
			t.Fatalf("singleton observation quantification is invalid: %+v", validation)
		}
		var relational semanticir.TestModel
		for _, test := range model.Tests {
			switch {
			case strings.HasSuffix(test.ID, "::test_relations"):
				relational = test
			}
		}
		if relational.Predicate.Kind != semanticir.PredicateAnd || len(relational.Predicate.Children) != 3 {
			t.Fatalf("relational predicate = %+v", relational.Predicate)
		}
		if relational.Predicate.Children[1].Kind != semanticir.PredicateOr {
			t.Fatalf("OR assertion not preserved: %+v", relational.Predicate.Children[1])
		}
		third := relational.Predicate.Children[2]
		if third.Kind != semanticir.PredicateNot || len(third.Children) != 1 || third.Children[0].Kind != semanticir.PredicateOutcomeEqual {
			t.Fatalf("cross-case inequality not relational: %+v", third)
		}
		for _, test := range model.Tests {
			if test.Predicate.Kind == "" {
				t.Fatalf("test %s has empty authoritative predicate", test.ID)
			}
		}
	})

	t.Run("pytest raises lowers statically but blocks unmodeled framework import", func(t *testing.T) {
		source := `from solution import classify
import pytest

def test_raise():
    with pytest.raises(ValueError):
        classify(0)
`
		request := frontendPythonRequest(t, semanticir.ArtifactTests, source,
			[]semanticir.Domain{frontendDomain("x", "0")}, "classify")
		zero := semanticir.Literal{Type: semanticir.TypeInteger}
		membership := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{
			{Kind: semanticir.ExprVariable, Type: semanticir.TypeInteger, Name: "x"},
			{Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &zero},
		}}
		request.FiniteDomains[0].Type = semanticir.TypeString
		request.FiniteDomains[0].Values[0].Groundings = []semanticir.GroundingAxiom{{
			OperationID: "classify", Kind: semanticir.GroundingMembership, Membership: &membership,
			ConcreteWitness: map[string]semanticir.Literal{"x": zero},
		}}
		conditions := semanticir.Assignment{"x": "0"}
		request.Groundings = []semanticir.AssignmentGrounding{{
			ID: semanticir.AssignmentGroundingID("classify", conditions), OperationID: "classify",
			Conditions: conditions, Inputs: map[string]semanticir.Literal{"x": zero},
		}}
		model, diagnostics := frontendpython.Translate(context.Background(), request)
		if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked {
			t.Fatalf("pytest import must block exact projection: diagnostics=%+v coverage=%+v", diagnostics, model.Coverage)
		}
		if len(model.Tests) != 1 || model.Tests[0].Predicate.Kind != semanticir.PredicateRaises || model.Tests[0].Predicate.Observe == nil || model.Tests[0].Predicate.Observe.ExceptionType != "ValueError" {
			t.Fatalf("raises assertion did not retain static audit detail: %+v", model.Tests)
		}
	})

	t.Run("weak OR derives unary accepted-outcome metadata", func(t *testing.T) {
		source := `from solution import decide

def test_weak():
    assert decide(True) == False or decide(True) == True
`
		request := frontendPythonRequest(t, semanticir.ArtifactTests, source,
			[]semanticir.Domain{frontendDomain("flag", "true", "false")}, "decide")
		request.FiniteDomains[0].Type = semanticir.TypeString
		for index := range request.FiniteDomains[0].Values {
			member := &request.FiniteDomains[0].Values[index]
			literal := semanticir.Literal{Type: semanticir.TypeBool, Bool: member.ID == "true"}
			membership := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{
				{Kind: semanticir.ExprVariable, Type: semanticir.TypeBool, Name: "flag"},
				{Kind: semanticir.ExprLiteral, Type: semanticir.TypeBool, Literal: &literal},
			}}
			member.Groundings = []semanticir.GroundingAxiom{{
				OperationID: "decide", Kind: semanticir.GroundingMembership, Membership: &membership,
				ConcreteWitness: map[string]semanticir.Literal{"flag": literal},
			}}
			assignment := semanticir.Assignment{"flag": member.ID}
			request.Groundings = append(request.Groundings, semanticir.AssignmentGrounding{
				ID: semanticir.AssignmentGroundingID("decide", assignment), OperationID: "decide",
				Conditions: assignment, Inputs: map[string]semanticir.Literal{"flag": literal},
			})
		}
		model, diagnostics := frontendpython.Translate(context.Background(), request)
		requireNoFrontendErrors(t, diagnostics)
		if validation := semanticir.ValidateArtifactModel(model); semanticir.HasErrors(validation) {
			t.Fatalf("weak-OR test artifact model is invalid: %+v", validation)
		}
		if len(model.Tests) != 1 || model.Tests[0].Predicate.Kind != semanticir.PredicateOr || len(model.Tests[0].AcceptedOutcomes) != 2 {
			t.Fatalf("weak OR translation = %+v", model.Tests)
		}
	})

	t.Run("operation-local domains and calls stay structural without runtime claims", func(t *testing.T) {
		source := `def fail(x):
    """Raise on the declared boundary."""
    if x == 0:
        raise ValueError("zero")
    return x

def wrap(x):
    return fail(x)
`
		request := frontendPythonRequest(t, semanticir.ArtifactCode, source, []semanticir.Domain{
			frontendDomain("fail.x", "0", "1"), frontendDomain("wrap.x", "0", "1"),
		}, "fail", "wrap")
		model, diagnostics := frontendpython.Translate(context.Background(), request)
		requireFrontendCodeExecutionBlocked(t, model, diagnostics)
		if len(model.Operations) != 2 || len(model.Operations[0].DomainIDs) != 1 || len(model.Operations[1].DomainIDs) != 1 || model.Operations[0].DomainIDs[0] == model.Operations[1].DomainIDs[0] {
			t.Fatalf("operation-local domains were not preserved: %+v", model.Operations)
		}
		if len(model.Operations[1].Body) != 1 || model.Operations[1].Body[0].Value == nil || model.Operations[1].Body[0].Value.Kind != semanticir.ExprCall {
			t.Fatalf("local call was not retained as structural IR: %+v", model.Operations[1].Body)
		}
	})

	t.Run("Python modulo is retained structurally without claiming execution", func(t *testing.T) {
		source := "def mod_two(x):\n    return x % 2\n"
		request := frontendPythonRequest(t, semanticir.ArtifactCode, source,
			[]semanticir.Domain{frontendDomain("x", "-1")}, "mod_two")
		model, diagnostics := frontendpython.Translate(context.Background(), request)
		requireFrontendCodeExecutionBlocked(t, model, diagnostics)
		if len(model.Operations) != 1 || len(model.Operations[0].Body) != 1 || model.Operations[0].Body[0].Value == nil || model.Operations[0].Body[0].Value.Operator != semanticir.OpMod {
			t.Fatalf("modulo structural IR = %+v", model.Operations)
		}
	})

	t.Run("Python explicit None lowers as optional null without runtime cases", func(t *testing.T) {
		source := "def explicit_none():\n    return None\n\ndef implicit_none():\n    pass\n"
		request := frontendPythonRequest(t, semanticir.ArtifactCode, source, nil, "explicit_none", "implicit_none")
		model, diagnostics := frontendpython.Translate(context.Background(), request)
		requireFrontendCodeExecutionBlocked(t, model, diagnostics)
		if len(model.Operations) != 2 || len(model.Operations[0].Body) != 1 || model.Operations[0].Body[0].Value == nil || model.Operations[0].Body[0].Value.Literal == nil || model.Operations[0].Body[0].Value.Literal.Type != semanticir.TypeOptional || !model.Operations[0].Body[0].Value.Literal.Null {
			t.Fatalf("Python None was not structurally optional-null: %+v", model.Operations)
		}
	})
}

func TestFrontendPythonBlocked(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"dynamic eval", "def unsafe(x):\n    return eval(x)\n"},
		{"reflection", "def unsafe(x):\n    return getattr(x, 'field')\n"},
		{"external import and call", "import os\ndef unsafe(x):\n    return os.system(x)\n"},
		{"runtime annotation", "def unsafe(x: int) -> int:\n    return x\n"},
		{"shadowed builtin call", "def unsafe(len):\n    return len('x')\n"},
		{"dynamic custom exception", "def unsafe(x):\n    raise CustomError('bad')\n"},
		{"integer overflow", "def unsafe(x):\n    return x + 1\n"},
		{"unsupported value is not a bare return", "def unsafe(x):\n    return [item for item in x]\n"},
		{"test module global assignment", "from solution import decide\nGLOBAL = True\ndef test_global():\n    assert decide(True) == GLOBAL\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kind := semanticir.ArtifactCode
			entries := []string{"unsafe"}
			domains := []semanticir.Domain{frontendDomain("x", "sample")}
			if strings.Contains(test.source, "def test_") {
				kind, entries = semanticir.ArtifactTests, []string{"decide"}
				domains = []semanticir.Domain{frontendDomain("value", "true", "false")}
			}
			request := frontendPythonRequest(t, kind, test.source, domains, entries...)
			if test.name == "integer overflow" {
				request.FiniteDomains = []semanticir.Domain{frontendDomain("x", "9223372036854775807")}
			}
			model, diagnostics := frontendpython.Translate(context.Background(), request)
			if !semanticir.HasErrors(diagnostics) {
				t.Fatalf("unsupported source produced no blocking diagnostic: %+v", model)
			}
			if model.Coverage.Status != semanticir.TranslationBlocked || len(model.Coverage.Unsupported) == 0 {
				t.Fatalf("coverage did not fail closed: %+v", model.Coverage)
			}
			if test.name == "unsupported value is not a bare return" && len(model.Operations) == 1 && len(model.Operations[0].Body) != 0 {
				t.Fatalf("unsupported list expression was approximated as a typed return: %+v", model.Operations[0].Body)
			}
		})
	}

	t.Run("frozen conftest hook", func(t *testing.T) {
		request := frontendPythonRequest(t, semanticir.ArtifactTests,
			"from solution import decide\ndef test_decide():\n    assert decide(True) == True\n",
			[]semanticir.Domain{frontendDomain("flag", "true", "false")}, "decide")
		body := []byte("def pytest_runtest_call(item):\n    raise RuntimeError('hook')\n")
		artifact := semanticir.ArtifactRef{ID: "python-conftest", Kind: semanticir.ArtifactTests, Path: "conftest.py", Digest: semanticir.DigestBytes(body)}
		if err := os.WriteFile(filepath.Join(request.Workspace.Root, artifact.Path), body, 0o600); err != nil {
			t.Fatal(err)
		}
		request.Workspace.Entries = append(request.Workspace.Entries, semanticir.WorkspaceEntry{Path: artifact.Path, Artifact: artifact})
		request.Workspace.TreeDigest, _ = semanticir.Digest(request.Workspace.Entries)
		request.RunnerCommand.TreeDigest = request.Workspace.TreeDigest
		model, diagnostics := frontendpython.Translate(context.Background(), request)
		if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked {
			t.Fatalf("conftest hook must block exact runner evidence: diagnostics=%+v coverage=%+v", diagnostics, model.Coverage)
		}
	})

	t.Run("one concrete call cannot stand for a non-singleton category", func(t *testing.T) {
		zero := semanticir.Literal{Type: semanticir.TypeInteger}
		membership := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpGE, Operands: []semanticir.Expression{
			{Kind: semanticir.ExprVariable, Type: semanticir.TypeInteger, Name: "x"},
			{Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &zero},
		}}
		domain := semanticir.Domain{ID: "decide.x", Type: semanticir.TypeString, Values: []semanticir.DomainValue{{
			ID: "nonnegative", Groundings: []semanticir.GroundingAxiom{{
				OperationID: "decide", Kind: semanticir.GroundingMembership, Membership: &membership,
				ConcreteWitness: map[string]semanticir.Literal{"x": zero},
			}},
		}}}
		request := frontendPythonRequest(t, semanticir.ArtifactTests,
			"from solution import decide\ndef test_zero_only():\n    assert decide(0) == 0\n",
			[]semanticir.Domain{domain}, "decide")
		request.Operations[0].Inputs[0].Type = semanticir.TypeInteger
		model, diagnostics := frontendpython.Translate(context.Background(), request)
		if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked {
			t.Fatalf("x=0 was unsoundly promoted to the entire x>=0 category (which could differ at x=1): diagnostics=%+v model=%+v", diagnostics, model)
		}
	})

	t.Run("semantic labels are not Python input literals", func(t *testing.T) {
		request := frontendPythonRequest(t, semanticir.ArtifactTests,
			"from solution import decide\ndef test_enabled():\n    assert decide(True) == True\n",
			[]semanticir.Domain{frontendDomain("decide.flag", "disabled", "enabled")}, "decide")
		request.Operations[0].Inputs[0].Type = semanticir.TypeBool
		model, diagnostics := frontendpython.Translate(context.Background(), request)
		if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked {
			t.Fatalf("ungrounded semantic labels were treated as Python booleans: diagnostics=%+v model=%+v", diagnostics, model)
		}
		if len(model.Tests) != 0 {
			t.Fatalf("ungrounded call leaked an authoritative test predicate: %+v", model.Tests)
		}
	})
}

func TestFrontendPythonRelational(t *testing.T) {
	source := `from solution import decide

def test_bounded_loop_and_relation():
    expected = True
    for value in (True, False):
        assert decide(value) == value
    assert decide(True) == decide(expected)
`
	request := frontendPythonRequest(t, semanticir.ArtifactTests, source,
		[]semanticir.Domain{frontendDomain("decide.flag", "true", "false")}, "decide")
	request.Operations[0].Inputs[0].Type = semanticir.TypeBool
	request.FiniteDomains[0].Type = semanticir.TypeString
	for index := range request.FiniteDomains[0].Values {
		member := &request.FiniteDomains[0].Values[index]
		literal := semanticir.Literal{Type: semanticir.TypeBool, Bool: member.ID == "true"}
		membership := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{
			{Kind: semanticir.ExprVariable, Type: semanticir.TypeBool, Name: "flag"},
			{Kind: semanticir.ExprLiteral, Type: semanticir.TypeBool, Literal: &literal},
		}}
		member.Groundings = []semanticir.GroundingAxiom{{
			OperationID: "decide", Kind: semanticir.GroundingMembership, Membership: &membership,
			ConcreteWitness: map[string]semanticir.Literal{"flag": literal},
		}}
		conditions := semanticir.Assignment{"decide.flag": member.ID}
		request.Groundings = append(request.Groundings, semanticir.AssignmentGrounding{
			ID: semanticir.AssignmentGroundingID("decide", conditions), OperationID: "decide",
			Conditions: conditions, Inputs: map[string]semanticir.Literal{"flag": literal},
		})
	}
	model, diagnostics := frontendpython.Translate(context.Background(), request)
	requireNoFrontendErrors(t, diagnostics)
	if validation := semanticir.ValidateArtifactModel(model); semanticir.HasErrors(validation) {
		t.Fatalf("bounded relational verifier model is invalid: %+v", validation)
	}
	if len(model.Tests) != 1 || model.Tests[0].Predicate.Kind != semanticir.PredicateAnd || len(model.Tests[0].Predicate.Children) != 3 {
		t.Fatalf("bounded loop/global relation predicate = %+v", model.Tests)
	}
	if model.Tests[0].Predicate.Children[2].Kind != semanticir.PredicateOutcomeEqual {
		t.Fatalf("cross-call equality was not preserved: %+v", model.Tests[0].Predicate.Children[2])
	}
	controls := 0
	for _, construct := range model.TestProjection.Constructs {
		if construct.Kind == semanticir.TestConstructControl {
			controls++
		}
	}
	if controls < 2 {
		t.Fatalf("assignment/loop controls lack compiler projection: %+v", model.TestProjection.Constructs)
	}
}

func TestFrontendPythonMaterialize(t *testing.T) {
	source := `def decide(flag):
    if flag:
        return True
    return False
`
	request := frontendPythonRequest(t, semanticir.ArtifactCode, source,
		[]semanticir.Domain{frontendDomain("flag", "true", "false")}, "decide")
	falseLiteral := semanticir.Literal{Type: semanticir.TypeBool}
	trueLiteral := semanticir.Literal{Type: semanticir.TypeBool, Bool: true}
	declaredFalse := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &falseLiteral, OperationID: "decide"}
	declaredFalse.ID = semanticir.OutcomeID(declaredFalse)
	declaredTrue := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &trueLiteral, OperationID: "decide"}
	declaredTrue.ID = semanticir.OutcomeID(declaredTrue)
	request.Outcomes = []semanticir.ObservableOutcome{declaredFalse, declaredTrue}
	request.Operations[0].OutcomeIDs = []string{declaredFalse.ID, declaredTrue.ID}
	exactBool := func(literal semanticir.Literal) semanticir.Expression {
		return semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{
			{Kind: semanticir.ExprVariable, Type: semanticir.TypeBool, Name: "flag"},
			{Kind: semanticir.ExprLiteral, Type: semanticir.TypeBool, Literal: &literal},
		}}
	}
	trueMembership := exactBool(trueLiteral)
	falseMembership := exactBool(falseLiteral)
	request.Operations[0].Inputs[0].Type = semanticir.TypeBool
	request.FiniteDomains[0].Type = semanticir.TypeString
	request.FiniteDomains[0].Values[0].Groundings = []semanticir.GroundingAxiom{{
		OperationID: "decide", Kind: semanticir.GroundingMembership, Membership: &trueMembership, ConcreteWitness: map[string]semanticir.Literal{"flag": trueLiteral},
	}}
	request.FiniteDomains[0].Values[1].Groundings = []semanticir.GroundingAxiom{{
		OperationID: "decide", Kind: semanticir.GroundingMembership, Membership: &falseMembership, ConcreteWitness: map[string]semanticir.Literal{"flag": falseLiteral},
	}}
	request.Groundings = []semanticir.AssignmentGrounding{
		{OperationID: "decide", Conditions: semanticir.Assignment{"flag": "true"}, Inputs: map[string]semanticir.Literal{"flag": trueLiteral}},
		{OperationID: "decide", Conditions: semanticir.Assignment{"flag": "false"}, Inputs: map[string]semanticir.Literal{"flag": falseLiteral}},
	}
	for index := range request.Groundings {
		request.Groundings[index].ID = semanticir.AssignmentGroundingID(request.Groundings[index].OperationID, request.Groundings[index].Conditions)
	}
	request.Options = map[string]string{
		"python.execution": "bound-cpython", "python.module": strings.TrimSuffix(request.Artifact.Path, filepath.Ext(request.Artifact.Path)), "python.package_root": ".",
	}
	request.Prover = frontendPythonProver(t)
	endLine := strings.Count(source, "\n")
	request.ChangedRanges = []semanticir.ChangedSourceRange{{
		ArtifactID: request.Artifact.ID, Path: request.Artifact.Path, StartLine: 1, EndLine: endLine,
		SliceDigest: request.Artifact.Digest,
		Provenance: semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{
			Path: request.Artifact.Path, StartLine: 1, StartColumn: 1, EndLine: endLine, EndColumn: 1,
		}, semanticir.TranslationTranslated),
	}}
	model, diagnostics := frontendpython.Translate(context.Background(), request)
	requireNoFrontendErrors(t, diagnostics)
	var trueOutcome string
	for _, outcome := range model.Outcomes {
		if outcome.Kind == semanticir.OutcomeReturn && outcome.Value != nil && outcome.Value.Type == semanticir.TypeBool && outcome.Value.Bool {
			trueOutcome = outcome.ID
		}
	}
	if trueOutcome == "" {
		t.Fatal("translated model has no true return outcome")
	}
	var decideOperation semanticir.Operation
	for _, operation := range model.Operations {
		if operation.ID == "decide" {
			decideOperation = operation
		}
	}
	compiledTask := &semanticir.Task{Outcomes: model.Outcomes, Operations: []semanticir.Operation{decideOperation}, CodeCases: model.Cases}
	witness := semanticir.Counterexample{
		ID: "witness-false-to-true", Obligation: semanticir.ObligationTestsSound,
		OperationID: "decide", Conditions: semanticir.Assignment{"flag": "false"}, TestPasses: true,
		Choices: []semanticir.BehaviorChoice{{
			Behavior:  semanticir.BehaviorRef{OperationID: "decide", Conditions: semanticir.Assignment{"flag": "false"}, Inputs: map[string]semanticir.Literal{"flag": falseLiteral}},
			OutcomeID: trueOutcome,
		}, {
			Behavior:  semanticir.BehaviorRef{OperationID: "decide", Conditions: semanticir.Assignment{"flag": "true"}, Inputs: map[string]semanticir.Literal{"flag": trueLiteral}},
			OutcomeID: trueOutcome, // already equals reference and needs no edit
		}},
	}
	plan, diagnostics := frontendpython.Materialize(context.Background(), semanticir.MaterializationRequest{
		Frontend: request, Task: compiledTask, Model: model, Counterexample: witness,
	})
	requireNoFrontendErrors(t, diagnostics)
	if len(plan.Edits) != 1 {
		t.Fatalf("edits = %+v, want one exact edit", plan.Edits)
	}
	if len(plan.Expected.Choices) != 2 {
		t.Fatalf("edit plan truncated relational behavior vector: %+v", plan.Expected.Choices)
	}
	edit := plan.Edits[0]
	replacement := string(edit.Replacement)
	if !strings.Contains(string(edit.ExpectedBytes), "if flag:") ||
		!strings.Contains(replacement, "if flag == False:\n        return True") ||
		!strings.Contains(replacement, "if flag == True:\n        return True") {
		t.Fatalf("materialized edit = %q -> %q", edit.ExpectedBytes, edit.Replacement)
	}
	if plan.Artifact.Digest != request.Artifact.Digest || plan.Provenance.ArtifactDigest != request.Artifact.Digest || plan.Provenance.Translation != semanticir.TranslationTranslated {
		t.Fatalf("edit plan is not digest anchored: %+v", plan)
	}
	mutated := append([]byte(nil), request.Source...)
	mutated = append(mutated[:edit.StartByte], append(edit.Replacement, mutated[edit.EndByte:]...)...)
	if !strings.Contains(string(mutated), "if flag == False:\n        return True") ||
		!strings.Contains(string(mutated), "if flag == True:\n        return True") {
		t.Fatalf("unexpected materialized source:\n%s", mutated)
	}

	blockedWitness := witness
	blockedWitness.Choices = append([]semanticir.BehaviorChoice(nil), witness.Choices...)
	blockedWitness.ID = "unknown-outcome"
	blockedWitness.Choices[0].OutcomeID = "outcome-not-in-task"
	blocked, blockedDiagnostics := frontendpython.Materialize(context.Background(), semanticir.MaterializationRequest{
		Frontend: request, Task: compiledTask, Model: model, Counterexample: blockedWitness,
	})
	if !semanticir.HasErrors(blockedDiagnostics) || len(blocked.Edits) != 0 {
		t.Fatalf("unrenderable outcome did not block: plan=%+v diagnostics=%+v", blocked, blockedDiagnostics)
	}
}
