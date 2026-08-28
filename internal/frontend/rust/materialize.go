package rust

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/semanticir"
)

// Materialize renders one semantic counterexample as an exact replacement of
// the terminal expression reached by its bounded assignment.
func Materialize(ctx context.Context, request semanticir.MaterializationRequest) (semanticir.EditPlan, []semanticir.Diagnostic) {
	frontend := request.Frontend
	whole := wholeSpan(frontend.Source)
	block := func(code semanticir.DiagnosticCode, message string, span sourceSpan) (semanticir.EditPlan, []semanticir.Diagnostic) {
		return semanticir.EditPlan{}, []semanticir.Diagnostic{diagnostic(frontend.Artifact, span, code, message)}
	}
	if err := ctx.Err(); err != nil {
		return block(semanticir.DiagnosticInvalidInput, "Rust materialization cancelled: "+err.Error(), whole)
	}
	if request.Task == nil {
		return block(semanticir.DiagnosticInvalidInput, "Rust materialization requires the compiled task", whole)
	}
	if request.Counterexample.ID == "" {
		return block(semanticir.DiagnosticInvalidInput, "Rust materialization requires a counterexample ID", whole)
	}
	if err := semanticir.VerifyArtifact(frontend.Artifact, frontend.Source); err != nil {
		return block(semanticir.DiagnosticStaleArtifact, err.Error(), whole)
	}
	if workspaceDiagnostics := validateRustWorkspace(frontend); semanticir.HasErrors(workspaceDiagnostics) {
		return semanticir.EditPlan{}, workspaceDiagnostics
	}
	if request.Model.Artifact != frontend.Artifact || request.Model.Language != semanticir.LanguageRust || request.Model.Kind != semanticir.ArtifactCode {
		return block(semanticir.DiagnosticInvalidReference, "materialization model does not bind the frozen Rust code artifact", whole)
	}
	if request.Model.Translator != frontend.Translator {
		return block(semanticir.DiagnosticStaleArtifact, "materialization model was produced by a different translator", whole)
	}
	if request.Model.Coverage.Status != semanticir.TranslationComplete || len(request.Model.Coverage.Unsupported) != 0 {
		return block(semanticir.DiagnosticIncomplete, "blocked or partial Rust translation cannot be materialized", whole)
	}
	functions, issues := parseRust(frontend.Source)
	if len(issues) != 0 {
		return block(semanticir.DiagnosticUnsupported, "frozen Rust source no longer parses in the strict frontend", issues[0].Span)
	}
	if _, compilerDiagnostics := validateWithRustc(ctx, frontend, functions); semanticir.HasErrors(compilerDiagnostics) {
		return semanticir.EditPlan{}, compilerDiagnostics
	}
	functionMap := make(map[string]functionDecl, len(functions))
	for _, fn := range functions {
		functionMap[fn.Name] = fn
	}
	owned := ownedMaterializationChoices(request.Model, request.Counterexample)
	if len(owned) == 0 {
		return block(semanticir.DiagnosticInvalidReference, "counterexample has no behavior choice owned by this Rust artifact", whole)
	}
	var edits []semanticir.ByteRangeReplacement
	var editSpans []sourceSpan
	var desiredIDs []string
	choicesByOperation := make(map[string][]semanticir.BehaviorChoice)
	for _, choice := range owned {
		if pointDiagnostic := validateRustBehaviorPoint(frontend, choice.Behavior); pointDiagnostic != nil {
			return semanticir.EditPlan{}, []semanticir.Diagnostic{*pointDiagnostic}
		}
		fn, ok := functionMap[choice.Behavior.OperationID]
		if !ok || fn.IsTest {
			return block(semanticir.DiagnosticInvalidReference, fmt.Sprintf("counterexample operation %q is not a materializable Rust function", choice.Behavior.OperationID), whole)
		}
		desired, ok := findOutcome(request.Task.Outcomes, choice.OutcomeID)
		taskOperation, operationOK := requestOperation(request.Task.Operations, choice.Behavior.OperationID)
		if !ok || !operationOK || desired.OperationID != choice.Behavior.OperationID || !containsString(taskOperation.OutcomeIDs, desired.ID) {
			return block(semanticir.DiagnosticInvalidReference, fmt.Sprintf("counterexample outcome %q is absent from the authoritative task outcome vocabulary", choice.OutcomeID), fn.Span)
		}
		if len(desired.Effects) != 0 {
			return block(semanticir.DiagnosticUnsupported, "Rust materialization cannot render outcome effects without an exact source effect target", fn.Span)
		}
		desiredIDs = appendUnique(desiredIDs, desired.ID)
		if _, ok, envDiagnostic := materializationCondition(frontend, fn, choice.Behavior.Conditions); !ok {
			return semanticir.EditPlan{}, []semanticir.Diagnostic{envDiagnostic}
		}
		if modelCaseHasOutcome(request.Model, choice.Behavior, desired.ID) {
			continue
		}
		choicesByOperation[fn.Name] = append(choicesByOperation[fn.Name], choice)
	}
	for operationID, choices := range choicesByOperation {
		fn := functionMap[operationID]
		replacement, renderDiagnostic := renderRustDispatch(frontend, fn, choices, request.Task.Outcomes)
		if renderDiagnostic != nil {
			return semanticir.EditPlan{}, []semanticir.Diagnostic{*renderDiagnostic}
		}
		expectedBytes := append([]byte(nil), frontend.Source[fn.Body.Span.Start.Offset:fn.Body.Span.End.Offset]...)
		if bytes.Equal(expectedBytes, replacement) {
			continue
		}
		edits = append(edits, semanticir.ByteRangeReplacement{StartByte: fn.Body.Span.Start.Offset, EndByte: fn.Body.Span.End.Offset, ExpectedBytes: expectedBytes, Replacement: replacement})
		editSpans = append(editSpans, fn.Body.Span)
	}
	if len(edits) == 0 {
		return block(semanticir.DiagnosticIncomplete, "all Rust-owned counterexample choices already match the frozen reference; materialization is a no-op", whole)
	}
	sortRustEdits(edits)
	modified := append([]byte(nil), frontend.Source...)
	for index := len(edits) - 1; index >= 0; index-- {
		edit := edits[index]
		modified = append(modified[:edit.StartByte], append(edit.Replacement, modified[edit.EndByte:]...)...)
	}
	modifiedRequest, cleanup, prepDiagnostic := materializedFrontendRequest(frontend, modified)
	if prepDiagnostic != nil {
		return semanticir.EditPlan{}, []semanticir.Diagnostic{*prepDiagnostic}
	}
	defer cleanup()
	modifiedFunctions, modifiedIssues := parseRust(modified)
	if len(modifiedIssues) != 0 {
		return block(semanticir.DiagnosticUnsupported, "rendered counterexample is not valid in the strict Rust subset: "+modifiedIssues[0].Message, editSpans[0])
	}
	if _, compilerDiagnostics := validateWithRustc(ctx, modifiedRequest, modifiedFunctions); semanticir.HasErrors(compilerDiagnostics) {
		return block(semanticir.DiagnosticUnsupported, "rendered counterexample does not type-check with pinned rustc: "+compilerDiagnostics[0].Message, editSpans[0])
	}
	retranslated, retranslateDiagnostics := Translate(ctx, modifiedRequest)
	if semanticir.HasErrors(retranslateDiagnostics) || retranslated.Coverage.Status != semanticir.TranslationComplete {
		message := "rendered counterexample does not retranslate completely"
		if len(retranslateDiagnostics) != 0 {
			message += ": " + retranslateDiagnostics[0].Message
		}
		return block(semanticir.DiagnosticUnsupported, message, editSpans[0])
	}
	for _, choice := range owned {
		if !modelCaseHasOutcome(retranslated, choice.Behavior, choice.OutcomeID) {
			return block(semanticir.DiagnosticUnsupported, fmt.Sprintf("retranslated Rust dispatch does not realize %s for %s", choice.OutcomeID, choice.Behavior.OperationID), editSpans[0])
		}
	}
	prov := provenance(frontend.Artifact, editSpans[0], semanticir.TranslationTranslated)
	expected := semanticir.ExpectedSemantics{
		OutcomeIDs: desiredIDs,
		Choices:    append([]semanticir.BehaviorChoice(nil), request.Counterexample.Choices...),
		TestPasses: request.Counterexample.TestPasses,
	}
	if len(owned) == 1 {
		expected.OperationID = owned[0].Behavior.OperationID
		expected.Conditions = cloneAssignment(owned[0].Behavior.Conditions)
	}
	plan := semanticir.EditPlan{
		ID:         "rust-materialize-" + request.Counterexample.ID,
		WitnessID:  request.Counterexample.ID,
		Artifact:   frontend.Artifact,
		Edits:      edits,
		Expected:   expected,
		Provenance: prov,
	}
	return plan, nil
}

