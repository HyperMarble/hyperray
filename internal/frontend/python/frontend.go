// Package python translates a fail-closed, finite subset of Python into Ray's
// shared Semantic IR. Parsing is delegated to Python's standard ast module;
// this package never infers Python structure from source text.
package python

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/semanticir"
)

const (
	// FrontendVersion is recorded by callers in certificates/tool metadata.
	FrontendVersion = "python-bound-cpython-v2"
	// Version is a concise alias shared with the other language frontends.
	Version         = FrontendVersion
	defaultMaxCases = 10000
)

//go:embed ast_bridge.py
var astBridge string

//go:embed concrete_runner.py
var concreteRunner string

//go:embed probe_harness.py
var probeHarness string

type bridgeLocation struct {
	Line      int `json:"line"`
	Column    int `json:"column"`
	EndLine   int `json:"end_line"`
	EndColumn int `json:"end_column"`
}

type bridgeUnsupported struct {
	Code     string         `json:"code"`
	Message  string         `json:"message"`
	Location bridgeLocation `json:"location"`
}

type bridgeResult struct {
	Functions          []pyFunction                `json:"functions"`
	ModuleDeclarations []pyDeclaration             `json:"module_declarations"`
	CallEdges          []pyCallEdge                `json:"call_edges"`
	Imports            []pyImport                  `json:"imports"`
	Unsupported        []bridgeUnsupported         `json:"unsupported"`
	CompilerIR         map[string][]pyCompilerNode `json:"compiler_ir"`
	CompilerIRDigest   string                      `json:"compiler_ir_digest"`
	ParseError         string                      `json:"parse_error"`
	Location           bridgeLocation              `json:"location"`
	RawInput           []byte                      `json:"-"`
	RawOutput          []byte                      `json:"-"`
}

type pyCompilerNode struct {
	ID       string `json:"id"`
	Offset   int    `json:"offset"`
	Opcode   string `json:"opcode"`
	Argument string `json:"argument"`
	Line     int    `json:"line"`
	EndLine  int    `json:"end_line"`
}

type pyDeclaration struct {
	Name     string         `json:"name"`
	Location bridgeLocation `json:"location"`
}

type pyCallEdge struct {
	Caller   string         `json:"caller"`
	Callee   string         `json:"callee"`
	Location bridgeLocation `json:"location"`
}

type pyImport struct {
	Module   string         `json:"module"`
	Names    []string       `json:"names"`
	Location bridgeLocation `json:"location"`
}

type pyFunction struct {
	Name         string         `json:"name"`
	Parameters   []string       `json:"parameters"`
	IsTest       bool           `json:"is_test"`
	IsEntryPoint bool           `json:"is_entry_point"`
	Location     bridgeLocation `json:"location"`
	Body         []pyStatement  `json:"body"`
}

type pyStatement struct {
	Kind          string         `json:"kind"`
	Source        string         `json:"source"`
	Target        string         `json:"target"`
	ExceptionType string         `json:"exception_type"`
	Message       string         `json:"message"`
	Location      bridgeLocation `json:"location"`
	Value         *pyExpression  `json:"value"`
	Body          []pyStatement  `json:"body"`
	Alternate     []pyStatement  `json:"alternate"`
	Catches       []pyCatch      `json:"catches"`
}

type pyCatch struct {
	ExceptionType string         `json:"exception_type"`
	Location      bridgeLocation `json:"location"`
	Body          []pyStatement  `json:"body"`
}

type pyExpression struct {
	Kind      string          `json:"kind"`
	Name      string          `json:"name"`
	Operator  string          `json:"operator"`
	Operators []string        `json:"operators"`
	Value     json.RawMessage `json:"value"`
	Args      []pyExpression  `json:"args"`
	Source    string          `json:"source"`
	Location  bridgeLocation  `json:"location"`
}

type bridgeRequest struct {
	Source          string            `json:"source"`
	Path            string            `json:"path"`
	ArtifactKind    string            `json:"artifact_kind"`
	EntryPoints     []string          `json:"entry_points"`
	TestEntryPoints []string          `json:"test_entry_points"`
	ResolvedModules map[string]string `json:"resolved_modules"`
	ModuleName      string            `json:"module_name"`
	Execution       string            `json:"execution"`
}

type concreteCase struct {
	ID           string               `json:"id"`
	Operation    string               `json:"operation"`
	Arguments    []semanticir.Literal `json:"arguments"`
	Constructors []string             `json:"constructors"`
}

type concreteRequest struct {
	Root         string         `json:"root"`
	PackageRoot  string         `json:"package_root"`
	Module       string         `json:"module"`
	SourcePath   string         `json:"source_path"`
	Operations   []string       `json:"operations"`
	Declarations []string       `json:"declarations"`
	Cases        []concreteCase `json:"cases"`
	Reverse      bool           `json:"reverse"`
	SignalPath   string         `json:"signal_path"`
}

type concreteResult struct {
	ID            string              `json:"id"`
	Line          int                 `json:"line"`
	Kind          string              `json:"kind"`
	Value         *semanticir.Literal `json:"value"`
	ExceptionType string              `json:"exception_type"`
	Message       string              `json:"message"`
	CompilerNodes []string            `json:"compiler_node_ids"`
	ProcessOutput []byte              `json:"-"`
	SignalValue   []byte              `json:"-"`
	SignalPath    string              `json:"-"`
}

type concreteResponse struct {
	Results         []concreteResult            `json:"results"`
	BytecodeDigest  string                      `json:"bytecode_digest"`
	CompilerNodes   map[string][]string         `json:"compiler_node_ids"`
	CompilerOpcodes map[string][]string         `json:"compiler_opcodes"`
	CompilerIR      map[string][]pyCompilerNode `json:"compiler_ir"`
	StartedAtUTC    string                      `json:"-"`
	ProcessCount    int                         `json:"-"`
	ProcessOutput   []byte                      `json:"-"`
	SignalValue     []byte                      `json:"-"`
	SignalPath      string                      `json:"-"`
}

type concreteBinding struct {
	operation  semanticir.Operation
	assignment semanticir.Assignment
	inputs     map[string]semanticir.Literal
	function   *pyFunction
}

type testAssertionInstance struct {
	statement *pyStatement
	assertion semanticir.Assertion
	predicate semanticir.TestPredicate
	controls  []*pyStatement
}

// Translate lowers a frozen Python artifact into typed Semantic IR. Any
// untranslated construct, invalid digest, missing input domain, unavailable
// parser, or non-enumerable behavior makes Coverage blocked and emits an error.
func Translate(ctx context.Context, request semanticir.FrontendRequest) (semanticir.ArtifactModel, []semanticir.Diagnostic) {
	model := baseModel(request)
	result, diagnostics := parse(ctx, request)
	if semanticir.HasErrors(diagnostics) {
		unsupported := []semanticir.UnsupportedConstruct{{
			Kind: "translation", Reason: diagnostics[0].Message,
			Provenance: wholeProvenance(request, semanticir.TranslationUnsupported),
		}}
		model.Coverage = blockedCoverage(request, unsupported)
		return model, diagnostics
	}

	lower := lowering{
		request:       request,
		model:         &model,
		variables:     make(map[string]semanticir.ValueType),
		bindings:      make(map[string]semanticir.Expression),
		testInstances: make(map[string][]testAssertionInstance),
	}
	for _, unsupported := range result.Unsupported {
		lower.unsupported(unsupported.Code, unsupported.Message, unsupported.Location)
	}
	for _, imported := range result.Imports {
		if !resolvedWorkspaceImport(request, imported.Module) {
			lower.unsupported("PY_UNRESOLVED_IMPORT", "import "+imported.Module+" is not a digest-bound Python entry in the frozen workspace", imported.Location)
		}
	}

	for i := range result.Functions {
		function := &result.Functions[i]
		if request.Kind == semanticir.ArtifactCode && !function.IsEntryPoint {
			continue
		}
		operation := lower.operation(function)
		model.Operations = append(model.Operations, operation)
	}

	operationByID := make(map[string]semanticir.Operation, len(model.Operations))
	for _, operation := range model.Operations {
		operationByID[operation.ID] = operation
	}
	for i := range result.Functions {
		function := &result.Functions[i]
		if function.IsTest || request.Kind == semanticir.ArtifactTests {
			model.Tests = append(model.Tests, lower.tests(function, operationByID)...)
		}
	}
	if request.Kind == semanticir.ArtifactTests && !semanticir.HasErrors(lower.diagnostics) {
		projection, runner, err := lower.testProjectionEvidence(result)
		if err != nil {
			lower.unsupported("PY_TEST_OBSERVATION_EVIDENCE", err.Error(), wholeBridgeLocation(request))
		} else {
			model.TestProjection = projection
			model.RunnerSelection = runner
		}
	}

	if !semanticir.HasErrors(lower.diagnostics) && request.Kind == semanticir.ArtifactCode {
		if concreteExecution(request) {
			lower.enumerateConcrete(ctx, result.Functions, result.ModuleDeclarations, result.CallEdges, operationByID)
		} else {
			// The AST model above is structural evidence for source locations,
			// tests, and exact edit anchors. It is not Python execution
			// semantics. In particular, digest-binding an AST evaluator to
			// CPython bytecode cannot prove that the evaluator implements the
			// frozen interpreter. Code outcomes therefore require the exact
			// fresh-process CPython mode here; silently evaluating the AST would
			// be an unsound approximation.
			lower.unsupported("PY_EXECUTION_REQUIRED", "code behavior requires python.execution=bound-cpython with exact finite groundings", wholeBridgeLocation(request))
		}
	}
	diagnostics = append(diagnostics, lower.diagnostics...)
	status := semanticir.TranslationComplete
	if semanticir.HasErrors(diagnostics) || len(model.Coverage.Unsupported) != 0 {
		status = semanticir.TranslationBlocked
	}
	model.Coverage.Status = status
	model.Coverage.TotalConstructs = lower.total + len(result.Unsupported)
	model.Coverage.TranslatedConstructs = lower.translated
	if request.Kind == semanticir.ArtifactTests && model.TestProjection != nil && status == semanticir.TranslationComplete {
		model.Coverage.TotalConstructs = len(model.TestProjection.Constructs)
		model.Coverage.TranslatedConstructs = len(model.TestProjection.Constructs)
	}
	model.Coverage.Provenance = wholeProvenance(request, status)
	if status == semanticir.TranslationBlocked && len(model.Coverage.Unsupported) == 0 {
		model.Coverage.Unsupported = []semanticir.UnsupportedConstruct{{
			Kind: "translation", Reason: "artifact translation is blocked by diagnostics",
			Provenance: wholeProvenance(request, semanticir.TranslationUnsupported),
		}}
	}
	return model, diagnostics
}

// Materialize converts every Python-owned component of a complete semantic
// counterexample vector into non-overlapping, digest-anchored replacements.
// It renders only exact typed terminals at exact grounded input points; any
// missing point, unrenderable outcome, partial vector, or stale model blocks
// the entire plan.
func Materialize(ctx context.Context, materialization semanticir.MaterializationRequest) (semanticir.EditPlan, []semanticir.Diagnostic) {
	request := materialization.Frontend
	witness := materialization.Counterexample
	plan := semanticir.EditPlan{
		ID:        "python-edit:" + witness.ID,
		WitnessID: witness.ID,
		Artifact:  request.Artifact,
		Expected: semanticir.ExpectedSemantics{
			Conditions: witness.Conditions, OperationID: witness.OperationID,
			OutcomeIDs: append([]string(nil), witness.ObservedOutcomes...),
			Choices:    append([]semanticir.BehaviorChoice(nil), witness.Choices...), TestPasses: witness.TestPasses,
		},
	}
	freshModel, diagnostics := Translate(ctx, request)
	if semanticir.HasErrors(diagnostics) {
		return plan, diagnostics
	}
	if materialization.Task == nil {
		return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticInvalidInput,
			"materialization requires the compiled task IR", wholeLocation(request), semanticir.TranslationUnsupported))
	}
	if materialization.Model.Artifact != request.Artifact || materialization.Model.Language != semanticir.LanguagePython ||
		materialization.Model.Kind != request.Kind || materialization.Model.Translator != request.Translator ||
		materialization.Model.Coverage.Status != semanticir.TranslationComplete {
		return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticStaleArtifact,
			"materialization model is absent, blocked, or does not match the frozen artifact", wholeLocation(request), semanticir.TranslationUnsupported))
	}
	freshDigest, freshErr := stableTranslationDigest(freshModel)
	providedDigest, providedErr := stableTranslationDigest(materialization.Model)
	if freshErr != nil || providedErr != nil || freshDigest != providedDigest {
		return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticStaleArtifact,
			"materialization model does not match a fresh translation of the frozen source", wholeLocation(request), semanticir.TranslationUnsupported))
	}
	if witness.ID == "" || len(witness.Choices) == 0 {
		return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticInvalidInput,
			"counterexample requires a non-empty ID and complete behavior choice vector", wholeLocation(request), semanticir.TranslationUnsupported))
	}

	parsed, parseDiagnostics := parse(ctx, request)
	diagnostics = append(diagnostics, parseDiagnostics...)
	if semanticir.HasErrors(diagnostics) {
		return plan, diagnostics
	}
	functions := indexFunctions(parsed.Functions)
	modelOperations := make(map[string]semanticir.Operation, len(materialization.Model.Operations))
	for _, operation := range materialization.Model.Operations {
		if operation.Kind != semanticir.OperationTest {
			modelOperations[operation.ID] = operation
		}
	}
	taskOutcomes := make(map[string]semanticir.ObservableOutcome, len(materialization.Task.Outcomes))
	for _, outcome := range materialization.Task.Outcomes {
		taskOutcomes[outcome.ID] = outcome
	}
	owned := 0
	desiredIDs := make([]string, 0)
	choicesByOperation := make(map[string][]semanticir.BehaviorChoice)
	changesByOperation := make(map[string]bool)
	seenBehavior := make(map[string]struct{})
	for _, choice := range witness.Choices {
		operation, belongs := modelOperations[choice.Behavior.OperationID]
		if !belongs {
			continue
		}
		owned++
		keyDigest, _ := semanticir.Digest(choice.Behavior)
		if _, duplicate := seenBehavior[keyDigest]; duplicate {
			return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticInvalidInput,
				"counterexample repeats one owned behavior component", wholeLocation(request), semanticir.TranslationUnsupported))
		}
		seenBehavior[keyDigest] = struct{}{}
		modelCase, ok := matchingCase(materialization.Model.Cases, choice.Behavior)
		if !ok {
			return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticInvalidReference,
				"owned counterexample choice has no exact translated code case", wholeLocation(request), semanticir.TranslationUnsupported))
		}
		taskCase, ok := matchingCase(materialization.Task.CodeCases, choice.Behavior)
		if !ok || taskCase.Provenance != modelCase.Provenance || !sameStrings(taskCase.OutcomeIDs, modelCase.OutcomeIDs) {
			return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticInvalidReference,
				"owned counterexample choice is absent from or stale against compiled task cases", wholeLocation(request), semanticir.TranslationUnsupported))
		}
		desired, ok := taskOutcomes[choice.OutcomeID]
		if !ok {
			return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticInvalidReference,
				"owned choice selects an outcome outside the task's authoritative vocabulary", wholeLocation(request), semanticir.TranslationUnsupported))
		}
		taskOp, ok := taskOperation(materialization.Task.Operations, choice.Behavior.OperationID)
		if !ok || !containsString(taskOp.OutcomeIDs, choice.OutcomeID) {
			return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticInvalidReference,
				"compiled task operation does not admit the selected outcome", wholeLocation(request), semanticir.TranslationUnsupported))
		}
		if renderableOutcome(desired) == false {
			return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticUnsupported,
				"owned choice outcome cannot be rendered as an exact Python terminal statement", modelCase.Provenance.Location, semanticir.TranslationUnsupported))
		}
		if len(choice.Behavior.Conditions) != len(operation.DomainIDs) {
			return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticInvalidInput,
				"owned choice does not bind exactly the operation-local domains", modelCase.Provenance.Location, semanticir.TranslationUnsupported))
		}
		exactInputs, exact := exactPythonAssignmentInputs(request, operation, choice.Behavior.Conditions)
		if !exact || !reflect.DeepEqual(choice.Behavior.Inputs, exactInputs) {
			return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticInvalidInput,
				"owned choice input point differs from the frozen exact assignment grounding", modelCase.Provenance.Location, semanticir.TranslationUnsupported))
		}
		choicesByOperation[choice.Behavior.OperationID] = append(choicesByOperation[choice.Behavior.OperationID], choice)
		desiredIDs = append(desiredIDs, choice.OutcomeID)
		if !containsString(modelCase.OutcomeIDs, choice.OutcomeID) {
			changesByOperation[choice.Behavior.OperationID] = true
		}
	}
	if owned == 0 {
		return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticInvalidReference,
			"counterexample has no choices owned by this Python artifact", wholeLocation(request), semanticir.TranslationUnsupported))
	}

	operationIDs := make([]string, 0, len(choicesByOperation))
	for operationID := range choicesByOperation {
		operationIDs = append(operationIDs, operationID)
	}
	sort.Strings(operationIDs)
	for _, operationID := range operationIDs {
		operation := modelOperations[operationID]
		modelCases := casesForOperation(materialization.Model.Cases, operationID)
		choices := choicesByOperation[operationID]
		if len(choices) != len(modelCases) {
			return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticIncomplete,
				"counterexample does not provide the complete reachable behavior vector for owned operation "+operationID, wholeLocation(request), semanticir.TranslationUnsupported))
		}
		if !changesByOperation[operationID] {
			continue // Reference-equal complete operation needs no source edit.
		}
		target := functions[operationID]
		if target == nil || target.IsTest || len(target.Body) == 0 {
			return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticInvalidReference,
				"owned choice operation is not a translated Python solution function", wholeLocation(request), semanticir.TranslationUnsupported))
		}
		start, _, ok := byteRange(request.Source, target.Body[0].Location)
		_, end, endOK := byteRange(request.Source, target.Body[len(target.Body)-1].Location)
		ok = ok && endOK
		if !ok {
			return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticInvalidProvenance,
				"function body source span is outside the frozen artifact", toLocation(request, target.Location), semanticir.TranslationUnsupported))
		}
		replacement, ok := renderOperationDispatch(request, operation, target, choices, taskOutcomes)
		if !ok {
			return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticUnsupported,
				"complete behavior vector cannot be rendered as exact typed Python dispatch", toLocation(request, target.Location), semanticir.TranslationUnsupported))
		}
		expected := append([]byte(nil), request.Source[start:end]...)
		if bytes.Equal(expected, replacement) {
			return plan, append(diagnostics, diagnostic(request, semanticir.DiagnosticInvalidReference,
				"changed behavior vector renders as a no-op", toLocation(request, target.Location), semanticir.TranslationUnsupported))
		}
		plan.Edits = append(plan.Edits, semanticir.ByteRangeReplacement{
			StartByte: start, EndByte: end, ExpectedBytes: expected, Replacement: replacement,
		})
	}
	sort.Slice(plan.Edits, func(i, j int) bool { return plan.Edits[i].StartByte < plan.Edits[j].StartByte })
	for i := 1; i < len(plan.Edits); i++ {
		if plan.Edits[i].StartByte < plan.Edits[i-1].EndByte {
			return semanticir.EditPlan{ID: plan.ID, WitnessID: plan.WitnessID, Artifact: plan.Artifact, Expected: plan.Expected},
				append(diagnostics, diagnostic(request, semanticir.DiagnosticUnsupported,
					"behavior vector requires overlapping Python source edits", wholeLocation(request), semanticir.TranslationUnsupported))
		}
	}
	if len(plan.Edits) != 0 {
		candidate := append([]byte(nil), request.Source...)
		for index := len(plan.Edits) - 1; index >= 0; index-- {
			edit := plan.Edits[index]
			candidate = append(candidate[:edit.StartByte], append(append([]byte(nil), edit.Replacement...), candidate[edit.EndByte:]...)...)
		}
		if err := compilePythonCandidate(ctx, request, candidate); err != nil {
			return semanticir.EditPlan{ID: plan.ID, WitnessID: plan.WitnessID, Artifact: plan.Artifact, Expected: plan.Expected},
				append(diagnostics, diagnostic(request, semanticir.DiagnosticUnsupported,
					"generated full-vector Python candidate does not compile with the bound interpreter: "+err.Error(), wholeLocation(request), semanticir.TranslationUnsupported))
		}
		candidateRequest, cleanup, err := materializedCandidateRequest(request, candidate)
		if err != nil {
			return semanticir.EditPlan{ID: plan.ID, WitnessID: plan.WitnessID, Artifact: plan.Artifact, Expected: plan.Expected},
				append(diagnostics, diagnostic(request, semanticir.DiagnosticUnsupported,
					"cannot construct an exact disposable candidate workspace: "+err.Error(), wholeLocation(request), semanticir.TranslationUnsupported))
		}
		defer cleanup()
		candidateModel, candidateDiagnostics := Translate(ctx, candidateRequest)
		if semanticir.HasErrors(candidateDiagnostics) || candidateModel.Coverage.Status != semanticir.TranslationComplete {
			reasons := make([]string, 0, len(candidateDiagnostics))
			for _, item := range candidateDiagnostics {
				if item.Severity == semanticir.SeverityError {
					reasons = append(reasons, item.Message)
				}
			}
			detail := "generated full-vector Python candidate does not freshly translate completely"
			if len(reasons) != 0 {
				detail += ": " + strings.Join(reasons, "; ")
			}
			return semanticir.EditPlan{ID: plan.ID, WitnessID: plan.WitnessID, Artifact: plan.Artifact, Expected: plan.Expected},
				append(diagnostics, diagnostic(request, semanticir.DiagnosticUnsupported,
					detail, wholeLocation(request), semanticir.TranslationUnsupported))
		}
		if concreteExecution(candidateRequest) {
			// The central executor, not a language frontend, attaches Replay.
			// Candidate translation is checked here against its frozen request
			// and complete fresh behavior table; pipeline confirmation replays
			// the final artifact independently before proof evidence is accepted.
			if validation := semanticir.ValidateArtifactScope(candidateRequest, candidateModel); semanticir.HasErrors(validation) {
				return semanticir.EditPlan{ID: plan.ID, WitnessID: plan.WitnessID, Artifact: plan.Artifact, Expected: plan.Expected},
					append(diagnostics, diagnostic(request, semanticir.DiagnosticUnsupported,
						"generated full-vector Python candidate differs from the declared semantic scope", wholeLocation(request), semanticir.TranslationUnsupported))
			}
		}
		for operationID, choices := range choicesByOperation {
			candidateCases := casesForOperation(candidateModel.Cases, operationID)
			if len(candidateCases) != len(choices) {
				return semanticir.EditPlan{ID: plan.ID, WitnessID: plan.WitnessID, Artifact: plan.Artifact, Expected: plan.Expected},
					append(diagnostics, diagnostic(request, semanticir.DiagnosticUnsupported,
						"fresh candidate changes the reachable category set for "+operationID, wholeLocation(request), semanticir.TranslationUnsupported))
			}
			for _, choice := range choices {
				observed, ok := matchingCase(candidateCases, choice.Behavior)
				if !ok || len(observed.OutcomeIDs) != 1 || observed.OutcomeIDs[0] != choice.OutcomeID {
					return semanticir.EditPlan{ID: plan.ID, WitnessID: plan.WitnessID, Artifact: plan.Artifact, Expected: plan.Expected},
						append(diagnostics, diagnostic(request, semanticir.DiagnosticUnsupported,
							"fresh candidate does not realize the complete requested behavior vector", wholeLocation(request), semanticir.TranslationUnsupported))
				}
			}
		}
	}
	sort.Strings(desiredIDs)
	plan.Expected.OutcomeIDs = compactStrings(desiredIDs)
	plan.Provenance = wholeProvenance(request, semanticir.TranslationTranslated)
	return plan, diagnostics
}

