package tests

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/HyperMarble/ray/internal/executor"
	frontendcpp "github.com/HyperMarble/ray/internal/frontend/cpp"
	"github.com/HyperMarble/ray/internal/semanticir"
)

func frontendCPPTool(t *testing.T) semanticir.ToolRef {
	t.Helper()
	path, err := exec.LookPath("clang++")
	if err != nil {
		t.Skip("clang++ is required for the C++ frontend gate")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	versionBytes, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("clang++ --version: %v: %s", err, versionBytes)
	}
	version := strings.TrimSpace(string(versionBytes))
	digest := sha256.Sum256(content)
	return semanticir.ToolRef{Name: "clang++", Path: path, Digest: "sha256:" + hex.EncodeToString(digest[:]), Version: version}
}

func frontendCPPProver(t *testing.T) semanticir.ToolRef {
	t.Helper()
	path, err := exec.LookPath("z3")
	if err != nil {
		t.Skip("z3 is required for the C++ frontend gate")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	versionBytes, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("z3 --version: %v: %s", err, versionBytes)
	}
	return semanticir.ToolRef{Name: "z3", Path: path, Digest: semanticir.DigestBytes(content), Version: strings.TrimSpace(string(versionBytes))}
}

func cppDomain(id string, values ...string) semanticir.Domain {
	typeName := semanticir.TypeInteger
	for _, value := range values {
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			typeName = semanticir.TypeString
			break
		}
	}
	if len(values) > 0 {
		allBool := true
		for _, value := range values {
			if value != "true" && value != "false" {
				allBool = false
				break
			}
		}
		if allBool {
			typeName = semanticir.TypeBool
		}
	}
	domain := semanticir.Domain{ID: id, Type: typeName, Values: []semanticir.DomainValue{}}
	for _, value := range values {
		literal := semanticir.Literal{Type: typeName}
		switch typeName {
		case semanticir.TypeBool:
			literal.Bool = value == "true"
		case semanticir.TypeInteger:
			literal.Integer, _ = strconv.ParseInt(value, 10, 64)
		case semanticir.TypeString:
			literal.String = value
		}
		domain.Values = append(domain.Values, semanticir.DomainValue{ID: value, Value: &literal})
	}
	return domain
}

func frontendCPPRequest(t *testing.T, kind semanticir.ArtifactKind, source string, domains []semanticir.Domain, entries ...string) semanticir.FrontendRequest {
	t.Helper()
	root := t.TempDir()
	sourceName := "artifact.cpp"
	sourcePath := filepath.Join(root, sourceName)
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	sourceDigest := semanticir.DigestBytes([]byte(source))
	artifact := semanticir.ArtifactRef{ID: "cpp-" + string(kind) + "-" + strings.TrimPrefix(sourceDigest, "sha256:")[:12], Kind: kind, Path: sourceName, Digest: sourceDigest}
	tool := frontendCPPTool(t)
	compileDB := []map[string]any{{
		"directory": root,
		"file":      sourceName,
		"arguments": []string{tool.Path, "-std=c++20", "-c", sourcePath, "-o", filepath.Join(root, "artifact.o")},
	}}
	compileBytes, err := json.Marshal(compileDB)
	if err != nil {
		t.Fatal(err)
	}
	compilePath := filepath.Join(root, "compile_commands.json")
	if err := os.WriteFile(compilePath, compileBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	compileArtifact := semanticir.ArtifactRef{ID: "cpp-compile-db", Kind: semanticir.ArtifactEnvironment, Path: "compile_commands.json", Digest: semanticir.DigestBytes(compileBytes)}
	provenance := semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: sourceName, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1}, semanticir.TranslationTranslated)
	compileProvenance := semanticir.NewProvenance(compileArtifact, semanticir.SourceLocation{Path: compileArtifact.Path, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1}, semanticir.TranslationTranslated)
	endLine := strings.Count(source, "\n") + 1
	if strings.HasSuffix(source, "\n") {
		endLine--
	}
	changedProvenance := semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: sourceName, StartLine: 1, StartColumn: 1, EndLine: endLine, EndColumn: 1}, semanticir.TranslationTranslated)
	environment := []semanticir.EnvironmentVariable{{Name: "PATH", Value: os.Getenv("PATH")}}
	environmentDigest, err := semanticir.Digest(environment)
	if err != nil {
		t.Fatal(err)
	}
	return semanticir.FrontendRequest{
		TaskID: "frontend-cpp", Artifact: artifact, Language: semanticir.LanguageCPP,
		Kind: kind, Source: []byte(source), EntryPoints: entries, FiniteDomains: domains,
		Translator: tool, Prover: frontendCPPProver(t), FocusArtifacts: []semanticir.ArtifactRef{artifact},
		ChangedRanges: []semanticir.ChangedSourceRange{{
			ArtifactID: artifact.ID, Path: artifact.Path, StartLine: 1, EndLine: endLine,
			SliceDigest: semanticir.DigestBytes([]byte(source)), Provenance: changedProvenance,
		}},
		Workspace: semanticir.WorkspaceRef{
			ID: "cpp-workspace", State: semanticir.WorkspaceSolutionNewTests, Root: root,
			TreeDigest:       semanticir.DigestBytes(append(append([]byte(nil), []byte(source)...), compileBytes...)),
			WorkingDirectory: ".", BuildCommand: tool.Path + " -c " + sourceName,
			Environment: environment, EnvironmentDigest: environmentDigest, ClearEnvironment: true, KillProcessGroup: true,
			CompilationDatabase: &compileArtifact,
			Entries: []semanticir.WorkspaceEntry{
				{Path: sourceName, Artifact: artifact, Provenance: provenance},
				{Path: compileArtifact.Path, Artifact: compileArtifact, Provenance: compileProvenance},
			},
			Provenance: provenance,
		},
	}
}