func ownedMaterializationChoices(model semanticir.ArtifactModel, counterexample semanticir.Counterexample) []semanticir.BehaviorChoice {
	operationSet := make(map[string]bool, len(model.Operations))
	for _, operation := range model.Operations {
		if operation.Kind != semanticir.OperationTest {
			operationSet[operation.ID] = true
		}
	}
	var matches []semanticir.BehaviorChoice
	for _, choice := range counterexample.Choices {
		if operationSet[choice.Behavior.OperationID] {
			matches = append(matches, choice)
		}
	}
	return matches
}

func validateRustBehaviorPoint(request semanticir.FrontendRequest, behavior semanticir.BehaviorRef) *semanticir.Diagnostic {
	operation, exists := requestOperation(request.Operations, behavior.OperationID)
	exact, singleton := semanticir.ExactGroundingInputs(operation, request.FiniteDomains, behavior.Conditions)
	if !exists || !singleton || behavior.Inputs == nil || !reflect.DeepEqual(behavior.Inputs, exact) {
		item := diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticUnsupported, "Rust behavior choice is not one exact frozen concrete input point")
		return &item
	}
	matched := false
	for _, grounding := range request.Groundings {
		if grounding.OperationID == behavior.OperationID && assignmentsEqual(grounding.Conditions, behavior.Conditions) && reflect.DeepEqual(grounding.Inputs, behavior.Inputs) {
			if matched {
				item := diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticOverlapping, "Rust behavior choice matches multiple frozen assignment groundings")
				return &item
			}
			matched = true
		}
	}
	if !matched {
		item := diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticInvalidReference, "Rust behavior choice is absent from the frozen assignment grounding registry")
		return &item
	}
	return nil
}