func materializedCandidateRequest(request semanticir.FrontendRequest, candidate []byte) (semanticir.FrontendRequest, func(), error) {
	packageRoot, err := normalizedPackageRoot(request)
	if err != nil {
		return semanticir.FrontendRequest{}, func() {}, err
	}
	temporaryRoot, err := os.MkdirTemp("", "ray-python-candidate-")
	if err != nil {
		return semanticir.FrontendRequest{}, func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(temporaryRoot) }
	temporaryRoot, err = filepath.EvalSymlinks(temporaryRoot)
	if err != nil {
		cleanup()
		return semanticir.FrontendRequest{}, func() {}, err
	}
	candidateRequest := request
	candidateRequest.Options = make(map[string]string, len(request.Options))
	for key, value := range request.Options {
		candidateRequest.Options[key] = value
	}
	candidateRequest.Options["python.package_root"] = filepath.ToSlash(packageRoot)
	candidateRequest.Source = append([]byte(nil), candidate...)
	candidateRequest.Artifact.Digest = semanticir.DigestBytes(candidate)
	candidateRequest.Workspace.Root = temporaryRoot
	candidateRequest.Workspace.Entries = append([]semanticir.WorkspaceEntry(nil), request.Workspace.Entries...)
	foundArtifact := false
	for index := range candidateRequest.Workspace.Entries {
		entry := &candidateRequest.Workspace.Entries[index]
		body, readErr := readWorkspaceEntry(request.Workspace.Root, entry.Path)
		if readErr != nil {
			cleanup()
			return semanticir.FrontendRequest{}, func() {}, readErr
		}
		if entry.Artifact.ID == request.Artifact.ID && entry.Artifact.Path == request.Artifact.Path {
			body = candidate
			entry.Artifact = candidateRequest.Artifact
			foundArtifact = true
		}
		target := filepath.Join(temporaryRoot, filepath.Clean(entry.Path))
		relative, relativeErr := filepath.Rel(temporaryRoot, target)
		if relativeErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			cleanup()
			return semanticir.FrontendRequest{}, func() {}, fmt.Errorf("workspace entry %q escapes candidate root", entry.Path)
		}
		if mkdirErr := os.MkdirAll(filepath.Dir(target), 0o700); mkdirErr != nil {
			cleanup()
			return semanticir.FrontendRequest{}, func() {}, mkdirErr
		}
		if writeErr := os.WriteFile(target, body, 0o600); writeErr != nil {
			cleanup()
			return semanticir.FrontendRequest{}, func() {}, writeErr
		}
	}
	if !foundArtifact {
		cleanup()
		return semanticir.FrontendRequest{}, func() {}, fmt.Errorf("focused artifact is absent from frozen workspace entries")
	}
	for index := range candidateRequest.FocusArtifacts {
		if candidateRequest.FocusArtifacts[index].ID == request.Artifact.ID && candidateRequest.FocusArtifacts[index].Path == request.Artifact.Path {
			candidateRequest.FocusArtifacts[index] = candidateRequest.Artifact
		}
	}
	candidateRequest.ChangedRanges = append([]semanticir.ChangedSourceRange(nil), request.ChangedRanges...)
	for index := range candidateRequest.ChangedRanges {
		changed := &candidateRequest.ChangedRanges[index]
		if changed.ArtifactID != request.Artifact.ID || changed.Path != request.Artifact.Path {
			continue
		}
		slice, sliceErr := sourceLineSlice(candidate, changed.StartLine, changed.EndLine)
		if sliceErr != nil {
			cleanup()
			return semanticir.FrontendRequest{}, func() {}, sliceErr
		}
		changed.SliceDigest = semanticir.DigestBytes(slice)
		changed.Provenance = semanticir.NewProvenance(candidateRequest.Artifact, semanticir.SourceLocation{
			Path: changed.Path, StartLine: changed.StartLine, StartColumn: 1, EndLine: changed.EndLine, EndColumn: 1,
		}, semanticir.TranslationTranslated)
	}
	candidateRequest.Workspace.TreeDigest, err = executor.WorkspaceDigest(temporaryRoot)
	if err != nil {
		cleanup()
		return semanticir.FrontendRequest{}, func() {}, err
	}
	return candidateRequest, cleanup, nil
}

func sourceLineSlice(source []byte, startLine, endLine int) ([]byte, error) {
	if startLine <= 0 || endLine < startLine {
		return nil, fmt.Errorf("invalid changed source line range %d-%d", startLine, endLine)
	}
	lines := bytes.SplitAfter(source, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if startLine > len(lines) || endLine > len(lines) {
		return nil, fmt.Errorf("changed source line range %d-%d exceeds candidate length %d", startLine, endLine, len(lines))
	}
	return bytes.Join(lines[startLine-1:endLine], nil), nil
}

type probeHarnessCase struct {
	ID           string                     `json:"id"`
	Operation    string                     `json:"operation"`
	Arguments    []semanticir.Literal       `json:"arguments"`
	Constructors []string                   `json:"constructors"`
	Expected     semanticir.RawOutcomeTrace `json:"expected"`
}

type probeHarnessRequest struct {
	Module                 string             `json:"module"`
	PackageRoot            string             `json:"package_root"`
	SourcePath             string             `json:"source_path"`
	ObservationPath        string             `json:"observation_path"`
	BytecodeDigest         string             `json:"bytecode_digest"`
	CompilerEvidenceDigest string             `json:"compiler_evidence_digest"`
	Cases                  []probeHarnessCase `json:"cases"`
}

// GenerateProbe builds a digest-bound, no-edit executable confirmation for a
// reference-correctness witness. The harness recompiles the selected module,
// checks the translated bytecode digest, executes every Python-owned behavior
// choice, and emits the complete typed witness only if all observations match.
func GenerateProbe(ctx context.Context, materialization semanticir.MaterializationRequest) (executor.ProbePlan, []semanticir.Diagnostic) {
	request := materialization.Frontend
	witness := materialization.Counterexample
	block := func(code semanticir.DiagnosticCode, message string) (executor.ProbePlan, []semanticir.Diagnostic) {
		return executor.ProbePlan{}, []semanticir.Diagnostic{diagnostic(request, code, message, wholeLocation(request), semanticir.TranslationUnsupported)}
	}
	if ctx == nil {
		return block(semanticir.DiagnosticInvalidInput, "probe generation requires a non-nil context")
	}
	if witness.Obligation != semanticir.ObligationReferenceCorrectness || witness.ID == "" || witness.OperationID == "" || len(witness.Choices) == 0 || len(witness.ObservedOutcomes) == 0 {
		return block(semanticir.DiagnosticInvalidInput, "direct Python probe requires a complete reference-correctness witness")
	}
	if materialization.Task == nil || request.Kind != semanticir.ArtifactCode || !concreteExecution(request) {
		return block(semanticir.DiagnosticUnsupported, "direct Python probe requires code translated with python.execution=bound-cpython")
	}
	if request.Workspace.State != semanticir.WorkspaceSolutionNewTests {
		return block(semanticir.DiagnosticInvalidInput, "direct probe requires the frozen solution+new-tests workspace")
	}
	fresh, diagnostics := Translate(ctx, request)
	if semanticir.HasErrors(diagnostics) {
		return executor.ProbePlan{}, diagnostics
	}
	if materialization.Model.Artifact != request.Artifact || materialization.Model.Coverage.Status != semanticir.TranslationComplete {
		return block(semanticir.DiagnosticStaleArtifact, "probe model is absent, blocked, or not bound to the frozen Python artifact")
	}
	freshDigest, freshErr := stableTranslationDigest(fresh)
	modelDigest, modelErr := stableTranslationDigest(materialization.Model)
	if freshErr != nil || modelErr != nil || freshDigest != modelDigest {
		return block(semanticir.DiagnosticStaleArtifact, "probe model does not match a fresh exact translation")
	}
	if len(fresh.ExhaustiveEvidence) != 1 || fresh.ExhaustiveEvidence[0].IRKind != semanticir.CompilerIRCPythonBytecode || fresh.ExhaustiveEvidence[0].Tool != request.Translator {
		return block(semanticir.DiagnosticIncomplete, "probe model lacks one exact bound CPython bytecode evidence record")
	}
	if witness.Provenance.ArtifactID != request.Artifact.ID || witness.Provenance.ArtifactDigest != request.Artifact.Digest {
		return block(semanticir.DiagnosticInvalidProvenance, "reference witness provenance is not anchored to the probed Python artifact")
	}

	operations := make(map[string]semanticir.Operation)
	for _, operation := range fresh.Operations {
		if operation.Kind != semanticir.OperationTest {
			operations[operation.ID] = operation
		}
	}
	if _, ok := operations[witness.OperationID]; !ok {
		return block(semanticir.DiagnosticInvalidReference, "reference witness violating operation is not owned by this Python artifact")
	}
	taskOutcomes := make(map[string]semanticir.ObservableOutcome, len(materialization.Task.Outcomes))
	for _, outcome := range materialization.Task.Outcomes {
		taskOutcomes[outcome.ID] = outcome
	}
	choicesByOperation := make(map[string][]semanticir.BehaviorChoice)
	var harnessCases []probeHarnessCase
	expected := semanticir.ExpectedSemantics{
		Conditions: cloneAssignment(witness.Conditions), OperationID: witness.OperationID,
		OutcomeIDs: append([]string(nil), witness.ObservedOutcomes...), Choices: append([]semanticir.BehaviorChoice(nil), witness.Choices...),
		TestPasses: witness.TestPasses,
	}
	for _, choice := range witness.Choices {
		operation, owned := operations[choice.Behavior.OperationID]
		if !owned {
			continue
		}
		modelCase, ok := matchingCase(fresh.Cases, choice.Behavior)
		if !ok || len(modelCase.OutcomeIDs) != 1 || modelCase.OutcomeIDs[0] != choice.OutcomeID {
			return block(semanticir.DiagnosticInvalidReference, "direct probe choice does not equal the frozen reference behavior")
		}
		outcome, ok := taskOutcomes[choice.OutcomeID]
		if !ok || len(outcome.Effects) != 0 {
			return block(semanticir.DiagnosticUnsupported, "direct Python probe outcome is absent or has unsupported declared effects")
		}
		item := probeHarnessCase{ID: modelCase.ID, Operation: operation.ID}
		exactInputs, exact := exactPythonAssignmentInputs(request, operation, choice.Behavior.Conditions)
		if !exact || !reflect.DeepEqual(choice.Behavior.Inputs, exactInputs) {
			return block(semanticir.DiagnosticUnsupported, "probe behavior has no exact outcome-independent Python input grounding")
		}
		for _, input := range operation.Inputs {
			literal, ok := exactInputs[input.Name]
			if !ok || literal.Type != input.Type {
				return block(semanticir.DiagnosticInvalidInput, "probe choice does not bind every typed operation input")
			}
			item.Arguments = append(item.Arguments, literal)
			item.Constructors = append(item.Constructors, "")
		}
		raw, ok := exactRawTraceForBehavior(fresh.ExhaustiveEvidence[0], choice.Behavior)
		if !ok {
			return block(semanticir.DiagnosticIncomplete, "translated exhaustive evidence has no repeatable raw trace for probe behavior")
		}
		classified, classifyErr := semanticir.ClassifyRawOutcome(operation, raw, choice.Behavior.Provenance)
		if classifyErr != nil || classified != choice.OutcomeID {
			return block(semanticir.DiagnosticInvalidReference, "raw exhaustive trace does not centrally classify as the probe choice")
		}
		item.Expected = raw
		harnessCases = append(harnessCases, item)
		expected.RuntimeOutcomes = append(expected.RuntimeOutcomes, semanticir.RuntimeOutcomeChoice{
			Behavior: choice.Behavior, RawOutcome: raw, MappingOutcomeID: classified,
		})
		choicesByOperation[operation.ID] = append(choicesByOperation[operation.ID], choice)
	}
	for operationID := range operations {
		if len(choicesByOperation[operationID]) != len(casesForOperation(fresh.Cases, operationID)) {
			return block(semanticir.DiagnosticIncomplete, "reference probe omits a reachable Python behavior choice for operation "+operationID)
		}
	}
	evidenceDigest, _ := semanticir.Digest(fresh.ExhaustiveEvidence[0])
	witnessDigest, _ := semanticir.Digest(witness)
	shortID := strings.TrimPrefix(witnessDigest, "sha256:")
	if len(shortID) > 16 {
		shortID = shortID[:16]
	}
	harnessPath := ".ray/probes/python-" + shortID + ".py"
	observationPath := ".ray/probes/python-" + shortID + ".observation.json"
	packageRoot, err := normalizedPackageRoot(request)
	if err != nil {
		return block(semanticir.DiagnosticInvalidInput, err.Error())
	}
	payload := probeHarnessRequest{
		Module: request.Options["python.module"], PackageRoot: filepath.ToSlash(packageRoot), SourcePath: filepath.ToSlash(request.Artifact.Path),
		ObservationPath: observationPath, BytecodeDigest: fresh.ExhaustiveEvidence[0].EmittedIRDigest,
		CompilerEvidenceDigest: evidenceDigest, Cases: harnessCases,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return block(semanticir.DiagnosticInvalidInput, "cannot encode Python probe harness: "+err.Error())
	}
	harnessBytes := []byte(strings.Replace(probeHarness, "__RAY_PAYLOAD__", strconv.Quote(string(payloadBytes)), 1))
	if bytes.Contains(harnessBytes, []byte("__RAY_PAYLOAD__")) {
		return block(semanticir.DiagnosticIncomplete, "Python probe harness payload substitution failed")
	}
	if err := compilePythonCandidate(ctx, request, harnessBytes); err != nil {
		return block(semanticir.DiagnosticUnsupported, "generated Python probe harness does not compile: "+err.Error())
	}

	var sourceArtifacts []semanticir.ArtifactRef
	seenSources := make(map[string]struct{})
	for _, entry := range request.Workspace.Entries {
		key := entry.Artifact.ID + "\x00" + entry.Artifact.Path
		if _, duplicate := seenSources[key]; duplicate {
			continue
		}
		seenSources[key] = struct{}{}
		sourceArtifacts = append(sourceArtifacts, entry.Artifact)
	}
	probeEnvironment, _ := exactWorkspaceEnvironment(request.Workspace)
	tool := request.Translator
	return executor.ProbePlan{
		ID: "python-probe:" + witness.ID, WitnessID: witness.ID, Obligation: witness.Obligation, Witness: witness,
		SourceArtifacts: sourceArtifacts,
		Workspace:       executor.ProbeWorkspace{ID: request.Workspace.ID, Root: request.Workspace.Root, State: request.Workspace.State, TreeSHA256: request.Workspace.TreeDigest},
		Tools:           []semanticir.ToolRef{request.Translator},
		Operations:      orderedProbeOperations(operations),
		Harness:         executor.ProbeHarness{Path: harnessPath, Bytes: harnessBytes, SHA256: semanticir.DigestBytes(harnessBytes), Mode: 0o500},
		Steps: []executor.ProbeStep{{
			ID: "run", Kind: executor.ProbeStepRun, Tool: &tool,
			Argv: []string{request.Translator.Path, "-P", "-B", harnessPath}, WorkDir: filepath.ToSlash(filepath.Clean(request.Workspace.WorkingDirectory)), Environment: probeEnvironment,
			Timeout: 30 * time.Second, PassSignal: executor.ExitCodeSignal(0), ObservationPath: observationPath,
		}},
		ExpectedSemantics: expected,
	}, diagnostics
}

func exactRawTraceForBehavior(evidence semanticir.ExhaustiveExecutionEvidence, behavior semanticir.BehaviorRef) (semanticir.RawOutcomeTrace, bool) {
	var found semanticir.RawOutcomeTrace
	for _, run := range evidence.Runs {
		matched := 0
		for _, observation := range run.Observations {
			if semanticir.BehaviorRefKey(observation.Behavior) != semanticir.BehaviorRefKey(behavior) {
				continue
			}
			if semanticir.ValidateRawOutcomeTrace(observation.RawOutcome) != nil {
				return semanticir.RawOutcomeTrace{}, false
			}
			if len(evidence.Runs) > 1 && found.Kind != "" && !reflect.DeepEqual(found, observation.RawOutcome) {
				return semanticir.RawOutcomeTrace{}, false
			}
			found = observation.RawOutcome
			matched++
		}
		if matched != 1 {
			return semanticir.RawOutcomeTrace{}, false
		}
	}
	return found, len(evidence.Runs) >= 2 && found.Kind != ""
}

func orderedProbeOperations(operations map[string]semanticir.Operation) []semanticir.Operation {
	ids := make([]string, 0, len(operations))
	for id := range operations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]semanticir.Operation, 0, len(ids))
	for _, id := range ids {
		result = append(result, operations[id])
	}
	return result
}

func stableTranslationDigest(model semanticir.ArtifactModel) (string, error) {
	return semanticir.ArtifactModelTranslationDigest(model)
}

func compilePythonCandidate(ctx context.Context, request semanticir.FrontendRequest, source []byte) error {
	command := exec.CommandContext(ctx, request.Translator.Path, "-P", "-B", "-c", "import sys; compile(sys.stdin.buffer.read(), sys.argv[1], 'exec', dont_inherit=True)", request.Artifact.Path)
	command.Stdin = bytes.NewReader(source)
	if err := configureWorkspaceCommand(command, request.Workspace); err != nil {
		return err
	}
	if output, err := command.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if len(output) != 0 {
			return fmt.Errorf("%s", strings.TrimSpace(string(output)))
		}
		return err
	}
	return nil
}

func matchingCase(cases []semanticir.BehaviorCase, behavior semanticir.BehaviorRef) (semanticir.BehaviorCase, bool) {
	var found *semanticir.BehaviorCase
	for i := range cases {
		if cases[i].OperationID != behavior.OperationID || !assignmentsEqual(cases[i].Conditions, behavior.Conditions) || !reflect.DeepEqual(cases[i].Inputs, behavior.Inputs) {
			continue
		}
		if found != nil {
			return semanticir.BehaviorCase{}, false
		}
		copy := cases[i]
		found = &copy
	}
	if found == nil {
		return semanticir.BehaviorCase{}, false
	}
	return *found, true
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	a, b := append([]string(nil), left...), append([]string(nil), right...)
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func matchingCaseProvenance(cases []semanticir.BehaviorCase, operationID string, conditions semanticir.Assignment) (semanticir.Provenance, bool) {
	var found *semanticir.Provenance
	for i := range cases {
		if cases[i].OperationID != operationID || !assignmentsEqual(cases[i].Conditions, conditions) {
			continue
		}
		if found != nil {
			return semanticir.Provenance{}, false
		}
		copy := cases[i].Provenance
		found = &copy
	}
	if found == nil {
		return semanticir.Provenance{}, false
	}
	return *found, true
}

func assignmentsEqual(left, right semanticir.Assignment) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func taskOperation(operations []semanticir.Operation, id string) (semanticir.Operation, bool) {
	for _, operation := range operations {
		if operation.ID == id {
			return operation, true
		}
	}
	return semanticir.Operation{}, false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func baseModel(request semanticir.FrontendRequest) semanticir.ArtifactModel {
	return semanticir.ArtifactModel{
		Artifact: request.Artifact, Language: semanticir.LanguagePython, Kind: request.Kind,
		Domains:     append([]semanticir.Domain(nil), request.FiniteDomains...),
		Groundings:  append([]semanticir.AssignmentGrounding(nil), request.Groundings...),
		Constraints: append([]semanticir.Constraint(nil), request.Constraints...),
		// This is only the closed observable alphabet O. Code behavior is
		// independently emitted as raw runtime traces and normalized centrally;
		// no required Spec row is available to this frontend.
		Outcomes:   append([]semanticir.ObservableOutcome(nil), request.Outcomes...),
		Translator: request.Translator,
		Coverage:   blockedCoverage(request, nil),
	}
}

func blockedCoverage(request semanticir.FrontendRequest, unsupported []semanticir.UnsupportedConstruct) semanticir.TranslationCoverage {
	return semanticir.TranslationCoverage{
		Status: semanticir.TranslationBlocked, Unsupported: unsupported,
		Provenance: wholeProvenance(request, semanticir.TranslationUnsupported),
	}
}

func parse(ctx context.Context, request semanticir.FrontendRequest) (bridgeResult, []semanticir.Diagnostic) {
	var result bridgeResult
	if err := validateRequest(request); err != nil {
		code := semanticir.DiagnosticInvalidInput
		if strings.Contains(err.Error(), "digest does not match") {
			code = semanticir.DiagnosticStaleArtifact
		}
		return result, []semanticir.Diagnostic{diagnostic(request, code, err.Error(), wholeLocation(request), semanticir.TranslationUnsupported)}
	}
	pythonPath := request.Translator.Path
	versionCommand := exec.CommandContext(ctx, pythonPath, "--version")
	if err := configureWorkspaceCommand(versionCommand, request.Workspace); err != nil {
		return result, []semanticir.Diagnostic{diagnostic(request, semanticir.DiagnosticInvalidInput,
			err.Error(), wholeLocation(request), semanticir.TranslationUnsupported)}
	}
	versionOutput, err := versionCommand.CombinedOutput()
	if err != nil {
		return result, []semanticir.Diagnostic{diagnostic(request, semanticir.DiagnosticUnsupported,
			"bound Python interpreter cannot report its version: "+err.Error(), wholeLocation(request), semanticir.TranslationUnsupported)}
	}
	if strings.TrimSpace(string(versionOutput)) != request.Translator.Version {
		return result, []semanticir.Diagnostic{diagnostic(request, semanticir.DiagnosticStaleArtifact,
			"bound Python interpreter version does not match the declared translator version", wholeLocation(request), semanticir.TranslationUnsupported)}
	}
	if err := validateWithOptionalRuff(ctx, request); err != nil {
		return result, []semanticir.Diagnostic{diagnostic(request, semanticir.DiagnosticUnsupported,
			err.Error(), wholeLocation(request), semanticir.TranslationUnsupported)}
	}
	modules, err := resolvedWorkspaceModules(request)
	if err != nil {
		return result, []semanticir.Diagnostic{diagnostic(request, semanticir.DiagnosticUnsupported,
			err.Error(), wholeLocation(request), semanticir.TranslationUnsupported)}
	}
	operationEntries := append([]string(nil), request.EntryPoints...)
	var testEntries []string
	if request.Kind == semanticir.ArtifactTests {
		operationEntries = operationEntries[:0]
		for _, entry := range request.EntryPoints {
			if strings.HasPrefix(entry, "test_") {
				testEntries = append(testEntries, entry)
			} else {
				operationEntries = append(operationEntries, entry)
			}
		}
		for _, operation := range request.Operations {
			operationEntries = append(operationEntries, operation.ID)
		}
	}
	payload, err := json.Marshal(bridgeRequest{
		Source: string(request.Source), Path: request.Artifact.Path,
		ArtifactKind: string(request.Kind), EntryPoints: compactOptionStrings(operationEntries),
		TestEntryPoints: compactOptionStrings(testEntries), ResolvedModules: modules,
		ModuleName: request.Options["python.module"], Execution: request.Options["python.execution"],
	})
	if err != nil {
		return result, []semanticir.Diagnostic{diagnostic(request, semanticir.DiagnosticInvalidInput,
			"cannot encode Python parser input: "+err.Error(), wholeLocation(request), semanticir.TranslationUnsupported)}
	}
	parserContext, cancelParser := context.WithTimeout(ctx, 30*time.Second)
	defer cancelParser()
	command := exec.CommandContext(parserContext, pythonPath, "-P", "-B", "-c", astBridge)
	command.Stdin = bytes.NewReader(payload)
	if err := configureWorkspaceCommand(command, request.Workspace); err != nil {
		return result, []semanticir.Diagnostic{diagnostic(request, semanticir.DiagnosticInvalidInput,
			err.Error(), wholeLocation(request), semanticir.TranslationUnsupported)}
	}
	output, err := command.Output()
	if err != nil {
		message := "Python ast parser failed"
		if ctx.Err() != nil {
			message = "Python ast parser canceled: " + ctx.Err().Error()
		} else if exit, ok := err.(*exec.ExitError); ok && len(exit.Stderr) != 0 {
			message += ": " + strings.TrimSpace(string(exit.Stderr))
		} else {
			message += ": " + err.Error()
		}
		return result, []semanticir.Diagnostic{diagnostic(request, semanticir.DiagnosticUnsupported,
			message, wholeLocation(request), semanticir.TranslationUnsupported)}
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, []semanticir.Diagnostic{diagnostic(request, semanticir.DiagnosticUnsupported,
			"Python ast parser returned invalid output: "+err.Error(), wholeLocation(request), semanticir.TranslationUnsupported)}
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return result, []semanticir.Diagnostic{diagnostic(request, semanticir.DiagnosticUnsupported,
			"Python ast parser returned trailing output", wholeLocation(request), semanticir.TranslationUnsupported)}
	}
	if result.ParseError != "" {
		return result, []semanticir.Diagnostic{diagnostic(request, semanticir.DiagnosticInvalidInput,
			"invalid Python syntax: "+result.ParseError, toLocation(request, result.Location), semanticir.TranslationUnsupported)}
	}
	result.RawInput = append([]byte(nil), payload...)
	result.RawOutput = append([]byte(nil), output...)
	return result, nil
}

func validateWithOptionalRuff(ctx context.Context, request semanticir.FrontendRequest) error {
	path := strings.TrimSpace(request.Options["python.ruff.path"])
	digest := strings.TrimSpace(request.Options["python.ruff.digest"])
	version := strings.TrimSpace(request.Options["python.ruff.version"])
	configured := path != "" || digest != "" || version != ""
	if !configured {
		return nil
	}
	if path == "" || digest == "" || version == "" || !filepath.IsAbs(path) || !digestPattern.MatchString(digest) {
		return fmt.Errorf("configured Ruff requires exact absolute python.ruff.path, sha256 digest, and version")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read bound Ruff binary: %w", err)
	}
	sum := sha256.Sum256(content)
	if digest != "sha256:"+hex.EncodeToString(sum[:]) {
		return fmt.Errorf("bound Ruff digest does not match executable bytes")
	}
	versionCommand := exec.CommandContext(ctx, path, "--version")
	if err := configureWorkspaceCommand(versionCommand, request.Workspace); err != nil {
		return err
	}
	versionOutput, err := versionCommand.CombinedOutput()
	if err != nil || strings.TrimSpace(string(versionOutput)) != version {
		return fmt.Errorf("bound Ruff version does not match configured identity")
	}
	command := exec.CommandContext(ctx, path, "check", "--no-cache", "--output-format=json", "--select=E9,F63,F7,F82", "--stdin-filename", request.Artifact.Path, "-")
	command.Stdin = bytes.NewReader(request.Source)
	if err := configureWorkspaceCommand(command, request.Workspace); err != nil {
		return err
	}
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("bound Ruff rejected frozen Python source: %s", message)
	}
	return nil
}

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func validateRequest(request semanticir.FrontendRequest) error {
	if request.Language != semanticir.LanguagePython {
		return fmt.Errorf("Python frontend requires language %q", semanticir.LanguagePython)
	}
	if request.Kind != semanticir.ArtifactCode && request.Kind != semanticir.ArtifactTests {
		return fmt.Errorf("Python frontend accepts only code or tests artifacts")
	}
	if request.Artifact.Kind != request.Kind {
		return fmt.Errorf("artifact kind %q does not match request kind %q", request.Artifact.Kind, request.Kind)
	}
	if request.Artifact.ID == "" || request.Artifact.Path == "" {
		return fmt.Errorf("artifact ID and path are required")
	}
	if len(request.Source) == 0 {
		return fmt.Errorf("Python source is empty")
	}
	if !utf8.Valid(request.Source) {
		return fmt.Errorf("Python source is not valid UTF-8")
	}
	if !digestPattern.MatchString(request.Artifact.Digest) {
		return fmt.Errorf("artifact digest must be lowercase sha256:<64 hex>")
	}
	sum := sha256.Sum256(request.Source)
	if request.Artifact.Digest != "sha256:"+hex.EncodeToString(sum[:]) {
		return fmt.Errorf("artifact digest does not match source bytes")
	}
	if request.Translator.Name == "" || request.Translator.Path == "" || request.Translator.Version == "" {
		return fmt.Errorf("translator name, absolute path, digest, and version are required")
	}
	if !filepath.IsAbs(request.Translator.Path) {
		return fmt.Errorf("translator path must be absolute")
	}
	if !digestPattern.MatchString(request.Translator.Digest) {
		return fmt.Errorf("translator digest must be lowercase sha256:<64 hex>")
	}
	translatorBytes, err := os.ReadFile(request.Translator.Path)
	if err != nil {
		return fmt.Errorf("cannot read bound translator binary: %w", err)
	}
	translatorSum := sha256.Sum256(translatorBytes)
	if request.Translator.Digest != "sha256:"+hex.EncodeToString(translatorSum[:]) {
		return fmt.Errorf("translator digest does not match interpreter bytes")
	}
	if err := validateWorkspace(request); err != nil {
		return err
	}
	if err := validatePythonOptions(request.Options); err != nil {
		return err
	}
	return nil
}

func validatePythonOptions(options map[string]string) error {
	allowed := map[string]struct{}{
		"python.execution": {}, "python.module": {}, "python.package_root": {}, "python.max_cases": {},
		"python.ruff.path": {}, "python.ruff.digest": {}, "python.ruff.version": {},
	}
	for key := range options {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("unsupported Python frontend option %q; semantic mappings must come from typed FrontendRequest scope", key)
		}
	}
	return nil
}

