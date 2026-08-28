package rust

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/semanticir"
)

// evaluatedOutcome is populated only from a pinned-rustc executable (or from
// a static expected literal in an assertion). It is never produced by
// interpreting Rust syntax in Go.
type evaluatedOutcome struct {
	Kind          semanticir.OutcomeKind
	Literal       *semanticir.Literal
	ExceptionType string
	Message       string
	Span          sourceSpan
}

type rustExecutionCase struct {
	Function   functionDecl
	Conditions semanticir.Assignment
	Arguments  []string
	Index      int
}

type rustExhaustiveSeed struct {
	Harness          []byte
	HarnessPath      string
	ExecutableDigest string
	Argv             []string
	WorkingDirectory string
	Steps            []semanticir.ProbeStep
	Groundings       []semanticir.AssignmentGrounding
	Runs             []semanticir.ExecutionRunEvidence
}

func enumerateBehavior(ctx context.Context, request semanticir.FrontendRequest, functions []functionDecl) ([]semanticir.BehaviorCase, []semanticir.RawReferenceCase, rustExhaustiveSeed, []semanticir.Diagnostic) {
	if request.Kind == semanticir.ArtifactTests {
		return nil, nil, rustExhaustiveSeed{}, nil
	}
	functionMap := make(map[string]bool, len(functions))
	for _, fn := range functions {
		functionMap[fn.Name] = true
	}
	for _, entry := range request.EntryPoints {
		if !functionMap[entry] {
			return nil, nil, rustExhaustiveSeed{}, []semanticir.Diagnostic{diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticInvalidReference, fmt.Sprintf("Rust entry point %q does not name a translated function", entry))}
		}
	}
	maxCases := 4096
	if raw := request.Options["max_cases"]; raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return nil, nil, rustExhaustiveSeed{}, []semanticir.Diagnostic{diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticInvalidInput, "max_cases must be a positive integer")}
		}
		maxCases = parsed
	}

	var executions []rustExecutionCase
	var diagnostics []semanticir.Diagnostic
	for _, fn := range functions {
		if fn.IsTest {
			continue
		}
		if len(request.Operations) != 0 {
			if _, requested := requestOperation(request.Operations, fn.Name); !requested {
				continue
			}
		}
		domains := make([]semanticir.Domain, len(fn.Parameters))
		combinations := 1
		valid := true
		for index, param := range fn.Parameters {
			domainID := findDomainID(request, fn.Name, param.Name)
			domain, ok := findDomain(request.FiniteDomains, domainID)
			if !ok || len(domain.Values) == 0 {
				diagnostics = append(diagnostics, diagnostic(request.Artifact, param.Span, semanticir.DiagnosticMissingDomain, fmt.Sprintf("parameter %s.%s has no non-empty finite domain", fn.Name, param.Name)))
				valid = false
				continue
			}
			if len(domain.Values) > maxCases/combinations {
				diagnostics = append(diagnostics, diagnostic(request.Artifact, fn.Span, semanticir.DiagnosticNonFinite, fmt.Sprintf("finite Cartesian product for %s exceeds max_cases=%d", fn.Name, maxCases)))
				valid = false
				continue
			}
			combinations *= len(domain.Values)
			domains[index] = domain
		}
		if !valid {
			continue
		}
		indices := make([]int, len(domains))
		for combination := 0; combination < combinations; combination++ {
			if err := ctx.Err(); err != nil {
				return nil, nil, rustExhaustiveSeed{}, append(diagnostics, diagnostic(request.Artifact, fn.Span, semanticir.DiagnosticInvalidInput, "Rust bounded execution cancelled: "+err.Error()))
			}
			conditions := make(semanticir.Assignment, len(fn.Parameters))
			arguments := make([]string, len(fn.Parameters))
			caseValid := true
			for index, param := range fn.Parameters {
				member := domains[index].Values[indices[index]]
				argument, ok := renderRustDomainArgument(domains[index], member, fn.Name, param.Name, param.Type)
				if !ok {
					diagnostics = append(diagnostics, diagnostic(request.Artifact, param.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("domain value %q cannot be rendered exactly as Rust type %s", member.ID, param.Type)))
					caseValid = false
					break
				}
				conditions[domains[index].ID] = member.ID
				arguments[index] = argument
			}
			if caseValid && !assignmentExcluded(request.Constraints, fn.Name, conditions) {
				executions = append(executions, rustExecutionCase{Function: fn, Conditions: conditions, Arguments: arguments, Index: len(executions)})
			}
			incrementIndices(indices, domains)
		}
	}
	if semanticir.HasErrors(diagnostics) {
		return nil, nil, rustExhaustiveSeed{}, deduplicateDiagnostics(diagnostics)
	}
	observed, executionSeed, executionDiagnostics := executeRustCases(ctx, request, executions)
	if semanticir.HasErrors(executionDiagnostics) {
		return nil, nil, executionSeed, executionDiagnostics
	}
	var rawCases []semanticir.RawReferenceCase
	for _, execution := range executions {
		value, ok := observed[execution.Index]
		if !ok {
			return nil, nil, executionSeed, []semanticir.Diagnostic{diagnostic(request.Artifact, execution.Function.Span, semanticir.DiagnosticIncomplete, fmt.Sprintf("pinned Rust harness emitted no result for %s case %d", execution.Function.Name, execution.Index))}
		}
		inputs, inputDiagnostics := rustExecutionInputs(request, execution)
		if len(inputDiagnostics) != 0 {
			return nil, nil, executionSeed, inputDiagnostics
		}
		prov := provenance(request.Artifact, execution.Function.Span, semanticir.TranslationTranslated)
		rawCases = append(rawCases, semanticir.RawReferenceCase{
			ID:          fmt.Sprintf("%s.case.%d", execution.Function.Name, execution.Index+1),
			Conditions:  cloneAssignment(execution.Conditions),
			OperationID: execution.Function.Name,
			Inputs:      inputs,
			Outcomes:    []semanticir.RawOutcomeTrace{rawOutcomeTrace(value)},
			Provenance:  prov,
		})
	}
	cases, normalizationDiagnostics := semanticir.NormalizeReferenceCases(request, rawCases)
	if semanticir.HasErrors(normalizationDiagnostics) {
		return nil, rawCases, executionSeed, normalizationDiagnostics
	}
	for runIndex := range executionSeed.Runs {
		for observationIndex := range executionSeed.Runs[runIndex].Observations {
			observation := &executionSeed.Runs[runIndex].Observations[observationIndex]
			operation, declared := requestOperation(request.Operations, observation.Behavior.OperationID)
			if !declared {
				return nil, nil, executionSeed, []semanticir.Diagnostic{diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticInvalidReference, "execution evidence refers to an undeclared Rust operation")}
			}
			mappedID, classifyErr := semanticir.ClassifyRawOutcome(operation, observation.RawOutcome, observation.Provenance)
			if classifyErr != nil {
				return nil, nil, executionSeed, []semanticir.Diagnostic{diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticInvalidInput, "classify Rust raw outcome trace: "+classifyErr.Error())}
			}
			observation.OutcomeIDs = []string{mappedID}
		}
		digest, err := semanticir.ExecutionObservationDigest(executionSeed.Runs[runIndex].Observations)
		if err != nil {
			return nil, nil, executionSeed, []semanticir.Diagnostic{diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticInvalidInput, "digest classified Rust observations: "+err.Error())}
		}
		executionSeed.Runs[runIndex].ObservationDigest = digest
	}
	return cases, rawCases, executionSeed, nil
}

