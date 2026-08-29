package cpp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

const exhaustiveCaseTimeout = 5 * time.Second

// buildAuthoritativeEvidence selects one non-overlapping semantic authority.
// Test artifacts are evidence about predicates rather than implementations.
// Code currently admits exact finite compiled execution only for free
// functions whose complete observable trace is terminal-only. Stateful
// receivers/effects require compiler-semantic instrumentation or a model
// checker and therefore fail closed.
func (l *lowerer) buildAuthoritativeEvidence(ctx context.Context) {
	if semanticir.HasErrors(l.diagnostics) {
		return
	}
	if l.request.Kind == semanticir.ArtifactTests {
		projection, runner, err := l.buildTestProjectionEvidence()
		if err != nil {
			l.block(nil, "test-observation-projection", err.Error(), semanticir.DiagnosticIncomplete)
			return
		}
		l.testProjection, l.runnerSelection = &projection, &runner
		// Source-lowering's accept counter is an internal traversal metric and
		// counts expression wrappers.  Test coverage is instead the exact
		// compiler-grounded pass-influencing construct inventory validated by
		// TestObservationProjection.
		l.total = len(projection.Constructs)
		l.translated = len(projection.Constructs)
		return
	}
	if l.request.Kind != semanticir.ArtifactCode {
		return
	}
	evidence, err := l.exhaustivelyExecute(ctx)
	if err != nil {
		l.block(nil, "compiler-realization", err.Error(), semanticir.DiagnosticIncomplete)
		return
	}
	l.exhaustiveEvidence = []semanticir.ExhaustiveExecutionEvidence{evidence}
	closure, err := l.buildScopeClosure(ctx)
	if err != nil {
		l.block(nil, "patch-scope-closure", err.Error(), semanticir.DiagnosticIncomplete)
		l.exhaustiveEvidence = nil
		return
	}
	l.scopeClosure = &closure
}

