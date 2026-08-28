package cpp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/ray/internal/semanticir"
)

func TestTranslateRealStatefulUnpauseSliceBlocksWithoutGenericReceiverConstruction(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(repository, "testdata", "e2e", "real-cpp-continue", "workspaces", "solution-new-tests")
	sourceRelative := filepath.Join("workspace", "src", "http_engine.cpp")
	source, err := os.ReadFile(filepath.Join(root, sourceRelative))
	if err != nil {
		t.Skipf("real C++ fixture unavailable: %v", err)
	}
	compiler, err := exec.LookPath("clang++")
	if err != nil {
		t.Skip("clang++ is required")
	}
	compiler, err = filepath.Abs(compiler)
	if err != nil {
		t.Fatal(err)
	}
	compilerBytes, err := os.ReadFile(compiler)
	if err != nil {
		t.Fatal(err)
	}
	version, err := exec.Command(compiler, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	toolDigest := sha256.Sum256(compilerBytes)
	tool := semanticir.ToolRef{Name: "clang++", Path: compiler, Digest: "sha256:" + hex.EncodeToString(toolDigest[:]), Version: strings.TrimSpace(string(version))}
	prover := cppTestExecutable(t, "z3")
	artifact := semanticir.ArtifactRef{ID: "real-http-engine", Kind: semanticir.ArtifactCode, Path: sourceRelative, Digest: semanticir.DigestBytes(source)}
	sourceProvenance := semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: sourceRelative, StartLine: 49, StartColumn: 1, EndLine: 60, EndColumn: 1}, semanticir.TranslationTranslated)
	stringValue := func(value string) *semanticir.Expression {
		return &semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeString, Literal: &semanticir.Literal{Type: semanticir.TypeString, String: value}, Provenance: sourceProvenance}
	}
	boolValue := func(value bool) *semanticir.Expression {
		return &semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeBool, Literal: &semanticir.Literal{Type: semanticir.TypeBool, Bool: value}, Provenance: sourceProvenance}
	}
	expectedOutcome := semanticir.ObservableOutcome{
		Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeUnit}, OperationID: "HttpEngine::unpause", Provenance: sourceProvenance,
		Effects: []semanticir.Effect{
			{ID: "effect-find", Kind: semanticir.EffectCall, Target: "HttpEngine::find_part", Provenance: sourceProvenance},
			{ID: "effect-unpause", Kind: semanticir.EffectCall, Target: "UploadPart::unpause", Provenance: sourceProvenance},
			{ID: "effect-phase", Kind: semanticir.EffectWrite, Target: "phase_", Value: stringValue("Sending"), Provenance: sourceProvenance},
			{ID: "effect-drive", Kind: semanticir.EffectWrite, Target: "need_body_drive_", Value: boolValue(true), Provenance: sourceProvenance},
			{ID: "effect-output", Kind: semanticir.EffectOutput, Target: "stdout", Value: stringValue("unpause file\n"), Provenance: sourceProvenance},
		},
	}
	expectedOutcome.ID = semanticir.OutcomeID(expectedOutcome)
	entries := freezeCPPTestEntries(t, root, artifact)
	var compilationDatabase semanticir.ArtifactRef
	for _, entry := range entries {
		if entry.Path == "compile_commands.json" {
			compilationDatabase = entry.Artifact
			break
		}
	}
	request := semanticir.FrontendRequest{
		TaskID:      "real-cpp-unpause",
		Artifact:    artifact,
		Language:    semanticir.LanguageCPP,
		Kind:        semanticir.ArtifactCode,
		Source:      source,
		EntryPoints: []string{"HttpEngine::unpause"},
		FiniteDomains: []semanticir.Domain{
			cppTestDomain("HttpEngine::unpause.part", semanticir.TypeString, semanticir.Literal{Type: semanticir.TypeString, String: "file"}),
			cppTestDomain("HttpEngine::unpause.phase_", semanticir.TypeString, semanticir.Literal{Type: semanticir.TypeString, String: "Paused"}),
			cppTestDomain("HttpEngine::unpause.expect_continue_", semanticir.TypeBool, semanticir.Literal{Type: semanticir.TypeBool, Bool: true}),
			cppTestDomain("HttpEngine::unpause.saw_continue_", semanticir.TypeBool, semanticir.Literal{Type: semanticir.TypeBool, Bool: true}),
		},
		Operations: []semanticir.Operation{{
			ID: "HttpEngine::unpause", Kind: semanticir.OperationCallable,
			DomainIDs:  []string{"HttpEngine::unpause.part", "HttpEngine::unpause.phase_", "HttpEngine::unpause.expect_continue_", "HttpEngine::unpause.saw_continue_"},
			OutcomeIDs: []string{expectedOutcome.ID}, Provenance: sourceProvenance,
		}},
		Outcomes:       []semanticir.ObservableOutcome{expectedOutcome},
		Translator:     tool,
		Prover:         prover,
		FocusArtifacts: []semanticir.ArtifactRef{artifact},
		ChangedRanges: []semanticir.ChangedSourceRange{{
			ArtifactID: artifact.ID, Path: artifact.Path, StartLine: 49, EndLine: 60,
			SliceDigest: func() string { body, _ := sourceLines(source, 49, 60); return semanticir.DigestBytes(body) }(), Provenance: sourceProvenance,
		}},
		Workspace: semanticir.WorkspaceRef{
			ID: "real-cpp-workspace", State: semanticir.WorkspaceSolutionNewTests, Root: root,
			TreeDigest: semanticir.DigestBytes([]byte("real-cpp-workspace")), WorkingDirectory: "workspace", BuildCommand: "make -C workspace",
			Environment: []semanticir.EnvironmentVariable{{Name: "PATH", Value: os.Getenv("PATH")}}, ClearEnvironment: true, KillProcessGroup: true,
			CompilationDatabase: &compilationDatabase, Entries: entries,
			Provenance: sourceProvenance,
		},
	}
	request.Workspace.EnvironmentDigest, err = semanticir.Digest(request.Workspace.Environment)
	if err != nil {
		t.Fatal(err)
	}
	model, diagnostics := Translate(context.Background(), request)
	if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked {
		t.Fatalf("stateful receiver semantics were not blocked: model=%+v diagnostics=%+v", model, diagnostics)
	}
	wantTargets := []string{"HttpEngine::find_part", "UploadPart::unpause", "phase_", "need_body_drive_", "stdout"}
	gotTargets := make([]string, 0)
	for _, outcome := range model.Outcomes {
		for _, effect := range outcome.Effects {
			gotTargets = append(gotTargets, effect.Target)
		}
	}
	if !sameStrings(gotTargets, wantTargets) {
		t.Fatalf("effect targets = %v, want %v; operations=%+v outcomes=%+v cases=%+v", gotTargets, wantTargets, model.Operations, model.Outcomes, model.Cases)
	}
	if len(model.CompilerEvidence) != 0 || len(model.ExhaustiveEvidence) != 0 {
		t.Fatalf("unsupported receiver acquired authoritative evidence: compiler=%+v exhaustive=%+v", model.CompilerEvidence, model.ExhaustiveEvidence)
	}
}

