package cpp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/semanticir"
)

var probePathCharacters = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

// GenerateProbe materializes a reference-correctness witness as a fresh,
// digest-bound executable harness. It does not edit the frozen source. The
// executor stages Harness only in its disposable copy of the frozen workspace.
func GenerateProbe(ctx context.Context, request semanticir.MaterializationRequest) (executor.ProbePlan, []semanticir.Diagnostic) {
	l := newLowerer(request.Frontend)
	block := func(code semanticir.DiagnosticCode, message string) (executor.ProbePlan, []semanticir.Diagnostic) {
		l.diagnostic(nil, code, message)
		return executor.ProbePlan{}, l.diagnostics
	}
	if ctx == nil {
		return block(semanticir.DiagnosticInvalidInput, "probe generation requires a non-nil context")
	}
	if err := ctx.Err(); err != nil {
		return block(semanticir.DiagnosticInvalidInput, fmt.Sprintf("probe generation context: %v", err))
	}
	if !l.validateRequest() {
		return executor.ProbePlan{}, l.diagnostics
	}
	if request.Task == nil {
		return block(semanticir.DiagnosticInvalidInput, "probe generation requires the compiled task IR")
	}
	witness := request.Counterexample
	if witness.ID == "" || witness.Obligation != semanticir.ObligationReferenceCorrectness {
		return block(semanticir.DiagnosticInvalidInput, "direct C++ probes require an identified reference-correctness witness")
	}
	if witness.Provenance.ArtifactID != request.Frontend.Artifact.ID || witness.Provenance.ArtifactDigest != request.Frontend.Artifact.Digest {
		return block(semanticir.DiagnosticInvalidProvenance, "probe witness provenance is not bound to the frozen C++ artifact")
	}
	if request.Model.Artifact != request.Frontend.Artifact || request.Model.Translator != request.Frontend.Translator || request.Model.Coverage.Status != semanticir.TranslationComplete {
		return block(semanticir.DiagnosticStaleArtifact, "probe model is incomplete or detached from the frozen C++ request")
	}
	if validation := semanticir.ValidateArtifactModel(request.Model); semanticir.HasErrors(validation) {
		return block(semanticir.DiagnosticInvalidReference, fmt.Sprintf("probe model fails shared validation: %+v", validation))
	}
	if validation := semanticir.ValidateArtifactScope(request.Frontend, request.Model); semanticir.HasErrors(validation) {
		return block(semanticir.DiagnosticInvalidReference, fmt.Sprintf("probe model differs from the exact frontend scope: %+v", validation))
	}
	observedTree, err := executor.WorkspaceDigest(request.Frontend.Workspace.Root)
	if err != nil {
		return block(semanticir.DiagnosticInvalidInput, fmt.Sprintf("hash probe workspace: %v", err))
	}
	if observedTree != request.Frontend.Workspace.TreeDigest {
		return block(semanticir.DiagnosticStaleArtifact, fmt.Sprintf("probe workspace digest is %s, request binds %s", observedTree, request.Frontend.Workspace.TreeDigest))
	}

	// Re-run translation against the immutable bytes and compiler before
	// constructing executable evidence. Any new blocker or semantic drift
	// invalidates the supplied model.
	fresh, diagnostics := Translate(ctx, request.Frontend)
	if semanticir.HasErrors(diagnostics) {
		return executor.ProbePlan{}, diagnostics
	}
	freshDigest, freshErr := semanticir.Digest(stableProbeModel(fresh))
	modelDigest, modelErr := semanticir.Digest(stableProbeModel(request.Model))
	if freshErr != nil || modelErr != nil || freshDigest != modelDigest {
		return block(semanticir.DiagnosticStaleArtifact, "probe retranslation differs from the supplied compiler-grounded model")
	}

	expected, err := probeExpectedSemantics(request)
	if err != nil {
		return block(semanticir.DiagnosticInvalidReference, err.Error())
	}
	layoutSource, semanticSource, err := l.probeSources(request, expected)
	if err != nil {
		return block(semanticir.DiagnosticUnsupported, err.Error())
	}
	if layoutSource != "" {
		return block(semanticir.DiagnosticUnsupported, "receiver layout instrumentation is not supported by the generic C++ probe")
	}
	workspaceRoot, ok := withinWorkspace(request.Frontend.Workspace.Root, ".")
	if !ok {
		return block(semanticir.DiagnosticInvalidInput, "probe workspace root cannot be canonicalized")
	}
	workDir, ok := withinWorkspace(request.Frontend.Workspace.Root, l.compileDirectory)
	if !ok {
		return block(semanticir.DiagnosticInvalidInput, "probe working directory escapes the frozen workspace")
	}
	// Both operands are canonical here. Mixing a raw /var path with its
	// /private/var resolution can manufacture an escaping ../../private path.
	workRelative, err := filepath.Rel(workspaceRoot, workDir)
	if err != nil {
		return block(semanticir.DiagnosticInvalidInput, fmt.Sprintf("resolve probe working directory: %v", err))
	}
	if workRelative == "" {
		workRelative = "."
	}
	workRelative = filepath.ToSlash(workRelative)

	safeID := strings.Trim(probePathCharacters.ReplaceAllString(witness.ID, "-"), "-")
	if safeID == "" {
		safeID = "witness"
	}
	harnessName := ".ray-cpp-probe-" + safeID + ".cpp"
	binaryName := ".ray-cpp-probe-" + safeID + ".bin"
	observationName := ".ray-cpp-probe-" + safeID + ".json"
	// Stage generated files in the declared working directory. The executor
	// recreates this workspace-relative layout in its isolated copy, so argv
	// never embeds a path in the original (possibly aliased) workspace.
	harnessPath := filepath.ToSlash(filepath.Clean(filepath.Join(workRelative, harnessName)))
	binaryPath := filepath.ToSlash(filepath.Clean(filepath.Join(workRelative, binaryName)))
	observationPath := filepath.ToSlash(filepath.Clean(filepath.Join(workRelative, observationName)))
	harnessArgument, err := relativeProbeArgument(workDir, filepath.Join(workspaceRoot, filepath.FromSlash(harnessPath)))
	if err != nil {
		return block(semanticir.DiagnosticInvalidInput, err.Error())
	}
	binaryArgument, err := relativeProbeArgument(workDir, filepath.Join(workspaceRoot, filepath.FromSlash(binaryPath)))
	if err != nil {
		return block(semanticir.DiagnosticInvalidInput, err.Error())
	}
	observationArgument, err := relativeProbeArgument(workDir, filepath.Join(workspaceRoot, filepath.FromSlash(observationPath)))
	if err != nil {
		return block(semanticir.DiagnosticInvalidInput, err.Error())
	}
	harness := []byte(semanticSource)

	artifacts, err := probeSourceArtifacts(request.Frontend)
	if err != nil {
		return block(semanticir.DiagnosticInvalidInput, err.Error())
	}
	plan := executor.ProbePlan{
		ID:              witness.ID + ":cpp-probe",
		WitnessID:       witness.ID,
		Obligation:      witness.Obligation,
		Witness:         witness,
		SourceArtifacts: artifacts,
		Workspace: executor.ProbeWorkspace{
			ID: request.Frontend.Workspace.ID, Root: request.Frontend.Workspace.Root,
			State: request.Frontend.Workspace.State, TreeSHA256: request.Frontend.Workspace.TreeDigest,
		},
		Tools:      []semanticir.ToolRef{request.Frontend.Translator},
		Operations: probeOperations(request.Model, expected),
		Harness: executor.ProbeHarness{
			Path: harnessPath, Bytes: harness, SHA256: semanticir.DigestBytes(harness), Mode: 0o600,
		},
		Steps: []executor.ProbeStep{
			{
				ID: "compile", Kind: executor.ProbeStepCompile, Tool: &request.Frontend.Translator,
				Argv:    append(append(append([]string{request.Frontend.Translator.Path, "-x", "c++"}, l.compileFlags...), "-fno-color-diagnostics", harnessArgument), "-o", binaryArgument),
				WorkDir: workRelative, Environment: cppWorkspaceEnvironment(request.Frontend.Workspace), Timeout: 30 * time.Second,
				PassSignal: executor.ExitCodeSignal(0), Outputs: []string{binaryPath},
			},
			{
				ID: "run", Kind: executor.ProbeStepRun, GeneratedExecutable: binaryPath,
				Argv: []string{binaryPath, observationArgument}, WorkDir: workRelative,
				Environment: cppWorkspaceEnvironment(request.Frontend.Workspace), Timeout: 30 * time.Second,
				PassSignal: executor.ExitCodeSignal(0), ObservationPath: observationPath,
			},
		},
		ExpectedSemantics: expected,
	}
	return plan, nil
}