func (l *lowerer) exhaustivelyExecute(ctx context.Context) (semanticir.ExhaustiveExecutionEvidence, error) {
	for _, operation := range l.operations {
		if operation.operation.Kind == semanticir.OperationTest {
			continue
		}
		if operation.operation.Kind != semanticir.OperationFunction {
			return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("operation %q requires receiver construction or callable state; exhaustive free-function execution cannot establish its semantics", operation.operation.ID)
		}
		if !cxxTypeName.MatchString(operation.operation.ID) {
			return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("operation %q has no exact renderable C++ callable name", operation.operation.ID)
		}
		for _, outcomeID := range operation.operation.OutcomeIDs {
			outcome, ok := l.outcomeByID(outcomeID)
			if !ok {
				return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("operation %q outcome %q is absent from the compiled vocabulary", operation.operation.ID, outcomeID)
			}
			if outcome.Kind == semanticir.OutcomeOther {
				continue
			}
			if len(outcome.Effects) != 0 {
				return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("operation %q outcome %q has call/write/output effects; exact compiler instrumentation is unavailable", operation.operation.ID, outcomeID)
			}
			if err := executableOutcome(outcome); err != nil {
				return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("operation %q outcome %q: %v", operation.operation.ID, outcomeID, err)
			}
		}
	}

	harness, err := l.renderExhaustiveHarness()
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, err
	}
	temporary, err := os.MkdirTemp("", "hyperray-cpp-exhaustive-")
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("create exhaustive compiler workspace: %v", err)
	}
	defer os.RemoveAll(temporary)
	binaryPath := filepath.Join(temporary, "harness.bin")

	common := []string{"-x", "c++"}
	common = append(common, l.compileFlags...)
	common = append(common, "-g", "-fsanitize=undefined", "-fno-sanitize-recover=all", "-fno-color-diagnostics")
	irArguments := append(append([]string(nil), common...), "-S", "-emit-llvm", "-o", "-", "-")
	ir, stderr, err := l.runCompiler(ctx, irArguments, harness)
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("emit exhaustive harness LLVM IR: %v: %s", err, strings.TrimSpace(string(stderr)))
	}
	if len(bytes.TrimSpace(stderr)) != 0 || !strings.Contains(string(ir), "target triple") || !strings.Contains(string(ir), "define ") {
		return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("exhaustive harness LLVM emission was incomplete or produced diagnostics: %s", strings.TrimSpace(string(stderr)))
	}
	compileArguments := append(append([]string(nil), common...), "-o", binaryPath, "-")
	_, stderr, err = l.runCompiler(ctx, compileArguments, harness)
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("compile exhaustive harness: %v: %s", err, strings.TrimSpace(string(stderr)))
	}
	if len(bytes.TrimSpace(stderr)) != 0 {
		return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("exhaustive harness compilation produced diagnostics: %s", strings.TrimSpace(string(stderr)))
	}
	binary, err := os.ReadFile(binaryPath)
	if err != nil || len(binary) == 0 {
		return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("read exhaustive harness executable: %v", err)
	}

	orders := [][]int{makeCaseOrder(len(l.cases), false), makeCaseOrder(len(l.cases), true)}
	runs := make([]semanticir.ExecutionRunEvidence, 0, len(orders))
	for runIndex, order := range orders {
		started := time.Now().UTC()
		observations := make([]semanticir.ExecutionObservation, 0, len(order))
		for _, caseIndex := range order {
			observation, runErr := l.executeCompiledCase(ctx, binaryPath, caseIndex)
			if runErr != nil {
				return semanticir.ExhaustiveExecutionEvidence{}, runErr
			}
			observations = append(observations, observation)
		}
		orderDigest, orderErr := semanticir.ExecutionOrderDigest(observations)
		observationDigest, observationErr := semanticir.ExecutionObservationDigest(observations)
		if orderErr != nil || observationErr != nil {
			return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("digest exhaustive run: order=%v observations=%v", orderErr, observationErr)
		}
		runs = append(runs, semanticir.ExecutionRunEvidence{
			ID: fmt.Sprintf("%s:run:%d", l.request.Artifact.ID, runIndex+1), StartedAtUTC: started.Format(time.RFC3339Nano),
			Observations: observations, OrderDigest: orderDigest, ObservationDigest: observationDigest,
			FreshProcessCount: len(observations), Provenance: l.provenance(nil, semanticir.TranslationTranslated),
		})
	}
	if len(runs) != 2 || runs[0].ObservationDigest != runs[1].ObservationDigest {
		return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("independent compiled executions produced different complete behavior relations")
	}
	groundings, err := l.executionGroundings()
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, err
	}
	environment := append([]semanticir.EnvironmentVariable(nil), l.request.Workspace.Environment...)
	environmentDigest, err := semanticir.Digest(environment)
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("digest exhaustive environment: %v", err)
	}
	// Argv records the deterministic IR-emission invocation. Harness contains
	// the exact stdin bytes; the separately bound executable digest records the
	// binary used for every fresh-process observation.
	argv := append([]string{l.request.Translator.Path}, irArguments...)
	executableDigest := semanticir.DigestBytes(binary)
	workingDirectory, err := l.evidenceWorkingDirectory()
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, err
	}
	steps, err := l.exhaustiveProbeSteps(common, harness, executableDigest, workingDirectory)
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, err
	}
	evidence := semanticir.ExhaustiveExecutionEvidence{
		ID: l.request.Artifact.ID + ":clang-exhaustive", Tool: l.request.Translator,
		SourceDigest: l.request.Artifact.Digest, WorkspaceTreeDigest: l.request.Workspace.TreeDigest,
		IRKind: semanticir.CompilerIRLLVM, EmittedIRDigest: semanticir.DigestBytes(ir),
		Harness: append([]byte(nil), harness...), HarnessPath: ".hyperray-cpp-exhaustive.cpp",
		HarnessDigest: semanticir.DigestBytes(harness), ExecutableDigest: executableDigest, Steps: steps,
		Argv: argv, WorkingDirectory: workingDirectory, Environment: environment, EnvironmentDigest: environmentDigest,
		ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: exhaustiveCaseTimeout.Milliseconds(),
		Groundings: groundings, CompleteAssignmentDigest: runs[0].ObservationDigest, Runs: runs, Complete: true,
		Provenance: l.provenance(nil, semanticir.TranslationTranslated),
	}
	coreDigest, err := semanticir.ExhaustiveExecutionCoreDigest(evidence)
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("digest exhaustive replay core: %v", err)
	}
	stepsDigest, err := semanticir.Digest(evidence.Steps)
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("digest exhaustive replay steps: %v", err)
	}
	generated := make([]semanticir.ProbeOutput, 0)
	for _, step := range evidence.Steps {
		generated = append(generated, step.Outputs...)
	}
	cleanupPaths := []string{".hyperray-cpp-exhaustive.bin"}
	cleanupDigest, err := semanticir.Digest(cleanupPaths)
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("digest exhaustive replay cleanup: %v", err)
	}
	evidence.Replay = semanticir.ExhaustiveReplayEvidence{
		CoreDigest: coreDigest, StepsDigest: stepsDigest, Runs: append([]semanticir.ExecutionRunEvidence(nil), evidence.Runs...),
		GeneratedOutputs: generated, CleanupPaths: cleanupPaths, CleanupDigest: cleanupDigest, Clean: true,
		Provenance: evidence.Provenance,
	}
	if err := l.adoptCompiledReferenceCases(runs[0].Observations); err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, err
	}
	return evidence, nil
}