func TestExhaustiveExecutionRejectsTamperedAdvisoryCase(t *testing.T) {
	request := minimalCPPExecutionRequest(t, `bool decide(bool flag) { return flag; }
`)
	lower := newLowerer(request)
	if !lower.validateRequest() {
		t.Fatalf("minimal request invalid: %+v", lower.diagnostics)
	}
	result, err := clangAST(context.Background(), request.Workspace, request.Translator.Path, lower.compileDirectory, lower.sourcePath, lower.compileFlags, astDumpFilters(request))
	if err != nil {
		t.Fatal(err)
	}
	lower.root, lower.llvmIR, lower.compilerWidths = result.Root, result.LLVMIR, result.IntegerWidths
	lower.discoverOperations()
	lower.enumerateCases()
	if semanticir.HasErrors(lower.diagnostics) || len(lower.cases) != 2 || lower.cases[0].OutcomeIDs[0] == lower.cases[1].OutcomeIDs[0] {
		t.Fatalf("unexpected advisory relation: cases=%+v diagnostics=%+v", lower.cases, lower.diagnostics)
	}
	// Simulate a syntactic/advisory lowering defect. The authoritative harness
	// executes the frozen Clang-compiled function and must reject the swapped
	// outcome rather than certifying the AST table through an SMT tautology.
	lower.cases[0].OutcomeIDs = append([]string(nil), lower.cases[1].OutcomeIDs...)
	if _, err := lower.exhaustivelyExecute(context.Background()); err == nil || !strings.Contains(err.Error(), "differs from provisional semantics") {
		t.Fatalf("compiled execution accepted tampered AST relation: %v", err)
	}
}