func frontendCPPTestRunner(t *testing.T, request *semanticir.FrontendRequest, testIDs []string) {
	t.Helper()
	ids := append([]string(nil), testIDs...)
	sort.Strings(ids)
	filter := "--gtest_filter=" + strings.Join(ids, ":")
	var runnerSource strings.Builder
	runnerSource.Write(request.Source)
	runnerSource.WriteString("\nint classify(int x) { return x + 1; }\n#include <cstring>\n")
	runnerSource.WriteString("int main(int argc, char **argv) {\n")
	runnerSource.WriteString("  if (argc != 2 || std::strcmp(argv[1], ")
	runnerSource.WriteString(strconv.Quote(filter))
	runnerSource.WriteString(") != 0) return 2;\n")
	for _, id := range ids {
		runnerSource.WriteString("  ")
		runnerSource.WriteString(id)
		runnerSource.WriteString("();\n")
	}
	runnerSource.WriteString("  return 0;\n}\n")
	runnerPath := filepath.Join(request.Workspace.Root, "cpp-gtest-runner")
	command := exec.Command(request.Translator.Path, "-x", "c++", "-std=c++20", "-o", runnerPath, "-")
	command.Stdin = strings.NewReader(runnerSource.String())
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile exact C++ test runner: %v: %s", err, output)
	}
	runnerBytes, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatal(err)
	}
	runner := semanticir.ToolRef{Name: "gtest", Path: runnerPath, Digest: semanticir.DigestBytes(runnerBytes), Version: "ray-cpp-gtest-fixture-v1"}
	run := exec.Command(runnerPath, filter)
	run.Env = []string{"PATH=" + os.Getenv("PATH")}
	run.Dir = request.Workspace.Root
	if output, err := run.CombinedOutput(); err != nil {
		t.Fatalf("execute exact C++ test runner: %v: %s", err, output)
	}
	configurationBytes, err := json.Marshal(struct {
		Schema       string   `json:"schema"`
		SourceDigest string   `json:"source_digest"`
		TestIDs      []string `json:"test_ids"`
		RunnerDigest string   `json:"runner_digest"`
	}{"ray-cpp-gtest-runner-v1", request.Artifact.Digest, ids, runner.Digest})
	if err != nil {
		t.Fatal(err)
	}
	configurationPath := "cpp-gtest-runner.json"
	if err := os.WriteFile(filepath.Join(request.Workspace.Root, configurationPath), configurationBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	configuration := semanticir.ArtifactRef{ID: "cpp-gtest-configuration", Kind: semanticir.ArtifactConfiguration, Path: configurationPath, Digest: semanticir.DigestBytes(configurationBytes)}
	configurationProvenance := semanticir.NewProvenance(configuration, semanticir.SourceLocation{Path: configuration.Path, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1}, semanticir.TranslationTranslated)
	runnerArtifact := semanticir.ArtifactRef{ID: "cpp-gtest-runner", Kind: semanticir.ArtifactEnvironment, Path: filepath.Base(runnerPath), Digest: runner.Digest}
	runnerProvenance := semanticir.NewProvenance(runnerArtifact, semanticir.SourceLocation{Path: runnerArtifact.Path, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1}, semanticir.TranslationTranslated)
	request.Workspace.Entries = append(request.Workspace.Entries,
		semanticir.WorkspaceEntry{Path: runnerArtifact.Path, Artifact: runnerArtifact, Provenance: runnerProvenance},
		semanticir.WorkspaceEntry{Path: configuration.Path, Artifact: configuration, Provenance: configurationProvenance},
	)
	request.Runner = runner
	request.Configuration = &configuration
	request.RunnerCommand = &semanticir.WorkspaceCommand{
		ID: "cpp-gtest-run", WorkspaceID: request.Workspace.ID, State: request.Workspace.State, TreeDigest: request.Workspace.TreeDigest,
		Command: runner.Path + " " + filter, WorkingDirectory: request.Workspace.WorkingDirectory,
		Environment: append([]semanticir.EnvironmentVariable(nil), request.Workspace.Environment...), EnvironmentDigest: request.Workspace.EnvironmentDigest,
		ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 30_000,
		PassSignal:   semanticir.PassSignal{Kind: semanticir.PassSignalExitCode, Expected: "0", Provenance: configurationProvenance},
		ExpectedPass: true, ObservedPass: true, ExitCode: 0, StdoutDigest: semanticir.DigestBytes(nil), StderrDigest: semanticir.DigestBytes(nil), SignalValueDigest: semanticir.DigestBytes([]byte("0")),
		Tools: []semanticir.ToolRef{runner}, Provenance: configurationProvenance,
	}
}