// stableProbeModel retains only the typed semantic projection for the drift
// comparison. The caller independently validates both complete evidence
// records, whose fresh timestamps/native binaries need not be byte-identical.
func stableProbeModel(model semanticir.ArtifactModel) semanticir.ArtifactModel {
	copy := model
	// Each complete model is validated independently above. Compiler output
	// bytes, native executable layout, proof transcripts, and wall-clock run
	// metadata may legitimately differ across two fresh invocations while the
	// typed semantic projection must remain byte-identical.
	copy.CompilerEvidence = nil
	copy.ExhaustiveEvidence = nil
	copy.ScopeClosure = nil
	return copy
}

func probeExpectedSemantics(request semanticir.MaterializationRequest) (semanticir.ExpectedSemantics, error) {
	byID := authoritativeOutcomes(request)
	operationByID := make(map[string]semanticir.Operation)
	for _, operation := range request.Model.Operations {
		if operation.Kind != semanticir.OperationTest {
			operationByID[operation.ID] = operation
		}
	}
	witness := request.Counterexample
	if len(witness.Choices) == 0 || len(witness.ObservedOutcomes) != len(witness.Choices) {
		return semanticir.ExpectedSemantics{}, fmt.Errorf("probe witness contains no complete ordered behavior vector")
	}
	expected := semanticir.ExpectedSemantics{
		Conditions: cloneAssignment(witness.Conditions), OperationID: witness.OperationID,
		OutcomeIDs: append([]string(nil), witness.ObservedOutcomes...),
		Choices:    append([]semanticir.BehaviorChoice(nil), witness.Choices...), TestPasses: witness.TestPasses,
	}
	for index, choice := range witness.Choices {
		if choice.Behavior.OperationID == "" || choice.OutcomeID == "" {
			return semanticir.ExpectedSemantics{}, fmt.Errorf("probe witness contains an incomplete behavior choice")
		}
		operation, exists := operationByID[choice.Behavior.OperationID]
		if !exists {
			return semanticir.ExpectedSemantics{}, fmt.Errorf("probe behavior %d operation %q is not owned by this C++ model", index, choice.Behavior.OperationID)
		}
		exactInputs, exact := newLowerer(request.Frontend).exactInputsForAssignment(operation, choice.Behavior.Conditions)
		if !exact || choice.Behavior.Inputs == nil || !reflect.DeepEqual(exactInputs, choice.Behavior.Inputs) {
			return semanticir.ExpectedSemantics{}, fmt.Errorf("probe behavior %d does not carry its exact full input map", index)
		}
		outcome, exists := byID[choice.OutcomeID]
		if !exists || semanticir.OutcomeID(outcome) != choice.OutcomeID || outcome.OperationID != operation.ID {
			return semanticir.ExpectedSemantics{}, fmt.Errorf("probe outcome %q is absent, non-canonical, or outside operation %q", choice.OutcomeID, operation.ID)
		}
		trace, err := probeRawTrace(request.Model, choice.Behavior)
		if err != nil {
			return semanticir.ExpectedSemantics{}, fmt.Errorf("probe behavior %d has no compiler-observed raw trace: %v", index, err)
		}
		classified, err := semanticir.ClassifyRawOutcome(operation, trace, choice.Behavior.Provenance)
		if err != nil || classified != choice.OutcomeID || witness.ObservedOutcomes[index] != choice.OutcomeID {
			return semanticir.ExpectedSemantics{}, fmt.Errorf("probe behavior %d raw trace does not classify as its Ray-owned outcome", index)
		}
		expected.RuntimeOutcomes = append(expected.RuntimeOutcomes, semanticir.RuntimeOutcomeChoice{
			Behavior: choice.Behavior, RawOutcome: trace, MappingOutcomeID: classified,
		})
	}
	return expected, nil
}