func TestTranslateClassifiesCompilerTraceIntoCanonicalOther(t *testing.T) {
	request := minimalCPPExecutionRequest(t, `int decide(bool flag) { return flag ? 7 : 9; }
`)
	provenance := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{
		Path: request.Artifact.Path, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 46,
	}, semanticir.TranslationTranslated)
	named := semanticir.ObservableOutcome{
		Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 5},
		OperationID: "decide", Provenance: provenance,
	}
	named.ID = semanticir.OutcomeID(named)
	other := semanticir.OtherOutcome("decide", provenance)
	request.Outcomes = []semanticir.ObservableOutcome{named, other}
	request.Operations = []semanticir.Operation{{
		ID: "decide", Kind: semanticir.OperationCallable, DomainIDs: []string{"flag"},
		OutcomeIDs: []string{named.ID, other.ID}, Provenance: provenance,
	}}
	bindMinimalCPPExecutionScope(t, &request)

	model, diagnostics := Translate(context.Background(), request)
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("canonical other classification blocked: %+v", diagnostics)
	}
	if model.Coverage.Status != semanticir.TranslationComplete || len(model.Cases) != 2 || len(model.ExhaustiveEvidence) != 1 {
		t.Fatalf("unexpected complement model: status=%s cases=%d evidence=%d", model.Coverage.Status, len(model.Cases), len(model.ExhaustiveEvidence))
	}
	for _, behaviorCase := range model.Cases {
		if len(behaviorCase.OutcomeIDs) != 1 || behaviorCase.OutcomeIDs[0] != other.ID {
			t.Fatalf("raw compiler return was not classified into canonical other: %+v", behaviorCase)
		}
	}
	for _, run := range model.ExhaustiveEvidence[0].Runs {
		for _, observation := range run.Observations {
			if observation.RawOutcome.Kind != semanticir.OutcomeReturn || observation.RawOutcome.Value == nil || observation.RawOutcome.Value.Integer == 5 {
				t.Fatalf("observation lost the actual raw return trace: %+v", observation)
			}
			if len(observation.OutcomeIDs) != 1 || observation.OutcomeIDs[0] != other.ID {
				t.Fatalf("raw observation did not centrally classify to other: %+v", observation)
			}
		}
	}
}