func frontendCPPSharedTestRunner(t *testing.T, requests []*semanticir.FrontendRequest, testIDs []string) {
	t.Helper()
	if len(requests) == 0 {
		t.Fatal("shared C++ runner has no test artifacts")
	}
	ids := append([]string(nil), testIDs...)
	sort.Strings(ids)
	filter := "--gtest_filter=" + strings.Join(ids, ":")
	var runnerSource strings.Builder
	for _, request := range requests {
		runnerSource.Write(request.Source)
		runnerSource.WriteByte('\n')
	}
	runnerSource.WriteString("int classify(int x) { return x + 1; }\n#include <cstring>\n")
	runnerSource.WriteString("int main(int argc, char **argv) {\n")
	runnerSource.WriteString("  if (argc != 2 || std::strcmp(argv[1], ")
	runnerSource.WriteString(strconv.Quote(filter))
	runnerSource.WriteString(") != 0) return 2;\n")
	for _, id := range ids {
		runnerSource.WriteString("  ")
		runnerSource.WriteString(id)
		runnerSource.WriteString("();\n")
	}
	runnerSource.WriteString("  return 0;\n}\n")
	runnerPath := filepath.Join(requests[0].Workspace.Root, "cpp-shared-gtest-runner")
	command := exec.Command(requests[0].Translator.Path, "-x", "c++", "-std=c++20", "-o", runnerPath, "-")
	command.Stdin = strings.NewReader(runnerSource.String())
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile shared C++ test runner: %v: %s", err, output)
	}
	runnerBytes, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatal(err)
	}
	runner := semanticir.ToolRef{Name: "gtest", Path: runnerPath, Digest: semanticir.DigestBytes(runnerBytes), Version: "ray-cpp-shared-gtest-fixture-v1"}
	run := exec.Command(runnerPath, filter)
	run.Env = []string{"PATH=" + os.Getenv("PATH")}
	run.Dir = requests[0].Workspace.Root
	if output, err := run.CombinedOutput(); err != nil {
		t.Fatalf("execute shared C++ test runner: %v: %s", err, output)
	}
	configurationBytes, err := json.Marshal(struct {
		Schema       string   `json:"schema"`
		TestIDs      []string `json:"test_ids"`
		RunnerDigest string   `json:"runner_digest"`
	}{"ray-cpp-shared-gtest-runner-v1", ids, runner.Digest})
	if err != nil {
		t.Fatal(err)
	}
	configuration := semanticir.ArtifactRef{ID: "cpp-shared-gtest-configuration", Kind: semanticir.ArtifactConfiguration, Path: "cpp-shared-gtest-runner.json", Digest: semanticir.DigestBytes(configurationBytes)}
	for _, request := range requests {
		localRunnerPath := filepath.Join(request.Workspace.Root, filepath.Base(runnerPath))
		if localRunnerPath != runnerPath {
			if err := os.WriteFile(localRunnerPath, runnerBytes, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(request.Workspace.Root, configuration.Path), configurationBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		configurationProvenance := semanticir.NewProvenance(configuration, semanticir.SourceLocation{Path: configuration.Path, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1}, semanticir.TranslationTranslated)
		localRunner := runner
		localRunner.Path = localRunnerPath
		runnerArtifact := semanticir.ArtifactRef{ID: request.Artifact.ID + ":shared-gtest-runner", Kind: semanticir.ArtifactEnvironment, Path: filepath.Base(localRunnerPath), Digest: localRunner.Digest}
		runnerProvenance := semanticir.NewProvenance(runnerArtifact, semanticir.SourceLocation{Path: runnerArtifact.Path, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1}, semanticir.TranslationTranslated)
		request.Workspace.Entries = append(request.Workspace.Entries,
			semanticir.WorkspaceEntry{Path: configuration.Path, Artifact: configuration, Provenance: configurationProvenance},
			semanticir.WorkspaceEntry{Path: runnerArtifact.Path, Artifact: runnerArtifact, Provenance: runnerProvenance},
		)
		request.Runner = localRunner
		request.Configuration = &configuration
		request.RunnerCommand = &semanticir.WorkspaceCommand{
			ID: "cpp-shared-gtest-run", WorkspaceID: request.Workspace.ID, State: request.Workspace.State, TreeDigest: request.Workspace.TreeDigest,
			Command: localRunner.Path + " " + filter, WorkingDirectory: request.Workspace.WorkingDirectory,
			Environment: append([]semanticir.EnvironmentVariable(nil), request.Workspace.Environment...), EnvironmentDigest: request.Workspace.EnvironmentDigest,
			ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 30_000,
			PassSignal:   semanticir.PassSignal{Kind: semanticir.PassSignalExitCode, Expected: "0", Provenance: configurationProvenance},
			ExpectedPass: true, ObservedPass: true, ExitCode: 0, StdoutDigest: semanticir.DigestBytes(nil), StderrDigest: semanticir.DigestBytes(nil), SignalValueDigest: semanticir.DigestBytes([]byte("0")),
			Tools: []semanticir.ToolRef{localRunner}, Provenance: configurationProvenance,
		}
	}
}

func cppRequireNoErrors(t *testing.T, diagnostics []semanticir.Diagnostic) {
	t.Helper()
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("unexpected C++ frontend errors: %+v", diagnostics)
	}
}

func cppBindOutcomeProvenance(request semanticir.FrontendRequest, outcome semanticir.ObservableOutcome) semanticir.ObservableOutcome {
	outcome.Provenance = semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{
		Path: request.Artifact.Path, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1,
	}, semanticir.TranslationTranslated)
	return outcome
}

func frontendCPPBindExactGroundings(t *testing.T, request *semanticir.FrontendRequest) {
	t.Helper()
	provenance := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{
		Path: request.Artifact.Path, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1,
	}, semanticir.TranslationTranslated)
	for operationIndex := range request.Operations {
		operation := &request.Operations[operationIndex]
		if len(operation.Inputs) == 0 {
			for _, domainID := range operation.DomainIDs {
				var domain *semanticir.Domain
				for index := range request.FiniteDomains {
					if request.FiniteDomains[index].ID == domainID {
						domain = &request.FiniteDomains[index]
					}
				}
				if domain == nil {
					t.Fatalf("missing fixture domain %q", domainID)
				}
				operation.Inputs = append(operation.Inputs, semanticir.Variable{Name: domainID, Type: domain.Type, DomainID: domainID, Provenance: provenance})
			}
		}
		operation.Provenance = provenance
		for domainIndex := range request.FiniteDomains {
			domain := &request.FiniteDomains[domainIndex]
			if !slices.Contains(operation.DomainIDs, domain.ID) {
				continue
			}
			var input semanticir.Variable
			for _, candidate := range operation.Inputs {
				if candidate.DomainID == domain.ID {
					input = candidate
				}
			}
			if input.Name == "" {
				t.Fatalf("fixture operation %q has no input for domain %q", operation.ID, domain.ID)
			}
			for valueIndex := range domain.Values {
				value := &domain.Values[valueIndex]
				literal, ok := value.TypedValue(*domain)
				if !ok {
					t.Fatalf("fixture domain %q value %q has no literal", domain.ID, value.ID)
				}
				membership := semanticir.Expression{
					Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ,
					Operands: []semanticir.Expression{
						{Kind: semanticir.ExprVariable, Type: input.Type, Name: input.Name, Provenance: provenance},
						{Kind: semanticir.ExprLiteral, Type: literal.Type, Literal: &literal, Provenance: provenance},
					}, Provenance: provenance,
				}
				value.Groundings = append(value.Groundings, semanticir.GroundingAxiom{
					OperationID: operation.ID, Kind: semanticir.GroundingMembership, Membership: &membership,
					ConcreteWitness: map[string]semanticir.Literal{input.Name: literal}, Provenance: provenance,
				})
			}
		}
		for _, assignment := range semanticir.EnumerateAssignments(request.FiniteDomains) {
			inputs := map[string]semanticir.Literal{}
			for _, input := range operation.Inputs {
				for _, domain := range request.FiniteDomains {
					if domain.ID != input.DomainID {
						continue
					}
					for _, value := range domain.Values {
						if value.ID == assignment[domain.ID] && value.Value != nil {
							inputs[input.Name] = *value.Value
						}
					}
				}
			}
			conditions := semanticir.Assignment{}
			for _, domainID := range operation.DomainIDs {
				conditions[domainID] = assignment[domainID]
			}
			request.Groundings = append(request.Groundings, semanticir.AssignmentGrounding{
				ID: semanticir.AssignmentGroundingID(operation.ID, conditions), OperationID: operation.ID,
				Conditions: conditions, Inputs: inputs, Provenance: provenance,
			})
		}
	}
}

