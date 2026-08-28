package rust

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/semanticir"
)

// GenerateProbe creates an isolated direct-execution witness for a reference
// behavior vector. The harness observes the frozen Rust source itself and
// writes the expected semantic JSON only after every choice matches.
func GenerateProbe(ctx context.Context, request semanticir.MaterializationRequest) (executor.ProbePlan, []semanticir.Diagnostic) {
	frontend := request.Frontend
	whole := wholeSpan(frontend.Source)
	block := func(code semanticir.DiagnosticCode, message string, span sourceSpan) (executor.ProbePlan, []semanticir.Diagnostic) {
		return executor.ProbePlan{}, []semanticir.Diagnostic{diagnostic(frontend.Artifact, span, code, message)}
	}
	if err := ctx.Err(); err != nil {
		return block(semanticir.DiagnosticInvalidInput, "Rust probe generation cancelled: "+err.Error(), whole)
	}
	if request.Task == nil || request.Counterexample.ID == "" || request.Counterexample.Obligation != semanticir.ObligationReferenceCorrectness {
		return block(semanticir.DiagnosticInvalidInput, "Rust direct probes require a reference-correctness counterexample and compiled task", whole)
	}
	if request.Counterexample.OperationID == "" || len(request.Counterexample.Choices) == 0 || len(request.Counterexample.ObservedOutcomes) == 0 {
		return block(semanticir.DiagnosticIncomplete, "Rust direct probe witness lacks operation, choices, or observed outcomes", whole)
	}
	if err := semanticir.VerifyArtifact(frontend.Artifact, frontend.Source); err != nil {
		return block(semanticir.DiagnosticStaleArtifact, err.Error(), whole)
	}
	if diagnostics := validateRustWorkspace(frontend); semanticir.HasErrors(diagnostics) {
		return executor.ProbePlan{}, diagnostics
	}
	if request.Model.Artifact != frontend.Artifact || request.Model.Translator != frontend.Translator || request.Model.Coverage.Status != semanticir.TranslationComplete {
		return block(semanticir.DiagnosticInvalidReference, "Rust direct probe model is not a complete translation of the frozen frontend request", whole)
	}
	for _, choice := range ownedMaterializationChoices(request.Model, request.Counterexample) {
		if pointDiagnostic := validateRustBehaviorPoint(frontend, choice.Behavior); pointDiagnostic != nil {
			return executor.ProbePlan{}, []semanticir.Diagnostic{*pointDiagnostic}
		}
		if !modelCaseHasOutcome(request.Model, choice.Behavior, choice.OutcomeID) {
			return block(semanticir.DiagnosticInvalidReference, "reference probe choice is not the compiler-observed frozen behavior", whole)
		}
	}
	expected, traces, expectedDiagnostics := probeExpectedSemantics(request)
	if semanticir.HasErrors(expectedDiagnostics) {
		return executor.ProbePlan{}, expectedDiagnostics
	}
	expectedJSON, err := json.Marshal(executor.ProbeObservation{Traces: traces})
	if err != nil {
		return block(semanticir.DiagnosticInvalidInput, "encode Rust probe semantics: "+err.Error(), whole)
	}
	harnessPath := ".ray-rust-probe-" + safeRustProbeID(request.Counterexample.ID) + ".rs"
	observationPath := ".ray-rust-probe-" + safeRustProbeID(request.Counterexample.ID) + ".json"
	binaryPath := ".ray-rust-probe-" + safeRustProbeID(request.Counterexample.ID) + ".bin"
	includePath, err := filepath.Rel(filepath.Dir(harnessPath), filepath.Clean(frontend.Artifact.Path))
	if err != nil || filepath.IsAbs(includePath) || includePath == ".." || strings.HasPrefix(includePath, ".."+string(filepath.Separator)) {
		return block(semanticir.DiagnosticInvalidReference, "Rust probe cannot include the focused artifact through a workspace-relative path", whole)
	}
	var checks string
	checks, expectedDiagnostics = renderScalarProbeChecks(frontend, request, whole)
	if semanticir.HasErrors(expectedDiagnostics) {
		return executor.ProbePlan{}, expectedDiagnostics
	}
	harness := []byte(fmt.Sprintf("include!(%s);\nfn main() {\n    std::panic::set_hook(Box::new(|_| {}));\n%s    std::fs::write(%s, &%s).expect(\"write exact Ray probe observation\");\n}\n", strconv.Quote(filepath.ToSlash(includePath)), checks, strconv.Quote(observationPath), rustByteSlice(expectedJSON)))
	workspaceDigest, err := executor.WorkspaceDigest(frontend.Workspace.Root)
	if err != nil || workspaceDigest != frontend.Workspace.TreeDigest {
		message := "Rust probe workspace digest does not match the frozen request"
		if err != nil {
			message += ": " + err.Error()
		}
		return block(semanticir.DiagnosticStaleArtifact, message, whole)
	}
	sources := make([]semanticir.ArtifactRef, 0, len(frontend.Workspace.Entries))
	for _, entry := range frontend.Workspace.Entries {
		sources = append(sources, entry.Artifact)
	}
	plan := executor.ProbePlan{
		ID: "rust-probe-" + request.Counterexample.ID, WitnessID: request.Counterexample.ID, Obligation: request.Counterexample.Obligation, Witness: request.Counterexample,
		SourceArtifacts: sources,
		Workspace:       executor.ProbeWorkspace{ID: frontend.Workspace.ID, Root: frontend.Workspace.Root, State: frontend.Workspace.State, TreeSHA256: frontend.Workspace.TreeDigest},
		Tools:           []semanticir.ToolRef{frontend.Translator},
		Operations:      append([]semanticir.Operation(nil), request.Task.Operations...),
		Harness:         executor.ProbeHarness{Path: harnessPath, Bytes: harness, SHA256: semanticir.DigestBytes(harness), Mode: 0o600},
		Steps: []executor.ProbeStep{
			{ID: "compile", Kind: executor.ProbeStepCompile, Tool: &frontend.Translator, Argv: []string{frontend.Translator.Path, "--edition=2021", "--crate-name", "ray_reference_probe", "-C", "panic=unwind", "-C", "overflow-checks=yes", harnessPath, "-o", binaryPath}, WorkDir: ".", Environment: rustWorkspaceEnvironment(frontend.Workspace), Timeout: 30 * time.Second, PassSignal: executor.ExitCodeSignal(0), Outputs: []string{binaryPath}},
			{ID: "run", Kind: executor.ProbeStepRun, GeneratedExecutable: binaryPath, Argv: []string{binaryPath}, WorkDir: ".", Environment: rustWorkspaceEnvironment(frontend.Workspace), Timeout: 30 * time.Second, PassSignal: executor.ExitCodeSignal(0), ObservationPath: observationPath},
		},
		ExpectedSemantics: expected,
	}
	return plan, nil
}

