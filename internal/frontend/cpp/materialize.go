package cpp

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

var cxxTypeName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_:]*$`)

// Materialize realizes a semantic counterexample as exact, digest-anchored
// C++ terminal replacements. It never invents a source edit recipe: the
// selected path is re-evaluated from the translated operation and the desired
// terminal is rendered from the task's authoritative outcome vocabulary.
func Materialize(ctx context.Context, request semanticir.MaterializationRequest) (semanticir.EditPlan, []semanticir.Diagnostic) {
	l := newLowerer(request.Frontend)
	block := func(code semanticir.DiagnosticCode, message string) (semanticir.EditPlan, []semanticir.Diagnostic) {
		l.diagnostic(nil, code, message)
		return semanticir.EditPlan{}, l.diagnostics
	}
	if err := ctx.Err(); err != nil {
		return block(semanticir.DiagnosticInvalidInput, fmt.Sprintf("materialization context: %v", err))
	}
	if !l.validateRequest() {
		return semanticir.EditPlan{}, l.diagnostics
	}
	if request.Task == nil {
		return block(semanticir.DiagnosticInvalidInput, "materialization requires the compiled task IR")
	}
	if request.Counterexample.ID == "" {
		return block(semanticir.DiagnosticInvalidInput, "counterexample id is required")
	}
	if request.Model.Artifact != request.Frontend.Artifact || request.Model.Language != semanticir.LanguageCPP || request.Model.Kind != request.Frontend.Kind {
		return block(semanticir.DiagnosticStaleArtifact, "translated model does not identify the frozen C++ frontend artifact")
	}
	if request.Model.Translator != request.Frontend.Translator {
		return block(semanticir.DiagnosticStaleArtifact, "translated model was produced by a different translator")
	}
	if request.Model.Coverage.Status != semanticir.TranslationComplete {
		return block(semanticir.DiagnosticIncomplete, "cannot materialize from an incomplete or blocked translation")
	}

	result, err := clangAST(ctx, request.Frontend.Workspace, request.Frontend.Translator.Path, l.compileDirectory, l.sourcePath, l.compileFlags, astDumpFilters(request.Frontend))
	if err != nil {
		return block(semanticir.DiagnosticUnsupported, fmt.Sprintf("rebuild clang AST for materialization: %v", err))
	}
	l.root = result.Root
	l.llvmIR = result.LLVMIR
	l.compilerWidths = result.IntegerWidths
	l.assertions = scanAssertions(request.Frontend.Source)
	l.discoverOperations()
	if semanticir.HasErrors(l.diagnostics) {
		return semanticir.EditPlan{}, l.diagnostics
	}

	operations := operationLookup(l.operations)
	outcomes := authoritativeOutcomes(request)
	choices := append([]semanticir.BehaviorChoice(nil), request.Counterexample.Choices...)
	if len(choices) == 0 && request.Counterexample.OperationID != "" && len(request.Counterexample.ObservedOutcomes) == 1 {
		choices = []semanticir.BehaviorChoice{{Behavior: semanticir.BehaviorRef{OperationID: request.Counterexample.OperationID, Conditions: cloneAssignment(request.Counterexample.Conditions), Provenance: request.Counterexample.Provenance}, OutcomeID: request.Counterexample.ObservedOutcomes[0]}}
	}
	if len(choices) == 0 {
		return block(semanticir.DiagnosticInvalidInput, "counterexample has no behavior choices to materialize")
	}

	editsByRange := make(map[string]semanticir.ByteRangeReplacement)
	relevant := 0
	for _, choice := range choices {
		operation, exists := operations[choice.Behavior.OperationID]
		if !exists {
			continue // another frozen source artifact owns this vector component
		}
		relevant++
		exactInputs, exact := l.exactInputsForAssignment(operation.operation, choice.Behavior.Conditions)
		if !exact || choice.Behavior.Inputs == nil || !reflect.DeepEqual(exactInputs, choice.Behavior.Inputs) {
			return block(semanticir.DiagnosticInvalidReference, fmt.Sprintf("counterexample behavior for operation %q is not bound to its exact full input map", operation.operation.ID))
		}
		desired, exists := outcomes[choice.OutcomeID]
		if !exists {
			return block(semanticir.DiagnosticInvalidReference, fmt.Sprintf("counterexample outcome %q is absent from the authoritative task/model vocabulary", choice.OutcomeID))
		}
		if semanticir.OutcomeID(desired) != choice.OutcomeID {
			return block(semanticir.DiagnosticInvalidReference, fmt.Sprintf("outcome %q is not canonically identified", choice.OutcomeID))
		}
		if !containsString(operation.operation.OutcomeIDs, choice.OutcomeID) && !containsString(taskOperationOutcomes(request.Task, operation.operation.ID), choice.OutcomeID) {
			return block(semanticir.DiagnosticInvalidReference, fmt.Sprintf("outcome %q is outside operation %q's local outcome universe", choice.OutcomeID, operation.operation.ID))
		}
		environment, ok := l.environmentFor(operation.operation, choice.Behavior.Conditions)
		if !ok {
			return semanticir.EditPlan{}, l.diagnostics
		}
		current, terminated, err := l.execute(operation.operation.Body, environment, operations, 0)
		if err != nil {
			return block(semanticir.DiagnosticUnsupported, fmt.Sprintf("evaluate materialization path %s: %v", operation.operation.ID, err))
		}
		if !terminated {
			return block(semanticir.DiagnosticUnsupported, fmt.Sprintf("operation %s path %s has no targetable terminal", operation.operation.ID, formatAssignment(choice.Behavior.Conditions)))
		}
		currentOutcome := outcomeFromTerminal(operation.operation.ID, current)
		if semanticir.ClassifyOutcome(operation.operation, currentOutcome) == choice.OutcomeID {
			continue
		}
		if len(desired.Effects) > 0 {
			return block(semanticir.DiagnosticUnsupported, fmt.Sprintf("outcome %q requires effect synthesis, which has no exact C++ renderer", choice.OutcomeID))
		}
		replacement, ok := renderTerminal(desired, operation.resultType)
		if !ok {
			return block(semanticir.DiagnosticUnsupported, fmt.Sprintf("outcome %q cannot be rendered as an exact C++ terminal", choice.OutcomeID))
		}
		start, end, ok := sourceBytesForLocation(request.Frontend.Source, current.provenance.Location)
		if !ok || start >= end {
			return block(semanticir.DiagnosticInvalidProvenance, fmt.Sprintf("terminal for %s has no valid byte range", operation.operation.ID))
		}
		expected := append([]byte(nil), request.Frontend.Source[start:end]...)
		if bytes.Equal(expected, replacement) {
			continue
		}
		key := fmt.Sprintf("%d:%d", start, end)
		if prior, exists := editsByRange[key]; exists && !bytes.Equal(prior.Replacement, replacement) {
			return block(semanticir.DiagnosticUnsupported, fmt.Sprintf("behavior choices require conflicting terminals at bytes [%d:%d)", start, end))
		}
		editsByRange[key] = semanticir.ByteRangeReplacement{StartByte: start, EndByte: end, ExpectedBytes: expected, Replacement: replacement}
	}
	if relevant == 0 {
		return block(semanticir.DiagnosticInvalidReference, "no counterexample behavior choice belongs to this C++ artifact")
	}
	if len(editsByRange) == 0 {
		return block(semanticir.DiagnosticUnsupported, "counterexample already matches every targetable C++ terminal; a non-no-op edit cannot be materialized")
	}
	edits := make([]semanticir.ByteRangeReplacement, 0, len(editsByRange))
	for _, edit := range editsByRange {
		edits = append(edits, edit)
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].StartByte < edits[j].StartByte })
	for index := 1; index < len(edits); index++ {
		if edits[index].StartByte < edits[index-1].EndByte {
			return block(semanticir.DiagnosticUnsupported, "materialized terminal ranges overlap")
		}
	}

	provenance := l.provenance(nil, semanticir.TranslationTranslated)
	plan := semanticir.EditPlan{
		ID:        request.Counterexample.ID + ":cpp",
		WitnessID: request.Counterexample.ID,
		Artifact:  request.Frontend.Artifact,
		Edits:     edits,
		Expected: semanticir.ExpectedSemantics{
			Conditions:  cloneAssignment(request.Counterexample.Conditions),
			OperationID: request.Counterexample.OperationID,
			OutcomeIDs:  append([]string(nil), request.Counterexample.ObservedOutcomes...),
			Choices:     choices,
			TestPasses:  request.Counterexample.TestPasses,
		},
		Provenance: provenance,
	}
	return plan, nil
}

func authoritativeOutcomes(request semanticir.MaterializationRequest) map[string]semanticir.ObservableOutcome {
	result := make(map[string]semanticir.ObservableOutcome)
	if request.Task != nil {
		for _, outcome := range request.Task.Outcomes {
			result[outcome.ID] = outcome
		}
	}
	for _, outcome := range request.Model.Outcomes {
		if _, exists := result[outcome.ID]; !exists {
			result[outcome.ID] = outcome
		}
	}
	return result
}

func taskOperationOutcomes(task *semanticir.Task, operationID string) []string {
	if task == nil {
		return nil
	}
	for _, operation := range task.Operations {
		if operation.ID == operationID {
			return operation.OutcomeIDs
		}
	}
	return nil
}

func renderTerminal(outcome semanticir.ObservableOutcome, resultType semanticir.ValueType) ([]byte, bool) {
	switch outcome.Kind {
	case semanticir.OutcomeReturn:
		if outcome.Value == nil || outcome.Value.Type == semanticir.TypeUnit {
			if resultType != semanticir.TypeUnit {
				return nil, false
			}
			return []byte("return"), true
		}
		if resultType != outcome.Value.Type {
			return nil, false
		}
		literal, ok := renderLiteral(*outcome.Value)
		if !ok {
			return nil, false
		}
		return append([]byte("return "), literal...), true
	case semanticir.OutcomeRaise:
		if !cxxTypeName.MatchString(outcome.ExceptionType) {
			return nil, false
		}
		if outcome.Message == "" {
			return []byte("throw " + outcome.ExceptionType + "{}"), true
		}
		return []byte("throw " + outcome.ExceptionType + "(" + strconv.Quote(outcome.Message) + ")"), true
	default:
		if outcome.Kind == semanticir.OutcomeSuccess && resultType == semanticir.TypeUnit {
			return []byte("return"), true
		}
		return nil, false
	}
}

func sourceBytesForLocation(source []byte, location semanticir.SourceLocation) (int, int, bool) {
	start, ok := byteOffsetForLineColumn(source, location.StartLine, location.StartColumn)
	if !ok {
		return 0, 0, false
	}
	endInclusive, ok := byteOffsetForLineColumn(source, location.EndLine, location.EndColumn)
	if !ok || endInclusive < start || endInclusive >= len(source) {
		return 0, 0, false
	}
	return start, endInclusive + 1, true
}

func byteOffsetForLineColumn(source []byte, targetLine, targetColumn int) (int, bool) {
	if targetLine < 1 || targetColumn < 1 {
		return 0, false
	}
	line, column := 1, 1
	for offset := 0; offset < len(source); offset++ {
		if line == targetLine && column == targetColumn {
			return offset, true
		}
		if source[offset] == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	if line == targetLine && column == targetColumn {
		return len(source), true
	}
	return 0, false
}