func TestFrontendCPP(t *testing.T) {
	t.Run("proven fixed scalar loop", func(t *testing.T) {
		source := `int sum_fixed() {
    int sum = 0;
    for (int i = 0; i < 3; ++i) {
        sum += i;
    }
    return sum;
}
`
		request := frontendCPPRequest(t, semanticir.ArtifactCode, source, nil, "sum_fixed")
		outcome := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 3}, OperationID: "sum_fixed"}
		outcome.ID = semanticir.OutcomeID(outcome)
		outcome = cppBindOutcomeProvenance(request, outcome)
		request.Outcomes = []semanticir.ObservableOutcome{outcome}
		request.Operations = []semanticir.Operation{{ID: "sum_fixed", Kind: semanticir.OperationCallable, OutcomeIDs: []string{outcome.ID}}}
		frontendCPPBindExactGroundings(t, &request)
		model, diagnostics := frontendcpp.Translate(context.Background(), request)
		cppRequireNoErrors(t, diagnostics)
		if model.Coverage.Status != semanticir.TranslationComplete || len(model.Cases) != 1 || len(model.ExhaustiveEvidence) != 1 {
			t.Fatalf("bounded loop did not produce one complete compiler-grounded case: coverage=%s cases=%d evidence=%d", model.Coverage.Status, len(model.Cases), len(model.ExhaustiveEvidence))
		}
		if len(model.Operations) != 1 || len(model.Operations[0].Body) != 1 || model.Operations[0].Body[0].Kind != semanticir.StmtReturn {
			t.Fatalf("bounded loop was not exactly unrolled into its terminal expression: %+v", model.Operations)
		}
		if validation := semanticir.ValidateArtifactScope(request, model); semanticir.HasErrors(validation) {
			t.Fatalf("bounded-loop model failed central scope validation: %+v", validation)
		}
	})

	t.Run("control flow calls throws and outcomes", func(t *testing.T) {
		source := `int helper(int x) {
    return x + 1;
}

int classify(int x) {
    if (x > 0 && x <= 2) {
        return helper(x);
    }
    if (x == 0) {
        throw 0;
    }
    switch (x) {
    case -1:
        return -1;
    default:
        return 3;
    }
}
`
		request := frontendCPPRequest(t, semanticir.ArtifactCode, source, []semanticir.Domain{cppDomain("x", "-1", "0", "1", "2")}, "helper", "classify")
		integerOutcome := func(operationID string, value int64) semanticir.ObservableOutcome {
			outcome := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: value}, OperationID: operationID}
			outcome.ID = semanticir.OutcomeID(outcome)
			return cppBindOutcomeProvenance(request, outcome)
		}
		helperZero, helperOne := integerOutcome("helper", 0), integerOutcome("helper", 1)
		helperTwo, helperThree := integerOutcome("helper", 2), integerOutcome("helper", 3)
		classifyNegative, classifyTwo := integerOutcome("classify", -1), integerOutcome("classify", 2)
		classifyThree := integerOutcome("classify", 3)
		raise := semanticir.ObservableOutcome{Kind: semanticir.OutcomeRaise, ExceptionType: "int", OperationID: "classify"}
		raise.ID = semanticir.OutcomeID(raise)
		raise = cppBindOutcomeProvenance(request, raise)
		request.Outcomes = []semanticir.ObservableOutcome{helperZero, helperOne, helperTwo, helperThree, classifyNegative, classifyTwo, classifyThree, raise}
		request.Operations = []semanticir.Operation{
			{ID: "helper", Kind: semanticir.OperationCallable, DomainIDs: []string{"x"}, OutcomeIDs: []string{helperZero.ID, helperOne.ID, helperTwo.ID, helperThree.ID}},
			{ID: "classify", Kind: semanticir.OperationCallable, DomainIDs: []string{"x"}, OutcomeIDs: []string{classifyNegative.ID, classifyTwo.ID, classifyThree.ID, raise.ID}},
		}
		frontendCPPBindExactGroundings(t, &request)
		model, diagnostics := frontendcpp.Translate(context.Background(), request)
		cppRequireNoErrors(t, diagnostics)
		if model.Coverage.Status != semanticir.TranslationComplete {
			t.Fatalf("coverage = %q: %+v", model.Coverage.Status, model.Coverage.Unsupported)
		}
		if model.Translator != request.Translator {
			t.Fatalf("translator identity not retained: %+v", model.Translator)
		}
		if validation := semanticir.ValidateArtifactModel(model); semanticir.HasErrors(validation) {
			t.Fatalf("compiler-grounded model validation failed: %+v", validation)
		}
		if validation := semanticir.ValidateArtifactScope(request, model); semanticir.HasErrors(validation) {
			t.Fatalf("compiler-grounded model scope failed: %+v", validation)
		}
		if len(model.Operations) != 2 || len(model.Cases) != 8 {
			t.Fatalf("operations=%d cases=%d, want 2 and 8", len(model.Operations), len(model.Cases))
		}
		var classify semanticir.Operation
		for _, operation := range model.Operations {
			if operation.ID == "classify" {
				classify = operation
			}
		}
		if len(classify.DomainIDs) != 1 || classify.DomainIDs[0] != "x" || len(classify.OutcomeIDs) < 3 {
			t.Fatalf("classify local universe = domains %v outcomes %v", classify.DomainIDs, classify.OutcomeIDs)
		}
		if len(classify.Body) < 3 || classify.Body[0].Kind != semanticir.StmtBranch || classify.Body[2].Kind != semanticir.StmtBranch {
			t.Fatalf("calls/if/switch did not lower into typed statements: %+v", classify.Body)
		}
		if classify.Body[0].Condition == nil || classify.Body[0].Condition.Kind != semanticir.ExprBool || classify.Body[0].Condition.Operator != semanticir.OpAnd {
			t.Fatalf("comparisons did not lower: %+v", classify.Body[0].Condition)
		}
		if len(classify.Body[0].Then) != 1 || classify.Body[0].Then[0].Value == nil || classify.Body[0].Then[0].Value.Kind != semanticir.ExprCall || classify.Body[0].Then[0].Value.Name != "helper" {
			t.Fatalf("nested source call did not lower exactly: %+v", classify.Body[0].Then)
		}
		if classify.Body[0].Provenance.Location.StartLine != 6 || classify.Body[2].Provenance.Location.StartLine != 13 {
			t.Fatalf("control-flow provenance is not exact: if=%+v switch=%+v", classify.Body[0].Provenance.Location, classify.Body[2].Provenance.Location)
		}
		var sawRaise bool
		for _, outcome := range model.Outcomes {
			if outcome.Kind == semanticir.OutcomeRaise && outcome.ExceptionType == "int" {
				sawRaise = true
			}
			if outcome.ID != semanticir.OutcomeID(outcome) {
				t.Fatalf("non-canonical outcome id: %+v", outcome)
			}
		}
		if !sawRaise {
			t.Fatalf("throw outcome missing: %+v", model.Outcomes)
		}
	})

	t.Run("common assertions retain a global cross-case predicate", func(t *testing.T) {
		source := `#include <cassert>
int classify(int);

void test_relations() {
    assert(classify(1) == 2);
    assert(classify(1) != classify(2));
}

void test_plain_assert() {
    assert(classify(1) == 2);
}
`
		request := frontendCPPRequest(t, semanticir.ArtifactTests, source, []semanticir.Domain{cppDomain("x", "0", "1", "2")}, "classify")
		returnTwo := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 2}, OperationID: "classify"}
		returnTwo.ID = semanticir.OutcomeID(returnTwo)
		returnThree := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 3}, OperationID: "classify"}
		returnThree.ID = semanticir.OutcomeID(returnThree)
		request.Operations = []semanticir.Operation{{ID: "classify", Kind: semanticir.OperationCallable, DomainIDs: []string{"x"}, OutcomeIDs: []string{returnTwo.ID, returnThree.ID}}}
		frontendCPPBindExactGroundings(t, &request)
		request.Outcomes = []semanticir.ObservableOutcome{cppBindOutcomeProvenance(request, returnTwo), cppBindOutcomeProvenance(request, returnThree)}
		frontendCPPTestRunner(t, &request, []string{"test_plain_assert", "test_relations"})
		model, diagnostics := frontendcpp.Translate(context.Background(), request)
		cppRequireNoErrors(t, diagnostics)
		if model.Coverage.Status != semanticir.TranslationComplete || len(model.Tests) != 2 {
			t.Fatalf("tests=%d coverage=%q unsupported=%+v", len(model.Tests), model.Coverage.Status, model.Coverage.Unsupported)
		}
		var relational, plain semanticir.TestModel
		for _, test := range model.Tests {
			switch test.ID {
			case "test_relations":
				relational = test
			case "test_plain_assert":
				plain = test
			}
		}
		if relational.Predicate.Kind != semanticir.PredicateAnd || len(relational.Predicate.Children) != 2 {
			t.Fatalf("relational predicate not conjoined: %+v", relational.Predicate)
		}
		second := relational.Predicate.Children[1]
		if second.Kind != semanticir.PredicateNot || len(second.Children) != 1 || second.Children[0].Kind != semanticir.PredicateOutcomeEqual {
			t.Fatalf("cross-case inequality was flattened: %+v", second)
		}
		if plain.Predicate.Kind != semanticir.PredicateOutcomeIn || plain.Predicate.Observe == nil || plain.Predicate.Observe.Behavior.Conditions["x"] != "1" {
			t.Fatalf("plain assert predicate = %+v", plain.Predicate)
		}
		for _, test := range model.Tests {
			if test.Predicate.Kind == "" || test.Provenance.Location.StartLine == 0 {
				t.Fatalf("test predicate/provenance incomplete: %+v", test)
			}
		}
		if model.TestProjection == nil || model.RunnerSelection == nil {
			t.Fatalf("compiler-derived test evidence is missing: projection=%+v runner=%+v", model.TestProjection, model.RunnerSelection)
		}
		if validation := semanticir.ValidateArtifactModel(model); semanticir.HasErrors(validation) {
			t.Fatalf("compiler-derived C++ test model invalid: %+v", validation)
		}
		if validation := semanticir.ValidateArtifactScope(request, model); semanticir.HasErrors(validation) {
			t.Fatalf("compiler-derived C++ test scope invalid: %+v", validation)
		}
	})
}