func executeRustCases(ctx context.Context, request semanticir.FrontendRequest, cases []rustExecutionCase) (map[int]evaluatedOutcome, rustExhaustiveSeed, []semanticir.Diagnostic) {
	if len(cases) == 0 {
		return map[int]evaluatedOutcome{}, rustExhaustiveSeed{}, []semanticir.Diagnostic{diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticIncomplete, "Rust exhaustive execution has no reachable assignments")}
	}
	var harness strings.Builder
	harness.Write(request.Source)
	harness.WriteString("\n\nfn main() {\n    std::panic::set_hook(Box::new(|_| {}));\n    let ray_case = std::env::args().nth(1).and_then(|value| value.parse::<usize>().ok()).unwrap_or_else(|| std::process::exit(64));\n    match ray_case {\n")
	for _, item := range cases {
		fmt.Fprintf(&harness, "        %d => ", item.Index)
		writeRustHarnessCase(&harness, item)
		harness.WriteString(",\n")
	}
	harness.WriteString("        _ => std::process::exit(64),\n    }\n}\n")
	discoveryBytes := []byte(harness.String())
	observed, discoveryDiagnostics := discoverRustOutcomes(ctx, request, discoveryBytes, cases)
	if semanticir.HasErrors(discoveryDiagnostics) {
		return nil, rustExhaustiveSeed{}, discoveryDiagnostics
	}
	verifiedBytes, exact := buildRustVerifiedHarness(request, cases, observed)
	if !exact {
		return nil, rustExhaustiveSeed{}, []semanticir.Diagnostic{diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticUnsupported, "Rust runtime outcome cannot be checked and emitted as canonical typed JSON")}
	}
	observed, seed, diagnostics := compileAndRunRustHarness(ctx, request, verifiedBytes, cases, observed)
	return observed, seed, diagnostics
}