func probeExpectedSemantics(request semanticir.MaterializationRequest) (semanticir.ExpectedSemantics, []semanticir.RawOutcomeTrace, []semanticir.Diagnostic) {
	expected := semanticir.ExpectedSemantics{
		Conditions:  cloneAssignment(request.Counterexample.Conditions),
		OperationID: request.Counterexample.OperationID,
		OutcomeIDs:  append([]string(nil), request.Counterexample.ObservedOutcomes...),
		Choices:     append([]semanticir.BehaviorChoice(nil), request.Counterexample.Choices...),
		TestPasses:  request.Counterexample.TestPasses,
	}
	traces := make([]semanticir.RawOutcomeTrace, 0, len(request.Counterexample.Choices))
	for _, choice := range request.Counterexample.Choices {
		outcome, exists := findOutcome(request.Task.Outcomes, choice.OutcomeID)
		if !exists || outcome.ID != semanticir.OutcomeID(outcome) || outcome.OperationID != choice.Behavior.OperationID {
			item := diagnostic(request.Frontend.Artifact, wholeSpan(request.Frontend.Source), semanticir.DiagnosticInvalidReference, "Rust probe witness refers outside the authoritative operation outcome vocabulary")
			return semanticir.ExpectedSemantics{}, nil, []semanticir.Diagnostic{item}
		}
		trace, err := rawTraceFromObservableOutcome(outcome)
		if err != nil {
			item := diagnostic(request.Frontend.Artifact, wholeSpan(request.Frontend.Source), semanticir.DiagnosticUnsupported, "Rust probe outcome has no exact raw runtime representation: "+err.Error())
			return semanticir.ExpectedSemantics{}, nil, []semanticir.Diagnostic{item}
		}
		expected.RuntimeOutcomes = append(expected.RuntimeOutcomes, semanticir.RuntimeOutcomeChoice{
			Behavior:         choice.Behavior,
			RawOutcome:       trace,
			MappingOutcomeID: choice.OutcomeID,
		})
		traces = append(traces, trace)
	}
	if len(traces) == 0 || len(request.Counterexample.ObservedOutcomes) != len(traces) {
		item := diagnostic(request.Frontend.Artifact, wholeSpan(request.Frontend.Source), semanticir.DiagnosticIncomplete, "Rust probe witness has no complete ordered raw runtime vector")
		return semanticir.ExpectedSemantics{}, nil, []semanticir.Diagnostic{item}
	}
	return expected, traces, nil
}