// adoptCompiledReferenceCases crosses into the Spec vocabulary only after the
// pinned executable has emitted raw typed runtime facts. The AST-derived cases
// are advisory harness expectations: a disagreement has already failed the
// compiled run above, and the sole outcome-alphabet join happens here through
// semanticir.NormalizeReferenceCases.
func (l *lowerer) adoptCompiledReferenceCases(observations []semanticir.ExecutionObservation) error {
	advisory := make(map[string]semanticir.BehaviorCase, len(l.cases))
	for _, behaviorCase := range l.cases {
		advisory[semanticir.BehaviorCaseKey(behaviorCase)] = behaviorCase
	}
	rawCases := make([]semanticir.RawReferenceCase, 0, len(observations))
	for _, observation := range observations {
		key := semanticir.BehaviorRefKey(observation.Behavior)
		behaviorCase, ok := advisory[key]
		if !ok {
			return fmt.Errorf("compiled execution emitted an undeclared concrete behavior point %s", key)
		}
		rawCases = append(rawCases, semanticir.RawReferenceCase{
			ID: behaviorCase.ID, Conditions: cloneAssignment(observation.Behavior.Conditions),
			OperationID: observation.Behavior.OperationID, Inputs: cloneLiteralMap(observation.Inputs),
			Outcomes: []semanticir.RawOutcomeTrace{observation.RawOutcome}, Provenance: observation.Provenance,
		})
	}
	normalized, diagnostics := semanticir.NormalizeReferenceCases(l.request, rawCases)
	if semanticir.HasErrors(diagnostics) {
		return fmt.Errorf("normalize independently compiled C++ reference outcomes: %+v", diagnostics)
	}
	if !sameBehaviorRelation(normalized, l.cases) {
		return fmt.Errorf("compiled C++ reference relation differs from the advisory source lowering")
	}
	l.rawReferenceCases = rawCases
	l.cases = normalized
	return nil
}

func sameBehaviorRelation(left, right []semanticir.BehaviorCase) bool {
	if len(left) != len(right) {
		return false
	}
	index := make(map[string]semanticir.BehaviorCase, len(left))
	for _, behaviorCase := range left {
		index[semanticir.BehaviorCaseKey(behaviorCase)] = behaviorCase
	}
	if len(index) != len(left) {
		return false
	}
	for _, behaviorCase := range right {
		candidate, ok := index[semanticir.BehaviorCaseKey(behaviorCase)]
		if !ok || candidate.ID != behaviorCase.ID || !reflect.DeepEqual(candidate.OutcomeIDs, behaviorCase.OutcomeIDs) {
			return false
		}
	}
	return true
}