func probeRawTrace(model semanticir.ArtifactModel, behavior semanticir.BehaviorRef) (semanticir.RawOutcomeTrace, error) {
	key := semanticir.BehaviorRefKey(behavior)
	var found *semanticir.RawOutcomeTrace
	for _, evidence := range model.ExhaustiveEvidence {
		for _, run := range evidence.Runs {
			for _, observation := range run.Observations {
				if semanticir.BehaviorRefKey(observation.Behavior) != key {
					continue
				}
				trace := observation.RawOutcome
				if found != nil && !reflect.DeepEqual(*found, trace) {
					return semanticir.RawOutcomeTrace{}, fmt.Errorf("independent compiler runs disagree")
				}
				found = &trace
			}
		}
	}
	if found == nil {
		return semanticir.RawOutcomeTrace{}, fmt.Errorf("behavior point %s is absent from exhaustive evidence", key)
	}
	if err := semanticir.ValidateExhaustiveRawOutcomeTrace(*found); err != nil {
		return semanticir.RawOutcomeTrace{}, err
	}
	return *found, nil
}

func probeOperations(model semanticir.ArtifactModel, expected semanticir.ExpectedSemantics) []semanticir.Operation {
	wanted := make(map[string]bool, len(expected.Choices))
	for _, choice := range expected.Choices {
		wanted[choice.Behavior.OperationID] = true
	}
	operations := make([]semanticir.Operation, 0, len(wanted))
	for _, operation := range model.Operations {
		if wanted[operation.ID] {
			operations = append(operations, operation)
		}
	}
	return operations
}