func TestFrontendCPPVerifierRelational(t *testing.T) {
	publicSource := `#include <cassert>
int classify(int);
void public_check() { assert(classify(0) == 1); }
`
	hiddenSource := `#include <cassert>
int classify(int);
void hidden_check() { assert(classify(1) == 2); }
`
	publicRequest := frontendCPPRequest(t, semanticir.ArtifactTests, publicSource, []semanticir.Domain{cppDomain("x", "0", "1")}, "classify")
	hiddenRequest := frontendCPPRequest(t, semanticir.ArtifactTests, hiddenSource, []semanticir.Domain{cppDomain("x", "0", "1")}, "classify")
	for _, request := range []*semanticir.FrontendRequest{&publicRequest, &hiddenRequest} {
		returnOne := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 1}, OperationID: "classify"}
		returnOne.ID = semanticir.OutcomeID(returnOne)
		returnTwo := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 2}, OperationID: "classify"}
		returnTwo.ID = semanticir.OutcomeID(returnTwo)
		request.Operations = []semanticir.Operation{{ID: "classify", Kind: semanticir.OperationCallable, DomainIDs: []string{"x"}, OutcomeIDs: []string{returnOne.ID, returnTwo.ID}}}
		frontendCPPBindExactGroundings(t, request)
		request.Outcomes = []semanticir.ObservableOutcome{cppBindOutcomeProvenance(*request, returnOne), cppBindOutcomeProvenance(*request, returnTwo)}
	}
	frontendCPPSharedTestRunner(t, []*semanticir.FrontendRequest{&publicRequest, &hiddenRequest}, []string{"hidden_check", "public_check"})
	models := make([]semanticir.ArtifactModel, 0, 2)
	for _, request := range []semanticir.FrontendRequest{publicRequest, hiddenRequest} {
		model, diagnostics := frontendcpp.Translate(context.Background(), request)
		cppRequireNoErrors(t, diagnostics)
		if model.Coverage.Status != semanticir.TranslationComplete || len(model.Tests) != 1 || model.RunnerSelection == nil || len(model.RunnerSelection.TestIDs) != 1 {
			t.Fatalf("shared runner did not retain one independent artifact predicate: coverage=%s tests=%d", model.Coverage.Status, len(model.Tests))
		}
		if !strings.HasSuffix(model.RunnerSelection.Command.Command, " --gtest_filter=hidden_check:public_check") {
			t.Fatalf("artifact runner command did not bind the same global public+hidden filter: %q", model.RunnerSelection.Command.Command)
		}
		if validation := semanticir.ValidateArtifactScope(request, model); semanticir.HasErrors(validation) {
			t.Fatalf("shared-runner artifact scope failed: %+v", validation)
		}
		models = append(models, model)
	}
	if models[0].Artifact.ID == models[1].Artifact.ID || models[0].Tests[0].ID == models[1].Tests[0].ID {
		t.Fatalf("public/hidden C++ verifier artifacts or tests are not globally unique")
	}
}