func validateWorkspace(request semanticir.FrontendRequest) error {
	workspace := request.Workspace
	if workspace.ID == "" || workspace.Root == "" || workspace.WorkingDirectory == "" {
		return fmt.Errorf("frozen workspace ID, absolute root, and working directory are required")
	}
	if !filepath.IsAbs(workspace.Root) {
		return fmt.Errorf("workspace root must be absolute")
	}
	working := filepath.Clean(workspace.WorkingDirectory)
	if filepath.IsAbs(working) || working == ".." || strings.HasPrefix(working, ".."+string(filepath.Separator)) {
		return fmt.Errorf("workspace working directory must stay within the frozen root")
	}
	if !digestPattern.MatchString(workspace.TreeDigest) {
		return fmt.Errorf("workspace tree digest must be lowercase sha256:<64 hex>")
	}
	if _, err := exactWorkspaceEnvironment(workspace); err != nil {
		return err
	}
	if workspace.BuildCommand == "" && workspace.CompilationDatabase == nil {
		return fmt.Errorf("frozen workspace requires a build command or compilation database")
	}
	switch workspace.State {
	case semanticir.WorkspaceBaseOldTests, semanticir.WorkspaceBaseNewTests, semanticir.WorkspaceSolutionNewTests:
	default:
		return fmt.Errorf("workspace state %q is invalid", workspace.State)
	}
	if len(workspace.Entries) == 0 || len(request.FocusArtifacts) == 0 {
		return fmt.Errorf("workspace entries and focus artifacts are required")
	}
	entries := make(map[semanticir.ArtifactRef]semanticir.WorkspaceEntry, len(workspace.Entries))
	for _, entry := range workspace.Entries {
		clean := filepath.Clean(entry.Path)
		if entry.Path == "" || filepath.IsAbs(entry.Path) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("workspace entry path %q escapes the workspace", entry.Path)
		}
		if entry.Artifact.Path != entry.Path || !digestPattern.MatchString(entry.Artifact.Digest) {
			return fmt.Errorf("workspace entry %q has an invalid artifact binding", entry.Path)
		}
		entries[entry.Artifact] = entry
	}
	focused := make(map[semanticir.ArtifactRef]struct{}, len(request.FocusArtifacts))
	for _, artifact := range request.FocusArtifacts {
		entry, ok := entries[artifact]
		if !ok {
			return fmt.Errorf("focus artifact %q is absent from the frozen workspace", artifact.ID)
		}
		content, err := readWorkspaceEntry(workspace.Root, entry.Path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		if artifact.Digest != "sha256:"+hex.EncodeToString(sum[:]) {
			return fmt.Errorf("focus artifact %q digest does not match workspace bytes", artifact.ID)
		}
		focused[artifact] = struct{}{}
	}
	if _, ok := focused[request.Artifact]; !ok {
		return fmt.Errorf("translated artifact is not in the explicit focus set")
	}
	return nil
}

func exactWorkspaceEnvironment(workspace semanticir.WorkspaceRef) ([]string, error) {
	if !workspace.ClearEnvironment || !workspace.KillProcessGroup {
		return nil, fmt.Errorf("frozen workspace must clear the ambient environment and kill subprocess groups")
	}
	digest, err := semanticir.Digest(workspace.Environment)
	if err != nil || workspace.EnvironmentDigest != digest {
		return nil, fmt.Errorf("frozen workspace environment digest does not match its exact entries")
	}
	result := make([]string, 0, len(workspace.Environment))
	previous := ""
	hashSeed := ""
	for index, variable := range workspace.Environment {
		if variable.Name == "" || strings.Contains(variable.Name, "=") || strings.ContainsRune(variable.Name, '\x00') || strings.ContainsRune(variable.Value, '\x00') || (index > 0 && variable.Name <= previous) {
			return nil, fmt.Errorf("frozen workspace environment must be strictly name-sorted, unique, and NUL-free")
		}
		if variable.Name == "PYTHONHASHSEED" {
			hashSeed = variable.Value
		}
		previous = variable.Name
		result = append(result, variable.Name+"="+variable.Value)
	}
	seed, seedErr := strconv.ParseUint(hashSeed, 10, 32)
	if hashSeed == "" || seedErr != nil || seed > math.MaxUint32 {
		return nil, fmt.Errorf("frozen Python execution requires a deterministic numeric PYTHONHASHSEED")
	}
	return result, nil
}

func configureWorkspaceCommand(command *exec.Cmd, workspace semanticir.WorkspaceRef) error {
	environment, err := exactWorkspaceEnvironment(workspace)
	if err != nil {
		return err
	}
	working := filepath.Join(workspace.Root, filepath.Clean(workspace.WorkingDirectory))
	resolvedRoot, err := filepath.EvalSymlinks(filepath.Clean(workspace.Root))
	if err != nil {
		return fmt.Errorf("resolve frozen workspace root: %w", err)
	}
	resolvedWorking, err := filepath.EvalSymlinks(working)
	if err != nil {
		return fmt.Errorf("resolve frozen workspace working directory: %w", err)
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedWorking)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("frozen workspace working directory escapes its root")
	}
	command.Env = environment
	command.Dir = resolvedWorking
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	command.WaitDelay = 2 * time.Second
	return nil
}

func readWorkspaceEntry(root, relative string) ([]byte, error) {
	rootClean := filepath.Clean(root)
	rootResolved, err := filepath.EvalSymlinks(rootClean)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve frozen workspace root: %w", err)
	}
	path := filepath.Join(rootClean, filepath.Clean(relative))
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve frozen workspace entry %q: %w", relative, err)
	}
	relativeResolved, err := filepath.Rel(rootResolved, resolved)
	if err != nil || relativeResolved == ".." || strings.HasPrefix(relativeResolved, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("workspace entry %q resolves outside the frozen root", relative)
	}
	content, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("cannot read frozen workspace entry %q: %w", relative, err)
	}
	return content, nil
}

func resolvedWorkspaceImport(request semanticir.FrontendRequest, module string) bool {
	modules, err := resolvedWorkspaceModules(request)
	if err != nil {
		return false
	}
	_, ok := modules[module]
	return ok
}

func resolvedWorkspaceModules(request semanticir.FrontendRequest) (map[string]string, error) {
	modules := make(map[string]string)
	packageRoot, err := normalizedPackageRoot(request)
	if err != nil {
		return nil, err
	}
	for _, entry := range request.Workspace.Entries {
		if entry.Artifact == request.Artifact {
			continue
		}
		relative := filepath.Clean(entry.Path)
		if packageRoot != "." {
			var err error
			relative, err = filepath.Rel(packageRoot, relative)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				continue
			}
		}
		if filepath.Base(relative) == "__init__.py" {
			relative = filepath.Dir(relative)
		} else if strings.HasSuffix(relative, ".py") {
			relative = strings.TrimSuffix(relative, ".py")
		} else {
			continue
		}
		moduleName := strings.ReplaceAll(filepath.ToSlash(relative), "/", ".")
		if moduleName == "" || moduleName == "." {
			continue
		}
		content, err := readWorkspaceEntry(request.Workspace.Root, entry.Path)
		if err != nil {
			return nil, err
		}
		if !utf8.Valid(content) {
			return nil, fmt.Errorf("frozen Python import %q is not UTF-8", moduleName)
		}
		sum := sha256.Sum256(content)
		if entry.Artifact.Digest != "sha256:"+hex.EncodeToString(sum[:]) {
			return nil, fmt.Errorf("frozen Python import %q digest does not match workspace bytes", moduleName)
		}
		if _, duplicate := modules[moduleName]; duplicate {
			return nil, fmt.Errorf("multiple frozen workspace entries resolve Python module %q", moduleName)
		}
		modules[moduleName] = string(content)
	}
	return modules, nil
}

type lowering struct {
	request       semanticir.FrontendRequest
	model         *semanticir.ArtifactModel
	diagnostics   []semanticir.Diagnostic
	variables     map[string]semanticir.ValueType
	bindings      map[string]semanticir.Expression
	testInstances map[string][]testAssertionInstance
	total         int
	translated    int
	outcomeIDs    map[string]struct{}
}

func (lower *lowering) unsupported(kind, reason string, location bridgeLocation) {
	provenance := semanticir.NewProvenance(lower.request.Artifact, toLocation(lower.request, location), semanticir.TranslationUnsupported)
	lower.model.Coverage.Unsupported = append(lower.model.Coverage.Unsupported, semanticir.UnsupportedConstruct{
		Kind: kind, Reason: reason, Provenance: provenance,
	})
	lower.diagnostics = append(lower.diagnostics, semanticir.Diagnostic{
		Severity: semanticir.SeverityError, Code: semanticir.DiagnosticUnsupported,
		Message: kind + ": " + reason, Provenance: provenance,
	})
}

func (lower *lowering) operation(function *pyFunction) semanticir.Operation {
	lower.total++
	lower.translated++
	provenance := semanticir.NewProvenance(lower.request.Artifact, toLocation(lower.request, function.Location), semanticir.TranslationTranslated)
	kind := semanticir.OperationFunction
	if function.IsTest || lower.request.Kind == semanticir.ArtifactTests {
		kind = semanticir.OperationTest
	}
	operation := semanticir.Operation{ID: function.Name, Kind: kind, Provenance: provenance}
	var declaredOperation *semanticir.Operation
	for _, declared := range lower.request.Operations {
		if declared.ID == function.Name {
			declaredCopy := declared
			declaredOperation = &declaredCopy
			break
		}
	}
	if declaredOperation != nil {
		operation.OutcomeIDs = append([]string(nil), declaredOperation.OutcomeIDs...)
	}
	previousVariables := lower.variables
	previousBindings := lower.bindings
	lower.variables = make(map[string]semanticir.ValueType)
	lower.bindings = make(map[string]semanticir.Expression)
	for index, name := range function.Parameters {
		domain, ok := domainFor(lower.request, function.Name, name, index)
		if !ok {
			lower.unsupported("PY_MISSING_DOMAIN", "parameter "+function.Name+"."+name+" has no explicitly mapped finite domain", function.Location)
			continue
		}
		valueType, ok := domainType(domain)
		if declaredOperation != nil && index < len(declaredOperation.Inputs) && declaredOperation.Inputs[index].Name == name && semanticir.ValidValueType(declaredOperation.Inputs[index].Type) {
			// Domain.Type classifies semantic labels. The independently
			// compiled operation input carries the raw Python type; labels
			// deliberately remain strings in strict IR.
			valueType, ok = declaredOperation.Inputs[index].Type, true
		}
		if !ok {
			lower.unsupported("PY_MIXED_DOMAIN", "domain "+domain.ID+" has values that do not share a Semantic IR type", function.Location)
			valueType = semanticir.TypeUnknown
		}
		lower.variables[name] = valueType
		operation.Inputs = append(operation.Inputs, semanticir.Variable{
			Name: name, Type: valueType, DomainID: domain.ID, Provenance: provenance,
		})
		operation.DomainIDs = append(operation.DomainIDs, domain.ID)
	}
	for i := range function.Body {
		operation.Body = append(operation.Body, lower.statements(&function.Body[i], kind == semanticir.OperationTest)...)
	}
	lower.variables = previousVariables
	lower.bindings = previousBindings
	return operation
}

func (lower *lowering) statements(statement *pyStatement, inTest bool) []semanticir.Statement {
	lower.total++
	provenance := semanticir.NewProvenance(lower.request.Artifact, toLocation(lower.request, statement.Location), semanticir.TranslationTranslated)
	switch statement.Kind {
	case "pass", "docstring":
		lower.translated++
		return nil
	case "assert", "assert_raises":
		if !inTest {
			lower.unsupported("PY_RUNTIME_ASSERT", "solution runtime assertions are not test observations", statement.Location)
			return nil
		}
		// Assertions are represented by TestModel rather than executable body.
		lower.translated++
		return nil
	case "return":
		value, ok := lower.expression(statement.Value)
		if statement.Value != nil && !ok {
			return nil
		}
		lower.translated++
		return []semanticir.Statement{{Kind: semanticir.StmtReturn, Value: value, Provenance: provenance}}
	case "raise":
		value, ok := lower.expression(statement.Value)
		if statement.Value != nil && !ok {
			return nil
		}
		exceptionType, message := raisedValue(value)
		if exceptionType == "" {
			lower.unsupported("PY_DYNAMIC_RAISE", "raised exception type is not statically named", statement.Location)
			return nil
		}
		lower.translated++
		return []semanticir.Statement{{
			Kind: semanticir.StmtRaise, Value: value, ExceptionType: exceptionType,
			Message: message, Provenance: provenance,
		}}
	case "call":
		value, ok := lower.expression(statement.Value)
		if !ok || value == nil {
			return nil
		}
		lower.translated++
		effect := semanticir.Effect{
			ID: "call:" + value.Name, Kind: semanticir.EffectCall, Target: value.Name,
			Value: value, Provenance: provenance,
		}
		return []semanticir.Statement{{Kind: semanticir.StmtCall, Value: value, Effects: []semanticir.Effect{effect}, Provenance: provenance}}
	case "assign":
		value, ok := lower.expression(statement.Value)
		if !ok || value == nil || statement.Target == "" {
			return nil
		}
		lower.variables[statement.Target] = value.Type
		lower.translated++
		return []semanticir.Statement{{Kind: semanticir.StmtAssign, Target: statement.Target, Value: value, Provenance: provenance}}
	case "loop":
		iterator, ok := lower.expression(statement.Value)
		if !ok || iterator == nil || statement.Target == "" {
			return nil
		}
		previous, existed := lower.variables[statement.Target]
		lower.variables[statement.Target] = sequenceElementType(iterator)
		loop := semanticir.Statement{Kind: semanticir.StmtLoop, Target: statement.Target, Iterator: iterator, Provenance: provenance}
		for i := range statement.Body {
			loop.Then = append(loop.Then, lower.statements(&statement.Body[i], inTest)...)
		}
		if existed {
			lower.variables[statement.Target] = previous
		} else {
			delete(lower.variables, statement.Target)
		}
		lower.translated++
		// Test assertions are represented in TestModel/TestProjection, not in
		// an executable Operation body. If a bounded test loop contains only
		// assertions, retaining an empty StmtLoop would claim a malformed
		// executable loop. Its control semantics remain compiler-bound in the
		// projection evidence built below.
		if inTest && len(loop.Then) == 0 {
			return nil
		}
		return []semanticir.Statement{loop}
	case "try":
		tryStatement := semanticir.Statement{Kind: semanticir.StmtTry, Provenance: provenance}
		for i := range statement.Body {
			tryStatement.Then = append(tryStatement.Then, lower.statements(&statement.Body[i], inTest)...)
		}
		for _, handler := range statement.Catches {
			clause := semanticir.CatchClause{
				ExceptionType: handler.ExceptionType,
				Provenance:    semanticir.NewProvenance(lower.request.Artifact, toLocation(lower.request, handler.Location), semanticir.TranslationTranslated),
			}
			for i := range handler.Body {
				clause.Body = append(clause.Body, lower.statements(&handler.Body[i], inTest)...)
			}
			tryStatement.Catches = append(tryStatement.Catches, clause)
		}
		lower.translated++
		return []semanticir.Statement{tryStatement}
	case "continue":
		lower.translated++
		return []semanticir.Statement{{Kind: semanticir.StmtContinue, Provenance: provenance}}
	case "branch":
		condition, ok := lower.expression(statement.Value)
		if !ok || condition == nil {
			return nil
		}
		branch := semanticir.Statement{Kind: semanticir.StmtBranch, Condition: condition, Provenance: provenance}
		for i := range statement.Body {
			branch.Then = append(branch.Then, lower.statements(&statement.Body[i], inTest)...)
		}
		for i := range statement.Alternate {
			branch.Else = append(branch.Else, lower.statements(&statement.Alternate[i], inTest)...)
		}
		lower.translated++
		return []semanticir.Statement{branch}
	default:
		lower.unsupported("PY_UNSUPPORTED_STATEMENT", "cannot lower statement kind "+statement.Kind, statement.Location)
		return nil
	}
}

func (lower *lowering) expression(expression *pyExpression) (*semanticir.Expression, bool) {
	if expression == nil {
		return nil, true
	}
	lower.total++
	provenance := semanticir.NewProvenance(lower.request.Artifact, toLocation(lower.request, expression.Location), semanticir.TranslationTranslated)
	result := semanticir.Expression{Provenance: provenance}
	switch expression.Kind {
	case "literal":
		literal, ok := decodeLiteral(expression.Value)
		if !ok {
			lower.unsupported("PY_UNSUPPORTED_LITERAL", "literal is outside bool/integer/string/unit Semantic IR", expression.Location)
			return nil, false
		}
		result.Kind, result.Type, result.Literal = semanticir.ExprLiteral, literal.Type, &literal
	case "name":
		if bound, exists := lower.bindings[expression.Name]; exists {
			result = bound
		} else {
			result.Kind, result.Type, result.Name = semanticir.ExprVariable, lower.variables[expression.Name], expression.Name
		}
	case "call":
		result.Kind, result.Type, result.Name = semanticir.ExprCall, semanticir.TypeUnknown, expression.Name
		for i := range expression.Args {
			operand, ok := lower.expression(&expression.Args[i])
			if !ok {
				return nil, false
			}
			result.Operands = append(result.Operands, *operand)
		}
	case "field":
		if len(expression.Args) != 1 {
			lower.unsupported("PY_INVALID_FIELD", "field expression requires one record operand", expression.Location)
			return nil, false
		}
		operand, ok := lower.expression(&expression.Args[0])
		if !ok {
			return nil, false
		}
		result.Kind, result.Type, result.Name, result.Operands = semanticir.ExprField, semanticir.TypeUnknown, expression.Name, []semanticir.Expression{*operand}
	case "index":
		if len(expression.Args) != 2 {
			lower.unsupported("PY_INVALID_INDEX", "index expression requires sequence and index operands", expression.Location)
			return nil, false
		}
		for i := range expression.Args {
			operand, ok := lower.expression(&expression.Args[i])
			if !ok {
				return nil, false
			}
			result.Operands = append(result.Operands, *operand)
		}
		result.Kind, result.Type = semanticir.ExprIndex, semanticir.TypeUnknown
	case "list", "tuple":
		result.Kind, result.Type = semanticir.ExprSequence, semanticir.TypeSequence
		if expression.Kind == "tuple" {
			result.Type = semanticir.TypeTuple
		}
		for i := range expression.Args {
			operand, ok := lower.expression(&expression.Args[i])
			if !ok {
				return nil, false
			}
			result.Operands = append(result.Operands, *operand)
		}
	case "dict":
		if len(expression.Args)%2 != 0 {
			lower.unsupported("PY_INVALID_RECORD", "dictionary record requires key/value pairs", expression.Location)
			return nil, false
		}
		result.Kind, result.Type = semanticir.ExprRecord, semanticir.TypeRecord
		for i := 0; i < len(expression.Args); i += 2 {
			key, ok := lower.expression(&expression.Args[i])
			if !ok || key.Literal == nil || key.Literal.Type != semanticir.TypeString {
				lower.unsupported("PY_DYNAMIC_RECORD_KEY", "record keys must be literal strings", expression.Location)
				return nil, false
			}
			value, ok := lower.expression(&expression.Args[i+1])
			if !ok {
				return nil, false
			}
			result.Operands = append(result.Operands,
				semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeString, Literal: key.Literal, Provenance: key.Provenance}, *value)
		}
	case "fstring":
		var combined *semanticir.Expression
		for i := range expression.Args {
			part, ok := lower.expression(&expression.Args[i])
			if !ok {
				return nil, false
			}
			if combined == nil {
				combined = part
				continue
			}
			combined = &semanticir.Expression{Kind: semanticir.ExprBinary, Type: semanticir.TypeString, Operator: semanticir.OpAdd, Operands: []semanticir.Expression{*combined, *part}, Provenance: provenance}
		}
		if combined == nil {
			literal := semanticir.Literal{Type: semanticir.TypeString}
			result.Kind, result.Type, result.Literal = semanticir.ExprLiteral, semanticir.TypeString, &literal
		} else {
			result = *combined
		}
	case "unary", "binary", "boolean":
		operator, ok := pythonOperator(expression.Operator)
		if !ok || (expression.Operator == "divide" || expression.Operator == "power") {
			lower.unsupported("PY_UNSUPPORTED_OPERATOR", "operator has no faithful bounded Python Semantic IR: "+expression.Operator, expression.Location)
			return nil, false
		}
		result.Operator = operator
		result.Kind = map[string]semanticir.ExpressionKind{"unary": semanticir.ExprUnary, "binary": semanticir.ExprBinary, "boolean": semanticir.ExprBool}[expression.Kind]
		for i := range expression.Args {
			operand, ok := lower.expression(&expression.Args[i])
			if !ok {
				return nil, false
			}
			result.Operands = append(result.Operands, *operand)
		}
		if expression.Kind == "boolean" || operator == semanticir.OpNot {
			result.Type = semanticir.TypeBool
			if expression.Kind == "boolean" {
				for _, operand := range result.Operands {
					if operand.Type != semanticir.TypeBool {
						lower.unsupported("PY_NON_BOOLEAN_LOGIC", "Python and/or returns an operand unless every operand is boolean", expression.Location)
						return nil, false
					}
				}
			}
		} else if len(result.Operands) != 0 {
			result.Type = result.Operands[0].Type
		}
	case "comparison":
		if len(expression.Operators) == 0 || len(expression.Args) != len(expression.Operators)+1 {
			lower.unsupported("PY_INVALID_COMPARISON", "comparison AST has inconsistent operands", expression.Location)
			return nil, false
		}
		comparisons := make([]semanticir.Expression, 0, len(expression.Operators))
		for i, name := range expression.Operators {
			left, ok := lower.expression(&expression.Args[i])
			if !ok {
				return nil, false
			}
			right, ok := lower.expression(&expression.Args[i+1])
			if !ok {
				return nil, false
			}
			operator, operatorOK := pythonOperator(name)
			if name == "is" || name == "is_not" {
				if (right.Literal != nil && right.Literal.Type == semanticir.TypeBool) || (left.Literal != nil && left.Literal.Type == semanticir.TypeBool) {
					operator = semanticir.OpEQ
					if name == "is_not" {
						operator = semanticir.OpNE
					}
					operatorOK = true
				} else {
					var nullable *semanticir.Expression
					if right.Literal != nil && right.Literal.Type == semanticir.TypeOptional && right.Literal.Null {
						nullable = left
					}
					if left.Literal != nil && left.Literal.Type == semanticir.TypeOptional && left.Literal.Null {
						nullable = right
					}
					if nullable == nil {
						lower.unsupported("PY_IDENTITY_COMPARISON", "identity comparison is supported only against None", expression.Location)
						return nil, false
					}
					nullCheck := semanticir.Expression{Kind: semanticir.ExprUnary, Type: semanticir.TypeBool, Operator: semanticir.OpIsNull, Operands: []semanticir.Expression{*nullable}, Provenance: provenance}
					if name == "is_not" {
						nullCheck = semanticir.Expression{Kind: semanticir.ExprUnary, Type: semanticir.TypeBool, Operator: semanticir.OpNot, Operands: []semanticir.Expression{nullCheck}, Provenance: provenance}
					}
					comparisons = append(comparisons, nullCheck)
					continue
				}
			}
			negateIn := name == "not_in"
			if !operatorOK {
				lower.unsupported("PY_UNSUPPORTED_COMPARISON", "comparison has no Semantic IR operator: "+name, expression.Location)
				return nil, false
			}
			comparison := semanticir.Expression{
				Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: operator,
				Operands: []semanticir.Expression{*left, *right}, Provenance: provenance,
			}
			if negateIn {
				comparison = semanticir.Expression{Kind: semanticir.ExprUnary, Type: semanticir.TypeBool, Operator: semanticir.OpNot, Operands: []semanticir.Expression{comparison}, Provenance: provenance}
			}
			comparisons = append(comparisons, comparison)
			if left.Type != semanticir.TypeUnknown && right.Type != semanticir.TypeUnknown && left.Type != right.Type {
				lower.unsupported("PY_CROSS_TYPE_COMPARISON", "Python cross-type comparison semantics are outside the strict IR type system", expression.Location)
				return nil, false
			}
		}
		if len(comparisons) == 1 {
			result = comparisons[0]
		} else {
			result.Kind, result.Type, result.Operator, result.Operands = semanticir.ExprBool, semanticir.TypeBool, semanticir.OpAnd, comparisons
		}
	default:
		lower.unsupported("PY_UNSUPPORTED_EXPRESSION", "cannot lower expression kind "+expression.Kind, expression.Location)
		return nil, false
	}
	lower.translated++
	return &result, true
}