func probeSourceArtifacts(request semanticir.FrontendRequest) ([]semanticir.ArtifactRef, error) {
	seenID, seenPath := map[string]bool{}, map[string]bool{}
	result := make([]semanticir.ArtifactRef, 0, len(request.Workspace.Entries))
	for _, entry := range request.Workspace.Entries {
		artifact := entry.Artifact
		if artifact.ID == "" || artifact.Path == "" || !semanticir.ValidDigest(artifact.Digest) {
			return nil, fmt.Errorf("probe source entry %q has incomplete immutable identity", entry.Path)
		}
		path := filepath.Clean(artifact.Path)
		if seenID[artifact.ID] || seenPath[path] {
			return nil, fmt.Errorf("probe source entries repeat artifact ID or path")
		}
		seenID[artifact.ID], seenPath[path] = true, true
		result = append(result, artifact)
	}
	if !seenID[request.Artifact.ID] {
		return nil, fmt.Errorf("probe sources omit the witness artifact")
	}
	return result, nil
}

func relativeProbeArgument(workDir, target string) (string, error) {
	relative, err := filepath.Rel(workDir, target)
	if err != nil {
		return "", fmt.Errorf("resolve staged probe path: %v", err)
	}
	if relative == "" || filepath.IsAbs(relative) {
		return "", fmt.Errorf("staged probe path is not workspace-relative")
	}
	return filepath.ToSlash(relative), nil
}