func TestTranslateUsesCompiledSourceOutcomeInsteadOfDeclaredExpectation(t *testing.T) {
	request := minimalCPPExecutionRequest(t, `int decide(bool flag) { return flag ? 2 : 2; }
`)
	provenance := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{
		Path: request.Artifact.Path, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 47,
	}, semanticir.TranslationTranslated)
	declared := func(value int64) semanticir.ObservableOutcome {
		outcome := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: value}, OperationID: "decide", Provenance: provenance}
		outcome.ID = semanticir.OutcomeID(outcome)
		return outcome
	}
	wrongExpected, actual := declared(1), declared(2)
	other := semanticir.OtherOutcome("decide", provenance)
	request.Outcomes = []semanticir.ObservableOutcome{wrongExpected, actual, other}
	request.Operations = []semanticir.Operation{{
		ID: "decide", Kind: semanticir.OperationCallable, DomainIDs: []string{"flag"},
		OutcomeIDs: []string{wrongExpected.ID, actual.ID, other.ID}, Provenance: provenance,
	}}
	bindMinimalCPPExecutionScope(t, &request)
	model, diagnostics := Translate(context.Background(), request)
	if semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationComplete || len(model.Cases) != 2 {
		t.Fatalf("source-outcome independence fixture failed: model=%+v diagnostics=%+v", model, diagnostics)
	}
	for _, behaviorCase := range model.Cases {
		if len(behaviorCase.OutcomeIDs) != 1 || behaviorCase.OutcomeIDs[0] != actual.ID {
			t.Fatalf("declared expected outcome leaked into source translation: case=%+v wrong=%s actual=%s", behaviorCase, wrongExpected.ID, actual.ID)
		}
	}
}

func TestTranslateProvenFixedScalarLoop(t *testing.T) {
	request := minimalCPPExecutionRequest(t, `int decide(bool flag) {
    int sum = 0;
    for (int i = 0; i < 3; ++i) {
        sum += i;
    }
    return flag ? sum : 0;
}
`)
	provenance := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{
		Path: request.Artifact.Path, StartLine: 1, StartColumn: 1, EndLine: 7, EndColumn: 1,
	}, semanticir.TranslationTranslated)
	var outcomes []semanticir.ObservableOutcome
	for _, value := range []int64{0, 3} {
		outcome := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: value}, OperationID: "decide", Provenance: provenance}
		outcome.ID = semanticir.OutcomeID(outcome)
		outcomes = append(outcomes, outcome)
	}
	other := semanticir.OtherOutcome("decide", provenance)
	request.Outcomes = append(outcomes, other)
	request.Operations = []semanticir.Operation{{
		ID: "decide", Kind: semanticir.OperationCallable, DomainIDs: []string{"flag"},
		OutcomeIDs: []string{outcomes[0].ID, outcomes[1].ID, other.ID}, Provenance: provenance,
	}}
	bindMinimalCPPExecutionScope(t, &request)
	model, diagnostics := Translate(context.Background(), request)
	if semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationComplete || len(model.Cases) != 2 || len(model.ExhaustiveEvidence) != 1 {
		t.Fatalf("fixed scalar loop was not compiler-grounded: status=%s cases=%d evidence=%d diagnostics=%+v", model.Coverage.Status, len(model.Cases), len(model.ExhaustiveEvidence), diagnostics)
	}
	if len(model.Operations) != 1 || len(model.Operations[0].Body) != 1 || model.Operations[0].Body[0].Kind != semanticir.StmtBranch {
		t.Fatalf("fixed scalar loop did not unroll into the source terminal relation: %+v", model.Operations)
	}
}