func (l *lowerer) exhaustiveProbeSteps(common []string, harness []byte, executableDigest, workingDirectory string) ([]semanticir.ProbeStep, error) {
	environment := append([]semanticir.EnvironmentVariable(nil), l.request.Workspace.Environment...)
	environmentDigest, _ := semanticir.Digest(environment)
	provenance := l.provenance(nil, semanticir.TranslationTranslated)
	emptyDigest := semanticir.DigestBytes(nil)
	noneSignalDigest := semanticir.DigestBytes(nil)
	output := semanticir.ProbeOutput{
		ID: "cpp-exhaustive-binary", Path: ".hyperray-cpp-exhaustive.bin", AfterDigest: executableDigest, Executable: true,
		Provenance: provenance,
	}
	compileArgv := append(append([]string(nil), common...), "-o", output.Path, "-")
	steps := []semanticir.ProbeStep{{
		ID: "compile", Kind: semanticir.ProbeStepSetup, Tool: l.request.Translator,
		Argv: compileArgv, Stdin: append([]byte(nil), harness...), StdinDigest: semanticir.DigestBytes(harness),
		WorkingDirectory: workingDirectory, Environment: environment, EnvironmentDigest: environmentDigest,
		ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: exhaustiveCaseTimeout.Milliseconds(),
		ExpectedExitCode: 0, ExpectedStdoutDigest: emptyDigest, ExpectedStderrDigest: emptyDigest, ExpectedSignalDigest: noneSignalDigest,
		SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone},
		Outputs:         []semanticir.ProbeOutput{output}, Provenance: provenance,
	}}
	for index, behaviorCase := range l.cases {
		outcome, ok := l.caseRawOutcomes[behaviorCase.ID]
		if !ok {
			return nil, fmt.Errorf("case %q has no exact provisional raw outcome", behaviorCase.ID)
		}
		rawOutcome, err := rawTraceFromOutcome(outcome)
		if err != nil {
			return nil, fmt.Errorf("case %q provisional raw outcome: %v", behaviorCase.ID, err)
		}
		stdout, err := semanticir.CanonicalJSON(rawOutcome)
		if err != nil {
			return nil, fmt.Errorf("encode case %q provisional raw outcome: %v", behaviorCase.ID, err)
		}
		steps = append(steps, semanticir.ProbeStep{
			ID: compiledCaseStepID(index), Kind: semanticir.ProbeStepRun, GeneratedExecutableID: output.ID,
			Argv: []string{strconv.Itoa(index)}, Stdin: []byte{}, StdinDigest: emptyDigest,
			WorkingDirectory: workingDirectory, Environment: environment, EnvironmentDigest: environmentDigest,
			ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: exhaustiveCaseTimeout.Milliseconds(),
			ExpectedExitCode: 0, ExpectedStdoutDigest: semanticir.DigestBytes(stdout), ExpectedStderrDigest: emptyDigest, ExpectedSignalDigest: semanticir.DigestBytes(stdout),
			SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalRawOutcomeStdout},
			Provenance:      behaviorCase.Provenance,
		})
	}
	return steps, nil
}

func (l *lowerer) evidenceWorkingDirectory() (string, error) {
	root := filepath.Clean(l.request.Workspace.Root)
	if canonical, err := filepath.EvalSymlinks(root); err == nil {
		root = canonical
	}
	directory := filepath.Clean(l.compileDirectory)
	if canonical, err := filepath.EvalSymlinks(directory); err == nil {
		directory = canonical
	}
	relative, err := filepath.Rel(root, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("resolve exhaustive compiler working directory: %v", err)
	}
	if relative == "" {
		relative = "."
	}
	return filepath.ToSlash(relative), nil
}

func compiledCaseStepID(caseIndex int) string { return fmt.Sprintf("run-case-%d", caseIndex) }

func executableOutcome(outcome semanticir.ObservableOutcome) error {
	switch outcome.Kind {
	case semanticir.OutcomeReturn, semanticir.OutcomeSuccess:
		if outcome.Value == nil {
			return fmt.Errorf("return has no typed value")
		}
		if outcome.Value.Type != semanticir.TypeUnit && outcome.Value.Type != semanticir.TypeBool && outcome.Value.Type != semanticir.TypeInteger && outcome.Value.Type != semanticir.TypeString {
			return fmt.Errorf("return type %q is not exactly observable", outcome.Value.Type)
		}
	case semanticir.OutcomeRaise:
		if !cxxTypeName.MatchString(outcome.ExceptionType) {
			return fmt.Errorf("exception type %q is not exactly renderable", outcome.ExceptionType)
		}
		if outcome.Message != "" && !strings.Contains(outcome.ExceptionType, "exception") && !strings.Contains(outcome.ExceptionType, "error") {
			return fmt.Errorf("exception message is not observable through %q", outcome.ExceptionType)
		}
	default:
		return fmt.Errorf("outcome kind %q is not executable", outcome.Kind)
	}
	return nil
}