func materializationCondition(request semanticir.FrontendRequest, fn functionDecl, conditions semanticir.Assignment) (string, bool, semanticir.Diagnostic) {
	var terms []string
	expectedKeys := make(map[string]bool, len(fn.Parameters))
	for _, param := range fn.Parameters {
		domainID := findDomainID(request, fn.Name, param.Name)
		valueID, ok := conditions[domainID]
		if domainID == "" || !ok {
			return "", false, diagnostic(request.Artifact, param.Span, semanticir.DiagnosticInvalidReference, fmt.Sprintf("counterexample has no value for operation-local domain of %s.%s", fn.Name, param.Name))
		}
		expectedKeys[domainID] = true
		domain, exists := findDomain(request.FiniteDomains, domainID)
		if !exists || !domainHasValue(domain, valueID) {
			return "", false, diagnostic(request.Artifact, param.Span, semanticir.DiagnosticInvalidReference, fmt.Sprintf("counterexample value %q is not in domain %s", valueID, domainID))
		}
		domainValue, exists := domainValueByID(domain, valueID)
		if !exists {
			return "", false, diagnostic(request.Artifact, param.Span, semanticir.DiagnosticInvalidReference, fmt.Sprintf("counterexample value %q is not in domain %s", valueID, domainID))
		}
		valueType, typeOK := rustValueType(param.Type)
		literal, ok := rustLiteralForDomainValue(domain, domainValue, fn.Name, param.Name, valueType)
		if !typeOK {
			ok = false
		}
		if !ok {
			return "", false, diagnostic(request.Artifact, param.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("counterexample value %q cannot be rendered as %s", valueID, param.Type))
		}
		rendered, ok := renderRustLiteral(literal)
		if !ok {
			return "", false, diagnostic(request.Artifact, param.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("counterexample value %q has no exact Rust literal", valueID))
		}
		terms = append(terms, param.Name+" == "+string(rendered))
	}
	if len(conditions) != len(expectedKeys) {
		return "", false, diagnostic(request.Artifact, fn.Span, semanticir.DiagnosticInvalidReference, "counterexample conditions contain domains outside the selected operation")
	}
	if len(terms) == 0 {
		return "true", true, semanticir.Diagnostic{}
	}
	return strings.Join(terms, " && "), true, semanticir.Diagnostic{}
}