func TestFrontendCPPBlocked(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		domains []semanticir.Domain
		entry   string
	}{
		{"unexpanded template", "template <typename T> T identity(T value) { return value; }\n", nil, "identity"},
		{"inline assembly", "void spin() { asm(\"nop\"); }\n", nil, "spin"},
		{"uncontrolled division", "int divide(int x) { return 1 / x; }\n", []semanticir.Domain{cppDomain("x", "0", "1")}, "divide"},
		{"signed overflow", "int increment(int x) { return x + 1; }\n", []semanticir.Domain{cppDomain("x", "2147483647")}, "increment"},
		{"unsigned wrap semantics", "unsigned increment(unsigned x) { return x + 1; }\n", []semanticir.Domain{cppDomain("x", "0", "1")}, "increment"},
		{"untranslated mutation", "int mutate(int x) { x += 1; return x; }\n", []semanticir.Domain{cppDomain("x", "0", "1")}, "mutate"},
		{"input dependent loop bound", "int sum_to(int n) { int sum = 0; for (int i = 0; i < n; ++i) sum += i; return sum; }\n", []semanticir.Domain{cppDomain("n", "0", "2")}, "sum_to"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := frontendCPPRequest(t, semanticir.ArtifactCode, test.source, test.domains, test.entry)
			model, diagnostics := frontendcpp.Translate(context.Background(), request)
			if !semanticir.HasErrors(diagnostics) {
				t.Fatalf("unsupported C++ produced no blocking diagnostic: %+v", model)
			}
			if model.Coverage.Status != semanticir.TranslationBlocked || len(model.Coverage.Unsupported) == 0 {
				t.Fatalf("coverage did not fail closed: %+v", model.Coverage)
			}
		})
	}

	t.Run("missing frozen compilation database", func(t *testing.T) {
		request := frontendCPPRequest(t, semanticir.ArtifactCode, "int f() { return 1; }\n", nil, "f")
		request.Workspace.CompilationDatabase = nil
		model, diagnostics := frontendcpp.Translate(context.Background(), request)
		if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked {
			t.Fatalf("missing compile context did not block: model=%+v diagnostics=%+v", model, diagnostics)
		}
	})

	t.Run("semantic options are rejected", func(t *testing.T) {
		request := frontendCPPRequest(t, semanticir.ArtifactCode, "int f() { return 1; }\n", nil, "f")
		request.Options = map[string]string{"domain.f.x": "x"}
		model, diagnostics := frontendcpp.Translate(context.Background(), request)
		if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked {
			t.Fatalf("ignored semantic option did not fail closed: model=%+v diagnostics=%+v", model, diagnostics)
		}
	})

	t.Run("spec constraint without compiler no-path evidence is rejected", func(t *testing.T) {
		request := frontendCPPRequest(t, semanticir.ArtifactCode, "int f() { return 1; }\n", nil, "f")
		request.Constraints = []semanticir.Constraint{{ID: "excluded", OperationID: "f", Conditions: semanticir.Assignment{}}}
		model, diagnostics := frontendcpp.Translate(context.Background(), request)
		if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked {
			t.Fatalf("spec-seeded reachability exclusion did not fail closed: model=%+v diagnostics=%+v", model, diagnostics)
		}
	})

	t.Run("ambient environment is rejected", func(t *testing.T) {
		request := frontendCPPRequest(t, semanticir.ArtifactCode, "int f() { return 1; }\n", nil, "f")
		request.Workspace.ClearEnvironment = false
		model, diagnostics := frontendcpp.Translate(context.Background(), request)
		if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked {
			t.Fatalf("ambient compiler environment did not block: model=%+v diagnostics=%+v", model, diagnostics)
		}
	})

	t.Run("environment digest mismatch is rejected", func(t *testing.T) {
		request := frontendCPPRequest(t, semanticir.ArtifactCode, "int f() { return 1; }\n", nil, "f")
		request.Workspace.EnvironmentDigest = semanticir.DigestBytes([]byte("detached environment"))
		model, diagnostics := frontendcpp.Translate(context.Background(), request)
		if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked {
			t.Fatalf("detached compiler environment did not block: model=%+v diagnostics=%+v", model, diagnostics)
		}
	})

	t.Run("one concrete assertion cannot stand for a non-singleton category", func(t *testing.T) {
		source := "#include <cassert>\nint classify(int);\nvoid test_nonnegative() { assert(classify(0) == 1); }\n"
		domain := semanticir.Domain{ID: "classify.case", Type: semanticir.TypeString, Values: []semanticir.DomainValue{{ID: "nonnegative"}}}
		request := frontendCPPRequest(t, semanticir.ArtifactTests, source, []semanticir.Domain{domain}, "classify")
		provenance := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1, EndLine: 3, EndColumn: 1}, semanticir.TranslationTranslated)
		zero := semanticir.Literal{Type: semanticir.TypeInteger, Integer: 0}
		membership := semanticir.Expression{
			Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpGE,
			Operands: []semanticir.Expression{
				{Kind: semanticir.ExprVariable, Type: semanticir.TypeInteger, Name: "x", Provenance: provenance},
				{Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &zero, Provenance: provenance},
			}, Provenance: provenance,
		}
		request.FiniteDomains[0].Values[0].Groundings = []semanticir.GroundingAxiom{{
			OperationID: "classify", Kind: semanticir.GroundingMembership, Membership: &membership,
			ConcreteWitness: map[string]semanticir.Literal{"x": zero}, Provenance: provenance,
		}}
		outcome := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 1}, OperationID: "classify", Provenance: provenance}
		outcome.ID = semanticir.OutcomeID(outcome)
		request.Outcomes = []semanticir.ObservableOutcome{outcome}
		request.Operations = []semanticir.Operation{{
			ID: "classify", Kind: semanticir.OperationCallable, DomainIDs: []string{"classify.case"}, OutcomeIDs: []string{outcome.ID},
			Inputs: []semanticir.Variable{{Name: "x", Type: semanticir.TypeInteger, Provenance: provenance}}, Provenance: provenance,
		}}
		model, diagnostics := frontendcpp.Translate(context.Background(), request)
		if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked {
			t.Fatalf("one x=0 test unsoundly projected over category x>=0: diagnostics=%+v", diagnostics)
		}
		if len(model.Tests) != 0 {
			t.Fatalf("blocked category assertion still emitted authoritative tests: %+v", model.Tests)
		}
	})
}