func (l *lowerer) outcomeByID(id string) (semanticir.ObservableOutcome, bool) {
	for _, outcome := range l.outcomes {
		if outcome.ID == id {
			return outcome, true
		}
	}
	for _, outcome := range l.request.Outcomes {
		if outcome.ID == id {
			return outcome, true
		}
	}
	return semanticir.ObservableOutcome{}, false
}

func (l *lowerer) renderExhaustiveHarness() ([]byte, error) {
	focusRelative, err := filepath.Rel(l.compileDirectory, l.sourcePath)
	if err != nil {
		return nil, fmt.Errorf("resolve exhaustive focus source: %v", err)
	}
	hasMain := regexpMain.Match(l.request.Source)
	if hasMain && regexpMainDirective.Match(l.request.Source) {
		return nil, fmt.Errorf("focus translation unit has preprocessor-dependent main semantics")
	}
	operations := map[string]loweredOperation{}
	for _, operation := range l.operations {
		operations[operation.operation.ID] = operation
	}
	var source strings.Builder
	source.WriteString("#include <cstdlib>\n#include <iostream>\n#include <stdexcept>\n#include <string>\n")
	if hasMain {
		source.WriteString("#define main ray_frozen_program_main_")
		source.WriteString(strings.TrimPrefix(l.request.Artifact.Digest, "sha256:")[:16])
		source.WriteByte('\n')
	}
	source.WriteString("#include ")
	source.WriteString(strconv.Quote(filepath.ToSlash(focusRelative)))
	if hasMain {
		source.WriteString("\n#undef main")
	}
	source.WriteString("\nint main(int argc, char **argv) {\n  if (argc != 2) return 120;\n")
	source.WriteString("  char *ray_end = nullptr; long ray_case = std::strtol(argv[1], &ray_end, 10);\n  if (!ray_end || *ray_end != '\\0') return 121;\n  switch (ray_case) {\n")
	for index, behaviorCase := range l.cases {
		operation, ok := operations[behaviorCase.OperationID]
		if !ok {
			return nil, fmt.Errorf("case %q refers to absent operation", behaviorCase.ID)
		}
		exactInputs, exact := l.exactInputsForAssignment(operation.operation, behaviorCase.Conditions)
		if !exact {
			return nil, fmt.Errorf("case %q does not uniquely fix every operation input", behaviorCase.ID)
		}
		arguments := make([]string, 0, len(operation.operation.Inputs))
		for _, input := range operation.operation.Inputs {
			literal, typed := exactInputs[input.Name]
			if !typed || literal.Type != input.Type {
				return nil, fmt.Errorf("case %q has no exact input for %s", behaviorCase.ID, input.Name)
			}
			rendered, renderOK := renderLiteral(literal)
			if !renderOK {
				return nil, fmt.Errorf("case %q input %s is not renderable", behaviorCase.ID, input.Name)
			}
			arguments = append(arguments, string(rendered))
		}
		call := operation.operation.ID + "(" + strings.Join(arguments, ", ") + ")"
		source.WriteString("  case ")
		source.WriteString(strconv.Itoa(index))
		source.WriteString(": {\n    try {\n")
		expected, ok := l.caseRawOutcomes[behaviorCase.ID]
		if !ok {
			return nil, fmt.Errorf("case %q has no exact provisional raw outcome", behaviorCase.ID)
		}
		if len(expected.Effects) != 0 {
			return nil, fmt.Errorf("case %q has observable effects; exact compiler instrumentation is unavailable", behaviorCase.ID)
		}
		if err := executableOutcome(expected); err != nil {
			return nil, fmt.Errorf("case %q provisional raw outcome: %v", behaviorCase.ID, err)
		}
		trace, traceErr := rawTraceFromOutcome(expected)
		if traceErr != nil {
			return nil, fmt.Errorf("case %q provisional raw outcome: %v", behaviorCase.ID, traceErr)
		}
		raw, err := semanticir.CanonicalJSON(trace)
		if err != nil {
			return nil, fmt.Errorf("encode case %q provisional raw outcome: %v", behaviorCase.ID, err)
		}
		switch expected.Kind {
		case semanticir.OutcomeReturn:
			if expected.Value == nil || expected.Value.Type != operation.resultType {
				return nil, fmt.Errorf("case %q return outcome type differs from compiler declaration", behaviorCase.ID)
			}
			if operation.resultType == semanticir.TypeUnit {
				source.WriteString("      ")
				source.WriteString(call)
				source.WriteString("; std::cout << ")
				source.WriteString(strconv.Quote(string(raw)))
				source.WriteString("; return 0;\n")
			} else {
				source.WriteString("      auto ray_value = ")
				source.WriteString(call)
				source.WriteString(";\n")
				rendered, ok := renderLiteral(*expected.Value)
				if !ok {
					return nil, fmt.Errorf("case %q has an unrenderable return outcome", behaviorCase.ID)
				}
				source.WriteString("      if (ray_value == ")
				source.Write(rendered)
				source.WriteString(") { std::cout << ")
				source.WriteString(strconv.Quote(string(raw)))
				source.WriteString("; return 0; }\n      return 122;\n")
			}
		case semanticir.OutcomeRaise:
			source.WriteString("      (void)")
			source.WriteString(call)
			source.WriteString("; return 122;\n")
			source.WriteString("    } catch (const ")
			source.WriteString(expected.ExceptionType)
			source.WriteString(" &ray_error) {\n")
			if expected.Message != "" {
				source.WriteString("      if (std::string(ray_error.what()) != ")
				source.WriteString(strconv.Quote(expected.Message))
				source.WriteString(") return 123;\n")
			}
			source.WriteString("      std::cout << ")
			source.WriteString(strconv.Quote(string(raw)))
			source.WriteString("; return 0;\n")
		default:
			return nil, fmt.Errorf("case %q has unsupported provisional terminal %q", behaviorCase.ID, expected.Kind)
		}
		source.WriteString("    } catch (...) { return 124; }\n  }\n")
	}
	source.WriteString("  default: return 125;\n  }\n}\n")
	return []byte(source.String()), nil
}