func findOutcome(outcomes []semanticir.ObservableOutcome, id string) (semanticir.ObservableOutcome, bool) {
	for _, outcome := range outcomes {
		if outcome.ID == id && semanticir.OutcomeID(outcome) == id {
			return outcome, true
		}
	}
	return semanticir.ObservableOutcome{}, false
}

func renderRustOutcome(source []byte, fn functionDecl, outcome semanticir.ObservableOutcome) ([]byte, bool) {
	if len(outcome.Effects) != 0 {
		return nil, false
	}
	switch outcome.Kind {
	case semanticir.OutcomeReturn:
		if _, isResult := rustResultType(fn.ReturnType); isResult || outcome.Value == nil {
			return nil, false
		}
		return renderRustLiteral(*outcome.Value)
	case semanticir.OutcomeSuccess:
		if _, isResult := rustResultType(fn.ReturnType); !isResult || outcome.Value != nil || outcome.ExceptionType != "" || outcome.Message != "" {
			return nil, false
		}
		return exactRustResultTerminal(source, fn, "Ok", "Result::Ok")
	case semanticir.OutcomeRaise:
		switch outcome.ExceptionType {
		case "panic":
			if outcome.Message == "" || outcome.Value != nil {
				return nil, false
			}
			return []byte("panic!(" + strconv.Quote(outcome.Message) + ")"), true
		case "Result::Err":
			if _, isResult := rustResultType(fn.ReturnType); !isResult || outcome.Value != nil || outcome.Message != "" {
				return nil, false
			}
			return exactRustResultTerminal(source, fn, "Err", "Result::Err")
		}
	}
	return nil, false
}

func exactRustResultTerminal(source []byte, fn functionDecl, names ...string) ([]byte, bool) {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	var matches []sourceSpan
	var visitExpression func(expression)
	var visitBlock func(block)
	visitExpression = func(value expression) {
		if value.Kind == expressionCall && allowed[value.Text] && len(value.Children) == 1 {
			matches = append(matches, value.Span)
		}
		for _, child := range value.Children {
			visitExpression(child)
		}
		if value.Then != nil {
			visitBlock(*value.Then)
		}
		if value.Else != nil {
			visitBlock(*value.Else)
		}
		for _, arm := range value.Arms {
			if arm.Guard != nil {
				visitExpression(*arm.Guard)
			}
			visitExpression(arm.Value)
		}
	}
	visitBlock = func(value block) {
		for _, statement := range value.Statements {
			visitExpression(statement.Expr)
		}
		if value.Tail != nil {
			visitExpression(*value.Tail)
		}
	}
	visitBlock(fn.Body)
	if len(matches) == 0 {
		return nil, false
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Start.Offset < matches[j].Start.Offset })
	span := matches[0]
	if span.Start.Offset < 0 || span.End.Offset > len(source) || span.Start.Offset >= span.End.Offset {
		return nil, false
	}
	return append([]byte(nil), source[span.Start.Offset:span.End.Offset]...), true
}