func pythonOperator(name string) (semanticir.Operator, bool) {
	operators := map[string]semanticir.Operator{
		"not": semanticir.OpNot, "negative": semanticir.OpNeg,
		"add": semanticir.OpAdd, "subtract": semanticir.OpSub, "multiply": semanticir.OpMul,
		"divide": semanticir.OpDiv, "modulo": semanticir.OpMod,
		"equal": semanticir.OpEQ, "not_equal": semanticir.OpNE,
		"less_than": semanticir.OpLT, "less_or_equal": semanticir.OpLE,
		"greater_than": semanticir.OpGT, "greater_or_equal": semanticir.OpGE,
		"and": semanticir.OpAnd, "or": semanticir.OpOr, "in": semanticir.OpIn, "not_in": semanticir.OpIn,
	}
	operator, ok := operators[name]
	return operator, ok
}

func (lower *lowering) tests(function *pyFunction, operations map[string]semanticir.Operation) []semanticir.TestModel {
	previousVariables, previousBindings := lower.variables, lower.bindings
	lower.variables = make(map[string]semanticir.ValueType)
	lower.bindings = make(map[string]semanticir.Expression)
	defer func() {
		lower.variables, lower.bindings = previousVariables, previousBindings
	}()

	var instances []testAssertionInstance
	if !lower.lowerTestBlock(function.Body, operations, &instances, nil) {
		return nil
	}
	if len(instances) == 0 {
		lower.unsupported("PY_TEST_WITHOUT_ORACLE", "test function has no exactly modeled assertion", function.Location)
		return nil
	}
	model := semanticir.TestModel{
		ID:         pythonTestID(lower.request.Artifact.ID, function.Name),
		Provenance: semanticir.NewProvenance(lower.request.Artifact, toLocation(lower.request, function.Location), semanticir.TranslationTranslated),
	}
	for _, instance := range instances {
		model.Assertions = append(model.Assertions, instance.assertion)
		model.Predicate.Children = append(model.Predicate.Children, instance.predicate)
	}
	if len(model.Predicate.Children) == 0 {
		return nil
	}
	if len(model.Predicate.Children) == 1 {
		model.Predicate = model.Predicate.Children[0]
	} else {
		model.Predicate.Kind = semanticir.PredicateAnd
		model.Predicate.Provenance = model.Provenance
	}
	if behavior, accepted, ok := unaryPredicateMetadata(model.Predicate); ok {
		model.OperationID = behavior.OperationID
		model.Conditions = cloneAssignment(behavior.Conditions)
		model.AcceptedOutcomes = accepted
	}
	lower.testInstances[model.ID] = append([]testAssertionInstance(nil), instances...)
	return []semanticir.TestModel{model}
}

func (lower *lowering) lowerTestBlock(body []pyStatement, operations map[string]semanticir.Operation, instances *[]testAssertionInstance, inheritedControls []*pyStatement) bool {
	controls := append([]*pyStatement(nil), inheritedControls...)
	for index := range body {
		statement := &body[index]
		switch statement.Kind {
		case "docstring", "pass":
			continue
		case "assert", "assert_raises":
			assertion, predicate, ok := lower.assertion(statement, operations)
			if !ok {
				return false
			}
			*instances = append(*instances, testAssertionInstance{
				statement: statement, assertion: assertion, predicate: predicate,
				controls: append([]*pyStatement(nil), controls...),
			})
		case "assign":
			value, ok := lower.expression(statement.Value)
			if !ok || value == nil || statement.Target == "" {
				return false
			}
			lower.bindings[statement.Target] = *value
			lower.variables[statement.Target] = value.Type
			controls = append(controls, statement)
		case "loop":
			values, ok := lower.finiteTestIterator(statement.Value)
			if !ok || len(values) == 0 {
				lower.unsupported("PY_DYNAMIC_TEST_LOOP", "test loop iterator is not an exact non-empty bounded literal/range", statement.Location)
				return false
			}
			for _, value := range values {
				lower.bindings[statement.Target] = value
				lower.variables[statement.Target] = value.Type
				if !lower.lowerTestBlock(statement.Body, operations, instances, append(controls, statement)) {
					return false
				}
			}
			// Python retains the final iteration target after a non-empty loop.
			controls = append(controls, statement)
		case "branch":
			condition, ok := lower.expression(statement.Value)
			if !ok || condition == nil || condition.Kind != semanticir.ExprLiteral || condition.Literal == nil || condition.Literal.Type != semanticir.TypeBool {
				lower.unsupported("PY_DYNAMIC_TEST_BRANCH", "test branch condition is not fixed by bounded local values", statement.Location)
				return false
			}
			selected := statement.Alternate
			if condition.Literal.Bool {
				selected = statement.Body
			}
			if !lower.lowerTestBlock(selected, operations, instances, append(controls, statement)) {
				return false
			}
			controls = append(controls, statement)
		default:
			lower.unsupported("PY_TEST_CONTROL", "test pass depends on unsupported "+statement.Kind+" semantics", statement.Location)
			return false
		}
	}
	return true
}

func (lower *lowering) finiteTestIterator(expression *pyExpression) ([]semanticir.Expression, bool) {
	iterator, ok := lower.expression(expression)
	if !ok || iterator == nil {
		return nil, false
	}
	if iterator.Kind == semanticir.ExprSequence {
		if len(iterator.Operands) > maxCases(lower.request) {
			return nil, false
		}
		return append([]semanticir.Expression(nil), iterator.Operands...), true
	}
	if iterator.Kind != semanticir.ExprCall || iterator.Name != "range" || len(iterator.Operands) < 1 || len(iterator.Operands) > 3 {
		return nil, false
	}
	arguments := make([]int64, len(iterator.Operands))
	for index, operand := range iterator.Operands {
		if operand.Kind != semanticir.ExprLiteral || operand.Literal == nil || operand.Literal.Type != semanticir.TypeInteger {
			return nil, false
		}
		arguments[index] = operand.Literal.Integer
	}
	start, stop, step := int64(0), arguments[0], int64(1)
	if len(arguments) >= 2 {
		start, stop = arguments[0], arguments[1]
	}
	if len(arguments) == 3 {
		step = arguments[2]
	}
	if step == 0 {
		return nil, false
	}
	values := make([]semanticir.Expression, 0)
	for value := start; (step > 0 && value < stop) || (step < 0 && value > stop); {
		if len(values) >= maxCases(lower.request) {
			return nil, false
		}
		literal := semanticir.Literal{Type: semanticir.TypeInteger, Integer: value}
		values = append(values, semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &literal, Provenance: iterator.Provenance})
		next := value + step
		if step > 0 && next <= value || step < 0 && next >= value {
			return nil, false
		}
		value = next
	}
	return values, true
}

func (lower *lowering) testProjectionEvidence(result bridgeResult) (*semanticir.TestObservationProjection, *semanticir.RunnerSelectionEvidence, error) {
	if len(result.RawInput) == 0 || len(result.RawOutput) == 0 || !digestPattern.MatchString(result.CompilerIRDigest) {
		return nil, nil, fmt.Errorf("CPython parser did not emit complete bytecode derivation bytes")
	}
	workingDirectory, err := filepath.EvalSymlinks(filepath.Join(lower.request.Workspace.Root, filepath.Clean(lower.request.Workspace.WorkingDirectory)))
	if err != nil {
		return nil, nil, fmt.Errorf("resolve test projection workdir: %w", err)
	}
	modelDigest, err := semanticir.Digest(lower.model.Tests)
	if err != nil {
		return nil, nil, err
	}
	testIDs := make([]string, 0, len(lower.model.Tests))
	modelByID := make(map[string]semanticir.TestModel, len(lower.model.Tests))
	for _, test := range lower.model.Tests {
		testIDs = append(testIDs, test.ID)
		modelByID[test.ID] = test
	}
	sort.Strings(testIDs)
	predicate := semanticir.StaticTestPredicate(lower.model.Tests, wholeProvenance(lower.request, semanticir.TranslationTranslated))
	predicateDigest, err := semanticir.Digest(predicate)
	if err != nil {
		return nil, nil, err
	}
	projection := &semanticir.TestObservationProjection{
		Source: lower.request.Artifact, TestIDs: append([]string(nil), testIDs...), PredicateDigest: predicateDigest,
		Complete: true, Provenance: wholeProvenance(lower.request, semanticir.TranslationTranslated),
		Derivation: semanticir.CompilerDerivationEvidence{
			SourceDigest: lower.request.Artifact.Digest, WorkspaceTreeDigest: lower.request.Workspace.TreeDigest,
			Tool: lower.request.Translator, IRKind: semanticir.CompilerIRCPythonBytecode, IRDigest: result.CompilerIRDigest,
			Steps: []semanticir.ProbeStep{{
				ID: "compile-test-observation-ir", Kind: semanticir.ProbeStepRun, Tool: lower.request.Translator,
				Argv: []string{"-P", "-B", "-c", astBridge}, Stdin: append([]byte(nil), result.RawInput...), StdinDigest: semanticir.DigestBytes(result.RawInput),
				WorkingDirectory: workingDirectory, Environment: append([]semanticir.EnvironmentVariable(nil), lower.request.Workspace.Environment...), EnvironmentDigest: lower.request.Workspace.EnvironmentDigest,
				ClearEnvironment: lower.request.Workspace.ClearEnvironment, KillProcessGroup: lower.request.Workspace.KillProcessGroup, TimeoutMillis: 30000,
				ExpectedExitCode: 0, ExpectedStdoutDigest: semanticir.DigestBytes(result.RawOutput), ExpectedStderrDigest: semanticir.DigestBytes(nil), ExpectedSignalDigest: semanticir.DigestBytes(nil),
				SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone},
				Provenance:      wholeProvenance(lower.request, semanticir.TranslationTranslated),
			}},
			Output: append([]byte(nil), result.RawOutput...), OutputDigest: semanticir.DigestBytes(result.RawOutput), DecodedModelDigest: modelDigest, Complete: true,
		},
	}
	moduleNodes := result.CompilerIR["<module>"]
	if len(moduleNodes) == 0 {
		return nil, nil, fmt.Errorf("test module has no compiled module bytecode")
	}
	allBehaviorsByOperation := make(map[string]map[string]semanticir.BehaviorRef)
	for _, model := range lower.model.Tests {
		for _, behavior := range predicateBehaviorRefs(model.Predicate) {
			digest := semanticir.BehaviorRefKey(behavior)
			if allBehaviorsByOperation[behavior.OperationID] == nil {
				allBehaviorsByOperation[behavior.OperationID] = make(map[string]semanticir.BehaviorRef)
			}
			allBehaviorsByOperation[behavior.OperationID][digest] = behavior
		}
	}
	if len(lower.request.Groundings) != 0 {
		quantification, err := lower.testQuantificationEvidence(allBehaviorsByOperation)
		if err != nil {
			return nil, nil, err
		}
		projection.Quantification = quantification
	}
	importConstructIDs := make(map[string][]string)
	for importIndex, imported := range result.Imports {
		if len(imported.Names) == 0 {
			return nil, nil, fmt.Errorf("test module import %s is not an explicit operation binding", imported.Module)
		}
		var nodeIDs []string
		for _, node := range moduleNodes {
			if node.Line <= imported.Location.EndLine && node.EndLine >= imported.Location.Line {
				nodeIDs = append(nodeIDs, node.ID)
			}
		}
		if len(nodeIDs) == 0 {
			return nil, nil, fmt.Errorf("test module import %s has no compiled module bytecode", imported.Module)
		}
		for _, name := range imported.Names {
			behaviors := allBehaviorsByOperation[name]
			if len(behaviors) == 0 {
				return nil, nil, fmt.Errorf("test import %s.%s is not a modeled BehaviorRef dependency", imported.Module, name)
			}
			constructID := fmt.Sprintf("module#import-%d-%s", importIndex+1, name)
			constructProvenance := semanticir.NewProvenance(lower.request.Artifact, toLocation(lower.request, imported.Location), semanticir.TranslationTranslated)
			constructDigest, _ := semanticir.Digest(struct {
				Module string
				Name   string
				Nodes  []string
			}{imported.Module, name, nodeIDs})
			projection.Constructs = append(projection.Constructs, semanticir.TestConstructEvidence{
				ID: constructID, ArtifactID: lower.request.Artifact.ID, Kind: semanticir.TestConstructCall,
				Digest: constructDigest, IRKind: semanticir.CompilerIRCPythonBytecode, IRDigest: result.CompilerIRDigest,
				Tool: lower.request.Translator, CompilerNodeIDs: append([]string(nil), nodeIDs...), Provenance: constructProvenance,
			})
			digests := make([]string, 0, len(behaviors))
			for digest := range behaviors {
				digests = append(digests, digest)
			}
			sort.Strings(digests)
			for _, digest := range digests {
				behavior := behaviors[digest]
				projection.Dependencies = append(projection.Dependencies, semanticir.TestBehaviorDependency{
					ConstructID: constructID, Kind: semanticir.TestDependencyRead, Behavior: behavior, Inputs: cloneLiteralMap(behavior.Inputs),
					CompilerNodeIDs: append([]string(nil), nodeIDs...), Provenance: constructProvenance,
				})
			}
			importConstructIDs[name] = append(importConstructIDs[name], constructID)
		}
	}
	functionByName := indexFunctions(result.Functions)
	for _, testID := range testIDs {
		functionName, ok := pythonTestFunctionName(lower.request.Artifact.ID, testID)
		if !ok {
			return nil, nil, fmt.Errorf("test %s is not globally scoped to artifact %s", testID, lower.request.Artifact.ID)
		}
		function := functionByName[functionName]
		model := modelByID[testID]
		if function == nil || len(function.Parameters) != 0 {
			return nil, nil, fmt.Errorf("test %s uses fixtures/parameters or has no selected function", testID)
		}
		instances := lower.testInstances[testID]
		assertions := make([]*pyStatement, 0, len(instances))
		assertionPredicates := make([]semanticir.TestPredicate, 0, len(instances))
		for _, instance := range instances {
			assertions = append(assertions, instance.statement)
			assertionPredicates = append(assertionPredicates, instance.predicate)
		}
		if len(assertions) != len(model.Assertions) {
			return nil, nil, fmt.Errorf("test %s assertion count differs from the static predicate", testID)
		}
		if len(assertionPredicates) != len(assertions) {
			return nil, nil, fmt.Errorf("test %s predicate leaves differ from source assertions", testID)
		}
		// The CPython bridge keys code objects by their local function name.
		// Semantic IR test IDs are globally namespaced as artifact::function;
		// using the latter as a bytecode-map key incorrectly made every valid
		// selected test look uncompiled.
		nodes := result.CompilerIR[functionName]
		if len(nodes) == 0 {
			return nil, nil, fmt.Errorf("test %s has no compiled bytecode", testID)
		}
		allNodeIDs := make([]string, 0, len(moduleNodes)+len(nodes))
		for _, node := range moduleNodes {
			allNodeIDs = append(allNodeIDs, node.ID)
		}
		for _, node := range nodes {
			allNodeIDs = append(allNodeIDs, node.ID)
		}
		controlIDs := make(map[*pyStatement]string)
		controlNodeIDs := make(map[*pyStatement][]string)
		for _, instance := range instances {
			for _, statement := range instance.controls {
				if _, exists := controlIDs[statement]; exists {
					continue
				}
				var statementNodes []string
				for _, node := range nodes {
					if node.Line <= statement.Location.EndLine && node.EndLine >= statement.Location.Line {
						statementNodes = append(statementNodes, node.ID)
					}
				}
				if len(statementNodes) == 0 {
					return nil, nil, fmt.Errorf("test %s %s control has no compiled bytecode", testID, statement.Kind)
				}
				constructID := fmt.Sprintf("%s#control-%d", testID, len(controlIDs)+1)
				constructDigest, _ := semanticir.Digest(struct {
					Kind   string
					Source string
					Nodes  []string
				}{statement.Kind, statement.Source, statementNodes})
				constructProvenance := semanticir.NewProvenance(lower.request.Artifact, toLocation(lower.request, statement.Location), semanticir.TranslationTranslated)
				projection.Constructs = append(projection.Constructs, semanticir.TestConstructEvidence{
					ID: constructID, ArtifactID: lower.request.Artifact.ID, Kind: semanticir.TestConstructControl,
					Digest: constructDigest, IRKind: semanticir.CompilerIRCPythonBytecode, IRDigest: result.CompilerIRDigest,
					Tool: lower.request.Translator, CompilerNodeIDs: statementNodes, Provenance: constructProvenance,
				})
				controlIDs[statement] = constructID
				controlNodeIDs[statement] = statementNodes
			}
		}
		assertionRoots := make([]string, 0, len(assertions))
		for assertionIndex, statement := range assertions {
			var nodeIDs []string
			callCount := 0
			for _, node := range nodes {
				if node.Line <= statement.Location.EndLine && node.EndLine >= statement.Location.Line {
					nodeIDs = append(nodeIDs, node.ID)
					if strings.Contains(node.Opcode, "CALL") {
						callCount++
					}
				}
			}
			behaviors := predicateBehaviorRefs(assertionPredicates[assertionIndex])
			if len(nodeIDs) == 0 || len(behaviors) == 0 || callCount < len(behaviors) {
				return nil, nil, fmt.Errorf("test %s assertion %d is not fully bound to call/assert bytecode", testID, assertionIndex+1)
			}
			constructID := fmt.Sprintf("%s#assertion-%d", testID, assertionIndex+1)
			constructDigest, _ := semanticir.Digest(struct {
				Source string
				Nodes  []string
			}{statement.Source, nodeIDs})
			constructProvenance := semanticir.NewProvenance(lower.request.Artifact, toLocation(lower.request, statement.Location), semanticir.TranslationTranslated)
			projection.Constructs = append(projection.Constructs, semanticir.TestConstructEvidence{
				ID: constructID, ArtifactID: lower.request.Artifact.ID, Kind: semanticir.TestConstructAssertion,
				Digest: constructDigest, IRKind: semanticir.CompilerIRCPythonBytecode, IRDigest: result.CompilerIRDigest,
				Tool: lower.request.Translator, CompilerNodeIDs: nodeIDs, Provenance: constructProvenance,
			})
			for _, behavior := range behaviors {
				projection.Dependencies = append(projection.Dependencies, semanticir.TestBehaviorDependency{
					ConstructID: constructID, Kind: semanticir.TestDependencyCall, Behavior: behavior, Inputs: cloneLiteralMap(behavior.Inputs),
					CompilerNodeIDs: append([]string(nil), nodeIDs...), Provenance: constructProvenance,
				})
			}
			constructIDs := []string{constructID}
			for _, control := range instances[assertionIndex].controls {
				controlID := controlIDs[control]
				constructIDs = append(constructIDs, controlID)
				controlProvenance := semanticir.NewProvenance(lower.request.Artifact, toLocation(lower.request, control.Location), semanticir.TranslationTranslated)
				for _, behavior := range behaviors {
					projection.Dependencies = append(projection.Dependencies, semanticir.TestBehaviorDependency{
						ConstructID: controlID, Kind: semanticir.TestDependencyRead, Behavior: behavior, Inputs: cloneLiteralMap(behavior.Inputs),
						CompilerNodeIDs: append([]string(nil), controlNodeIDs[control]...), Provenance: controlProvenance,
					})
				}
			}
			for _, behavior := range behaviors {
				constructIDs = append(constructIDs, importConstructIDs[behavior.OperationID]...)
			}
			constructIDs = compactOptionStrings(constructIDs)
			sort.Strings(constructIDs)
			counter := 0
			assertionRoots = append(assertionRoots, appendProjectionPredicate(
				projection, assertionPredicates[assertionIndex], testID, assertionIndex+1,
				&counter, nodeIDs, constructIDs,
			))
		}
		rootID := assertionRoots[0]
		if len(assertionRoots) > 1 {
			rootID = testID + "#pass"
			projection.Nodes = append(projection.Nodes, semanticir.TestProjectionNode{
				ID: rootID, Kind: semanticir.PredicateAnd, Children: assertionRoots,
				CompilerNodeIDs: allNodeIDs, Provenance: model.Predicate.Provenance,
			})
		}
		projection.PassRoots = append(projection.PassRoots, semanticir.TestPassRoot{
			TestID: testID, NodeID: rootID, CompilerNodeIDs: allNodeIDs,
		})
	}
	runner, err := lower.runnerSelection(testIDs, predicateDigest)
	if err != nil {
		return nil, nil, err
	}
	return projection, runner, nil
}