var (
	regexpMain          = regexp.MustCompile(`(?m)\b(?:int|void|bool|auto)\s+main\s*\(`)
	regexpMainDirective = regexp.MustCompile(`(?m)^\s*#\s*(?:define|undef|ifn?def)\s+main\b`)
)

func (l *lowerer) runCompiler(ctx context.Context, arguments []string, stdin []byte) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, l.request.Translator.Path, arguments...)
	configureCPPCommand(command, l.request.Workspace)
	command.Dir = l.compileDirectory
	command.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func makeCaseOrder(count int, reverse bool) []int {
	order := make([]int, count)
	for index := range order {
		order[index] = index
	}
	if reverse {
		for left, right := 0, len(order)-1; left < right; left, right = left+1, right-1 {
			order[left], order[right] = order[right], order[left]
		}
	}
	return order
}

func (l *lowerer) executeCompiledCase(ctx context.Context, binaryPath string, caseIndex int) (semanticir.ExecutionObservation, error) {
	if caseIndex < 0 || caseIndex >= len(l.cases) {
		return semanticir.ExecutionObservation{}, fmt.Errorf("compiled case index %d is outside the model", caseIndex)
	}
	behaviorCase := l.cases[caseIndex]
	runCtx, cancel := context.WithTimeout(ctx, exhaustiveCaseTimeout)
	defer cancel()
	command := exec.CommandContext(runCtx, binaryPath, strconv.Itoa(caseIndex))
	configureCPPCommand(command, l.request.Workspace)
	command.Dir = l.compileDirectory
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	runErr := command.Run()
	exitCode := 0
	if runErr != nil {
		var exitError *exec.ExitError
		if errors.As(runErr, &exitError) {
			exitCode = exitError.ExitCode()
		} else {
			return semanticir.ExecutionObservation{}, fmt.Errorf("execute compiled case %q: %v", behaviorCase.ID, runErr)
		}
	}
	expectedOutcome, ok := l.caseRawOutcomes[behaviorCase.ID]
	if !ok {
		return semanticir.ExecutionObservation{}, fmt.Errorf("compiled case %q has no exact provisional raw outcome", behaviorCase.ID)
	}
	expectedTrace, err := rawTraceFromOutcome(expectedOutcome)
	if err != nil {
		return semanticir.ExecutionObservation{}, fmt.Errorf("form raw outcome for compiled case %q: %v", behaviorCase.ID, err)
	}
	expected, err := semanticir.CanonicalJSON(expectedTrace)
	if err != nil {
		return semanticir.ExecutionObservation{}, fmt.Errorf("encode raw outcome for compiled case %q: %v", behaviorCase.ID, err)
	}
	if exitCode != 0 || !bytes.Equal(stdout.Bytes(), expected) || stderr.Len() != 0 {
		return semanticir.ExecutionObservation{}, fmt.Errorf("compiled case %q differs from provisional semantics: exit=%d stdout=%q stderr=%q want=%q", behaviorCase.ID, exitCode, stdout.String(), stderr.String(), string(expected))
	}
	rawOutcome, err := decodeRawOutcomeTrace(stdout.Bytes())
	if err != nil {
		return semanticir.ExecutionObservation{}, fmt.Errorf("decode compiled case %q raw outcome: %v", behaviorCase.ID, err)
	}
	operation, exists := operationLookup(l.operations)[behaviorCase.OperationID]
	if !exists {
		return semanticir.ExecutionObservation{}, fmt.Errorf("compiled case %q operation is absent", behaviorCase.ID)
	}
	mappedOutcomeID, err := semanticir.ClassifyRawOutcome(operation.operation, rawOutcome, behaviorCase.Provenance)
	if err != nil || len(behaviorCase.OutcomeIDs) != 1 || mappedOutcomeID != behaviorCase.OutcomeIDs[0] {
		return semanticir.ExecutionObservation{}, fmt.Errorf("compiled case %q differs from provisional semantics: raw trace maps to %q, want %v: %v", behaviorCase.ID, mappedOutcomeID, behaviorCase.OutcomeIDs, err)
	}
	var inputs map[string]semanticir.Literal
	for _, grounding := range l.request.Groundings {
		if grounding.OperationID == behaviorCase.OperationID && assignmentsEqual(grounding.Conditions, behaviorCase.Conditions) {
			if inputs != nil {
				return semanticir.ExecutionObservation{}, fmt.Errorf("case %q has duplicate exact input maps", behaviorCase.ID)
			}
			inputs = cloneLiteralMap(grounding.Inputs)
		}
	}
	if inputs == nil {
		// Legacy direct-literal fixtures do not carry the strict assignment
		// registry. Derive their complete input map from the one-input domains.
		for _, lowered := range l.operations {
			if lowered.operation.ID != behaviorCase.OperationID {
				continue
			}
			inputs = map[string]semanticir.Literal{}
			for _, input := range lowered.operation.Inputs {
				valueID, exists := behaviorCase.Conditions[input.DomainID]
				literal, ok := typedDomainLiteralForInput(l.findDomain(input.DomainID), valueID, lowered.operation.ID, input.Name)
				if !exists || !ok {
					return semanticir.ExecutionObservation{}, fmt.Errorf("case %q has no exact grounding for input %s", behaviorCase.ID, input.Name)
				}
				inputs[input.Name] = literal
			}
		}
	}
	return semanticir.ExecutionObservation{
		Behavior: semanticir.BehaviorRef{OperationID: behaviorCase.OperationID, Conditions: cloneAssignment(behaviorCase.Conditions), Inputs: cloneLiteralMap(inputs), Provenance: behaviorCase.Provenance},
		Inputs:   inputs, StepID: compiledCaseStepID(caseIndex), RawOutcome: rawOutcome,
		OutcomeIDs: []string{mappedOutcomeID}, ExitCode: exitCode,
		Stdout: append([]byte(nil), stdout.Bytes()...), StdoutDigest: semanticir.DigestBytes(stdout.Bytes()),
		Stderr: append([]byte(nil), stderr.Bytes()...), StderrDigest: semanticir.DigestBytes(stderr.Bytes()),
		SignalValue: append([]byte(nil), expected...), SignalValueDigest: semanticir.DigestBytes(expected), Provenance: behaviorCase.Provenance,
	}, nil
}