func discoverRustOutcomes(ctx context.Context, request semanticir.FrontendRequest, source []byte, cases []rustExecutionCase) (map[int]evaluatedOutcome, []semanticir.Diagnostic) {
	whole := wholeSpan(request.Source)
	tempDir, err := os.MkdirTemp("", "ray-rust-discovery-*")
	if err != nil {
		return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, "create Rust discovery workspace: "+err.Error())}
	}
	defer os.RemoveAll(tempDir)
	sourcePath, executablePath := filepath.Join(tempDir, "discovery.rs"), filepath.Join(tempDir, "discovery")
	if err := os.WriteFile(sourcePath, source, 0o600); err != nil {
		return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, "write Rust discovery harness: "+err.Error())}
	}
	args := []string{"--edition=2021", "--crate-name", "ray_frontend_discovery", "-C", "panic=unwind", "-C", "overflow-checks=yes", sourcePath, "-o", executablePath}
	compileContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	compile := exec.CommandContext(compileContext, request.Translator.Path, args...)
	configureRustCommand(compile, request.Workspace)
	compile.Dir = tempDir
	output, compileErr := compile.CombinedOutput()
	cancel()
	if compileErr != nil {
		return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticUnsupported, "pinned rustc could not compile the discovery harness: "+strings.TrimSpace(string(output)))}
	}
	observed := make(map[int]evaluatedOutcome, len(cases))
	for _, item := range cases {
		runContext, cancelRun := context.WithTimeout(ctx, 30*time.Second)
		command := exec.CommandContext(runContext, executablePath, strconv.Itoa(item.Index))
		configureRustCommand(command, request.Workspace)
		command.Dir = tempDir
		stdout, runErr := command.Output()
		cancelRun()
		if runErr != nil {
			return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, item.Function.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("Rust discovery execution failed for %s assignment %d: %v", item.Function.Name, item.Index, runErr))}
		}
		parsed, parseDiagnostics := parseRustHarnessOutput(request, stdout, cases)
		value, exists := parsed[item.Index]
		if semanticir.HasErrors(parseDiagnostics) || !exists || len(parsed) != 1 {
			if len(parseDiagnostics) != 0 {
				return nil, parseDiagnostics
			}
			return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, item.Function.Span, semanticir.DiagnosticIncomplete, "Rust discovery process did not emit exactly its selected assignment")}
		}
		observed[item.Index] = value
	}
	return observed, nil
}

func buildRustVerifiedHarness(request semanticir.FrontendRequest, cases []rustExecutionCase, observed map[int]evaluatedOutcome) ([]byte, bool) {
	var harness strings.Builder
	harness.Write(request.Source)
	harness.WriteString("\n\nfn main() {\n    std::panic::set_hook(Box::new(|_| {}));\n    let ray_case = std::env::args().nth(1).and_then(|value| value.parse::<usize>().ok()).unwrap_or_else(|| std::process::exit(64));\n    match ray_case {\n")
	for _, item := range cases {
		value, exists := observed[item.Index]
		if !exists {
			return nil, false
		}
		raw := rawOutcomeTrace(value)
		encoded, err := semanticir.CanonicalJSON(raw)
		if err != nil {
			return nil, false
		}
		fmt.Fprintf(&harness, "        %d => ", item.Index)
		if !writeRustVerifiedHarnessCase(&harness, item, value, raw, string(encoded)) {
			return nil, false
		}
		harness.WriteString(",\n")
	}
	harness.WriteString("        _ => std::process::exit(64),\n    }\n}\n")
	return []byte(harness.String()), true
}

func writeRustVerifiedHarnessCase(harness *strings.Builder, item rustExecutionCase, value evaluatedOutcome, outcome semanticir.RawOutcomeTrace, encoded string) bool {
	emit := "print!(\"{}\", " + strconv.Quote(encoded) + ")"
	fmt.Fprintf(harness, "{ let observed = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| %s(%s))); match observed { ", item.Function.Name, strings.Join(item.Arguments, ", "))
	switch outcome.Kind {
	case semanticir.OutcomeReturn:
		if outcome.Value == nil {
			return false
		}
		literal, ok := renderRustLiteral(*outcome.Value)
		if !ok {
			return false
		}
		fmt.Fprintf(harness, "Ok(value) if value == %s => %s, _ => std::process::exit(65) } }", literal, emit)
	case semanticir.OutcomeSuccess:
		if value.Literal == nil {
			return false
		}
		literal, ok := renderRustLiteral(*value.Literal)
		if !ok {
			return false
		}
		fmt.Fprintf(harness, "Ok(Ok(value)) if value == %s => %s, _ => std::process::exit(65) } }", literal, emit)
	case semanticir.OutcomeRaise:
		switch outcome.ExceptionType {
		case "Result::Err":
			if value.Literal == nil {
				return false
			}
			literal, ok := renderRustLiteral(*value.Literal)
			if !ok {
				return false
			}
			fmt.Fprintf(harness, "Ok(Err(value)) if value == %s => %s, _ => std::process::exit(65) } }", literal, emit)
		case "panic":
			fmt.Fprintf(harness, "Err(payload) => { let message = if let Some(value) = payload.downcast_ref::<&str>() { (*value).to_string() } else if let Some(value) = payload.downcast_ref::<String>() { value.clone() } else { String::from(\"<non-string-panic>\") }; if message == %s { %s } else { std::process::exit(65) } }, _ => std::process::exit(65) } }", strconv.Quote(outcome.Message), emit)
		default:
			return false
		}
	default:
		return false
	}
	return true
}