func (l *lowerer) probeSources(request semanticir.MaterializationRequest, expected semanticir.ExpectedSemantics) (string, string, error) {
	choices := request.Counterexample.Choices
	if len(choices) == 0 {
		return "", "", fmt.Errorf("probe witness has no complete behavior vector")
	}
	owned := make([]semanticir.BehaviorChoice, 0, len(choices))
	operationByID := map[string]semanticir.Operation{}
	for _, operation := range request.Model.Operations {
		if operation.Kind != semanticir.OperationTest {
			operationByID[operation.ID] = operation
		}
	}
	for _, choice := range choices {
		if _, exists := operationByID[choice.Behavior.OperationID]; exists {
			owned = append(owned, choice)
		}
	}
	if len(owned) == 0 {
		return "", "", fmt.Errorf("probe witness has no behavior owned by this C++ artifact")
	}
	for _, choice := range owned {
		operation := operationByID[choice.Behavior.OperationID]
		if operation.Kind == semanticir.OperationMethod {
			return "", "", fmt.Errorf("operation %q requires receiver construction; record invariants, ownership, and private state initialization are not exactly derivable", operation.ID)
		}
		if operation.Kind != semanticir.OperationFunction && operation.Kind != semanticir.OperationCallable {
			return "", "", fmt.Errorf("operation %q is not a probeable free function", operation.ID)
		}
		if !cxxTypeName.MatchString(operation.ID) {
			return "", "", fmt.Errorf("operation %q cannot be rendered as an exact C++ callable", operation.ID)
		}
	}
	outcomes := authoritativeOutcomes(request)
	traceByChoice := make(map[string]semanticir.RawOutcomeTrace, len(expected.RuntimeOutcomes))
	for _, runtime := range expected.RuntimeOutcomes {
		key, err := semanticir.Digest(runtime.Behavior)
		if err != nil {
			return "", "", fmt.Errorf("digest probe behavior: %v", err)
		}
		traceByChoice[key+"\x00"+runtime.MappingOutcomeID] = runtime.RawOutcome
	}
	focusRelative, err := filepath.Rel(l.compileDirectory, l.sourcePath)
	if err != nil {
		return "", "", fmt.Errorf("resolve focus source for probe: %v", err)
	}
	hasMain := regexp.MustCompile(`(?m)\b(?:int|void|bool|auto)\s+main\s*\(`).Match(request.Frontend.Source)
	if hasMain && regexp.MustCompile(`(?m)^\s*#\s*(?:define|undef|ifn?def)\s+main\b`).Match(request.Frontend.Source) {
		return "", "", fmt.Errorf("focus translation unit has preprocessor-dependent main semantics that cannot be isolated exactly")
	}
	traces := make([]semanticir.RawOutcomeTrace, 0, len(owned))
	for _, choice := range owned {
		key, err := semanticir.Digest(choice.Behavior)
		if err != nil {
			return "", "", fmt.Errorf("digest probe behavior: %v", err)
		}
		trace, exists := traceByChoice[key+"\x00"+choice.OutcomeID]
		if !exists {
			return "", "", fmt.Errorf("probe choice %q has no ordered raw runtime trace", choice.OutcomeID)
		}
		traces = append(traces, trace)
	}
	expectedJSON, err := json.Marshal(executor.ProbeObservation{Traces: traces})
	if err != nil {
		return "", "", fmt.Errorf("encode exact probe observation: %v", err)
	}
	var source strings.Builder
	source.WriteString("#include <fstream>\n#include <iostream>\n#include <sstream>\n#include <stdexcept>\n#include <string>\n")
	if hasMain {
		source.WriteString("#define main ray_frozen_program_main_")
		source.WriteString(strings.TrimPrefix(request.Frontend.Artifact.Digest, "sha256:")[:16])
		source.WriteByte('\n')
	}
	source.WriteString("#include ")
	source.WriteString(strconv.Quote(filepath.ToSlash(focusRelative)))
	if hasMain {
		source.WriteString("\n#undef main")
	}
	source.WriteString("\nint main(int argc, char **argv) {\n  if (argc != 2) return 10;\n")
	for index, choice := range owned {
		operation := operationByID[choice.Behavior.OperationID]
		outcome, exists := outcomes[choice.OutcomeID]
		if !exists {
			return "", "", fmt.Errorf("probe choice outcome %q is absent", choice.OutcomeID)
		}
		exactInputs, exact := l.exactInputsForAssignment(operation, choice.Behavior.Conditions)
		if !exact {
			return "", "", fmt.Errorf("probe behavior for operation %q does not uniquely fix every input", operation.ID)
		}
		arguments := make([]string, 0, len(operation.Inputs))
		for _, input := range operation.Inputs {
			literal, ok := exactInputs[input.Name]
			if !ok || literal.Type != input.Type {
				return "", "", fmt.Errorf("probe assignment has no exact C++ literal for input %s", input.Name)
			}
			rendered, ok := renderLiteral(literal)
			if !ok {
				return "", "", fmt.Errorf("probe assignment input %s cannot be rendered", input.Name)
			}
			arguments = append(arguments, string(rendered))
		}
		expectedOutput, effectErr := probeOutput(outcome)
		if effectErr != nil {
			return "", "", fmt.Errorf("operation %s: %v", operation.ID, effectErr)
		}
		failure := 20 + index*4
		source.WriteString("  { std::ostringstream ray_output; std::streambuf *ray_old = std::cout.rdbuf(ray_output.rdbuf());\n")
		call := operation.ID + "(" + strings.Join(arguments, ", ") + ")"
		switch outcome.Kind {
		case semanticir.OutcomeReturn, semanticir.OutcomeSuccess:
			source.WriteString("    try { ")
			if outcome.Value == nil || outcome.Value.Type == semanticir.TypeUnit {
				source.WriteString(call + "; ")
			} else {
				rendered, ok := renderLiteral(*outcome.Value)
				if !ok {
					return "", "", fmt.Errorf("outcome %q return literal cannot be rendered", outcome.ID)
				}
				source.WriteString("auto ray_value = " + call + "; if (!(ray_value == " + string(rendered) + ")) { std::cout.rdbuf(ray_old); return " + strconv.Itoa(failure) + "; } ")
			}
			source.WriteString("} catch (...) { std::cout.rdbuf(ray_old); return " + strconv.Itoa(failure+1) + "; }\n")
		case semanticir.OutcomeRaise:
			if !cxxTypeName.MatchString(outcome.ExceptionType) {
				return "", "", fmt.Errorf("outcome %q exception type cannot be rendered", outcome.ID)
			}
			source.WriteString("    bool ray_raised = false; try { " + call + "; } catch (const " + outcome.ExceptionType + " &ray_error) { ray_raised = true;")
			if outcome.Message != "" {
				if !strings.Contains(outcome.ExceptionType, "exception") && !strings.Contains(outcome.ExceptionType, "error") {
					return "", "", fmt.Errorf("outcome %q exception message is not observable through a supported std::exception type", outcome.ID)
				}
				source.WriteString(" if (std::string(ray_error.what()) != " + strconv.Quote(outcome.Message) + ") { std::cout.rdbuf(ray_old); return " + strconv.Itoa(failure) + "; }")
			}
			source.WriteString(" } catch (...) { std::cout.rdbuf(ray_old); return " + strconv.Itoa(failure+1) + "; } if (!ray_raised) { std::cout.rdbuf(ray_old); return " + strconv.Itoa(failure+2) + "; }\n")
		default:
			return "", "", fmt.Errorf("outcome %q kind %q is not probeable", outcome.ID, outcome.Kind)
		}
		source.WriteString("    std::cout.rdbuf(ray_old); if (ray_output.str() != " + strconv.Quote(expectedOutput) + ") return " + strconv.Itoa(failure+3) + "; }\n")
	}
	source.WriteString("  const std::string ray_json = ")
	source.WriteString(strconv.Quote(string(expectedJSON)))
	source.WriteString(";\n  std::ofstream ray_observation(argv[1], std::ios::binary | std::ios::trunc);\n")
	source.WriteString("  ray_observation.write(ray_json.data(), static_cast<std::streamsize>(ray_json.size()));\n  ray_observation.close();\n  return ray_observation ? 0 : 11;\n}\n")
	return "", source.String(), nil
}

func probeOutput(outcome semanticir.ObservableOutcome) (string, error) {
	var output strings.Builder
	for _, effect := range outcome.Effects {
		if effect.Kind != semanticir.EffectOutput || effect.Target != "stdout" || effect.Value == nil || effect.Value.Literal == nil || effect.Value.Literal.Type != semanticir.TypeString {
			return "", fmt.Errorf("effect %s:%s requires instrumentation that cannot be constructed exactly", effect.Kind, effect.Target)
		}
		output.WriteString(effect.Value.Literal.String)
	}
	return output.String(), nil
}