func TestTranslatePureFreeFunctionWithScalarConditionalBindings(t *testing.T) {
	source := `int resolve_required(int raw, int size) {
    if (raw == 0) {
        return -1;
    }
    int index = raw > 0 ? raw - 1 : size + raw;
    return index >= 0 && index < size ? index : -1;
}
`
	request := minimalCPPExecutionRequest(t, source)
	request.EntryPoints = []string{"resolve_required"}
	integerDomain := func(id string, values ...int64) semanticir.Domain {
		domain := semanticir.Domain{ID: id, Type: semanticir.TypeInteger}
		for _, value := range values {
			literal := semanticir.Literal{Type: semanticir.TypeInteger, Integer: value}
			domain.Values = append(domain.Values, semanticir.DomainValue{ID: fmt.Sprintf("%d", value), Value: &literal})
		}
		return domain
	}
	request.FiniteDomains = []semanticir.Domain{integerDomain("raw", -2, -1, 0, 1, 2, 4), integerDomain("size", 0, 1, 3)}
	var outcomeIDs []string
	for _, value := range []int64{-1, 0, 1, 2} {
		outcome := semanticir.ObservableOutcome{
			Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: value}, OperationID: "resolve_required",
			Provenance: semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1, EndLine: 7, EndColumn: 1}, semanticir.TranslationTranslated),
		}
		outcome.ID = semanticir.OutcomeID(outcome)
		request.Outcomes = append(request.Outcomes, outcome)
		outcomeIDs = append(outcomeIDs, outcome.ID)
	}
	request.Operations = []semanticir.Operation{{ID: "resolve_required", Kind: semanticir.OperationCallable, DomainIDs: []string{"raw", "size"}, OutcomeIDs: outcomeIDs}}
	bindMinimalCPPExecutionScope(t, &request)

	model, diagnostics := Translate(context.Background(), request)
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("pure scalar conditional translation blocked: %+v", diagnostics)
	}
	if model.Coverage.Status != semanticir.TranslationComplete || len(model.Cases) != 18 || len(model.ExhaustiveEvidence) != 1 {
		t.Fatalf("unexpected scalar conditional model: status=%s cases=%d evidence=%d unsupported=%+v", model.Coverage.Status, len(model.Cases), len(model.ExhaustiveEvidence), model.Coverage.Unsupported)
	}
	for _, behaviorCase := range model.Cases {
		if len(behaviorCase.OutcomeIDs) != 1 || !containsString(outcomeIDs, behaviorCase.OutcomeIDs[0]) {
			t.Fatalf("case escaped declared outcome universe: %+v", behaviorCase)
		}
	}
}

func TestDecodeRawOutcomeTraceRejectsSemanticIdentity(t *testing.T) {
	value := semanticir.Literal{Type: semanticir.TypeInteger, Integer: 7}
	trace := semanticir.RawOutcomeTrace{Kind: semanticir.OutcomeReturn, Value: &value, Effects: []semanticir.RawEffectTrace{}}
	canonical, err := semanticir.CanonicalJSON(trace)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeRawOutcomeTrace(canonical); err != nil {
		t.Fatalf("canonical raw trace rejected: %v", err)
	}
	for _, field := range []string{"id", "operation_id", "provenance"} {
		forged := append([]byte(nil), canonical[:len(canonical)-1]...)
		forged = append(forged, []byte(",\""+field+"\":\"forged\"}")...)
		if _, err := decodeRawOutcomeTrace(forged); err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("raw trace accepted forged semantic field %q: %v", field, err)
		}
	}
}

func TestNormalizedCompilationArgumentsUsesOnlyFrozenPATH(t *testing.T) {
	compiler, err := exec.LookPath("clang++")
	if err != nil {
		t.Skip("clang++ is required")
	}
	compiler, err = filepath.Abs(compiler)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	source := filepath.Join(root, "focus.cpp")
	workspace := semanticir.WorkspaceRef{Environment: []semanticir.EnvironmentVariable{{Name: "PATH", Value: filepath.Dir(compiler)}}}
	command := compilationCommand{Directory: root, File: source, Arguments: []string{filepath.Base(compiler), "-std=c++20", "-c", source}}
	flags, err := normalizedCompilationArguments(command, source, compiler, workspace)
	if err != nil || !sameStrings(flags, []string{"-std=c++20"}) {
		t.Fatalf("frozen PATH compiler did not normalize exactly: flags=%v err=%v", flags, err)
	}
	command.Arguments[0] = filepath.Join("tools", filepath.Base(compiler))
	if _, err := normalizedCompilationArguments(command, source, compiler, workspace); err == nil || !strings.Contains(err.Error(), "relative path") {
		t.Fatalf("workspace-relative compiler path was accepted: %v", err)
	}
	command.Arguments[0] = filepath.Base(compiler)
	workspace.Environment = nil
	if _, err := normalizedCompilationArguments(command, source, compiler, workspace); err == nil || !strings.Contains(err.Error(), "frozen workspace PATH") {
		t.Fatalf("ambient PATH compiler lookup was accepted: %v", err)
	}
}