func writeRustHarnessCase(harness *strings.Builder, item rustExecutionCase) {
	fmt.Fprintf(harness, "    { let observed = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| %s(%s))); match observed { ", item.Function.Name, strings.Join(item.Arguments, ", "))
	if successType, errorType, result := rustResultTypes(item.Function.ReturnType); result {
		harness.WriteString("Ok(Ok(value)) => ")
		writeRustPrint(harness, item.Index, "S", successType, "value")
		harness.WriteString(", Ok(Err(value)) => ")
		writeRustPrint(harness, item.Index, "E", errorType, "value")
	} else {
		valueType, _ := rustValueType(item.Function.ReturnType)
		harness.WriteString("Ok(value) => ")
		writeRustPrint(harness, item.Index, "R", valueType, "value")
	}
	fmt.Fprintf(harness, ", Err(payload) => { let message = if let Some(value) = payload.downcast_ref::<&str>() { (*value).to_string() } else if let Some(value) = payload.downcast_ref::<String>() { value.clone() } else { String::from(\"<non-string-panic>\") }; println!(\"__RAY__\\t%d\\tP\\t{:?}\", message); } } }\n", item.Index)
}

func writeRustPrint(harness *strings.Builder, index int, kind string, valueType semanticir.ValueType, variable string) {
	switch valueType {
	case semanticir.TypeString:
		fmt.Fprintf(harness, "println!(\"__RAY__\\t%d\\t%s\\t{:?}\", %s)", index, kind, variable)
	case semanticir.TypeUnit:
		fmt.Fprintf(harness, "{ let _ = %s; println!(\"__RAY__\\t%d\\t%s\\t\"); }", variable, index, kind)
	default:
		fmt.Fprintf(harness, "println!(\"__RAY__\\t%d\\t%s\\t{}\", %s)", index, kind, variable)
	}
}