func TestFrontendCPPMaterialize(t *testing.T) {
	source := `bool decide(bool flag) {
    if (flag) {
        return true;
    }
    return false;
}
`
	request := frontendCPPRequest(t, semanticir.ArtifactCode, source, []semanticir.Domain{cppDomain("flag", "true", "false")}, "decide")
	falseOutcome := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeBool, Bool: false}, OperationID: "decide"}
	falseOutcome.ID = semanticir.OutcomeID(falseOutcome)
	trueDeclared := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeBool, Bool: true}, OperationID: "decide"}
	trueDeclared.ID = semanticir.OutcomeID(trueDeclared)
	request.Outcomes = []semanticir.ObservableOutcome{cppBindOutcomeProvenance(request, falseOutcome), cppBindOutcomeProvenance(request, trueDeclared)}
	request.Operations = []semanticir.Operation{{ID: "decide", Kind: semanticir.OperationCallable, DomainIDs: []string{"flag"}, OutcomeIDs: []string{falseOutcome.ID, trueDeclared.ID}}}
	frontendCPPBindExactGroundings(t, &request)
	model, diagnostics := frontendcpp.Translate(context.Background(), request)
	cppRequireNoErrors(t, diagnostics)
	var trueOutcome string
	for _, outcome := range model.Outcomes {
		if outcome.Kind == semanticir.OutcomeReturn && outcome.Value != nil && outcome.Value.Type == semanticir.TypeBool && outcome.Value.Bool {
			trueOutcome = outcome.ID
		}
	}
	if trueOutcome == "" {
		t.Fatalf("true outcome missing: %+v", model.Outcomes)
	}
	witness := semanticir.Counterexample{
		ID: "cpp-false-to-true", Obligation: semanticir.ObligationTestsSound,
		OperationID: "decide", Conditions: semanticir.Assignment{"flag": "false"}, TestPasses: true,
		ObservedOutcomes: []string{trueOutcome},
		Choices: []semanticir.BehaviorChoice{{Behavior: semanticir.BehaviorRef{
			OperationID: "decide", Conditions: semanticir.Assignment{"flag": "false"}, Inputs: map[string]semanticir.Literal{"flag": {Type: semanticir.TypeBool, Bool: false}},
		}, OutcomeID: trueOutcome}},
	}
	plan, diagnostics := frontendcpp.Materialize(context.Background(), semanticir.MaterializationRequest{
		Frontend: request, Task: &semanticir.Task{Operations: model.Operations, Outcomes: model.Outcomes}, Model: model, Counterexample: witness,
	})
	cppRequireNoErrors(t, diagnostics)
	if len(plan.Edits) != 1 {
		t.Fatalf("edits = %+v, want one", plan.Edits)
	}
	edit := plan.Edits[0]
	if string(edit.ExpectedBytes) != "return false" || string(edit.Replacement) != "return true" {
		t.Fatalf("materialized edit = %q -> %q", edit.ExpectedBytes, edit.Replacement)
	}
	if plan.Artifact.Digest != request.Artifact.Digest || plan.Provenance.ArtifactDigest != request.Artifact.Digest || plan.Provenance.Translation != semanticir.TranslationTranslated {
		t.Fatalf("plan is not digest anchored: %+v", plan)
	}
	mutated := append([]byte(nil), request.Source...)
	mutated = append(mutated[:edit.StartByte], append(edit.Replacement, mutated[edit.EndByte:]...)...)
	if !strings.Contains(string(mutated), "return true;\n}\n") {
		t.Fatalf("unexpected materialized source:\n%s", mutated)
	}

	blockedWitness := witness
	blockedWitness.ID = "cpp-unknown-outcome"
	blockedWitness.Choices = append([]semanticir.BehaviorChoice(nil), witness.Choices...)
	blockedWitness.Choices[0].OutcomeID = "outcome-not-in-task"
	blocked, blockedDiagnostics := frontendcpp.Materialize(context.Background(), semanticir.MaterializationRequest{
		Frontend: request, Task: &semanticir.Task{Operations: model.Operations, Outcomes: model.Outcomes}, Model: model, Counterexample: blockedWitness,
	})
	if !semanticir.HasErrors(blockedDiagnostics) || len(blocked.Edits) != 0 {
		t.Fatalf("unrenderable outcome did not block: plan=%+v diagnostics=%+v", blocked, blockedDiagnostics)
	}
}