func minimalCPPExecutionRequest(t *testing.T, source string) semanticir.FrontendRequest {
	t.Helper()
	root := t.TempDir()
	compiler, err := exec.LookPath("clang++")
	if err != nil {
		t.Skip("clang++ is required")
	}
	compiler, _ = filepath.Abs(compiler)
	compilerBytes, err := os.ReadFile(compiler)
	if err != nil {
		t.Fatal(err)
	}
	version, err := exec.Command(compiler, "--version").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	tool := semanticir.ToolRef{Name: "clang++", Path: compiler, Digest: semanticir.DigestBytes(compilerBytes), Version: strings.TrimSpace(string(version))}
	prover := cppTestExecutable(t, "z3")
	sourcePath := filepath.Join(root, "artifact.cpp")
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := semanticir.ArtifactRef{ID: "cpp-tamper", Kind: semanticir.ArtifactCode, Path: "artifact.cpp", Digest: semanticir.DigestBytes([]byte(source))}
	compileCommands, err := json.Marshal([]compilationCommand{{Directory: root, File: "artifact.cpp", Arguments: []string{compiler, "-std=c++20", "-c", sourcePath}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "compile_commands.json"), compileCommands, 0o600); err != nil {
		t.Fatal(err)
	}
	compileArtifact := semanticir.ArtifactRef{ID: "cpp-tamper-db", Kind: semanticir.ArtifactEnvironment, Path: "compile_commands.json", Digest: semanticir.DigestBytes(compileCommands)}
	endLine := strings.Count(source, "\n") + 1
	if strings.HasSuffix(source, "\n") {
		endLine--
	}
	provenance := semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1, EndLine: endLine, EndColumn: 1}, semanticir.TranslationTranslated)
	compileProvenance := semanticir.NewProvenance(compileArtifact, semanticir.SourceLocation{Path: compileArtifact.Path, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1}, semanticir.TranslationTranslated)
	environment := []semanticir.EnvironmentVariable{{Name: "PATH", Value: os.Getenv("PATH")}}
	environmentDigest, _ := semanticir.Digest(environment)
	boolLiteral := func(value bool) *semanticir.Literal {
		return &semanticir.Literal{Type: semanticir.TypeBool, Bool: value}
	}
	domain := semanticir.Domain{ID: "flag", Type: semanticir.TypeBool, Values: []semanticir.DomainValue{{ID: "false", Value: boolLiteral(false)}, {ID: "true", Value: boolLiteral(true)}}}
	return semanticir.FrontendRequest{
		TaskID: "cpp-tamper", Artifact: artifact, Language: semanticir.LanguageCPP, Kind: semanticir.ArtifactCode,
		Source: []byte(source), EntryPoints: []string{"decide"}, FiniteDomains: []semanticir.Domain{domain}, Translator: tool, Prover: prover,
		FocusArtifacts: []semanticir.ArtifactRef{artifact}, ChangedRanges: []semanticir.ChangedSourceRange{{ArtifactID: artifact.ID, Path: artifact.Path, StartLine: 1, EndLine: endLine, SliceDigest: semanticir.DigestBytes([]byte(source)), Provenance: provenance}},
		Workspace: semanticir.WorkspaceRef{
			ID: "cpp-tamper-workspace", State: semanticir.WorkspaceSolutionNewTests, Root: root,
			TreeDigest: semanticir.DigestBytes(append([]byte(source), compileCommands...)), WorkingDirectory: ".", BuildCommand: compiler + " -c artifact.cpp",
			Environment: environment, EnvironmentDigest: environmentDigest, ClearEnvironment: true, KillProcessGroup: true,
			CompilationDatabase: &compileArtifact,
			Entries:             []semanticir.WorkspaceEntry{{Path: artifact.Path, Artifact: artifact, Provenance: provenance}, {Path: compileArtifact.Path, Artifact: compileArtifact, Provenance: compileProvenance}},
			Provenance:          provenance,
		},
	}
}