func compileAndRunRustHarness(ctx context.Context, request semanticir.FrontendRequest, source []byte, cases []rustExecutionCase, expected map[int]evaluatedOutcome) (observed map[int]evaluatedOutcome, seed rustExhaustiveSeed, diagnostics []semanticir.Diagnostic) {
	whole := wholeSpan(request.Source)
	seed = rustExhaustiveSeed{Harness: append([]byte(nil), source...)}
	root := request.Workspace.Root
	beforeDigest, err := executor.WorkspaceDigest(root)
	if err != nil || beforeDigest != request.Workspace.TreeDigest {
		return nil, seed, []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticStaleArtifact, "Rust execution workspace differs from its frozen tree digest")}
	}
	token := strings.TrimPrefix(request.Artifact.Digest, "sha256:")[:16]
	sourceRelative := ".ray-rust-exhaustive-" + token + ".rs"
	executableRelative := ".ray-rust-exhaustive-" + token
	sourcePath := filepath.Join(root, sourceRelative)
	executablePath := filepath.Join(root, executableRelative)
	for _, path := range []string{sourcePath, executablePath} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			return nil, seed, []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidReference, "Rust exhaustive replay path is not fresh: "+filepath.Base(path))}
		}
	}
	defer func() {
		for _, path := range []string{sourcePath, executablePath} {
			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, "clean Rust exhaustive replay artifact: "+removeErr.Error()))
			}
		}
		afterDigest, digestErr := executor.WorkspaceDigest(root)
		if digestErr != nil || afterDigest != request.Workspace.TreeDigest {
			diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticStaleArtifact, "Rust exhaustive execution did not restore the frozen workspace"))
		}
		if semanticir.HasErrors(diagnostics) {
			observed = nil
		}
	}()
	if err := os.WriteFile(sourcePath, source, 0o700); err != nil {
		return nil, seed, []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, "write Rust execution harness: "+err.Error())}
	}
	args := []string{"--edition=2021", "--crate-name", "ray_frontend_harness", "-C", "panic=unwind", "-C", "overflow-checks=yes", sourceRelative, "-o", executableRelative}
	seed.Argv = append([]string{request.Translator.Path}, args...)
	seed.WorkingDirectory = "."
	seed.HarnessPath = sourceRelative
	compileContext, cancelCompile := context.WithTimeout(ctx, 30*time.Second)
	compile := exec.CommandContext(compileContext, request.Translator.Path, args...)
	configureRustCommand(compile, request.Workspace)
	compile.Dir = root
	var compileStdout, compileStderr bytes.Buffer
	compile.Stdout, compile.Stderr = &compileStdout, &compileStderr
	err = compile.Run()
	cancelCompile()
	if err != nil {
		return nil, seed, []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticUnsupported, "pinned rustc could not compile the exact bounded execution harness: "+strings.TrimSpace(compileStderr.String()))}
	}
	executable, err := os.ReadFile(executablePath)
	if err != nil {
		return nil, seed, []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticIncomplete, "read exact Rust harness executable: "+err.Error())}
	}
	seed.ExecutableDigest = semanticir.DigestBytes(executable)
	seed.Steps = append(seed.Steps, semanticir.ProbeStep{
		ID: "compile", Kind: semanticir.ProbeStepSetup, Tool: request.Translator, Argv: append([]string(nil), args...),
		StdinDigest: semanticir.DigestBytes(nil), WorkingDirectory: ".",
		Environment: append([]semanticir.EnvironmentVariable(nil), request.Workspace.Environment...), EnvironmentDigest: request.Workspace.EnvironmentDigest,
		ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 30000, ExpectedExitCode: 0,
		ExpectedStdoutDigest: semanticir.DigestBytes(compileStdout.Bytes()), ExpectedStderrDigest: semanticir.DigestBytes(compileStderr.Bytes()), ExpectedSignalDigest: semanticir.DigestBytes(nil),
		SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone},
		Outputs:         []semanticir.ProbeOutput{{ID: "rust-exhaustive-executable", Path: executableRelative, AfterDigest: seed.ExecutableDigest, Executable: true, Provenance: provenance(request.Artifact, whole, semanticir.TranslationTranslated)}},
		Provenance:      provenance(request.Artifact, whole, semanticir.TranslationTranslated),
	})
	groundings, groundingDiagnostics := rustExecutionGroundings(request, cases)
	if semanticir.HasErrors(groundingDiagnostics) {
		return nil, seed, groundingDiagnostics
	}
	seed.Groundings = groundings
	forward := make([]int, len(cases))
	for index := range cases {
		forward[index] = index
	}
	reverse := append([]int(nil), forward...)
	for left, right := 0, len(reverse)-1; left < right; left, right = left+1, right-1 {
		reverse[left], reverse[right] = reverse[right], reverse[left]
	}
	canonical := make(map[int]evaluatedOutcome, len(expected))
	for runIndex, order := range [][]int{forward, reverse} {
		runStarted := time.Now().UTC().Format(time.RFC3339Nano)
		observations := make([]semanticir.ExecutionObservation, 0, len(cases))
		for _, position := range order {
			item := cases[position]
			value, exists := expected[item.Index]
			if !exists {
				return nil, seed, []semanticir.Diagnostic{diagnostic(request.Artifact, item.Function.Span, semanticir.DiagnosticIncomplete, "verified Rust harness has no discovered raw outcome")}
			}
			processContext, cancel := context.WithTimeout(ctx, 30*time.Second)
			command := exec.CommandContext(processContext, executablePath, strconv.Itoa(item.Index))
			configureRustCommand(command, request.Workspace)
			command.Dir = root
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			runErr := command.Run()
			cancel()
			if runErr != nil {
				return nil, seed, []semanticir.Diagnostic{diagnostic(request.Artifact, item.Function.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("fresh Rust execution failed for %s assignment %d: %v: %s", item.Function.Name, item.Index, runErr, strings.TrimSpace(stderr.String())))}
			}
			inputs, inputDiagnostics := rustExecutionInputs(request, item)
			if semanticir.HasErrors(inputDiagnostics) {
				return nil, seed, inputDiagnostics
			}
			trace := rawOutcomeTrace(value)
			outcome := semanticOutcome(request.Artifact, item.Function.Name, value)
			signal, signalErr := semanticir.CanonicalJSON(trace)
			if signalErr != nil {
				return nil, seed, []semanticir.Diagnostic{diagnostic(request.Artifact, item.Function.Span, semanticir.DiagnosticInvalidInput, "encode exact Rust runtime outcome: "+signalErr.Error())}
			}
			if !bytes.Equal(stdout.Bytes(), signal) {
				return nil, seed, []semanticir.Diagnostic{diagnostic(request.Artifact, item.Function.Span, semanticir.DiagnosticUnsupported, "verified Rust process did not directly emit its canonical typed raw outcome")}
			}
			stepID := fmt.Sprintf("run-%d-case-%d", runIndex+1, item.Index)
			seed.Steps = append(seed.Steps, semanticir.ProbeStep{
				ID: stepID, Kind: semanticir.ProbeStepRun, GeneratedExecutableID: "rust-exhaustive-executable", Argv: []string{strconv.Itoa(item.Index)},
				StdinDigest: semanticir.DigestBytes(nil), WorkingDirectory: ".",
				Environment: append([]semanticir.EnvironmentVariable(nil), request.Workspace.Environment...), EnvironmentDigest: request.Workspace.EnvironmentDigest,
				ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 30000, ExpectedExitCode: 0,
				ExpectedStdoutDigest: semanticir.DigestBytes(stdout.Bytes()), ExpectedStderrDigest: semanticir.DigestBytes(stderr.Bytes()), ExpectedSignalDigest: semanticir.DigestBytes(signal),
				SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalRawOutcomeStdout},
				Provenance:      provenance(request.Artifact, item.Function.Span, semanticir.TranslationTranslated),
			})
			observations = append(observations, semanticir.ExecutionObservation{
				Behavior: semanticir.BehaviorRef{OperationID: item.Function.Name, Conditions: cloneAssignment(item.Conditions), Inputs: cloneLiteralMap(inputs), Provenance: provenance(request.Artifact, item.Function.Span, semanticir.TranslationTranslated)},
				Inputs:   inputs, StepID: stepID, RawOutcome: trace, OutcomeIDs: []string{outcome.ID}, ExitCode: 0,
				Stdout: append([]byte(nil), stdout.Bytes()...), StdoutDigest: semanticir.DigestBytes(stdout.Bytes()),
				Stderr: append([]byte(nil), stderr.Bytes()...), StderrDigest: semanticir.DigestBytes(stderr.Bytes()),
				SignalValue: signal, SignalValueDigest: semanticir.DigestBytes(signal),
				Provenance: provenance(request.Artifact, item.Function.Span, semanticir.TranslationTranslated),
			})
		}
		observationDigest, observationErr := semanticir.ExecutionObservationDigest(observations)
		orderDigest, orderErr := semanticir.ExecutionOrderDigest(observations)
		if observationErr != nil || orderErr != nil {
			return nil, seed, []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, "digest exhaustive Rust observations")}
		}
		seed.Runs = append(seed.Runs, semanticir.ExecutionRunEvidence{
			ID: fmt.Sprintf("rust-run-%d", runIndex+1), StartedAtUTC: runStarted, Observations: observations,
			OrderDigest: orderDigest, ObservationDigest: observationDigest, FreshProcessCount: len(observations),
			Provenance: provenance(request.Artifact, whole, semanticir.TranslationTranslated),
		})
		for index, value := range expected {
			canonical[index] = value
		}
	}
	return canonical, seed, nil
}