func (lower *lowering) testQuantificationEvidence(behaviorsByOperation map[string]map[string]semanticir.BehaviorRef) ([]semanticir.TestQuantificationEvidence, error) {
	type category struct {
		key      string
		behavior semanticir.BehaviorRef
		inputs   map[string]semanticir.Literal
	}
	byCategory := make(map[string]category)
	for operationID, behaviors := range behaviorsByOperation {
		operation, ok := taskOperation(lower.request.Operations, operationID)
		if !ok {
			return nil, fmt.Errorf("test behavior refers to undeclared operation %s", operationID)
		}
		for _, behavior := range behaviors {
			inputs, singleton := semanticir.ExactGroundingInputs(operation, lower.request.FiniteDomains, behavior.Conditions)
			if !singleton || !reflect.DeepEqual(inputs, behavior.Inputs) {
				return nil, fmt.Errorf("test behavior %s is not one exact singleton compiler-independent input point", operationID)
			}
			key, err := semanticir.Digest(struct {
				OperationID string
				Conditions  semanticir.Assignment
			}{operationID, behavior.Conditions})
			if err != nil {
				return nil, err
			}
			if prior, duplicate := byCategory[key]; duplicate {
				if !reflect.DeepEqual(prior.inputs, inputs) {
					return nil, fmt.Errorf("test category %s has more than one concrete input point", operationID)
				}
				continue
			}
			categoryBehavior := behavior
			categoryBehavior.Inputs = nil
			byCategory[key] = category{key: key, behavior: categoryBehavior, inputs: cloneLiteralMap(inputs)}
		}
	}
	keys := make([]string, 0, len(byCategory))
	for key := range byCategory {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	records := make([]semanticir.TestQuantificationEvidence, 0, len(keys))
	for _, key := range keys {
		item := byCategory[key]
		inputSet := []map[string]semanticir.Literal{cloneLiteralMap(item.inputs)}
		digest, err := semanticir.TestConcreteInputsDigest(inputSet)
		if err != nil {
			return nil, fmt.Errorf("test category concrete input digest: %w", err)
		}
		records = append(records, semanticir.TestQuantificationEvidence{
			Behavior: item.behavior, Kind: semanticir.TestQuantificationSingleton,
			ConcreteInputs: inputSet, ConcreteInputsDigest: digest,
			Result: semanticir.ProofProved, Provenance: item.behavior.Provenance,
		})
	}
	return records, nil
}

func appendProjectionPredicate(
	projection *semanticir.TestObservationProjection,
	predicate semanticir.TestPredicate,
	testID string,
	assertionIndex int,
	counter *int,
	compilerNodeIDs []string,
	constructIDs []string,
) string {
	*counter++
	id := fmt.Sprintf("%s#assertion-%d-predicate-%d", testID, assertionIndex, *counter)
	node := semanticir.TestProjectionNode{
		ID: id, Kind: predicate.Kind, Observe: predicate.Observe, Left: predicate.Left, Right: predicate.Right,
		CompilerNodeIDs: append([]string(nil), compilerNodeIDs...), ConstructIDs: append([]string(nil), constructIDs...),
		Provenance: predicate.Provenance,
	}
	for _, child := range predicate.Children {
		node.Children = append(node.Children, appendProjectionPredicate(
			projection, child, testID, assertionIndex, counter, compilerNodeIDs, constructIDs,
		))
	}
	projection.Nodes = append(projection.Nodes, node)
	return id
}

func predicateBehaviorRefs(predicate semanticir.TestPredicate) []semanticir.BehaviorRef {
	byDigest := make(map[string]semanticir.BehaviorRef)
	var visit func(semanticir.TestPredicate)
	visit = func(item semanticir.TestPredicate) {
		if item.Observe != nil && item.Observe.Behavior.OperationID != "" {
			digest := semanticir.BehaviorRefKey(item.Observe.Behavior)
			byDigest[digest] = item.Observe.Behavior
		}
		for _, behavior := range []*semanticir.BehaviorRef{item.Left, item.Right} {
			if behavior == nil || behavior.OperationID == "" {
				continue
			}
			digest := semanticir.BehaviorRefKey(*behavior)
			byDigest[digest] = *behavior
		}
		for _, child := range item.Children {
			visit(child)
		}
	}
	visit(predicate)
	digests := make([]string, 0, len(byDigest))
	for digest := range byDigest {
		digests = append(digests, digest)
	}
	sort.Strings(digests)
	result := make([]semanticir.BehaviorRef, 0, len(digests))
	for _, digest := range digests {
		result = append(result, byDigest[digest])
	}
	return result
}

func (lower *lowering) runnerSelection(testIDs []string, predicateDigest string) (*semanticir.RunnerSelectionEvidence, error) {
	request := lower.request
	if request.RunnerCommand == nil || request.Configuration == nil || request.Configuration.Kind != semanticir.ArtifactConfiguration {
		return nil, fmt.Errorf("frozen Python runner command/configuration is absent")
	}
	command := *request.RunnerCommand
	if request.Runner.Name == "" || request.Runner.Path == "" || command.Command == "" {
		return nil, fmt.Errorf("frozen Python runner identity/command is incomplete")
	}
	if request.Runner != request.Translator {
		return nil, fmt.Errorf("exact Python test runner must be the same digest-bound CPython tool used to derive bytecode")
	}
	if command.WorkspaceID != request.Workspace.ID || command.TreeDigest != request.Workspace.TreeDigest || command.State != request.Workspace.State {
		return nil, fmt.Errorf("Python runner command differs from the translated frozen workspace")
	}
	if command.WorkingDirectory != request.Workspace.WorkingDirectory {
		return nil, fmt.Errorf("Python runner command changes the frozen workspace working directory")
	}
	if _, err := exactWorkspaceEnvironment(semanticir.WorkspaceRef{
		Root: request.Workspace.Root, WorkingDirectory: command.WorkingDirectory,
		Environment: command.Environment, EnvironmentDigest: command.EnvironmentDigest,
		ClearEnvironment: command.ClearEnvironment, KillProcessGroup: command.KillProcessGroup,
	}); err != nil {
		return nil, fmt.Errorf("Python runner environment is not hermetic: %w", err)
	}
	disablePlugins := false
	for _, variable := range command.Environment {
		if variable.Name == "PYTEST_DISABLE_PLUGIN_AUTOLOAD" && variable.Value == "1" {
			disablePlugins = true
		}
		if variable.Name == "PYTEST_ADDOPTS" || variable.Name == "PYTEST_PLUGINS" {
			return nil, fmt.Errorf("Python runner uses implicit pytest options/plugins")
		}
	}
	if !disablePlugins {
		return nil, fmt.Errorf("Python runner does not disable ambient pytest plugin autoload")
	}
	configurationFound := false
	for _, entry := range request.Workspace.Entries {
		base := filepath.Base(filepath.Clean(entry.Path))
		if base == "conftest.py" || base == "sitecustomize.py" || base == "usercustomize.py" {
			return nil, fmt.Errorf("Python runner workspace contains unmodeled import/plugin hook %s", entry.Path)
		}
		if entry.Artifact != *request.Configuration {
			continue
		}
		content, err := readWorkspaceEntry(request.Workspace.Root, entry.Path)
		if err != nil {
			return nil, err
		}
		if semanticir.DigestBytes(content) != request.Configuration.Digest || len(content) != 0 {
			return nil, fmt.Errorf("Python runner configuration must be the exact frozen empty file")
		}
		configurationFound = true
	}
	if !configurationFound {
		return nil, fmt.Errorf("Python runner configuration is absent from the frozen workspace entries")
	}
	script, err := directPythonTestRunnerScript(request.Artifact.Path, request.Configuration.Path, testIDs)
	if err != nil {
		return nil, err
	}
	wantCommand := []string{request.Runner.Path, "-P", "-I", "-S", "-c", script}
	fields, err := parseCanonicalPythonRunnerCommand(command.Command)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(fields, wantCommand) {
		return nil, fmt.Errorf("Python runner does not select exactly the translated test IDs")
	}
	wantText, err := canonicalPythonRunnerCommand(wantCommand)
	if err != nil || command.Command != wantText {
		return nil, fmt.Errorf("Python runner command is not the canonical single-quoted direct invocation")
	}
	foundTool := false
	for _, tool := range command.Tools {
		foundTool = foundTool || tool == request.Runner
	}
	if !foundTool || command.PassSignal.Kind != semanticir.PassSignalExitCode || command.PassSignal.Expected != "0" {
		return nil, fmt.Errorf("Python runner command does not define conjunctive pytest exit success")
	}
	configurationProvenance := semanticir.NewProvenance(*request.Configuration, semanticir.SourceLocation{
		Path: request.Configuration.Path, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1,
	}, semanticir.TranslationTranslated)
	return &semanticir.RunnerSelectionEvidence{
		TestIDs: append([]string(nil), testIDs...), PredicateDigest: predicateDigest, Configuration: *request.Configuration,
		Verifier: request.Runner, Command: command, ConjunctivePass: true, Complete: true, Provenance: configurationProvenance,
	}, nil
}

// parseCanonicalPythonRunnerCommand implements a deliberately tiny shell
// grammar: safe unquoted atoms plus one or more POSIX single-quoted atoms.
// Quoted atoms are opaque, so Python punctuation cannot become shell syntax;
// the decoded argv is then compared byte-for-byte with the generated runner.
func parseCanonicalPythonRunnerCommand(command string) ([]string, error) {
	if command == "" || strings.ContainsAny(command, "\t\r\n\x00") {
		return nil, fmt.Errorf("Python runner command is not canonical POSIX token text")
	}
	var result []string
	for offset := 0; offset < len(command); {
		if command[offset] == ' ' {
			return nil, fmt.Errorf("Python runner command contains non-canonical spacing")
		}
		if command[offset] == '\'' {
			end := strings.IndexByte(command[offset+1:], '\'')
			if end < 0 {
				return nil, fmt.Errorf("Python runner command contains an unterminated quoted atom")
			}
			end += offset + 1
			if end == offset+1 || (end+1 < len(command) && command[end+1] != ' ') {
				return nil, fmt.Errorf("Python runner command contains a non-canonical quoted atom")
			}
			result = append(result, command[offset+1:end])
			offset = end + 1
		} else {
			end := strings.IndexByte(command[offset:], ' ')
			if end < 0 {
				end = len(command)
			} else {
				end += offset
			}
			atom := command[offset:end]
			for _, character := range atom {
				if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_./:+,@%-", character) {
					return nil, fmt.Errorf("Python runner command contains shell syntax outside a quoted atom")
				}
			}
			result = append(result, atom)
			offset = end
		}
		if offset == len(command) {
			break
		}
		offset++ // exactly one ASCII separator, checked at the next iteration
		if offset == len(command) || command[offset] == ' ' {
			return nil, fmt.Errorf("Python runner command contains non-canonical spacing")
		}
	}
	if len(result) != 6 {
		return nil, fmt.Errorf("Python runner command does not contain exactly six arguments")
	}
	return result, nil
}

func canonicalPythonRunnerCommand(argv []string) (string, error) {
	if len(argv) != 6 || strings.ContainsRune(argv[5], '\'') || argv[5] == "" {
		return "", fmt.Errorf("Python runner argv cannot be represented by the closed POSIX grammar")
	}
	for _, atom := range argv[:5] {
		if atom == "" {
			return "", fmt.Errorf("Python runner argv contains an empty shell atom")
		}
		for _, character := range atom {
			if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_./:+,@%-", character) {
				return "", fmt.Errorf("Python runner argv contains unsafe shell syntax")
			}
		}
	}
	return strings.Join(argv[:5], " ") + " '" + argv[5] + "'", nil
}

func directPythonTestRunnerScript(artifactPath, configurationPath string, testIDs []string) (string, error) {
	if artifactPath == "" || configurationPath == "" || strings.ContainsAny(artifactPath, " \\'\"\t\r\n") || strings.ContainsAny(configurationPath, " \\'\"\t\r\n") {
		return "", fmt.Errorf("Python test/configuration artifact path cannot be represented by the direct frozen runner")
	}
	var builder strings.Builder
	builder.WriteString("import pathlib,runpy,sys;assert pathlib.Path(")
	builder.WriteString(strconv.Quote(configurationPath))
	builder.WriteString(").read_bytes()==b\"\";sys.path.insert(0,\".\");m=runpy.run_path(")
	builder.WriteString(strconv.Quote(artifactPath))
	builder.WriteByte(')')
	for _, testID := range testIDs {
		_, localID, ok := strings.Cut(testID, "::")
		if !ok || !pythonExceptionName.MatchString(localID) {
			return "", fmt.Errorf("Python test ID cannot be represented by the direct frozen runner: %s", testID)
		}
		builder.WriteString(";m[")
		builder.WriteString(strconv.Quote(localID))
		builder.WriteString("]()")
	}
	return builder.String(), nil
}

func pythonTestID(artifactID, functionName string) string {
	return artifactID + "::" + functionName
}

func pythonTestFunctionName(artifactID, testID string) (string, bool) {
	prefix := artifactID + "::"
	if !strings.HasPrefix(testID, prefix) || !pythonExceptionName.MatchString(strings.TrimPrefix(testID, prefix)) {
		return "", false
	}
	return strings.TrimPrefix(testID, prefix), true
}

func collectTestAssertions(body []pyStatement, found *[]*pyStatement, conditional bool, lower *lowering) {
	for i := range body {
		statement := &body[i]
		if statement.Kind == "assert" || statement.Kind == "assert_raises" {
			if conditional {
				lower.unsupported("PY_CONDITIONAL_ASSERT", "conditional test assertions cannot describe one exact accepted outcome set", statement.Location)
				continue
			}
			*found = append(*found, statement)
		}
		if statement.Kind == "branch" {
			collectTestAssertions(statement.Body, found, true, lower)
			collectTestAssertions(statement.Alternate, found, true, lower)
		}
	}
}

func (lower *lowering) assertion(statement *pyStatement, operations map[string]semanticir.Operation) (semanticir.Assertion, semanticir.TestPredicate, bool) {
	provenance := semanticir.NewProvenance(lower.request.Artifact, toLocation(lower.request, statement.Location), semanticir.TranslationTranslated)
	if statement.Value == nil {
		lower.unsupported("PY_EMPTY_ASSERT", "assertion has no predicate", statement.Location)
		return semanticir.Assertion{}, semanticir.TestPredicate{}, false
	}
	if statement.Kind == "assert_raises" {
		call, ok := lower.expression(statement.Value)
		if !ok || call == nil || call.Kind != semanticir.ExprCall {
			return semanticir.Assertion{}, semanticir.TestPredicate{}, false
		}
		conditions, inputs, ok := lower.conditionsForCall(call, operations)
		if !ok {
			lower.unsupported("PY_ASSERT_CONDITIONS", "pytest.raises call arguments do not select exact declared domain values", statement.Location)
			return semanticir.Assertion{}, semanticir.TestPredicate{}, false
		}
		behavior := semanticir.BehaviorRef{OperationID: call.Name, Conditions: conditions, Inputs: inputs, Provenance: provenance}
		observation := semanticir.Observation{
			Kind: semanticir.ObserveRaise, Behavior: behavior, ExceptionType: statement.ExceptionType,
			Message: statement.Message, Provenance: provenance,
		}
		return semanticir.Assertion{
			Kind: semanticir.AssertRaises, Actual: call, ExceptionType: statement.ExceptionType,
			Message: statement.Message, Provenance: provenance,
		}, semanticir.TestPredicate{Kind: semanticir.PredicateRaises, Observe: &observation, Provenance: provenance}, true
	}
	predicate, ok := lower.expression(statement.Value)
	if !ok || predicate == nil {
		return semanticir.Assertion{}, semanticir.TestPredicate{}, false
	}
	assertion := semanticir.Assertion{Kind: semanticir.AssertTrue, Actual: predicate, Provenance: provenance}
	if predicate.Kind == semanticir.ExprCompare && len(predicate.Operands) == 2 && (predicate.Operator == semanticir.OpEQ || predicate.Operator == semanticir.OpNE) {
		actual := predicate.Operands[0]
		want := predicate.Operands[1]
		if actual.Kind != semanticir.ExprCall && want.Kind == semanticir.ExprCall {
			actual, want = want, actual
		}
		assertion.Actual, assertion.Expected = &actual, &want
		if predicate.Operator == semanticir.OpEQ {
			assertion.Kind = semanticir.AssertEqual
		} else {
			assertion.Kind = semanticir.AssertNotEqual
		}
	} else if predicate.Kind == semanticir.ExprUnary && predicate.Operator == semanticir.OpNot && len(predicate.Operands) == 1 {
		actual := predicate.Operands[0]
		assertion.Kind, assertion.Actual = semanticir.AssertFalse, &actual
	}
	global, ok := lower.testPredicate(predicate, operations, provenance)
	if !ok {
		lower.unsupported("PY_UNSUPPORTED_TEST_RELATION", "assertion cannot be expressed as an exact global behavior predicate", statement.Location)
		return semanticir.Assertion{}, semanticir.TestPredicate{}, false
	}
	if behavior, accepted, unary := unaryPredicateMetadata(global); unary {
		assertion.OutcomeIDs = append([]string(nil), accepted...)
		_ = behavior
	}
	return assertion, global, true
}

func (lower *lowering) testPredicate(expression *semanticir.Expression, operations map[string]semanticir.Operation, provenance semanticir.Provenance) (semanticir.TestPredicate, bool) {
	if expression == nil {
		return semanticir.TestPredicate{}, false
	}
	if expression.Kind == semanticir.ExprBool && (expression.Operator == semanticir.OpAnd || expression.Operator == semanticir.OpOr) {
		kind := semanticir.PredicateAnd
		if expression.Operator == semanticir.OpOr {
			kind = semanticir.PredicateOr
		}
		result := semanticir.TestPredicate{Kind: kind, Provenance: provenance}
		for i := range expression.Operands {
			child, ok := lower.testPredicate(&expression.Operands[i], operations, expression.Operands[i].Provenance)
			if !ok {
				return semanticir.TestPredicate{}, false
			}
			result.Children = append(result.Children, child)
		}
		return result, len(result.Children) != 0
	}
	if expression.Kind == semanticir.ExprUnary && expression.Operator == semanticir.OpNot && len(expression.Operands) == 1 {
		child, ok := lower.testPredicate(&expression.Operands[0], operations, expression.Operands[0].Provenance)
		if !ok {
			return semanticir.TestPredicate{}, false
		}
		return semanticir.TestPredicate{Kind: semanticir.PredicateNot, Children: []semanticir.TestPredicate{child}, Provenance: provenance}, true
	}
	if expression.Kind == semanticir.ExprCompare && len(expression.Operands) == 2 && (expression.Operator == semanticir.OpEQ || expression.Operator == semanticir.OpNE) {
		left, right := &expression.Operands[0], &expression.Operands[1]
		leftCall, rightCall := directCall(left), directCall(right)
		if leftCall != nil && rightCall != nil {
			leftRef, ok := lower.behaviorRef(leftCall, operations)
			if !ok {
				return semanticir.TestPredicate{}, false
			}
			rightRef, ok := lower.behaviorRef(rightCall, operations)
			if !ok {
				return semanticir.TestPredicate{}, false
			}
			leaf := semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeEqual, Left: &leftRef, Right: &rightRef, Provenance: provenance}
			if expression.Operator == semanticir.OpNE {
				return semanticir.TestPredicate{Kind: semanticir.PredicateNot, Children: []semanticir.TestPredicate{leaf}, Provenance: provenance}, true
			}
			return leaf, true
		}
		if leftCall == nil && rightCall != nil {
			left, right, leftCall = right, left, rightCall
		}
		if leftCall == nil || right.Kind != semanticir.ExprLiteral || right.Literal == nil {
			return semanticir.TestPredicate{}, false
		}
		behavior, ok := lower.behaviorRef(leftCall, operations)
		if !ok {
			return semanticir.TestPredicate{}, false
		}
		outcomes := []string{semanticir.OutcomeID(semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: right.Literal, OperationID: leftCall.Name})}
		observation := semanticir.Observation{Kind: semanticir.ObserveOutcome, Behavior: behavior, OutcomeIDs: outcomes, Provenance: provenance}
		leaf := semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeIn, Observe: &observation, Provenance: provenance}
		if expression.Operator == semanticir.OpNE {
			return semanticir.TestPredicate{Kind: semanticir.PredicateNot, Children: []semanticir.TestPredicate{leaf}, Provenance: provenance}, true
		}
		return leaf, true
	}
	if call := directCall(expression); call != nil {
		behavior, ok := lower.behaviorRef(call, operations)
		if !ok {
			return semanticir.TestPredicate{}, false
		}
		literal := semanticir.Literal{Type: semanticir.TypeBool, Bool: true}
		outcomes := []string{semanticir.OutcomeID(semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &literal, OperationID: call.Name})}
		observation := semanticir.Observation{Kind: semanticir.ObserveOutcome, Behavior: behavior, OutcomeIDs: outcomes, Provenance: provenance}
		return semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeIn, Observe: &observation, Provenance: provenance}, true
	}
	return semanticir.TestPredicate{}, false
}

func directCall(expression *semanticir.Expression) *semanticir.Expression {
	if expression != nil && expression.Kind == semanticir.ExprCall {
		return expression
	}
	return nil
}

func (lower *lowering) behaviorRef(call *semanticir.Expression, operations map[string]semanticir.Operation) (semanticir.BehaviorRef, bool) {
	conditions, inputs, ok := lower.conditionsForCall(call, operations)
	if !ok {
		return semanticir.BehaviorRef{}, false
	}
	return semanticir.BehaviorRef{OperationID: call.Name, Conditions: conditions, Inputs: inputs, Provenance: call.Provenance}, true
}