func bindMinimalCPPExecutionScope(t *testing.T, request *semanticir.FrontendRequest) {
	t.Helper()
	provenance := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{
		Path: request.Artifact.Path, StartLine: 1, StartColumn: 1,
		EndLine: strings.Count(string(request.Source), "\n") + 1, EndColumn: 1,
	}, semanticir.TranslationTranslated)
	request.Groundings = nil
	for operationIndex := range request.Operations {
		operation := &request.Operations[operationIndex]
		operation.Inputs = nil
		assignments := []semanticir.Assignment{{}}
		for _, domainID := range operation.DomainIDs {
			var domain *semanticir.Domain
			for domainIndex := range request.FiniteDomains {
				if request.FiniteDomains[domainIndex].ID == domainID {
					domain = &request.FiniteDomains[domainIndex]
					break
				}
			}
			if domain == nil {
				t.Fatalf("operation %q lacks domain %q", operation.ID, domainID)
			}
			input := semanticir.Variable{Name: domainID, Type: domain.Type, DomainID: domainID, Provenance: provenance}
			operation.Inputs = append(operation.Inputs, input)
			for valueIndex := range domain.Values {
				value := &domain.Values[valueIndex]
				if value.Value == nil {
					t.Fatalf("domain %q value %q has no exact literal", domain.ID, value.ID)
				}
				literal := *value.Value
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
			var next []semanticir.Assignment
			for _, assignment := range assignments {
				for _, value := range domain.Values {
					copy := cloneAssignment(assignment)
					copy[domainID] = value.ID
					next = append(next, copy)
				}
			}
			assignments = next
		}
		for _, assignment := range assignments {
			inputs := make(map[string]semanticir.Literal, len(operation.Inputs))
			for _, input := range operation.Inputs {
				valueID := assignment[input.DomainID]
				for _, domain := range request.FiniteDomains {
					if domain.ID != input.DomainID {
						continue
					}
					for _, value := range domain.Values {
						if value.ID == valueID && value.Value != nil {
							inputs[input.Name] = *value.Value
						}
					}
				}
			}
			request.Groundings = append(request.Groundings, semanticir.AssignmentGrounding{
				ID: semanticir.AssignmentGroundingID(operation.ID, assignment), OperationID: operation.ID,
				Conditions: assignment, Inputs: inputs, Provenance: provenance,
			})
		}
	}
}

func cppTestExecutable(t *testing.T, name string) semanticir.ToolRef {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s is required", name)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	version, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --version: %v: %s", name, err, version)
	}
	return semanticir.ToolRef{Name: name, Path: path, Digest: semanticir.DigestBytes(body), Version: strings.TrimSpace(string(version))}
}

func cppTestDomain(id string, typeName semanticir.ValueType, values ...semanticir.Literal) semanticir.Domain {
	domain := semanticir.Domain{ID: id, Type: typeName}
	for _, value := range values {
		copy := value
		valueID := copy.String
		if typeName == semanticir.TypeBool {
			if copy.Bool {
				valueID = "true"
			} else {
				valueID = "false"
			}
		}
		domain.Values = append(domain.Values, semanticir.DomainValue{ID: valueID, Value: &copy})
	}
	return domain
}

func freezeCPPTestEntries(t *testing.T, root string, focus semanticir.ArtifactRef) []semanticir.WorkspaceEntry {
	t.Helper()
	var entries []semanticir.WorkspaceEntry
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		artifact := semanticir.ArtifactRef{ID: "real-entry-" + strings.NewReplacer(string(filepath.Separator), "-", ".", "-").Replace(relative), Kind: semanticir.ArtifactEnvironment, Path: relative, Digest: semanticir.DigestBytes(bytes)}
		if relative == focus.Path {
			artifact = focus
		}
		provenance := semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: relative, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1}, semanticir.TranslationTranslated)
		entries = append(entries, semanticir.WorkspaceEntry{Path: relative, Artifact: artifact, Provenance: provenance})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}