func rustExecutionGroundings(request semanticir.FrontendRequest, cases []rustExecutionCase) ([]semanticir.AssignmentGrounding, []semanticir.Diagnostic) {
	byBehavior := make(map[string]semanticir.AssignmentGrounding, len(request.Groundings))
	for _, grounding := range request.Groundings {
		key := grounding.OperationID + "\x00" + assignmentKey(grounding.Conditions)
		if _, duplicate := byBehavior[key]; duplicate {
			return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticOverlapping, "Rust request repeats an assignment grounding")}
		}
		byBehavior[key] = grounding
	}
	groundings := make([]semanticir.AssignmentGrounding, 0, len(cases))
	seen := make(map[string]bool, len(cases))
	for _, item := range cases {
		key := item.Function.Name + "\x00" + assignmentKey(item.Conditions)
		grounding, exists := byBehavior[key]
		operation, operationExists := requestOperation(request.Operations, item.Function.Name)
		exact, singleton := semanticir.ExactGroundingInputs(operation, request.FiniteDomains, item.Conditions)
		inputs, inputDiagnostics := rustExecutionInputs(request, item)
		if len(inputDiagnostics) != 0 {
			return nil, inputDiagnostics
		}
		if !exists || !operationExists || !singleton || grounding.ID != semanticir.AssignmentGroundingID(item.Function.Name, item.Conditions) || !reflect.DeepEqual(grounding.Inputs, exact) || !reflect.DeepEqual(inputs, exact) {
			return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, item.Function.Span, semanticir.DiagnosticUnsupported, "reachable Rust assignment lacks one exact outcome-independent full input grounding")}
		}
		seen[key] = true
		groundings = append(groundings, grounding)
	}
	if len(seen) != len(request.Groundings) {
		return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticInvalidReference, "Rust request groundings differ from the complete reachable operation assignment set")}
	}
	return groundings, nil
}

func rustExecutionInputs(request semanticir.FrontendRequest, item rustExecutionCase) (map[string]semanticir.Literal, []semanticir.Diagnostic) {
	inputs := make(map[string]semanticir.Literal, len(item.Function.Parameters))
	for _, parameter := range item.Function.Parameters {
		domainID := findDomainID(request, item.Function.Name, parameter.Name)
		domain, domainExists := findDomain(request.FiniteDomains, domainID)
		valueID, assigned := item.Conditions[domainID]
		member, memberExists := domainValueByID(domain, valueID)
		valueType, typeOK := rustValueType(parameter.Type)
		literal, exact := rustLiteralForDomainValue(domain, member, item.Function.Name, parameter.Name, valueType)
		if !domainExists || !assigned || !memberExists || !typeOK || !exact {
			return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, parameter.Span, semanticir.DiagnosticInvalidReference, "Rust execution input is not exactly grounded")}
		}
		inputs[parameter.Name] = literal
	}
	return inputs, nil
}

func buildRustExhaustiveEvidence(request semanticir.FrontendRequest, compiler rustCompilerOutput, seed rustExhaustiveSeed) ([]semanticir.ExhaustiveExecutionEvidence, []semanticir.Diagnostic) {
	whole := wholeSpan(request.Source)
	if compiler.MIR == "" || len(seed.Harness) == 0 || !semanticir.ValidDigest(seed.ExecutableDigest) || len(seed.Runs) < 2 {
		return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticIncomplete, "Rust exhaustive execution lacks MIR, harness, executable, or independent runs")}
	}
	completeDigest := seed.Runs[0].ObservationDigest
	if !semanticir.ValidDigest(completeDigest) {
		return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticIncomplete, "Rust exhaustive assignment digest is malformed")}
	}
	for _, run := range seed.Runs[1:] {
		if run.ObservationDigest != completeDigest {
			return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticUnsupported, "independent Rust execution runs disagree on the complete observation relation")}
		}
	}
	evidence := semanticir.ExhaustiveExecutionEvidence{
		ID: "rustc-exhaustive-" + request.Artifact.ID, Tool: request.Translator,
		SourceDigest: request.Artifact.Digest, WorkspaceTreeDigest: request.Workspace.TreeDigest,
		IRKind: semanticir.CompilerIRRustMIR, EmittedIRDigest: semanticir.DigestBytes([]byte(compiler.MIR)),
		Harness: append([]byte(nil), seed.Harness...), HarnessPath: seed.HarnessPath, HarnessDigest: semanticir.DigestBytes(seed.Harness), ExecutableDigest: seed.ExecutableDigest,
		Steps: append([]semanticir.ProbeStep(nil), seed.Steps...),
		Argv:  append([]string(nil), seed.Argv...), WorkingDirectory: seed.WorkingDirectory,
		Environment: append([]semanticir.EnvironmentVariable(nil), request.Workspace.Environment...), EnvironmentDigest: request.Workspace.EnvironmentDigest,
		ClearEnvironment: request.Workspace.ClearEnvironment, KillProcessGroup: request.Workspace.KillProcessGroup, TimeoutMillis: 30000,
		Groundings: append([]semanticir.AssignmentGrounding(nil), seed.Groundings...), CompleteAssignmentDigest: completeDigest,
		Runs: append([]semanticir.ExecutionRunEvidence(nil), seed.Runs...), Complete: true,
		Provenance: provenance(request.Artifact, whole, semanticir.TranslationTranslated),
	}
	return []semanticir.ExhaustiveExecutionEvidence{evidence}, nil
}