func rawTraceFromOutcome(outcome semanticir.ObservableOutcome) (semanticir.RawOutcomeTrace, error) {
	trace := semanticir.RawOutcomeTrace{
		Kind: outcome.Kind, Value: outcome.Value, ExceptionType: outcome.ExceptionType,
		Message: outcome.Message, Effects: []semanticir.RawEffectTrace{},
	}
	for _, effect := range outcome.Effects {
		var value *semanticir.Literal
		if effect.Value != nil {
			if effect.Value.Kind != semanticir.ExprLiteral || effect.Value.Literal == nil || len(effect.Value.Operands) != 0 {
				return semanticir.RawOutcomeTrace{}, fmt.Errorf("effect %s:%s value is not one exact runtime literal", effect.Kind, effect.Target)
			}
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

func decodeRawOutcomeTrace(data []byte) (semanticir.RawOutcomeTrace, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var trace semanticir.RawOutcomeTrace
	if err := decoder.Decode(&trace); err != nil {
		return semanticir.RawOutcomeTrace{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return semanticir.RawOutcomeTrace{}, fmt.Errorf("raw outcome contains trailing JSON value")
		}
		return semanticir.RawOutcomeTrace{}, fmt.Errorf("raw outcome has trailing bytes: %v", err)
	}
	if err := semanticir.ValidateRawOutcomeTrace(trace); err != nil {
		return semanticir.RawOutcomeTrace{}, err
	}
	canonical, err := semanticir.CanonicalJSON(trace)
	if err != nil || !bytes.Equal(canonical, data) {
		return semanticir.RawOutcomeTrace{}, fmt.Errorf("raw outcome is not canonical JSON")
	}
	return trace, nil
}

func (l *lowerer) mustOutcomeByID(id string) semanticir.ObservableOutcome {
	outcome, _ := l.outcomeByID(id)
	return outcome
}

func (l *lowerer) executionGroundings() ([]semanticir.AssignmentGrounding, error) {
	result := append([]semanticir.AssignmentGrounding(nil), l.request.Groundings...)
	if len(result) != 0 {
		return result, nil
	}
	// Legacy direct-literal fixtures have no assignment registry. Preserve a
	// deterministic compatibility record; strict requests always use the
	// frozen request registry above.
	for _, behaviorCase := range l.cases {
		var inputs map[string]semanticir.Literal
		for _, operation := range l.operations {
			if operation.operation.ID != behaviorCase.OperationID {
				continue
			}
			inputs = map[string]semanticir.Literal{}
			for _, input := range operation.operation.Inputs {
				valueID := behaviorCase.Conditions[input.DomainID]
				literal, ok := typedDomainLiteralForInput(l.findDomain(input.DomainID), valueID, operation.operation.ID, input.Name)
				if !ok {
					return nil, fmt.Errorf("case %q input %q has no exact C++ grounding", behaviorCase.ID, input.Name)
				}
				inputs[input.Name] = literal
			}
		}
		result = append(result, semanticir.AssignmentGrounding{
			ID: semanticir.AssignmentGroundingID(behaviorCase.OperationID, behaviorCase.Conditions), OperationID: behaviorCase.OperationID,
			Conditions: cloneAssignment(behaviorCase.Conditions), Inputs: inputs, Provenance: behaviorCase.Provenance,
		})
	}
	return result, nil
}

func cloneLiteralMap(source map[string]semanticir.Literal) map[string]semanticir.Literal {
	result := make(map[string]semanticir.Literal, len(source))
	for name, literal := range source {
		result[name] = literal
	}
	return result
}

func assignmentsEqual(left, right semanticir.Assignment) bool {
	if len(left) != len(right) {
		return false
	}
	for domainID, valueID := range left {
		if right[domainID] != valueID {
			return false
		}
	}
	return true
}