func unaryPredicateMetadata(predicate semanticir.TestPredicate) (semanticir.BehaviorRef, []string, bool) {
	switch predicate.Kind {
	case semanticir.PredicateOutcomeIn:
		if predicate.Observe == nil {
			return semanticir.BehaviorRef{}, nil, false
		}
		return predicate.Observe.Behavior, append([]string(nil), predicate.Observe.OutcomeIDs...), true
	case semanticir.PredicateRaises:
		if predicate.Observe == nil {
			return semanticir.BehaviorRef{}, nil, false
		}
		return predicate.Observe.Behavior, append([]string(nil), predicate.Observe.OutcomeIDs...), true
	case semanticir.PredicateOr:
		var behavior semanticir.BehaviorRef
		var outcomes []string
		for i, child := range predicate.Children {
			candidate, accepted, ok := unaryPredicateMetadata(child)
			if !ok {
				return semanticir.BehaviorRef{}, nil, false
			}
			if i == 0 {
				behavior = candidate
			} else if semanticir.BehaviorRefKey(behavior) != semanticir.BehaviorRefKey(candidate) {
				return semanticir.BehaviorRef{}, nil, false
			}
			outcomes = append(outcomes, accepted...)
		}
		sort.Strings(outcomes)
		outcomes = compactStrings(outcomes)
		return behavior, outcomes, len(predicate.Children) != 0
	}
	return semanticir.BehaviorRef{}, nil, false
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func findCall(expression *semanticir.Expression) *semanticir.Expression {
	if expression == nil {
		return nil
	}
	if expression.Kind == semanticir.ExprCall {
		return expression
	}
	for i := range expression.Operands {
		if call := findCall(&expression.Operands[i]); call != nil {
			return call
		}
	}
	return nil
}

func sequenceElementType(expression *semanticir.Expression) semanticir.ValueType {
	if expression == nil {
		return semanticir.TypeUnknown
	}
	if (expression.Kind == semanticir.ExprSequence || expression.Kind == semanticir.ExprLiteral) && len(expression.Operands) != 0 {
		want := expression.Operands[0].Type
		for _, operand := range expression.Operands[1:] {
			if operand.Type != want {
				return semanticir.TypeUnknown
			}
		}
		return want
	}
	return semanticir.TypeUnknown
}

func (lower *lowering) conditionsForCall(call *semanticir.Expression, operations map[string]semanticir.Operation) (semanticir.Assignment, map[string]semanticir.Literal, bool) {
	operation, local := operations[call.Name]
	if !local {
		for _, declared := range lower.request.Operations {
			if declared.ID == call.Name {
				operation, local = declared, true
				break
			}
		}
	}
	domainIDs := make([]string, 0, len(call.Operands))
	if local {
		for index, input := range operation.Inputs {
			domainID := input.DomainID
			if domainID == "" && index < len(operation.DomainIDs) {
				domainID = operation.DomainIDs[index]
			}
			if domainID == "" {
				return nil, nil, false
			}
			domainIDs = append(domainIDs, domainID)
		}
	} else {
		return nil, nil, false
	}
	if len(domainIDs) != len(call.Operands) {
		return nil, nil, false
	}
	conditions := make(semanticir.Assignment, len(domainIDs))
	inputs := make(map[string]semanticir.Literal, len(domainIDs))
	for i, operand := range call.Operands {
		if operand.Kind != semanticir.ExprLiteral || operand.Literal == nil {
			return nil, nil, false
		}
		domain := findDomain(lower.request.FiniteDomains, domainIDs[i])
		if domain == nil {
			return nil, nil, false
		}
		inputName := ""
		if i < len(operation.Inputs) {
			inputName = operation.Inputs[i].Name
			if semanticir.ValidValueType(operation.Inputs[i].Type) && operation.Inputs[i].Type != operand.Literal.Type {
				return nil, nil, false
			}
		}
		if inputName == "" {
			return nil, nil, false
		}
		valueID, ok := domainValueIDForOperationLiteral(*domain, *operand.Literal, operation.ID, inputName)
		if !ok {
			return nil, nil, false
		}
		conditions[domainIDs[i]] = valueID
		inputs[inputName] = *operand.Literal
	}
	if len(lower.request.Groundings) != 0 {
		exact, ok := exactPythonAssignmentInputs(lower.request, operation, conditions)
		if !ok || !reflect.DeepEqual(exact, inputs) {
			return nil, nil, false
		}
	}
	return conditions, inputs, true
}

func (lower *lowering) enumerateConcrete(ctx context.Context, functions []pyFunction, declarations []pyDeclaration, callEdges []pyCallEdge, operations map[string]semanticir.Operation) {
	// A Spec constraint is not evidence that the corresponding Python path is
	// unreachable. Until the CPython graph proves each excluded point, using
	// those constraints to prune execution would make reference translation
	// depend on R. Fail closed instead.
	if len(lower.request.Constraints) != 0 {
		lower.unsupported("PY_CONSTRAINT_REACHABILITY", "Spec-excluded assignments require independent CPython path-unreachability evidence", wholeBridgeLocation(lower.request))
		return
	}
	module := strings.TrimSpace(lower.request.Options["python.module"])
	if module == "" {
		lower.unsupported("PY_CONCRETE_CONFIG", "python.execution=exhaustive requires python.module", wholeBridgeLocation(lower.request))
		return
	}
	packageRoot, packageErr := normalizedPackageRoot(lower.request)
	if packageErr != nil {
		lower.unsupported("PY_CONCRETE_CONFIG", packageErr.Error(), wholeBridgeLocation(lower.request))
		return
	}
	rootFromWorkDir, rootErr := filepath.Rel(filepath.Clean(lower.request.Workspace.WorkingDirectory), ".")
	if rootErr != nil || filepath.IsAbs(rootFromWorkDir) {
		lower.unsupported("PY_CONCRETE_CONFIG", "workspace root cannot be represented relative to the frozen working directory", wholeBridgeLocation(lower.request))
		return
	}

	functionIndex := indexFunctions(functions)
	bindings := make(map[string]concreteBinding)
	var cases []concreteCase
	caseNumber := 0
	operationIDs := make([]string, 0, len(operations))
	for operationID := range operations {
		operationIDs = append(operationIDs, operationID)
	}
	sort.Strings(operationIDs)
	for _, operationID := range operationIDs {
		operation := operations[operationID]
		if operation.Kind == semanticir.OperationTest {
			continue
		}
		function := functionIndex[operation.ID]
		if function == nil {
			lower.unsupported("PY_CONCRETE_OPERATION", "declared operation "+operation.ID+" is not a selected module-level function", wholeBridgeLocation(lower.request))
			continue
		}
		if len(operation.Inputs) != len(function.Parameters) {
			lower.unsupported("PY_CONCRETE_SIGNATURE", "declared operation inputs do not match Python positional parameters for "+operation.ID, function.Location)
			continue
		}
		localDomains := make([]semanticir.Domain, 0, len(operation.Inputs))
		for _, input := range operation.Inputs {
			domain := findDomain(lower.request.FiniteDomains, input.DomainID)
			if domain == nil {
				lower.unsupported("PY_DOMAIN_ENUMERATION", "operation input references missing domain "+input.DomainID, function.Location)
				localDomains = nil
				break
			}
			localDomains = append(localDomains, *domain)
		}
		if localDomains == nil {
			continue
		}
		assignments, err := enumerateAssignments(localDomains, maxCases(lower.request))
		if err != nil {
			lower.unsupported("PY_DOMAIN_ENUMERATION", err.Error(), function.Location)
			continue
		}
		for _, assignment := range assignments {
			exactInputs, exact := exactPythonAssignmentInputs(lower.request, operation, assignment)
			if !exact {
				lower.unsupported("PY_TYPED_DOMAIN", "domain assignment "+semanticir.AssignmentGroundingID(operation.ID, assignment)+" has no exact outcome-independent Python input grounding", function.Location)
				continue
			}
			item := concreteCase{Operation: operation.ID}
			valid := true
			for _, input := range operation.Inputs {
				literal, ok := exactInputs[input.Name]
				if !ok || literal.Type != input.Type {
					valid = false
					break
				}
				item.Arguments = append(item.Arguments, literal)
				item.Constructors = append(item.Constructors, "")
			}
			if !valid {
				continue
			}
			caseNumber++
			item.ID = fmt.Sprintf("%s#concrete-%d", operation.ID, caseNumber)
			cases = append(cases, item)
			bindings[item.ID] = concreteBinding{operation: operation, assignment: cloneAssignment(assignment), inputs: cloneLiteralMap(exactInputs), function: function}
		}
	}
	if semanticir.HasErrors(lower.diagnostics) {
		return
	}
	if len(cases) == 0 {
		lower.unsupported("PY_EMPTY_EXECUTION", "exhaustive execution has no reachable declared assignments", wholeBridgeLocation(lower.request))
		return
	}

	request := concreteRequest{
		Root: filepath.ToSlash(rootFromWorkDir), PackageRoot: packageRoot, Module: module,
		SourcePath: lower.request.Artifact.Path, Cases: cases,
	}
	for _, declaration := range declarations {
		request.Declarations = append(request.Declarations, declaration.Name)
	}
	sort.Strings(request.Declarations)
	for _, operationID := range operationIDs {
		if operations[operationID].Kind != semanticir.OperationTest {
			request.Operations = append(request.Operations, operationID)
		}
	}
	forward, err := runConcrete(ctx, lower.request, request)
	if err != nil {
		lower.unsupported("PY_CONCRETE_EXECUTION", err.Error(), wholeBridgeLocation(lower.request))
		return
	}
	request.Reverse = true
	reverse, err := runConcrete(ctx, lower.request, request)
	if err != nil {
		lower.unsupported("PY_CONCRETE_EXECUTION", "repeat run failed: "+err.Error(), wholeBridgeLocation(lower.request))
		return
	}
	if !sameConcreteResponseSemantics(forward, reverse) || forward.BytecodeDigest == "" {
		lower.unsupported("PY_NONDETERMINISTIC", "fresh forward/reverse exhaustive runs or compiled bytecode differ", wholeBridgeLocation(lower.request))
		return
	}
	if len(forward.Results) != len(cases) {
		lower.unsupported("PY_INCOMPLETE_EXECUTION", "interpreter did not return exactly one result for every reachable assignment", wholeBridgeLocation(lower.request))
		return
	}

	seenResult := make(map[string]struct{}, len(forward.Results))
	for index, result := range forward.Results {
		binding, ok := bindings[result.ID]
		if !ok {
			lower.unsupported("PY_UNDECLARED_RESULT", "interpreter returned an unknown case ID", wholeBridgeLocation(lower.request))
			return
		}
		if _, duplicate := seenResult[result.ID]; duplicate || result.Line <= 0 {
			lower.unsupported("PY_INVALID_RESULT", "interpreter returned duplicate or unlocated case evidence", binding.function.Location)
			return
		}
		seenResult[result.ID] = struct{}{}
		location := binding.function.Location
		provenance := semanticir.NewProvenance(lower.request.Artifact, toLocation(lower.request, location), semanticir.TranslationTranslated)
		raw, rawErr := rawTraceFromConcreteResult(result)
		if rawErr != nil {
			lower.unsupported("PY_INVALID_RESULT", rawErr.Error(), location)
			return
		}
		lower.model.RawReferenceCases = append(lower.model.RawReferenceCases, semanticir.RawReferenceCase{
			ID: fmt.Sprintf("%s#case-%d", binding.operation.ID, index+1), Conditions: binding.assignment,
			OperationID: binding.operation.ID, Inputs: cloneLiteralMap(binding.inputs), Outcomes: []semanticir.RawOutcomeTrace{raw}, Provenance: provenance,
		})
	}
	var normalizationDiagnostics []semanticir.Diagnostic
	lower.model.Cases, normalizationDiagnostics = semanticir.NormalizeReferenceCases(lower.request, lower.model.RawReferenceCases)
	if semanticir.HasErrors(normalizationDiagnostics) {
		for _, item := range normalizationDiagnostics {
			lower.unsupported("PY_REFERENCE_NORMALIZATION", item.Message, wholeBridgeLocation(lower.request))
		}
		return
	}
	evidence, evidenceErr := concreteExhaustiveEvidence(ctx, lower.request, lower.model.Cases, cases, bindings, forward, reverse)
	if evidenceErr != nil {
		lower.unsupported("PY_EXECUTION_EVIDENCE", evidenceErr.Error(), wholeBridgeLocation(lower.request))
		return
	}
	lower.model.ExhaustiveEvidence = []semanticir.ExhaustiveExecutionEvidence{evidence}
	closure, closureErr := concreteScopeClosure(ctx, lower.request, functions, declarations, operations, forward, evidence)
	if closureErr != nil {
		lower.unsupported("PY_SCOPE_CLOSURE", closureErr.Error(), wholeBridgeLocation(lower.request))
		return
	}
	lower.model.ScopeClosure = closure
}

func concreteScopeClosure(ctx context.Context, request semanticir.FrontendRequest, functions []pyFunction, moduleDeclarations []pyDeclaration, operations map[string]semanticir.Operation, response concreteResponse, execution semanticir.ExhaustiveExecutionEvidence) (*semanticir.ScopeClosureEvidence, error) {
	if len(request.ChangedRanges) == 0 {
		return nil, fmt.Errorf("exact changed source ranges are required")
	}
	// This frontend currently proves only the closed no-call patch slice.  A
	// CALL opcode or another Python source declaration can introduce a caller
	// edge that exhaustive input observations do not establish; such a slice
	// is rejected instead of asserting an incomplete graph.
	for _, entry := range request.Workspace.Entries {
		if entry.Artifact == request.Artifact || entry.Artifact.Kind == semanticir.ArtifactTests || !strings.HasSuffix(entry.Path, ".py") {
			continue
		}
		return nil, fmt.Errorf("frozen Python source %s is outside the compiler-derived no-call closure", entry.Path)
	}
	if len(moduleDeclarations) != len(operations) {
		return nil, fmt.Errorf("focused module declares %d top-level callables but proof scope owns %d operations", len(moduleDeclarations), len(operations))
	}
	functionByName := indexFunctions(functions)
	declarationByName := make(map[string]pyDeclaration, len(moduleDeclarations))
	for _, declaration := range moduleDeclarations {
		if declaration.Name == "" {
			return nil, fmt.Errorf("focused module contains an unnamed declaration")
		}
		if _, duplicate := declarationByName[declaration.Name]; duplicate {
			return nil, fmt.Errorf("focused module repeats declaration %s", declaration.Name)
		}
		declarationByName[declaration.Name] = declaration
	}
	moduleName := strings.TrimSpace(request.Options["python.module"])
	closure := &semanticir.ScopeClosureEvidence{
		SourceArtifacts: []semanticir.ArtifactRef{request.Artifact}, WorkspaceTreeDigest: request.Workspace.TreeDigest,
		Compiler: request.Translator, Prover: request.Prover, CompilerIRDigest: execution.EmittedIRDigest,
		ChangedRanges: append([]semanticir.ChangedSourceRange(nil), request.ChangedRanges...),
		Completeness:  semanticir.ProofProved, Complete: true,
		Provenance: wholeProvenance(request, semanticir.TranslationTranslated),
	}
	var allNodes []string
	for operationID, operation := range operations {
		if operation.Kind == semanticir.OperationTest {
			continue
		}
		function := functionByName[operationID]
		declaration, declared := declarationByName[operationID]
		if function == nil || !declared {
			return nil, fmt.Errorf("operation %s has no unique module declaration", operationID)
		}
		if pythonFunctionHasCall(function) {
			return nil, fmt.Errorf("operation %s contains a call and requires a compiler-derived resolved caller graph", operationID)
		}
		nodes := append([]string(nil), response.CompilerNodes[operationID]...)
		opcodes := append([]string(nil), response.CompilerOpcodes[operationID]...)
		sort.Strings(nodes)
		if len(nodes) == 0 || len(opcodes) != len(nodes) {
			return nil, fmt.Errorf("operation %s has no complete disassembly node set", operationID)
		}
		for _, opcode := range opcodes {
			if pythonOpcodeHasUnmodeledState(opcode) {
				return nil, fmt.Errorf("operation %s bytecode %s can read or mutate unmodeled state", operationID, opcode)
			}
		}
		allNodes = append(allNodes, nodes...)
		location := toLocation(request, declaration.Location)
		changed := false
		for _, changedRange := range request.ChangedRanges {
			if changedRange.ArtifactID != request.Artifact.ID || changedRange.Path != request.Artifact.Path {
				return nil, fmt.Errorf("changed range is outside the focused Python artifact")
			}
			if location.StartLine <= changedRange.EndLine && location.EndLine >= changedRange.StartLine {
				changed = true
			}
		}
		if !changed {
			return nil, fmt.Errorf("operation %s is outside the exact changed declaration slice", operationID)
		}
		declarationID := request.Artifact.ID + "::" + operationID
		declarationProvenance := semanticir.NewProvenance(request.Artifact, location, semanticir.TranslationTranslated)
		closure.Declarations = append(closure.Declarations, semanticir.CompilerDeclaration{
			ID: declarationID, QualifiedName: strings.TrimPrefix(moduleName+"."+operationID, "."), Artifact: request.Artifact,
			Location: location, CompilerNodeIDs: nodes, Changed: true, Provenance: declarationProvenance,
		})
		closure.ImpactedDeclarationIDs = append(closure.ImpactedDeclarationIDs, declarationID)
		closure.OperationOwners = append(closure.OperationOwners, semanticir.OperationOwner{OperationID: operationID, DeclarationID: declarationID})
	}
	sort.Strings(closure.ImpactedDeclarationIDs)
	sort.Slice(closure.Declarations, func(i, j int) bool { return closure.Declarations[i].ID < closure.Declarations[j].ID })
	sort.Slice(closure.OperationOwners, func(i, j int) bool {
		return closure.OperationOwners[i].OperationID < closure.OperationOwners[j].OperationID
	})
	allNodes = compactOptionStrings(allNodes)
	sort.Strings(allNodes)
	graphDigest, err := semanticir.ScopeClosureGraphDigest(*closure)
	if err != nil {
		return nil, fmt.Errorf("digest patch-scope graph: %w", err)
	}
	sourceDigest, err := semanticir.Digest(closure.SourceArtifacts)
	if err != nil {
		return nil, fmt.Errorf("digest patch-scope sources: %w", err)
	}
	scope := semanticir.CompilerPredicate{
		Logic: semanticir.ProofLogicSMTLIB2, Formula: []byte("false"), Tool: request.Translator,
		IRDigest: execution.EmittedIRDigest, CompilerNodeIDs: allNodes,
	}
	scope.DeclarationsDigest = semanticir.DigestBytes(scope.Declarations)
	scope.FormulaDigest = semanticir.DigestBytes(scope.Formula)
	proofContext := semanticir.CompilerProofContext{
		SourceDigest: sourceDigest, WorkspaceTreeDigest: request.Workspace.TreeDigest, EmittedIRDigest: execution.EmittedIRDigest,
		HarnessDigest: graphDigest, Compiler: request.Translator,
	}
	proof, err := runPythonProof(ctx, request, semanticir.NewProofClaim(semanticir.ClaimScopeClosure, proofContext, scope, nil, nil))
	if err != nil || proof.Result != semanticir.SolverUNSAT {
		return nil, fmt.Errorf("replay patch-scope closure proof: %v", err)
	}
	closure.CompletenessProof = proof
	return closure, nil
}

func pythonOpcodeHasUnmodeledState(opcode string) bool {
	switch opcode {
	case "LOAD_GLOBAL", "LOAD_NAME", "LOAD_DEREF", "LOAD_CLASSDEREF", "LOAD_ATTR",
		"STORE_GLOBAL", "STORE_NAME", "STORE_DEREF", "STORE_ATTR", "STORE_SUBSCR",
		"DELETE_GLOBAL", "DELETE_NAME", "DELETE_DEREF", "DELETE_ATTR", "DELETE_SUBSCR",
		"IMPORT_NAME", "IMPORT_FROM", "IMPORT_STAR", "LOAD_BUILD_CLASS",
		"RETURN_GENERATOR", "YIELD_VALUE", "YIELD_FROM", "SEND", "GET_AWAITABLE",
		"BEFORE_WITH", "WITH_EXCEPT_START", "BEFORE_ASYNC_WITH", "GET_AITER", "GET_ANEXT":
		return true
	default:
		return false
	}
}

func pythonFunctionHasCall(function *pyFunction) bool {
	var expressionHasCall func(*pyExpression) bool
	expressionHasCall = func(expression *pyExpression) bool {
		if expression == nil {
			return false
		}
		if expression.Kind == "call" {
			return true
		}
		for index := range expression.Args {
			if expressionHasCall(&expression.Args[index]) {
				return true
			}
		}
		return false
	}
	var statementsHaveCall func([]pyStatement) bool
	statementsHaveCall = func(statements []pyStatement) bool {
		for index := range statements {
			statement := &statements[index]
			if statement.Kind == "call" || expressionHasCall(statement.Value) || statementsHaveCall(statement.Body) || statementsHaveCall(statement.Alternate) {
				return true
			}
			for _, catch := range statement.Catches {
				if statementsHaveCall(catch.Body) {
					return true
				}
			}
		}
		return false
	}
	return statementsHaveCall(function.Body)
}

func runPythonProof(ctx context.Context, request semanticir.FrontendRequest, claim semanticir.ProofClaim) (semanticir.ReplayableProof, error) {
	query, err := semanticir.CanonicalProofQuery(claim)
	if err != nil {
		return semanticir.ReplayableProof{}, err
	}
	proof := semanticir.ReplayableProof{
		Claim: claim, Logic: semanticir.ProofLogicSMTLIB2, Query: query, QueryDigest: semanticir.DigestBytes(query),
		Prover: request.Prover, Argv: []string{"-in", "-smt2"}, WorkingDirectory: "/",
		Environment: append([]semanticir.EnvironmentVariable(nil), request.Workspace.Environment...), EnvironmentDigest: request.Workspace.EnvironmentDigest,
		ClearEnvironment: request.Workspace.ClearEnvironment, KillProcessGroup: request.Workspace.KillProcessGroup, TimeoutMillis: 10000,
		SubjectDigests: semanticir.ProofClaimSubjectDigests(claim),
	}
	proofContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	command := exec.CommandContext(proofContext, request.Prover.Path, proof.Argv...)
	command.Stdin = bytes.NewReader(query)
	if err := configureWorkspaceCommand(command, request.Workspace); err != nil {
		return semanticir.ReplayableProof{}, err
	}
	command.Dir = "/"
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		if proofContext.Err() != nil {
			return semanticir.ReplayableProof{}, proofContext.Err()
		}
		return semanticir.ReplayableProof{}, fmt.Errorf("proof replay failed: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stderr.Len() != 0 {
		return semanticir.ReplayableProof{}, fmt.Errorf("proof replay emitted stderr: %s", strings.TrimSpace(stderr.String()))
	}
	proof.SolverOutput = append([]byte(nil), stdout.Bytes()...)
	proof.SolverOutputDigest = semanticir.DigestBytes(proof.SolverOutput)
	fields := strings.Fields(stdout.String())
	if len(fields) == 0 {
		return semanticir.ReplayableProof{}, fmt.Errorf("proof replay returned empty output")
	}
	proof.Result = semanticir.SolverResult(fields[0])
	return proof, nil
}

func concreteExhaustiveEvidence(ctx context.Context, request semanticir.FrontendRequest, behaviorCases []semanticir.BehaviorCase, cases []concreteCase, bindings map[string]concreteBinding, forward, reverse concreteResponse) (semanticir.ExhaustiveExecutionEvidence, error) {
	provenance := wholeProvenance(request, semanticir.TranslationTranslated)
	workingDirectory, err := filepath.EvalSymlinks(filepath.Join(request.Workspace.Root, filepath.Clean(request.Workspace.WorkingDirectory)))
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("resolve execution working directory: %w", err)
	}
	packageRoot, err := normalizedPackageRoot(request)
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, err
	}
	rootFromWorkDir, err := filepath.Rel(filepath.Clean(request.Workspace.WorkingDirectory), ".")
	if err != nil || filepath.IsAbs(rootFromWorkDir) {
		return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("represent workspace root relative to working directory")
	}
	caseByID := make(map[string]concreteCase, len(cases))
	operationSet := make(map[string]struct{})
	for _, item := range cases {
		caseByID[item.ID] = item
		operationSet[item.Operation] = struct{}{}
	}
	concreteOperations := make([]string, 0, len(operationSet))
	for operationID := range operationSet {
		concreteOperations = append(concreteOperations, operationID)
	}
	sort.Strings(concreteOperations)
	concreteDeclarations := make([]string, 0, len(forward.CompilerNodes))
	for declaration := range forward.CompilerNodes {
		concreteDeclarations = append(concreteDeclarations, declaration)
	}
	sort.Strings(concreteDeclarations)
	var steps []semanticir.ProbeStep
	buildRun := func(id string, response concreteResponse) (semanticir.ExecutionRunEvidence, error) {
		run := semanticir.ExecutionRunEvidence{
			ID: id, StartedAtUTC: response.StartedAtUTC, FreshProcessCount: response.ProcessCount,
			Provenance: provenance,
		}
		for _, result := range response.Results {
			binding, ok := bindings[result.ID]
			item, itemOK := caseByID[result.ID]
			if !ok || !itemOK {
				return semanticir.ExecutionRunEvidence{}, fmt.Errorf("execution transcript refers to unknown assignment %s", result.ID)
			}
			behaviorCase, ok := matchingCase(behaviorCases, semanticir.BehaviorRef{OperationID: binding.operation.ID, Conditions: binding.assignment, Inputs: binding.inputs})
			if !ok {
				return semanticir.ExecutionRunEvidence{}, fmt.Errorf("execution transcript has no unique modeled case for %s", result.ID)
			}
			if len(item.Arguments) != len(binding.operation.Inputs) {
				return semanticir.ExecutionRunEvidence{}, fmt.Errorf("execution transcript argument count differs for %s", result.ID)
			}
			inputs := make(map[string]semanticir.Literal, len(item.Arguments))
			for index, input := range binding.operation.Inputs {
				if _, exists := binding.assignment[input.DomainID]; !exists {
					return semanticir.ExecutionRunEvidence{}, fmt.Errorf("execution transcript omits domain %s", input.DomainID)
				}
				inputs[input.Name] = item.Arguments[index]
			}
			raw, err := rawTraceFromConcreteResult(result)
			if err != nil {
				return semanticir.ExecutionRunEvidence{}, fmt.Errorf("raw terminal %s: %w", result.ID, err)
			}
			classifiedID, err := semanticir.ClassifyRawOutcome(binding.operation, raw, behaviorCase.Provenance)
			if err != nil {
				return semanticir.ExecutionRunEvidence{}, fmt.Errorf("classify raw terminal %s: %w", result.ID, err)
			}
			if len(behaviorCase.OutcomeIDs) != 1 || behaviorCase.OutcomeIDs[0] != classifiedID {
				return semanticir.ExecutionRunEvidence{}, fmt.Errorf("raw terminal %s differs from modeled outcome", result.ID)
			}
			signalValue, err := semanticir.CanonicalJSON(raw)
			if err != nil {
				return semanticir.ExecutionRunEvidence{}, fmt.Errorf("encode raw terminal for %s: %w", result.ID, err)
			}
			if !bytes.Equal(signalValue, result.SignalValue) || result.SignalPath == "" || len(result.ProcessOutput) == 0 {
				return semanticir.ExecutionRunEvidence{}, fmt.Errorf("interpreter signal for %s differs from canonical typed raw outcome: got %q want %q", result.ID, result.SignalValue, signalValue)
			}
			stepID := id + ":" + result.ID
			stepRequest := concreteRequest{
				Root: filepath.ToSlash(rootFromWorkDir), PackageRoot: packageRoot, Module: request.Options["python.module"],
				SourcePath: request.Artifact.Path, Operations: concreteOperations, Declarations: concreteDeclarations, Cases: []concreteCase{item},
				SignalPath: filepath.ToSlash(filepath.Join(request.Workspace.WorkingDirectory, result.SignalPath)),
			}
			stdin, err := json.Marshal(stepRequest)
			if err != nil {
				return semanticir.ExecutionRunEvidence{}, fmt.Errorf("encode direct run step %s: %w", stepID, err)
			}
			steps = append(steps, semanticir.ProbeStep{
				ID: stepID, Kind: semanticir.ProbeStepRun, Tool: request.Translator,
				Argv: []string{"-P", "-B", "-c", concreteRunner}, Stdin: stdin, StdinDigest: semanticir.DigestBytes(stdin),
				WorkingDirectory: workingDirectory, Environment: append([]semanticir.EnvironmentVariable(nil), request.Workspace.Environment...), EnvironmentDigest: request.Workspace.EnvironmentDigest,
				ClearEnvironment: request.Workspace.ClearEnvironment, KillProcessGroup: request.Workspace.KillProcessGroup, TimeoutMillis: 30000,
				ExpectedExitCode: 0, ExpectedStdoutDigest: semanticir.DigestBytes(result.ProcessOutput), ExpectedStderrDigest: semanticir.DigestBytes(nil), ExpectedSignalDigest: semanticir.DigestBytes(signalValue),
				SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalRawOutcomeFile, Path: result.SignalPath},
				Provenance:      behaviorCase.Provenance,
			})
			run.Observations = append(run.Observations, semanticir.ExecutionObservation{
				Behavior: semanticir.BehaviorRef{
					OperationID: binding.operation.ID, Conditions: cloneAssignment(binding.assignment), Inputs: cloneLiteralMap(inputs), Provenance: behaviorCase.Provenance,
				},
				Inputs: inputs, StepID: stepID, RawOutcome: raw, OutcomeIDs: []string{classifiedID}, ExitCode: 0,
				Stdout: append([]byte(nil), result.ProcessOutput...), StdoutDigest: semanticir.DigestBytes(result.ProcessOutput), Stderr: nil, StderrDigest: semanticir.DigestBytes(nil), SignalValue: signalValue, SignalValueDigest: semanticir.DigestBytes(signalValue),
				Provenance: behaviorCase.Provenance,
			})
		}
		var err error
		run.ObservationDigest, err = semanticir.ExecutionObservationDigest(run.Observations)
		if err != nil {
			return semanticir.ExecutionRunEvidence{}, err
		}
		run.OrderDigest, err = semanticir.ExecutionOrderDigest(run.Observations)
		if err != nil {
			return semanticir.ExecutionRunEvidence{}, err
		}
		return run, nil
	}
	forwardRun, err := buildRun("forward", forward)
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, err
	}
	reverseRun, err := buildRun("reverse", reverse)
	if err != nil {
		return semanticir.ExhaustiveExecutionEvidence{}, err
	}
	if forwardRun.ObservationDigest != reverseRun.ObservationDigest {
		return semanticir.ExhaustiveExecutionEvidence{}, fmt.Errorf("fresh-process repetitions disagree")
	}
	evidence := semanticir.ExhaustiveExecutionEvidence{
		ID: "python-exhaustive:" + request.Artifact.ID, Tool: request.Translator,
		SourceDigest: request.Artifact.Digest, WorkspaceTreeDigest: request.Workspace.TreeDigest,
		IRKind: semanticir.CompilerIRCPythonBytecode, EmittedIRDigest: forward.BytecodeDigest,
		Harness: []byte(concreteRunner), HarnessPath: "<cpython-argv-c>", HarnessDigest: semanticir.DigestBytes([]byte(concreteRunner)), ExecutableDigest: request.Translator.Digest,
		Steps: steps,
		Argv:  []string{request.Translator.Path, "-P", "-B", "-c", concreteRunner}, WorkingDirectory: workingDirectory,
		Environment: append([]semanticir.EnvironmentVariable(nil), request.Workspace.Environment...), EnvironmentDigest: request.Workspace.EnvironmentDigest,
		ClearEnvironment: request.Workspace.ClearEnvironment, KillProcessGroup: request.Workspace.KillProcessGroup, TimeoutMillis: 30000,
		Groundings: append([]semanticir.AssignmentGrounding(nil), request.Groundings...), CompleteAssignmentDigest: forwardRun.ObservationDigest,
		Runs: []semanticir.ExecutionRunEvidence{forwardRun, reverseRun}, Complete: true, Provenance: provenance,
	}
	return evidence, nil
}