func parseRustHarnessOutput(request semanticir.FrontendRequest, stdout []byte, cases []rustExecutionCase) (map[int]evaluatedOutcome, []semanticir.Diagnostic) {
	byIndex := make(map[int]rustExecutionCase, len(cases))
	for _, item := range cases {
		byIndex[item.Index] = item
	}
	result := make(map[int]evaluatedOutcome, len(cases))
	for _, line := range bytes.Split(stdout, []byte{'\n'}) {
		if !bytes.HasPrefix(line, []byte("__RAY__\t")) {
			continue
		}
		parts := strings.SplitN(string(line), "\t", 4)
		if len(parts) != 4 {
			return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticUnsupported, "malformed pinned Rust harness record")}
		}
		index, err := strconv.Atoi(parts[1])
		item, exists := byIndex[index]
		if err != nil || !exists {
			return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticUnsupported, "unknown pinned Rust harness record")}
		}
		if _, duplicate := result[index]; duplicate {
			return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticUnsupported, "duplicate pinned Rust harness record")}
		}
		observed, ok := parseRustObserved(item.Function, parts[2], parts[3])
		if !ok {
			return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, item.Function.Span, semanticir.DiagnosticUnsupported, "pinned Rust harness emitted a value outside the typed Semantic IR subset")}
		}
		observed.Span = item.Function.Span
		result[index] = observed
	}
	return result, nil
}

func parseRustObserved(fn functionDecl, kind, encoded string) (evaluatedOutcome, bool) {
	if kind == "P" {
		message, err := strconv.Unquote(encoded)
		return evaluatedOutcome{Kind: semanticir.OutcomeRaise, ExceptionType: "panic", Message: message}, err == nil
	}
	valueType, ok := rustValueType(fn.ReturnType)
	outcomeKind := semanticir.OutcomeReturn
	exception := ""
	if successType, errorType, result := rustResultTypes(fn.ReturnType); result {
		if kind == "S" {
			valueType, outcomeKind = successType, semanticir.OutcomeSuccess
		} else if kind == "E" {
			valueType, outcomeKind, exception = errorType, semanticir.OutcomeRaise, "Result::Err"
		} else {
			return evaluatedOutcome{}, false
		}
	} else if !ok || kind != "R" {
		return evaluatedOutcome{}, false
	}
	literal, ok := parseHarnessLiteral(valueType, encoded)
	if !ok {
		return evaluatedOutcome{}, false
	}
	return evaluatedOutcome{Kind: outcomeKind, Literal: &literal, ExceptionType: exception}, true
}

func parseHarnessLiteral(valueType semanticir.ValueType, encoded string) (semanticir.Literal, bool) {
	switch valueType {
	case semanticir.TypeBool:
		value, err := strconv.ParseBool(encoded)
		return semanticir.Literal{Type: valueType, Bool: value}, err == nil
	case semanticir.TypeInteger:
		value, err := strconv.ParseInt(encoded, 10, 64)
		return semanticir.Literal{Type: valueType, Integer: value}, err == nil
	case semanticir.TypeString:
		value, err := strconv.Unquote(encoded)
		return semanticir.Literal{Type: valueType, String: value}, err == nil
	case semanticir.TypeUnit:
		return semanticir.Literal{Type: valueType}, encoded == ""
	default:
		return semanticir.Literal{}, false
	}
}

func renderRustDomainArgument(domain semanticir.Domain, value semanticir.DomainValue, operationID, inputName, rustType string) (string, bool) {
	want, typeOK := rustValueType(rustType)
	literal, ok := rustLiteralForDomainValue(domain, value, operationID, inputName, want)
	if !typeOK || !ok || literal.Type != want {
		return "", false
	}
	rendered, ok := renderRustLiteral(literal)
	if !ok {
		return "", false
	}
	if strings.ReplaceAll(rustType, " ", "") == "String" && literal.Type == semanticir.TypeString {
		return "String::from(" + string(rendered) + ")", true
	}
	return string(rendered), true
}