func rawTraceFromObservableOutcome(outcome semanticir.ObservableOutcome) (semanticir.RawOutcomeTrace, error) {
	trace := semanticir.RawOutcomeTrace{
		Kind: outcome.Kind, Value: outcome.Value, ExceptionType: outcome.ExceptionType,
		Message: outcome.Message, Effects: []semanticir.RawEffectTrace{},
	}
	for _, effect := range outcome.Effects {
		if effect.Value != nil && (effect.Value.Kind != semanticir.ExprLiteral || effect.Value.Literal == nil || len(effect.Value.Operands) != 0) {
			return semanticir.RawOutcomeTrace{}, fmt.Errorf("effect %s:%s is not an exact literal", effect.Kind, effect.Target)
		}
		var value *semanticir.Literal
		if effect.Value != nil {
			literal := *effect.Value.Literal
			value = &literal
		}
		trace.Effects = append(trace.Effects, semanticir.RawEffectTrace{Kind: effect.Kind, Target: effect.Target, Value: value})
	}
	if err := semanticir.ValidateRawOutcomeTrace(trace); err != nil {
		return semanticir.RawOutcomeTrace{}, err
	}
	return trace, nil
}

func renderScalarProbeChecks(frontend semanticir.FrontendRequest, request semanticir.MaterializationRequest, whole sourceSpan) (string, []semanticir.Diagnostic) {
	functions, issues := parseRust(frontend.Source)
	if len(issues) != 0 {
		return "", []semanticir.Diagnostic{diagnostic(frontend.Artifact, issues[0].Span, semanticir.DiagnosticUnsupported, "Rust probe source is outside the strict span-mapping subset")}
	}
	byName := make(map[string]functionDecl, len(functions))
	for _, fn := range functions {
		byName[fn.Name] = fn
	}
	var result strings.Builder
	for index, choice := range request.Counterexample.Choices {
		fn, ok := byName[choice.Behavior.OperationID]
		if !ok || fn.IsTest {
			return "", []semanticir.Diagnostic{diagnostic(frontend.Artifact, whole, semanticir.DiagnosticInvalidReference, "Rust probe choice does not name a source-defined callable")}
		}
		arguments, item := rustProbeArguments(frontend, fn, choice.Behavior.Conditions)
		if item != nil {
			return "", []semanticir.Diagnostic{*item}
		}
		outcome, ok := findOutcome(request.Task.Outcomes, choice.OutcomeID)
		if !ok || len(outcome.Effects) != 0 {
			return "", []semanticir.Diagnostic{diagnostic(frontend.Artifact, fn.Span, semanticir.DiagnosticUnsupported, "scalar Rust probe cannot observe the requested outcome effects")}
		}
		predicate, ok := rustProbeOutcomePredicate(fn.ReturnType, outcome, "observed")
		if !ok {
			return "", []semanticir.Diagnostic{diagnostic(frontend.Artifact, fn.Span, semanticir.DiagnosticUnsupported, "Rust probe cannot compare the requested outcome exactly")}
		}
		fmt.Fprintf(&result, "    { let observed = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| %s(%s))); if !(%s) { std::process::exit(%d); } }\n", fn.Name, strings.Join(arguments, ", "), predicate, 70+index%20)
	}
	return result.String(), nil
}