/*
The previous implementation encoded the observed concrete truth table as an
SMT "compiler" predicate.  That is not a derivation of CPython semantics and
is deliberately excluded from the executable frontend.  Exact singleton
groundings now use concreteExhaustiveEvidence above; genuine semantic
categories must use a real compiler/model-checker frontend or block.

func concreteCompilerEvidence(ctx context.Context, request semanticir.FrontendRequest, operations map[string]semanticir.Operation, behaviorCases []semanticir.BehaviorCase, cases []concreteCase, bindings map[string]concreteBinding, response concreteResponse) (semanticir.CompilerEvidence, error) {
	provenance := wholeProvenance(request, semanticir.TranslationTranslated)
	prover, err := boundSMTProver(ctx, request)
	if err != nil {
		return semanticir.CompilerEvidence{}, err
	}
	environmentDigest, err := semanticir.Digest(request.Workspace.Environment)
	if err != nil {
		return semanticir.CompilerEvidence{}, err
	}
	evidence := semanticir.CompilerEvidence{
		ID: "python-bytecode:" + request.Artifact.ID, Tool: request.Translator, Prover: prover,
		SourceDigest: request.Artifact.Digest, WorkspaceTreeDigest: request.Workspace.TreeDigest,
		Argv: []string{request.Translator.Path, "-P", "-B", "-c", concreteRunner}, EnvironmentDigest: environmentDigest,
		IRKind: semanticir.CompilerIRCPythonBytecode, EmittedIRDigest: response.BytecodeDigest,
		Provenance: provenance,
	}
	packageRoot, err := normalizedPackageRoot(request)
	if err != nil {
		return semanticir.CompilerEvidence{}, err
	}
	evidence.HarnessDigest, err = semanticir.Digest(struct {
		RunnerDigest string
		Module       string
		PackageRoot  string
		SourcePath   string
		Cases        []concreteCase
	}{semanticir.DigestBytes([]byte(concreteRunner)), request.Options["python.module"], filepath.ToSlash(packageRoot), filepath.ToSlash(request.Artifact.Path), cases})
	if err != nil {
		return semanticir.CompilerEvidence{}, err
	}
	proofContext := semanticir.CompilerProofContext{
		SourceDigest: request.Artifact.Digest, WorkspaceTreeDigest: request.Workspace.TreeDigest,
		EmittedIRDigest: response.BytecodeDigest, HarnessDigest: evidence.HarnessDigest, Compiler: request.Translator,
	}
	results := make(map[string]concreteResult, len(response.Results))
	for _, result := range response.Results {
		results[result.ID] = result
	}
	operationIDs := make([]string, 0, len(operations))
	for operationID := range operations {
		operationIDs = append(operationIDs, operationID)
	}
	sort.Strings(operationIDs)
	attachedConstraints := make(map[string]struct{})
	predicateByLabel := make(map[string]semanticir.CompilerPredicate)
	scopeByOperation := make(map[string]semanticir.CompilerPredicate)
	behaviorByCase := make(map[string]semanticir.CompilerPredicate)
	for _, operationID := range operationIDs {
		operation := operations[operationID]
		scope, membership, behaviors, err := buildPythonProofScope(request, operation, behaviorCases, cases, bindings, results, response.BytecodeDigest)
		if err != nil {
			return semanticir.CompilerEvidence{}, err
		}
		scopeByOperation[operationID] = scope
		evidence.OperationScopes = append(evidence.OperationScopes, semanticir.OperationScopeEvidence{
			OperationID: operationID, ScopePredicateDigest: scope.FormulaDigest, ScopePredicate: scope, Provenance: provenance,
		})
		for key, predicate := range membership {
			predicateByLabel[operationID+"\x00"+key] = predicate
		}
		for caseID, predicate := range behaviors {
			behaviorByCase[caseID] = predicate
		}
		for _, domainID := range operation.DomainIDs {
			domain := findDomain(request.FiniteDomains, domainID)
			if domain == nil {
				return semanticir.CompilerEvidence{}, fmt.Errorf("compiler evidence operation %s references missing domain %s", operationID, domainID)
			}
			partition := semanticir.DomainPartitionEvidence{
				OperationID: operationID, DomainID: domainID, ScopePredicateDigest: scope.FormulaDigest, ScopePredicate: scope,
				Totality: semanticir.ProofProved, Disjointness: semanticir.ProofProved, Provenance: provenance,
			}
			for _, member := range domain.Values {
				var witnessIDs []string
				for _, item := range cases {
					binding := bindings[item.ID]
					if binding.operation.ID != operationID || binding.assignment[domainID] != member.ID {
						continue
					}
					witnessIDs = append(witnessIDs, item.ID)
				}
				sort.Strings(witnessIDs)
				membershipPredicate := membership[domainID+"\x00"+member.ID]
				claimKind := semanticir.ClaimReachability
				if len(witnessIDs) == 0 {
					claimKind = semanticir.ClaimUnreachability
				}
				claim := semanticir.NewProofClaim(claimKind, proofContext, scope, []semanticir.CompilerPredicate{membershipPredicate}, nil)
				proof, proofErr := runReplayableSMT(ctx, prover, request.Workspace, claim)
				if proofErr != nil {
					return semanticir.CompilerEvidence{}, proofErr
				}
				label := semanticir.LabelPathEvidence{
					ValueID: member.ID, PredicateDigest: membershipPredicate.FormulaDigest, MembershipPredicate: membershipPredicate, CompilerNodeIDs: append([]string(nil), membershipPredicate.CompilerNodeIDs...),
					ReachabilityProofDigest: proof.QueryDigest, ReachabilityProof: proof, Provenance: provenance,
				}
				if len(witnessIDs) == 0 {
					if proof.Result != semanticir.SolverUNSAT {
						return semanticir.CompilerEvidence{}, fmt.Errorf("unexecuted label %s/%s/%s is not proved unreachable", operationID, domainID, member.ID)
					}
					label.Reachability = semanticir.ProofRefuted
				} else {
					if proof.Result != semanticir.SolverSAT {
						return semanticir.CompilerEvidence{}, fmt.Errorf("executed label %s/%s/%s is not solver-reachable", operationID, domainID, member.ID)
					}
					concreteWitness, ok := strictPythonLiteralFromDomainMember(*domain, member)
					if !ok {
						return semanticir.CompilerEvidence{}, fmt.Errorf("domain label %s/%s/%s has no concrete Python witness", operationID, domainID, member.ID)
					}
					label.Reachability = semanticir.ProofProved
					label.ConcreteWitness = &concreteWitness
					label.WitnessDigest, _ = semanticir.Digest(concreteWitness)
				}
				partition.Labels = append(partition.Labels, label)
			}
			memberships := make([]semanticir.CompilerPredicate, 0, len(domain.Values))
			for _, member := range domain.Values {
				memberships = append(memberships, membership[domainID+"\x00"+member.ID])
			}
			partition.TotalityProof, err = runReplayableSMT(ctx, prover, request.Workspace, semanticir.NewProofClaim(semanticir.ClaimTotality, proofContext, scope, memberships, nil))
			if err != nil || partition.TotalityProof.Result != semanticir.SolverUNSAT {
				return semanticir.CompilerEvidence{}, fmt.Errorf("domain partition %s/%s totality replay failed: %v", operationID, domainID, err)
			}
			partition.TotalityProofDigest = partition.TotalityProof.QueryDigest
			partition.DisjointnessProof, err = runReplayableSMT(ctx, prover, request.Workspace, semanticir.NewProofClaim(semanticir.ClaimDisjointness, proofContext, scope, memberships, nil))
			if err != nil || partition.DisjointnessProof.Result != semanticir.SolverUNSAT {
				return semanticir.CompilerEvidence{}, fmt.Errorf("domain partition %s/%s disjointness replay failed: %v", operationID, domainID, err)
			}
			partition.DisjointnessProofDigest = partition.DisjointnessProof.QueryDigest
			for _, constraint := range request.Constraints {
				if constraint.OperationID != operationID {
					continue
				}
				constraintMemberships := make([]semanticir.CompilerPredicate, 0, len(operation.DomainIDs))
				for _, localDomainID := range operation.DomainIDs {
					valueID, exists := constraint.Conditions[localDomainID]
					if !exists {
						return semanticir.CompilerEvidence{}, fmt.Errorf("constraint %s is not a complete operation assignment", constraint.ID)
					}
					constraintMemberships = append(constraintMemberships, membership[localDomainID+"\x00"+valueID])
				}
				proof, proofErr := runReplayableSMT(ctx, prover, request.Workspace, semanticir.NewProofClaim(semanticir.ClaimExclusion, proofContext, scope, constraintMemberships, nil))
				if proofErr != nil || proof.Result != semanticir.SolverUNSAT {
					return semanticir.CompilerEvidence{}, fmt.Errorf("constraint %s exclusion replay failed: %v", constraint.ID, proofErr)
				}
				partition.Exclusions = append(partition.Exclusions, semanticir.ConstraintPathEvidence{
					ConstraintID: constraint.ID, Result: semanticir.ProofProved, ProofDigest: proof.QueryDigest, Proof: proof, Provenance: provenance,
				})
				attachedConstraints[constraint.ID] = struct{}{}
			}
			evidence.Partitions = append(evidence.Partitions, partition)
		}
	}
	for _, constraint := range request.Constraints {
		if _, ok := attachedConstraints[constraint.ID]; !ok {
			return semanticir.CompilerEvidence{}, fmt.Errorf("constraint %s cannot be attached to an operation domain partition", constraint.ID)
		}
	}
	for _, behaviorCase := range behaviorCases {
		scope, exists := scopeByOperation[behaviorCase.OperationID]
		behavior, behaviorExists := behaviorByCase[behaviorCase.ID]
		if !exists || !behaviorExists || len(behaviorCase.OutcomeIDs) == 0 {
			return semanticir.CompilerEvidence{}, fmt.Errorf("behavior case %s lacks an exact compiler category predicate", behaviorCase.ID)
		}
		memberships := make([]semanticir.CompilerPredicate, 0, len(behaviorCase.Conditions))
		categoryDigests := make([]string, 0, len(behaviorCase.Conditions))
		for _, domainID := range operations[behaviorCase.OperationID].DomainIDs {
			valueID, ok := behaviorCase.Conditions[domainID]
			if !ok {
				return semanticir.CompilerEvidence{}, fmt.Errorf("behavior case %s omits domain %s", behaviorCase.ID, domainID)
			}
			predicate, ok := predicateByLabel[behaviorCase.OperationID+"\x00"+domainID+"\x00"+valueID]
			if !ok {
				return semanticir.CompilerEvidence{}, fmt.Errorf("behavior case %s has no category predicate for %s", behaviorCase.ID, domainID)
			}
			memberships = append(memberships, predicate)
			categoryDigests = append(categoryDigests, predicate.FormulaDigest)
		}
		sort.Strings(categoryDigests)
		outcomePredicates := make([]semanticir.CompilerOutcomePredicate, 0, len(behaviorCase.OutcomeIDs))
		for _, outcomeID := range behaviorCase.OutcomeIDs {
			outcomePredicates = append(outcomePredicates, semanticir.CompilerOutcomePredicate{OutcomeID: outcomeID, Predicate: behavior})
		}
		claim := semanticir.NewProofClaim(semanticir.ClaimRealization, proofContext, scope, memberships, outcomePredicates)
		proof, proofErr := runReplayableSMT(ctx, prover, request.Workspace, claim)
		if proofErr != nil || proof.Result != semanticir.SolverUNSAT {
			return semanticir.CompilerEvidence{}, fmt.Errorf("behavior case %s realization replay failed: %v", behaviorCase.ID, proofErr)
		}
		evidence.BehaviorProofs = append(evidence.BehaviorProofs, semanticir.BehaviorRealizationEvidence{
			BehaviorCaseID: behaviorCase.ID,
			Behavior: semanticir.BehaviorRef{
				OperationID: behaviorCase.OperationID, Conditions: cloneAssignment(behaviorCase.Conditions), Inputs: cloneLiteralMap(behaviorCase.Inputs), Provenance: behaviorCase.Provenance,
			},
			OutcomeIDs:     append([]string(nil), behaviorCase.OutcomeIDs...), CategoryPredicateDigests: categoryDigests,
			RealizationProof: proof, Provenance: behaviorCase.Provenance,
		})
	}
	evidence.TotalConstructs = len(response.Results) + len(evidence.Partitions) + len(request.Constraints) + len(evidence.BehaviorProofs)
	if evidence.TotalConstructs == 0 {
		return semanticir.CompilerEvidence{}, fmt.Errorf("compiler evidence is empty")
	}
	evidence.TranslatedConstructs = evidence.TotalConstructs
	return evidence, nil
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func buildPythonProofScope(request semanticir.FrontendRequest, operation semanticir.Operation, behaviorCases []semanticir.BehaviorCase, cases []concreteCase, bindings map[string]concreteBinding, results map[string]concreteResult, irDigest string) (semanticir.CompilerPredicate, map[string]semanticir.CompilerPredicate, map[string]semanticir.CompilerPredicate, error) {
	indices := make(map[string]map[string]int, len(operation.DomainIDs))
	var declarations strings.Builder
	var scopeTerms []string
	for domainIndex, domainID := range operation.DomainIDs {
		domain := findDomain(request.FiniteDomains, domainID)
		if domain == nil || len(domain.Values) == 0 {
			return semanticir.CompilerPredicate{}, nil, nil, fmt.Errorf("compiler scope references empty domain %s", domainID)
		}
		fmt.Fprintf(&declarations, "(declare-const d%d Int)\n", domainIndex)
		scopeTerms = append(scopeTerms, fmt.Sprintf("(and (<= 0 d%d) (< d%d %d))", domainIndex, domainIndex, len(domain.Values)))
		indices[domainID] = make(map[string]int, len(domain.Values))
		for valueIndex, member := range domain.Values {
			if member.ID == "" {
				return semanticir.CompilerPredicate{}, nil, nil, fmt.Errorf("domain %s has an empty label", domainID)
			}
			if _, duplicate := indices[domainID][member.ID]; duplicate {
				return semanticir.CompilerPredicate{}, nil, nil, fmt.Errorf("domain %s repeats label %s", domainID, member.ID)
			}
			indices[domainID][member.ID] = valueIndex
		}
	}
	declarations.WriteString("(declare-const outcome Int)\n")
	for _, constraint := range request.Constraints {
		if constraint.OperationID != operation.ID {
			continue
		}
		formula, err := smtAssignmentFormula(operation, indices, constraint.Conditions)
		if err != nil {
			return semanticir.CompilerPredicate{}, nil, nil, fmt.Errorf("constraint %s: %w", constraint.ID, err)
		}
		scopeTerms = append(scopeTerms, "(not "+formula+")")
	}

	operationNodes := make(map[string]struct{})
	caseNodes := make(map[string][]string)
	for _, item := range cases {
		binding, ok := bindings[item.ID]
		if !ok || binding.operation.ID != operation.ID {
			continue
		}
		for _, node := range results[item.ID].CompilerNodes {
			operationNodes[node] = struct{}{}
		}
	}
	allNodes := sortedSet(operationNodes)
	if len(allNodes) == 0 {
		return semanticir.CompilerPredicate{}, nil, nil, fmt.Errorf("operation %s has no executed bytecode nodes", operation.ID)
	}

	var localCases []semanticir.BehaviorCase
	var outcomeIDs []string
	for _, behaviorCase := range behaviorCases {
		if behaviorCase.OperationID != operation.ID {
			continue
		}
		if len(behaviorCase.OutcomeIDs) != 1 {
			return semanticir.CompilerPredicate{}, nil, nil, fmt.Errorf("behavior case %s is not a single exact observed outcome", behaviorCase.ID)
		}
		localCases = append(localCases, behaviorCase)
		outcomeIDs = append(outcomeIDs, behaviorCase.OutcomeIDs[0])
	}
	outcomeIDs = compactOptionStrings(outcomeIDs)
	sort.Strings(outcomeIDs)
	outcomeIndex := make(map[string]int, len(outcomeIDs))
	for index, outcomeID := range outcomeIDs {
		outcomeIndex[outcomeID] = index
	}
	if len(localCases) == 0 {
		return semanticir.CompilerPredicate{}, nil, nil, fmt.Errorf("operation %s has no exhaustive behavior cases", operation.ID)
	}
	for _, behaviorCase := range localCases {
		assignmentFormula, err := smtAssignmentFormula(operation, indices, behaviorCase.Conditions)
		if err != nil {
			return semanticir.CompilerPredicate{}, nil, nil, fmt.Errorf("behavior case %s: %w", behaviorCase.ID, err)
		}
		scopeTerms = append(scopeTerms, fmt.Sprintf("(=> %s (= outcome %d))", assignmentFormula, outcomeIndex[behaviorCase.OutcomeIDs[0]]))
		for _, item := range cases {
			binding := bindings[item.ID]
			if binding.operation.ID == operation.ID && assignmentsEqual(binding.assignment, behaviorCase.Conditions) {
				caseNodes[behaviorCase.ID] = append([]string(nil), results[item.ID].CompilerNodes...)
				break
			}
		}
		if len(caseNodes[behaviorCase.ID]) == 0 {
			return semanticir.CompilerPredicate{}, nil, nil, fmt.Errorf("behavior case %s has no executed compiler nodes", behaviorCase.ID)
		}
	}

	declarationBytes := []byte(strings.TrimSpace(declarations.String()))
	scope := pythonCompilerPredicate(request.Translator, irDigest, declarationBytes, smtAndFormula(scopeTerms), allNodes)
	memberships := make(map[string]semanticir.CompilerPredicate)
	for domainIndex, domainID := range operation.DomainIDs {
		domain := findDomain(request.FiniteDomains, domainID)
		for valueIndex, member := range domain.Values {
			nodeSet := make(map[string]struct{})
			for _, item := range cases {
				binding := bindings[item.ID]
				if binding.operation.ID != operation.ID || binding.assignment[domainID] != member.ID {
					continue
				}
				for _, node := range results[item.ID].CompilerNodes {
					nodeSet[node] = struct{}{}
				}
			}
			nodes := sortedSet(nodeSet)
			if len(nodes) == 0 {
				nodes = append([]string(nil), allNodes...)
			}
			memberships[domainID+"\x00"+member.ID] = pythonCompilerPredicate(request.Translator, irDigest, declarationBytes, fmt.Sprintf("(= d%d %d)", domainIndex, valueIndex), nodes)
		}
	}
	behaviors := make(map[string]semanticir.CompilerPredicate, len(localCases))
	for _, behaviorCase := range localCases {
		behaviors[behaviorCase.ID] = pythonCompilerPredicate(request.Translator, irDigest, declarationBytes, fmt.Sprintf("(= outcome %d)", outcomeIndex[behaviorCase.OutcomeIDs[0]]), caseNodes[behaviorCase.ID])
	}
	return scope, memberships, behaviors, nil
}

func pythonCompilerPredicate(tool semanticir.ToolRef, irDigest string, declarations []byte, formula string, nodeIDs []string) semanticir.CompilerPredicate {
	formulaBytes := []byte(formula)
	return semanticir.CompilerPredicate{
		Logic:        semanticir.ProofLogicSMTLIB2,
		Declarations: append([]byte(nil), declarations...), DeclarationsDigest: semanticir.DigestBytes(declarations),
		Formula: formulaBytes, FormulaDigest: semanticir.DigestBytes(formulaBytes),
		Tool: tool, IRDigest: irDigest, CompilerNodeIDs: append([]string(nil), nodeIDs...),
	}
}

func smtAndFormula(terms []string) string {
	if len(terms) == 0 {
		return "true"
	}
	if len(terms) == 1 {
		return terms[0]
	}
	return "(and " + strings.Join(terms, " ") + ")"
}

func smtAssignmentFormula(operation semanticir.Operation, indices map[string]map[string]int, assignment semanticir.Assignment) (string, error) {
	parts := make([]string, 0, len(assignment))
	for domainIndex, domainID := range operation.DomainIDs {
		valueID, ok := assignment[domainID]
		if !ok {
			continue
		}
		valueIndex, ok := indices[domainID][valueID]
		if !ok {
			return "", fmt.Errorf("assignment selects unknown label %s/%s", domainID, valueID)
		}
		parts = append(parts, fmt.Sprintf("(= d%d %d)", domainIndex, valueIndex))
	}
	if len(parts) != len(assignment) {
		return "", fmt.Errorf("assignment is empty or refers outside operation %s", operation.ID)
	}
	if len(parts) == 0 {
		if len(operation.DomainIDs) == 0 {
			return "true", nil
		}
		return "", fmt.Errorf("assignment is empty or refers outside operation %s", operation.ID)
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	return "(and " + strings.Join(parts, " ") + ")", nil
}

func runReplayableSMT(ctx context.Context, prover semanticir.ToolRef, workspace semanticir.WorkspaceRef, claim semanticir.ProofClaim) (semanticir.ReplayableProof, error) {
	query, err := semanticir.CanonicalProofQuery(claim)
	if err != nil {
		return semanticir.ReplayableProof{}, fmt.Errorf("build canonical SMT claim: %w", err)
	}
	// The canonical SMT query is self-contained. Use a stable absolute cwd so
	// candidate-workspace cleanup cannot make persisted replay evidence stale;
	// source/workspace identity remains bound by CompilerProofContext digests.
	workingDirectory := "/"
	proof := semanticir.ReplayableProof{
		Claim: claim, Logic: semanticir.ProofLogicSMTLIB2, Query: append([]byte(nil), query...), QueryDigest: semanticir.DigestBytes(query),
		Prover: prover, Argv: []string{"-in", "-smt2"}, WorkingDirectory: workingDirectory,
		Environment: append([]semanticir.EnvironmentVariable(nil), workspace.Environment...), ClearEnvironment: workspace.ClearEnvironment, KillProcessGroup: workspace.KillProcessGroup, TimeoutMillis: 10000,
		SubjectDigests: semanticir.ProofClaimSubjectDigests(claim),
	}
	proof.EnvironmentDigest, _ = semanticir.Digest(proof.Environment)
	proofContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	command := exec.CommandContext(proofContext, prover.Path, proof.Argv...)
	command.Stdin = bytes.NewReader(query)
	if err := configureWorkspaceCommand(command, workspace); err != nil {
		return proof, err
	}
	command.Dir = workingDirectory
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err = command.Run()
	if err != nil {
		if proofContext.Err() != nil {
			return proof, fmt.Errorf("SMT replay timed out or canceled: %w", proofContext.Err())
		}
		if stderr.Len() != 0 {
			return proof, fmt.Errorf("SMT replay failed: %s", strings.TrimSpace(stderr.String()))
		}
		return proof, fmt.Errorf("SMT replay failed: %w", err)
	}
	if stderr.Len() != 0 {
		return proof, fmt.Errorf("SMT replay emitted stderr: %s", strings.TrimSpace(stderr.String()))
	}
	output := stdout.Bytes()
	proof.SolverOutput = append([]byte(nil), output...)
	proof.SolverOutputDigest = semanticir.DigestBytes(output)
	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return proof, fmt.Errorf("SMT prover returned empty output")
	}
	proof.Result = semanticir.SolverResult(fields[0])
	if proof.Result != semanticir.SolverSAT && proof.Result != semanticir.SolverUNSAT && proof.Result != semanticir.SolverUnknown {
		return proof, fmt.Errorf("SMT prover returned unsupported result %q", fields[0])
	}
	return proof, nil
}
*/

func runConcrete(ctx context.Context, frontend semanticir.FrontendRequest, request concreteRequest) (concreteResponse, error) {
	ordered := append([]concreteCase(nil), request.Cases...)
	if request.Reverse {
		for left, right := 0, len(ordered)-1; left < right; left, right = left+1, right-1 {
			ordered[left], ordered[right] = ordered[right], ordered[left]
		}
	}
	response := concreteResponse{StartedAtUTC: time.Now().UTC().Format(time.RFC3339Nano)}
	order := "forward"
	if request.Reverse {
		order = "reverse"
	}
	for _, item := range ordered {
		single := request
		single.Cases = []concreteCase{item}
		single.Reverse = false
		caseDigest, _ := semanticir.Digest(item.ID)
		signalName := ".ray-python-signal-" + order + "-" + strings.TrimPrefix(caseDigest, "sha256:")[:16] + ".json"
		single.SignalPath = filepath.ToSlash(filepath.Join(frontend.Workspace.WorkingDirectory, signalName))
		partial, err := runConcreteProcess(ctx, frontend, single)
		if err != nil {
			return concreteResponse{}, fmt.Errorf("case %s: %w", item.ID, err)
		}
		if len(partial.Results) != 1 || partial.Results[0].ID != item.ID || !digestPattern.MatchString(partial.BytecodeDigest) {
			return concreteResponse{}, fmt.Errorf("case %s returned an incomplete per-assignment transcript", item.ID)
		}
		result := partial.Results[0]
		result.ProcessOutput = append([]byte(nil), partial.ProcessOutput...)
		result.SignalValue = append([]byte(nil), partial.SignalValue...)
		result.SignalPath = signalName
		response.Results = append(response.Results, result)
		response.ProcessCount++
		if response.BytecodeDigest == "" {
			response.BytecodeDigest = partial.BytecodeDigest
			response.CompilerNodes = partial.CompilerNodes
			response.CompilerOpcodes = partial.CompilerOpcodes
		} else if response.BytecodeDigest != partial.BytecodeDigest {
			return concreteResponse{}, fmt.Errorf("case %s compiled different operation bytecode", item.ID)
		} else {
			wantNodes, _ := semanticir.Digest(response.CompilerNodes)
			gotNodes, _ := semanticir.Digest(partial.CompilerNodes)
			if wantNodes != gotNodes {
				return concreteResponse{}, fmt.Errorf("case %s compiled different operation node identities", item.ID)
			}
			wantOpcodes, _ := semanticir.Digest(response.CompilerOpcodes)
			gotOpcodes, _ := semanticir.Digest(partial.CompilerOpcodes)
			if wantOpcodes != gotOpcodes {
				return concreteResponse{}, fmt.Errorf("case %s compiled different operation opcode inventory", item.ID)
			}
		}
	}
	return response, nil
}