func rustLiteralForDomainValue(domain semanticir.Domain, value semanticir.DomainValue, operationID, inputName string, want semanticir.ValueType) (semanticir.Literal, bool) {
	if len(value.Groundings) != 0 {
		var grounded *semanticir.Literal
		for _, axiom := range value.Groundings {
			if axiom.OperationID != operationID {
				continue
			}
			if grounded != nil || axiom.Kind != semanticir.GroundingMembership || axiom.Membership == nil || len(axiom.Exact) != 0 {
				return semanticir.Literal{}, false
			}
			witness, witnessExists := axiom.ConcreteWitness[inputName]
			literal, exact := rustExactMembershipLiteral(*axiom.Membership, inputName)
			if !witnessExists || !exact || !reflect.DeepEqual(literal, witness) || literal.Type != want {
				return semanticir.Literal{}, false
			}
			copy := literal
			grounded = &copy
		}
		if grounded == nil {
			return semanticir.Literal{}, false
		}
		return *grounded, true
	}
	literal, ok := value.TypedValue(domain)
	return literal, ok && literal.Type == want
}

// rustExactMembershipLiteral recognizes only a conjunction containing an
// equality between the requested input and one typed literal. That equality
// alone fixes the concrete input for the entire semantic category. Ordered,
// disjunctive, negated, or computed predicates are intentionally not treated
// as singleton categories merely because they carry a concrete witness.
func rustExactMembershipLiteral(expression semanticir.Expression, inputName string) (semanticir.Literal, bool) {
	if expression.Kind == semanticir.ExprBool && expression.Operator == semanticir.OpAnd && len(expression.Operands) == 2 {
		left, leftExact := rustExactMembershipLiteral(expression.Operands[0], inputName)
		right, rightExact := rustExactMembershipLiteral(expression.Operands[1], inputName)
		switch {
		case leftExact && rightExact:
			return left, reflect.DeepEqual(left, right)
		case leftExact:
			return left, true
		case rightExact:
			return right, true
		default:
			return semanticir.Literal{}, false
		}
	}
	if expression.Kind != semanticir.ExprCompare || expression.Operator != semanticir.OpEQ || len(expression.Operands) != 2 {
		return semanticir.Literal{}, false
	}
	for index := 0; index < 2; index++ {
		variable, literal := expression.Operands[index], expression.Operands[1-index]
		if variable.Kind == semanticir.ExprVariable && variable.Name == inputName && literal.Kind == semanticir.ExprLiteral && literal.Literal != nil && len(literal.Operands) == 0 && variable.Type == literal.Literal.Type {
			return *literal.Literal, semanticir.ValidateLiteral(*literal.Literal) == nil
		}
	}
	return semanticir.Literal{}, false
}

func rustResultTypes(typeName string) (semanticir.ValueType, semanticir.ValueType, bool) {
	compact := strings.ReplaceAll(typeName, " ", "")
	if !strings.HasPrefix(compact, "Result<") || !strings.HasSuffix(compact, ">") {
		return semanticir.TypeUnknown, semanticir.TypeUnknown, false
	}
	parts := splitTopLevelString(compact[len("Result<"):len(compact)-1], ',')
	if len(parts) != 2 {
		return semanticir.TypeUnknown, semanticir.TypeUnknown, false
	}
	success, successOK := rustValueType(parts[0])
	failure, failureOK := rustValueType(parts[1])
	return success, failure, successOK && failureOK
}

func assignmentExcluded(constraints []semanticir.Constraint, operationID string, assignment semanticir.Assignment) bool {
	for _, constraint := range constraints {
		if constraint.OperationID == operationID && assignmentsEqual(constraint.Conditions, assignment) {
			return true
		}
	}
	return false
}

func semanticOutcome(artifact semanticir.ArtifactRef, operationID string, value evaluatedOutcome) semanticir.ObservableOutcome {
	outcome, err := semanticir.ObservableOutcomeFromTrace(operationID, rawOutcomeTrace(value), provenance(artifact, value.Span, semanticir.TranslationTranslated))
	if err != nil {
		return semanticir.ObservableOutcome{}
	}
	return outcome
}

func rawOutcomeTrace(value evaluatedOutcome) semanticir.RawOutcomeTrace {
	literal := value.Literal
	if value.Kind == semanticir.OutcomeSuccess || value.Kind == semanticir.OutcomeRaise {
		literal = nil
	}
	return semanticir.RawOutcomeTrace{Kind: value.Kind, Value: literal, ExceptionType: value.ExceptionType, Message: value.Message}
}

func outcomeID(operationID string, value evaluatedOutcome) string {
	outcome, err := semanticir.ObservableOutcomeFromTrace(operationID, rawOutcomeTrace(value), semanticir.Provenance{})
	if err != nil {
		return ""
	}
	return outcome.ID
}

func incrementIndices(indices []int, domains []semanticir.Domain) {
	for index := len(indices) - 1; index >= 0; index-- {
		indices[index]++
		if indices[index] < len(domains[index].Values) {
			return
		}
		indices[index] = 0
	}
}