func renderRustLiteral(literal semanticir.Literal) ([]byte, bool) {
	switch literal.Type {
	case semanticir.TypeBool:
		return []byte(strconv.FormatBool(literal.Bool)), true
	case semanticir.TypeInteger:
		return []byte(strconv.FormatInt(literal.Integer, 10)), true
	case semanticir.TypeString:
		return []byte(strconv.Quote(literal.String)), true
	case semanticir.TypeUnit:
		return []byte("()"), true
	default:
		return nil, false
	}
}

func domainHasValue(domain semanticir.Domain, id string) bool {
	for _, value := range domain.Values {
		if value.ID == id {
			return true
		}
	}
	return false
}

func domainValueByID(domain semanticir.Domain, id string) (semanticir.DomainValue, bool) {
	for _, value := range domain.Values {
		if value.ID == id {
			return value, true
		}
	}
	return semanticir.DomainValue{}, false
}

func renderRustDispatch(request semanticir.FrontendRequest, fn functionDecl, choices []semanticir.BehaviorChoice, outcomes []semanticir.ObservableOutcome) ([]byte, *semanticir.Diagnostic) {
	seen := make(map[string]string, len(choices))
	var branches []string
	for _, choice := range choices {
		condition, ok, item := materializationCondition(request, fn, choice.Behavior.Conditions)
		if !ok {
			return nil, &item
		}
		key := assignmentKey(choice.Behavior.Conditions)
		if previous, exists := seen[key]; exists && previous != choice.OutcomeID {
			diagnostic := diagnostic(request.Artifact, fn.Span, semanticir.DiagnosticOverlapping, "counterexample assigns two outcomes to the same Rust behavior")
			return nil, &diagnostic
		}
		seen[key] = choice.OutcomeID
		outcome, found := findOutcome(outcomes, choice.OutcomeID)
		if !found {
			diagnostic := diagnostic(request.Artifact, fn.Span, semanticir.DiagnosticInvalidReference, fmt.Sprintf("counterexample outcome %q is absent from the authoritative task outcome vocabulary", choice.OutcomeID))
			return nil, &diagnostic
		}
		rendered, found := renderRustOutcome(request.Source, fn, outcome)
		if !found {
			diagnostic := diagnostic(request.Artifact, fn.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("outcome %s cannot be rendered exactly for Rust return type %s", outcome.ID, fn.ReturnType))
			return nil, &diagnostic
		}
		branches = append(branches, "if "+condition+" { "+string(rendered)+" }")
	}
	if len(branches) == 0 {
		diagnostic := diagnostic(request.Artifact, fn.Span, semanticir.DiagnosticIncomplete, "counterexample has no differing Rust behavior to dispatch")
		return nil, &diagnostic
	}
	original := string(request.Source[fn.Body.Span.Start.Offset:fn.Body.Span.End.Offset])
	return []byte("{\n    " + strings.Join(branches, " else ") + " else " + original + "\n}"), nil
}

func assignmentKey(assignment semanticir.Assignment) string {
	keys := make([]string, 0, len(assignment))
	for key := range assignment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var result strings.Builder
	for _, key := range keys {
		result.WriteString(key)
		result.WriteByte(0)
		result.WriteString(assignment[key])
		result.WriteByte(0)
	}
	return result.String()
}

func modelCaseHasOutcome(model semanticir.ArtifactModel, behavior semanticir.BehaviorRef, outcomeID string) bool {
	for _, behaviorCase := range model.Cases {
		if behaviorCase.OperationID == behavior.OperationID && assignmentsEqual(behaviorCase.Conditions, behavior.Conditions) && reflect.DeepEqual(behaviorCase.Inputs, behavior.Inputs) && containsString(behaviorCase.OutcomeIDs, outcomeID) {
			return true
		}
	}
	return false
}