func runConcreteProcess(ctx context.Context, frontend semanticir.FrontendRequest, request concreteRequest) (concreteResponse, error) {
	var response concreteResponse
	payload, err := json.Marshal(request)
	if err != nil {
		return response, fmt.Errorf("cannot encode exhaustive interpreter request: %w", err)
	}
	processContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(processContext, frontend.Translator.Path, "-P", "-B", "-c", concreteRunner)
	command.Stdin = bytes.NewReader(payload)
	if err := configureWorkspaceCommand(command, frontend.Workspace); err != nil {
		return response, err
	}
	if request.SignalPath == "" {
		return response, fmt.Errorf("exhaustive interpreter signal path is empty")
	}
	signalAbsolute := filepath.Join(frontend.Workspace.Root, filepath.Clean(request.SignalPath))
	rootResolved, resolveErr := filepath.EvalSymlinks(filepath.Clean(frontend.Workspace.Root))
	if resolveErr != nil {
		return response, fmt.Errorf("resolve signal workspace: %w", resolveErr)
	}
	signalParent, resolveErr := filepath.EvalSymlinks(filepath.Dir(signalAbsolute))
	if resolveErr != nil {
		return response, fmt.Errorf("resolve signal parent: %w", resolveErr)
	}
	if relative, relativeErr := filepath.Rel(rootResolved, signalParent); relativeErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return response, fmt.Errorf("exhaustive interpreter signal path escapes workspace")
	}
	if _, statErr := os.Lstat(signalAbsolute); statErr == nil {
		return response, fmt.Errorf("exhaustive interpreter signal path already exists")
	} else if !os.IsNotExist(statErr) {
		return response, fmt.Errorf("inspect exhaustive signal path: %w", statErr)
	}
	output, err := command.Output()
	if err != nil {
		if processContext.Err() != nil {
			return response, fmt.Errorf("exhaustive interpreter canceled: %w", processContext.Err())
		}
		if exit, ok := err.(*exec.ExitError); ok && len(exit.Stderr) != 0 {
			return response, fmt.Errorf("exhaustive interpreter failed: %s", strings.TrimSpace(string(exit.Stderr)))
		}
		return response, fmt.Errorf("exhaustive interpreter failed: %w", err)
	}
	defer func() { _ = os.Remove(signalAbsolute) }()
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return response, fmt.Errorf("exhaustive interpreter returned invalid evidence: %w", err)
	}
	signal, err := os.ReadFile(signalAbsolute)
	if err != nil {
		return response, fmt.Errorf("read exhaustive typed signal: %w", err)
	}
	if err := os.Remove(signalAbsolute); err != nil {
		return response, fmt.Errorf("remove exhaustive typed signal: %w", err)
	}
	response.ProcessOutput = append([]byte(nil), output...)
	response.SignalValue = signal
	response.SignalPath = request.SignalPath
	return response, nil
}

func sameConcreteResponseSemantics(left, right concreteResponse) bool {
	leftNodes, _ := semanticir.Digest(left.CompilerNodes)
	rightNodes, _ := semanticir.Digest(right.CompilerNodes)
	if left.BytecodeDigest != right.BytecodeDigest || leftNodes != rightNodes || len(left.Results) != len(right.Results) {
		return false
	}
	leftResults := append([]concreteResult(nil), left.Results...)
	rightResults := append([]concreteResult(nil), right.Results...)
	sort.Slice(leftResults, func(i, j int) bool { return leftResults[i].ID < leftResults[j].ID })
	sort.Slice(rightResults, func(i, j int) bool { return rightResults[i].ID < rightResults[j].ID })
	leftDigest, leftErr := semanticir.Digest(leftResults)
	rightDigest, rightErr := semanticir.Digest(rightResults)
	return leftErr == nil && rightErr == nil && leftDigest == rightDigest
}

func rawTraceFromConcreteResult(result concreteResult) (semanticir.RawOutcomeTrace, error) {
	trace := semanticir.RawOutcomeTrace{ExceptionType: result.ExceptionType, Message: result.Message}
	switch result.Kind {
	case "return":
		if result.Value == nil {
			return semanticir.RawOutcomeTrace{}, fmt.Errorf("return result has no typed value")
		}
		trace.Kind, trace.Value = semanticir.OutcomeReturn, result.Value
	case "raise":
		if result.ExceptionType == "" {
			return semanticir.RawOutcomeTrace{}, fmt.Errorf("raise result has no exception type")
		}
		trace.Kind = semanticir.OutcomeRaise
	default:
		return semanticir.RawOutcomeTrace{}, fmt.Errorf("interpreter returned unsupported outcome kind %s", result.Kind)
	}
	if err := semanticir.ValidateRawOutcomeTrace(trace); err != nil {
		return semanticir.RawOutcomeTrace{}, err
	}
	return trace, nil
}

func findDomainValue(domain semanticir.Domain, id string) *semanticir.DomainValue {
	for index := range domain.Values {
		if domain.Values[index].ID == id {
			return &domain.Values[index]
		}
	}
	return nil
}

func statementLocationAtLine(statements []pyStatement, line int) bridgeLocation {
	var best bridgeLocation
	for index := range statements {
		statement := &statements[index]
		if line < statement.Location.Line || line > statement.Location.EndLine {
			continue
		}
		best = statement.Location
		if nested := statementLocationAtLine(statement.Body, line); nested.Line != 0 {
			best = nested
		}
		if nested := statementLocationAtLine(statement.Alternate, line); nested.Line != 0 {
			best = nested
		}
		for _, handler := range statement.Catches {
			if nested := statementLocationAtLine(handler.Body, line); nested.Line != 0 {
				best = nested
			}
		}
	}
	return best
}

func wholeBridgeLocation(request semanticir.FrontendRequest) bridgeLocation {
	location := wholeLocation(request)
	return bridgeLocation{Line: location.StartLine, Column: location.StartColumn, EndLine: location.EndLine, EndColumn: location.EndColumn}
}

func indexFunctions(functions []pyFunction) map[string]*pyFunction {
	result := make(map[string]*pyFunction, len(functions))
	for i := range functions {
		result[functions[i].Name] = &functions[i]
	}
	return result
}

func domainFor(request semanticir.FrontendRequest, operationID, parameter string, parameterIndex int) (*semanticir.Domain, bool) {
	for _, operation := range request.Operations {
		if operation.ID != operationID {
			continue
		}
		for _, input := range operation.Inputs {
			if input.Name == parameter {
				if input.DomainID != "" {
					if domain := findDomain(request.FiniteDomains, input.DomainID); domain != nil {
						return domain, true
					}
					return nil, false
				}
				if parameterIndex >= 0 && parameterIndex < len(operation.DomainIDs) {
					if domain := findDomain(request.FiniteDomains, operation.DomainIDs[parameterIndex]); domain != nil {
						return domain, true
					}
				}
				return nil, false
			}
		}
		if parameterIndex >= 0 && parameterIndex < len(operation.DomainIDs) {
			if domain := findDomain(request.FiniteDomains, operation.DomainIDs[parameterIndex]); domain != nil {
				return domain, true
			}
		}
		return nil, false
	}
	return nil, false
}

func findDomain(domains []semanticir.Domain, id string) *semanticir.Domain {
	for i := range domains {
		if domains[i].ID == id {
			return &domains[i]
		}
	}
	return nil
}

func domainContains(domain semanticir.Domain, id string) bool {
	for _, value := range domain.Values {
		if value.ID == id {
			return true
		}
	}
	return false
}

func domainType(domain *semanticir.Domain) (semanticir.ValueType, bool) {
	if domain == nil || len(domain.Values) == 0 {
		return semanticir.TypeUnknown, false
	}
	want := semanticir.TypeUnknown
	for _, domainValue := range domain.Values {
		literal, ok := strictPythonLiteralFromDomainMember(*domain, domainValue)
		if !ok {
			return semanticir.TypeUnknown, false
		}
		if want == semanticir.TypeUnknown {
			want = literal.Type
		} else if want != literal.Type {
			return semanticir.TypeUnknown, false
		}
	}
	return want, semanticir.ValidValueType(want)
}

func strictPythonLiteralFromDomainMember(domain semanticir.Domain, member semanticir.DomainValue) (semanticir.Literal, bool) {
	if member.Value != nil {
		if member.Value.Type == domain.Type {
			return *member.Value, true
		}
	}
	return semanticir.Literal{}, false
}

func exactPythonLiteralFromGrounding(domain semanticir.Domain, member semanticir.DomainValue, operationID, inputName string) (semanticir.Literal, bool) {
	if literal, ok := strictPythonLiteralFromDomainMember(domain, member); ok {
		return literal, true
	}
	axiom, ok := member.GroundingFor(operationID)
	if !ok || axiom.Kind != semanticir.GroundingMembership || axiom.Membership == nil {
		return semanticir.Literal{}, false
	}
	literal, ok := axiom.ConcreteWitness[inputName]
	if !ok || semanticir.ValidateLiteral(literal) != nil {
		return semanticir.Literal{}, false
	}
	satisfied, err := semanticir.EvaluateGroundingMembership(*axiom.Membership, axiom.ConcreteWitness)
	if err != nil || !satisfied || !membershipFixesLiteral(*axiom.Membership, inputName, literal) {
		return semanticir.Literal{}, false
	}
	return literal, true
}

// exactPythonAssignmentInputs consumes the outcome-independent grounding
// registry produced by the frozen spec.  It also checks that the selected
// label memberships themselves uniquely fix the same inputs, so a category's
// representative witness can never be mistaken for exhaustive semantics.
func exactPythonAssignmentInputs(request semanticir.FrontendRequest, operation semanticir.Operation, assignment semanticir.Assignment) (map[string]semanticir.Literal, bool) {
	if len(request.Groundings) == 0 {
		// Legacy unit fixtures predate AssignmentGrounding. They remain exact
		// only when each selected DomainValue directly carries a singleton.
		inputs := make(map[string]semanticir.Literal, len(operation.Inputs))
		for _, input := range operation.Inputs {
			domain := findDomain(request.FiniteDomains, input.DomainID)
			if domain == nil {
				return nil, false
			}
			member := findDomainValue(*domain, assignment[input.DomainID])
			if member == nil {
				return nil, false
			}
			literal, ok := exactPythonLiteralFromGrounding(*domain, *member, operation.ID, input.Name)
			if !ok || literal.Type != input.Type {
				return nil, false
			}
			inputs[input.Name] = literal
		}
		return inputs, len(inputs) == len(operation.Inputs)
	}

	var registry *semanticir.AssignmentGrounding
	for index := range request.Groundings {
		candidate := &request.Groundings[index]
		if candidate.OperationID != operation.ID || !reflect.DeepEqual(candidate.Conditions, assignment) {
			continue
		}
		if registry != nil {
			return nil, false
		}
		registry = candidate
	}
	membershipInputs, ok := semanticir.ExactGroundingInputs(operation, request.FiniteDomains, assignment)
	if !ok {
		return nil, false
	}
	if registry != nil && (registry.ID != semanticir.AssignmentGroundingID(operation.ID, assignment) || !reflect.DeepEqual(membershipInputs, registry.Inputs)) {
		return nil, false
	}
	for _, input := range operation.Inputs {
		literal, exists := membershipInputs[input.Name]
		if !exists || literal.Type != input.Type || semanticir.ValidateLiteral(literal) != nil {
			return nil, false
		}
	}
	return cloneLiteralMap(membershipInputs), len(membershipInputs) == len(operation.Inputs)
}

func cloneLiteralMap(source map[string]semanticir.Literal) map[string]semanticir.Literal {
	result := make(map[string]semanticir.Literal, len(source))
	for name, literal := range source {
		result[name] = literal
	}
	return result
}

// membershipFixesLiteral recognizes only a syntactic equality conjunct that
// pins this operation input to the supplied witness. A range, OR, call, or
// other category remains compiler/model-checker work and therefore blocks the
// exhaustive concrete path.
func membershipFixesLiteral(expression semanticir.Expression, inputName string, literal semanticir.Literal) bool {
	if expression.Kind == semanticir.ExprBool && expression.Operator == semanticir.OpAnd {
		for _, operand := range expression.Operands {
			if membershipFixesLiteral(operand, inputName, literal) {
				return true
			}
		}
		return false
	}
	if expression.Kind != semanticir.ExprCompare || expression.Operator != semanticir.OpEQ || len(expression.Operands) != 2 {
		return false
	}
	left, right := expression.Operands[0], expression.Operands[1]
	if left.Kind == semanticir.ExprLiteral && right.Kind == semanticir.ExprVariable {
		left, right = right, left
	}
	return left.Kind == semanticir.ExprVariable && left.Name == inputName && right.Kind == semanticir.ExprLiteral && right.Literal != nil && reflect.DeepEqual(*right.Literal, literal)
}

func enumerateAssignments(domains []semanticir.Domain, maximum int) ([]semanticir.Assignment, error) {
	count := 1
	for _, domain := range domains {
		if domain.ID == "" || len(domain.Values) == 0 {
			return nil, fmt.Errorf("domain %q is empty", domain.ID)
		}
		if count > maximum/len(domain.Values) {
			return nil, fmt.Errorf("finite domain product exceeds configured max_cases=%d", maximum)
		}
		count *= len(domain.Values)
	}
	assignments := []semanticir.Assignment{{}}
	for _, domain := range domains {
		next := make([]semanticir.Assignment, 0, len(assignments)*len(domain.Values))
		for _, assignment := range assignments {
			for _, value := range domain.Values {
				copy := cloneAssignment(assignment)
				copy[domain.ID] = value.ID
				next = append(next, copy)
			}
		}
		assignments = next
	}
	return assignments, nil
}

func maxCases(request semanticir.FrontendRequest) int {
	if request.Options != nil {
		if value, err := strconv.Atoi(request.Options["python.max_cases"]); err == nil && value > 0 {
			return value
		}
	}
	return defaultMaxCases
}

func splitOption(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func compactOptionStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value == "" {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func concreteExecution(request semanticir.FrontendRequest) bool {
	mode := request.Options["python.execution"]
	return mode == "exhaustive" || mode == "bound-cpython"
}

func normalizedPackageRoot(request semanticir.FrontendRequest) (string, error) {
	configured := strings.TrimSpace(request.Options["python.package_root"])
	if configured == "" {
		configured = request.Workspace.WorkingDirectory
	}
	clean := filepath.Clean(configured)
	if filepath.IsAbs(clean) {
		rootResolved, err := filepath.EvalSymlinks(filepath.Clean(request.Workspace.Root))
		if err != nil {
			return "", fmt.Errorf("cannot resolve frozen workspace root: %w", err)
		}
		configuredResolved, err := filepath.EvalSymlinks(clean)
		if err != nil {
			return "", fmt.Errorf("cannot resolve python.package_root: %w", err)
		}
		clean, err = filepath.Rel(rootResolved, configuredResolved)
		if err != nil {
			return "", fmt.Errorf("cannot relativize python.package_root: %w", err)
		}
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("python.package_root must stay within the frozen workspace")
	}
	return clean, nil
}

func cloneAssignment(assignment semanticir.Assignment) semanticir.Assignment {
	clone := make(semanticir.Assignment, len(assignment))
	for key, value := range assignment {
		clone[key] = value
	}
	return clone
}

func decodeLiteral(raw json.RawMessage) (semanticir.Literal, bool) {
	if bytes.Equal(raw, []byte("null")) || len(raw) == 0 {
		return semanticir.Literal{Type: semanticir.TypeOptional, Null: true}, true
	}
	var boolean bool
	if json.Unmarshal(raw, &boolean) == nil && (bytes.Equal(raw, []byte("true")) || bytes.Equal(raw, []byte("false"))) {
		return semanticir.Literal{Type: semanticir.TypeBool, Bool: boolean}, true
	}
	var integer int64
	if json.Unmarshal(raw, &integer) == nil {
		return semanticir.Literal{Type: semanticir.TypeInteger, Integer: integer}, true
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return semanticir.Literal{Type: semanticir.TypeString, String: text}, true
	}
	return semanticir.Literal{}, false
}

func literalDomainID(literal semanticir.Literal) string {
	switch literal.Type {
	case semanticir.TypeBool:
		if literal.Bool {
			return "true"
		}
		return "false"
	case semanticir.TypeInteger:
		return strconv.FormatInt(literal.Integer, 10)
	case semanticir.TypeString:
		return literal.String
	case semanticir.TypeOptional, semanticir.TypeUnit:
		return "None"
	default:
		return ""
	}
}

func domainValueIDForOperationLiteral(domain semanticir.Domain, literal semanticir.Literal, operationID, inputName string) (string, bool) {
	want, err := semanticir.Digest(literal)
	if err != nil {
		return "", false
	}
	for _, member := range domain.Values {
		value, ok := exactPythonLiteralFromGrounding(domain, member, operationID, inputName)
		if !ok {
			continue
		}
		got, err := semanticir.Digest(value)
		if err == nil && got == want {
			return member.ID, true
		}
	}
	// Semantic labels are not Python values. A test call can name a behavior
	// point only through an exact typed spec grounding (or a direct typed
	// singleton retained for compatibility), never by parsing or comparing the
	// label text.
	return "", false
}

func raisedValue(expression *semanticir.Expression) (string, string) {
	if expression == nil {
		return "", ""
	}
	if expression.Kind == semanticir.ExprCall {
		message := ""
		if len(expression.Operands) > 0 && expression.Operands[0].Literal != nil && expression.Operands[0].Literal.Type == semanticir.TypeString {
			message = expression.Operands[0].Literal.String
		}
		return expression.Name, message
	}
	if expression.Kind == semanticir.ExprVariable {
		return expression.Name, ""
	}
	return "", ""
}

func pyRaisedValue(expression *pyExpression) (string, string) {
	if expression == nil {
		return "", ""
	}
	if expression.Kind != "call" && expression.Kind != "name" {
		return "", ""
	}
	message := ""
	if len(expression.Args) > 0 {
		literal, ok := decodeLiteral(expression.Args[0].Value)
		if ok && literal.Type == semanticir.TypeString {
			message = literal.String
		}
	}
	return expression.Name, message
}

var pythonExceptionName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func casesForOperation(cases []semanticir.BehaviorCase, operationID string) []semanticir.BehaviorCase {
	var result []semanticir.BehaviorCase
	for _, behaviorCase := range cases {
		if behaviorCase.OperationID == operationID {
			result = append(result, behaviorCase)
		}
	}
	return result
}

func renderableOutcome(outcome semanticir.ObservableOutcome) bool {
	_, ok := renderOutcome(outcome)
	return ok
}

func renderOperationDispatch(request semanticir.FrontendRequest, operation semanticir.Operation, function *pyFunction, choices []semanticir.BehaviorChoice, outcomes map[string]semanticir.ObservableOutcome) ([]byte, bool) {
	if len(operation.Inputs) != len(function.Parameters) || len(choices) == 0 {
		return nil, false
	}
	type renderedChoice struct {
		key       string
		condition string
		terminal  string
	}
	rendered := make([]renderedChoice, 0, len(choices))
	for _, choice := range choices {
		var predicates []string
		exactInputs, exact := exactPythonAssignmentInputs(request, operation, choice.Behavior.Conditions)
		if !exact || !reflect.DeepEqual(choice.Behavior.Inputs, exactInputs) {
			return nil, false
		}
		for index, input := range operation.Inputs {
			literal, ok := exactInputs[input.Name]
			value, ok := renderLiteral(literal)
			if !ok {
				return nil, false
			}
			predicates = append(predicates, function.Parameters[index]+" == "+value)
		}
		terminal, ok := renderOutcome(outcomes[choice.OutcomeID])
		if !ok {
			return nil, false
		}
		key := semanticir.BehaviorRefKey(choice.Behavior)
		rendered = append(rendered, renderedChoice{key: key, condition: strings.Join(predicates, " and "), terminal: string(terminal)})
	}
	sort.Slice(rendered, func(i, j int) bool { return rendered[i].key < rendered[j].key })

	indent := sourceIndentation(request.Source, function.Body[0].Location)
	var builder strings.Builder
	for index, item := range rendered {
		if index != 0 {
			builder.WriteByte('\n')
			builder.WriteString(indent)
		}
		if item.condition == "" {
			if len(rendered) != 1 {
				return nil, false
			}
			builder.WriteString(item.terminal)
			continue
		}
		builder.WriteString("if ")
		builder.WriteString(item.condition)
		builder.WriteString(":\n")
		builder.WriteString(indent)
		builder.WriteString("    ")
		builder.WriteString(item.terminal)
	}
	return []byte(builder.String()), true
}

func sourceIndentation(source []byte, location bridgeLocation) string {
	if location.Line < 1 {
		return ""
	}
	line := 1
	start := 0
	for index, value := range source {
		if line == location.Line {
			start = index
			break
		}
		if value == '\n' {
			line++
		}
	}
	end := start + location.Column - 1
	if end < start || end > len(source) {
		return ""
	}
	return string(source[start:end])
}

func renderLiteral(literal semanticir.Literal) (string, bool) {
	switch literal.Type {
	case semanticir.TypeUnit:
		return "None", true
	case semanticir.TypeOptional:
		if literal.Null {
			return "None", true
		}
	case semanticir.TypeBool:
		if literal.Bool {
			return "True", true
		}
		return "False", true
	case semanticir.TypeInteger:
		return strconv.FormatInt(literal.Integer, 10), true
	case semanticir.TypeString:
		return strconv.Quote(literal.String), true
	case semanticir.TypeSequence, semanticir.TypeTuple:
		if literal.Elements == nil {
			return "", false
		}
		items := make([]string, 0, len(literal.Elements.Values))
		for _, element := range literal.Elements.Values {
			item, ok := renderLiteral(element)
			if !ok {
				return "", false
			}
			items = append(items, item)
		}
		if literal.Type == semanticir.TypeSequence {
			return "[" + strings.Join(items, ", ") + "]", true
		}
		if len(items) == 1 {
			return "(" + items[0] + ",)", true
		}
		return "(" + strings.Join(items, ", ") + ")", true
	}
	return "", false
}

func renderOutcome(outcome semanticir.ObservableOutcome) ([]byte, bool) {
	if len(outcome.Effects) != 0 {
		return nil, false
	}
	switch outcome.Kind {
	case semanticir.OutcomeReturn:
		if outcome.Value == nil {
			return nil, false
		}
		value, ok := renderLiteral(*outcome.Value)
		if ok {
			return []byte("return " + value), true
		}
	case semanticir.OutcomeRaise:
		if !pythonExceptionName.MatchString(outcome.ExceptionType) {
			return nil, false
		}
		return []byte("raise " + outcome.ExceptionType + "(" + strconv.Quote(outcome.Message) + ")"), true
	}
	return nil, false
}

func byteRange(source []byte, location bridgeLocation) (int, int, bool) {
	if location.Line < 1 || location.EndLine < location.Line || location.Column < 1 || location.EndColumn < 1 {
		return 0, 0, false
	}
	lineStarts := []int{0}
	for i, value := range source {
		if value == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	if location.Line > len(lineStarts) || location.EndLine > len(lineStarts) {
		return 0, 0, false
	}
	start := lineStarts[location.Line-1] + location.Column - 1
	end := lineStarts[location.EndLine-1] + location.EndColumn
	if start < 0 || end < start || end > len(source) {
		return 0, 0, false
	}
	return start, end, true
}

func diagnostic(request semanticir.FrontendRequest, code semanticir.DiagnosticCode, message string, location semanticir.SourceLocation, status semanticir.TranslationStatus) semanticir.Diagnostic {
	return semanticir.Diagnostic{
		Severity: semanticir.SeverityError, Code: code, Message: message,
		Provenance: semanticir.NewProvenance(request.Artifact, location, status),
	}
}

func toLocation(request semanticir.FrontendRequest, location bridgeLocation) semanticir.SourceLocation {
	return semanticir.SourceLocation{
		Path: request.Artifact.Path, StartLine: location.Line, StartColumn: location.Column,
		EndLine: location.EndLine, EndColumn: location.EndColumn,
	}
}

func wholeLocation(request semanticir.FrontendRequest) semanticir.SourceLocation {
	lines := bytes.Split(request.Source, []byte("\n"))
	endLine := len(lines)
	endColumn := len(lines[len(lines)-1])
	if endColumn == 0 {
		endColumn = 1
	}
	return semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1, EndLine: endLine, EndColumn: endColumn}
}

func wholeProvenance(request semanticir.FrontendRequest, status semanticir.TranslationStatus) semanticir.Provenance {
	return semanticir.NewProvenance(request.Artifact, wholeLocation(request), status)
}

// StableOutcomeIDs returns the artifact's outcome IDs in deterministic order.
// It is useful to construct semantic witnesses without depending on slice order.
func StableOutcomeIDs(model semanticir.ArtifactModel) []string {
	ids := make([]string, 0, len(model.Outcomes))
	for _, outcome := range model.Outcomes {
		ids = append(ids, outcome.ID)
	}
	sort.Strings(ids)
	return ids
}