func rustProbeArguments(request semanticir.FrontendRequest, fn functionDecl, conditions semanticir.Assignment) ([]string, *semanticir.Diagnostic) {
	arguments := make([]string, len(fn.Parameters))
	seen := make(map[string]bool, len(fn.Parameters))
	for index, param := range fn.Parameters {
		domainID := findDomainID(request, fn.Name, param.Name)
		valueID, exists := conditions[domainID]
		domain, domainExists := findDomain(request.FiniteDomains, domainID)
		member, memberExists := domainValueByID(domain, valueID)
		if domainID == "" || !exists || !domainExists || !memberExists {
			item := diagnostic(request.Artifact, param.Span, semanticir.DiagnosticInvalidReference, "Rust probe assignment does not exactly bind "+fn.Name+"."+param.Name)
			return nil, &item
		}
		argument, exact := renderRustDomainArgument(domain, member, fn.Name, param.Name, param.Type)
		if !exact {
			item := diagnostic(request.Artifact, param.Span, semanticir.DiagnosticUnsupported, "Rust probe domain label has no exact argument literal")
			return nil, &item
		}
		seen[domainID] = true
		arguments[index] = argument
	}
	if len(seen) != len(conditions) {
		item := diagnostic(request.Artifact, fn.Span, semanticir.DiagnosticInvalidReference, "Rust probe assignment contains domains outside its operation")
		return nil, &item
	}
	return arguments, nil
}

func rustProbeOutcomePredicate(returnType string, outcome semanticir.ObservableOutcome, variable string) (string, bool) {
	switch outcome.Kind {
	case semanticir.OutcomeReturn:
		if outcome.Value == nil {
			return "", false
		}
		rendered, ok := renderRustLiteral(*outcome.Value)
		if !ok {
			return "", false
		}
		return "matches!(" + variable + ", Ok(value) if value == " + string(rendered) + ")", true
	case semanticir.OutcomeSuccess:
		if _, _, result := rustResultTypes(returnType); !result {
			return "", false
		}
		return "matches!(" + variable + ", Ok(Ok(_)))", true
	case semanticir.OutcomeRaise:
		if outcome.ExceptionType == "Result::Err" {
			if _, _, result := rustResultTypes(returnType); !result {
				return "", false
			}
			return "matches!(" + variable + ", Ok(Err(_)))", true
		}
		if outcome.ExceptionType == "panic" {
			message := strconv.Quote(outcome.Message)
			return "match " + variable + " { Err(payload) => { let message = if let Some(value) = payload.downcast_ref::<&str>() { *value } else if let Some(value) = payload.downcast_ref::<String>() { value.as_str() } else { \"<non-string-panic>\" }; message == " + message + " }, _ => false }", true
		}
	}
	return "", false
}

func rustByteSlice(value []byte) string {
	parts := make([]string, len(value))
	for index, item := range value {
		parts[index] = strconv.Itoa(int(item))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func safeRustProbeID(value string) string {
	var result strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}
	if result.Len() == 0 {
		return "witness"
	}
	return result.String()
}