func sortRustEdits(edits []semanticir.ByteRangeReplacement) {
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].StartByte < edits[j].StartByte })
}

func materializedFrontendRequest(frontend semanticir.FrontendRequest, source []byte) (semanticir.FrontendRequest, func(), *semanticir.Diagnostic) {
	request := frontend
	request.Source = append([]byte(nil), source...)
	request.Artifact.Digest = semanticir.DigestBytes(source)
	tempDir, err := os.MkdirTemp("", "ray-rust-materialized-*")
	if err != nil {
		item := diagnostic(frontend.Artifact, wholeSpan(frontend.Source), semanticir.DiagnosticInvalidInput, "create materialized Rust workspace: "+err.Error())
		return semanticir.FrontendRequest{}, func() {}, &item
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	relative := filepath.Clean(request.Artifact.Path)
	if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		cleanup()
		item := diagnostic(frontend.Artifact, wholeSpan(frontend.Source), semanticir.DiagnosticInvalidReference, "materialized Rust artifact path is not workspace-relative")
		return semanticir.FrontendRequest{}, func() {}, &item
	}
	path := filepath.Join(tempDir, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		cleanup()
		item := diagnostic(frontend.Artifact, wholeSpan(frontend.Source), semanticir.DiagnosticInvalidInput, "create materialized Rust artifact directory: "+err.Error())
		return semanticir.FrontendRequest{}, func() {}, &item
	}
	if err := os.WriteFile(path, source, 0o600); err != nil {
		cleanup()
		item := diagnostic(frontend.Artifact, wholeSpan(frontend.Source), semanticir.DiagnosticInvalidInput, "write materialized Rust artifact: "+err.Error())
		return semanticir.FrontendRequest{}, func() {}, &item
	}
	prov := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	entry := semanticir.WorkspaceEntry{Path: relative, Artifact: request.Artifact, Provenance: prov}
	treeDigest, treeErr := executor.WorkspaceDigest(tempDir)
	if treeErr != nil {
		cleanup()
		item := diagnostic(frontend.Artifact, wholeSpan(frontend.Source), semanticir.DiagnosticInvalidInput, "digest materialized Rust workspace: "+treeErr.Error())
		return semanticir.FrontendRequest{}, func() {}, &item
	}
	request.Workspace = semanticir.WorkspaceRef{
		ID: request.Workspace.ID, State: request.Workspace.State, Root: tempDir,
		TreeDigest: treeDigest, WorkingDirectory: ".", BuildCommand: request.Workspace.BuildCommand,
		Environment: append([]semanticir.EnvironmentVariable(nil), request.Workspace.Environment...), EnvironmentDigest: request.Workspace.EnvironmentDigest,
		ClearEnvironment: request.Workspace.ClearEnvironment, KillProcessGroup: request.Workspace.KillProcessGroup,
		Entries: []semanticir.WorkspaceEntry{entry}, Provenance: prov,
	}
	request.FocusArtifacts = []semanticir.ArtifactRef{request.Artifact}
	for index := range request.ChangedRanges {
		changed := &request.ChangedRanges[index]
		changed.ArtifactID, changed.Path, changed.Provenance = request.Artifact.ID, request.Artifact.Path, prov
		body, exact := rustChangedRangeBytes(source, changed.StartLine, changed.EndLine)
		if !exact {
			cleanup()
			item := diagnostic(frontend.Artifact, wholeSpan(frontend.Source), semanticir.DiagnosticInvalidReference, "materialized Rust edit invalidates a frozen changed-range boundary")
			return semanticir.FrontendRequest{}, func() {}, &item
		}
		changed.SliceDigest = semanticir.DigestBytes(body)
	}
	return request, cleanup, nil
}