func TestFrontendCPPGenerateProbe(t *testing.T) {
	source := `bool decide(bool flag) {
    return flag;
}
`
	request := frontendCPPRequest(t, semanticir.ArtifactCode, source, []semanticir.Domain{cppDomain("flag", "true", "false")}, "decide")
	falseOutcome := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeBool, Bool: false}, OperationID: "decide"}
	falseOutcome.ID = semanticir.OutcomeID(falseOutcome)
	trueOutcome := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeBool, Bool: true}, OperationID: "decide"}
	trueOutcome.ID = semanticir.OutcomeID(trueOutcome)
	request.Outcomes = []semanticir.ObservableOutcome{cppBindOutcomeProvenance(request, falseOutcome), cppBindOutcomeProvenance(request, trueOutcome)}
	request.Operations = []semanticir.Operation{{ID: "decide", Kind: semanticir.OperationCallable, DomainIDs: []string{"flag"}, OutcomeIDs: []string{falseOutcome.ID, trueOutcome.ID}}}
	frontendCPPBindExactGroundings(t, &request)
	treeDigest, err := executor.WorkspaceDigest(request.Workspace.Root)
	if err != nil {
		t.Fatal(err)
	}
	request.Workspace.TreeDigest = treeDigest
	model, diagnostics := frontendcpp.Translate(context.Background(), request)
	cppRequireNoErrors(t, diagnostics)
	if validation := semanticir.ValidateArtifactModel(model); semanticir.HasErrors(validation) {
		t.Fatalf("probe source model invalid: %+v", validation)
	}
	choices := make([]semanticir.BehaviorChoice, 0, len(model.Cases))
	observed := make([]string, 0, len(model.Cases))
	for _, behaviorCase := range model.Cases {
		var inputs map[string]semanticir.Literal
		for _, grounding := range model.Groundings {
			if grounding.OperationID == behaviorCase.OperationID && reflect.DeepEqual(grounding.Conditions, behaviorCase.Conditions) {
				inputs = grounding.Inputs
			}
		}
		if inputs == nil {
			t.Fatalf("case has no exact full input map: %+v", behaviorCase)
		}
		choices = append(choices, semanticir.BehaviorChoice{
			Behavior:  semanticir.BehaviorRef{OperationID: behaviorCase.OperationID, Conditions: behaviorCase.Conditions, Inputs: inputs, Provenance: behaviorCase.Provenance},
			OutcomeID: behaviorCase.OutcomeIDs[0],
		})
		observed = append(observed, behaviorCase.OutcomeIDs[0])
	}
	if len(choices) != 2 {
		t.Fatalf("probe choices=%d, want complete two-case vector", len(choices))
	}
	witness := semanticir.Counterexample{
		ID: "cpp-reference-probe", Obligation: semanticir.ObligationReferenceCorrectness,
		OperationID: choices[0].Behavior.OperationID, Conditions: choices[0].Behavior.Conditions,
		Choices: choices, ObservedOutcomes: observed, ExpectedOutcomes: append([]string(nil), observed...),
		Provenance: model.Cases[0].Provenance,
	}
	materialization := semanticir.MaterializationRequest{
		Frontend: request,
		Task:     &semanticir.Task{Operations: model.Operations, Outcomes: model.Outcomes},
		Model:    model, Counterexample: witness,
	}
	plan, diagnostics := frontendcpp.GenerateProbe(context.Background(), materialization)
	cppRequireNoErrors(t, diagnostics)
	if plan.WitnessID != witness.ID || len(plan.Harness.Bytes) == 0 || plan.Harness.SHA256 != semanticir.DigestBytes(plan.Harness.Bytes) {
		t.Fatalf("probe plan is not witness/harness bound: %+v", plan)
	}
	if len(plan.Steps) != 2 || plan.Steps[0].Kind != executor.ProbeStepCompile || plan.Steps[1].Kind != executor.ProbeStepRun {
		t.Fatalf("probe does not use an ordered compile/run plan: %+v", plan.Steps)
	}
	paths := []string{plan.Harness.Path, plan.Steps[0].WorkDir, plan.Steps[1].WorkDir, plan.Steps[1].ObservationPath, plan.Steps[1].GeneratedExecutable}
	paths = append(paths, plan.Steps[0].Outputs...)
	for _, path := range paths {
		if filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, "../") || strings.Contains(path, "/../") {
			t.Fatalf("probe path escaped canonical isolated-workspace coordinates: %q in %+v", path, plan.Steps)
		}
	}
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true executable is required for probe baseline")
	}
	report := executor.ConfirmProbes(context.Background(), executor.TaskEnvironment{
		Command: []string{truePath}, WorkspaceRoot: request.Workspace.Root, WorkDir: request.Workspace.Root,
		Timeout: 10 * time.Second, ExactEnvironment: true, PassSignal: executor.ExitCodeSignal(0),
	}, []executor.ProbePlan{plan})
	if report.Status != executor.StatusConfirmed || len(report.Blockers) != 0 {
		t.Fatalf("generated C++ direct probe was not confirmed: %+v", report)
	}
}
