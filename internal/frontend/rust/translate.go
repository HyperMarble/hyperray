// Package rust implements Ray's strict, finite Rust frontend.
//
// It deliberately supports a closed Rust subset. Any source-owned construct
// outside that subset produces error diagnostics and blocked coverage; no
// source text is treated as an opaque semantic fact.
package rust

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/HyperMarble/ray/internal/semanticir"
)

const Version = "rust-strict-v1"

func requestOperation(operations []semanticir.Operation, id string) (semanticir.Operation, bool) {
	for _, operation := range operations {
		if operation.ID == id {
			return operation, true
		}
	}
	return semanticir.Operation{}, false
}

func requestHasOutcome(outcomes []semanticir.ObservableOutcome, id string) bool {
	for _, outcome := range outcomes {
		if outcome.ID == id && outcome.ID == semanticir.OutcomeID(outcome) {
			return true
		}
	}
	return false
}

// Translate lowers a frozen Rust code or test artifact into Semantic IR.
func Translate(ctx context.Context, request semanticir.FrontendRequest) (semanticir.ArtifactModel, []semanticir.Diagnostic) {
	whole := wholeSpan(request.Source)
	model := semanticir.ArtifactModel{
		Artifact:    request.Artifact,
		Language:    semanticir.LanguageRust,
		Kind:        request.Kind,
		Domains:     append([]semanticir.Domain(nil), request.FiniteDomains...),
		Groundings:  append([]semanticir.AssignmentGrounding(nil), request.Groundings...),
		Constraints: append([]semanticir.Constraint(nil), request.Constraints...),
		Translator:  request.Translator,
		Coverage: semanticir.TranslationCoverage{
			Status:     semanticir.TranslationBlocked,
			Provenance: provenance(request.Artifact, whole, semanticir.TranslationUnsupported),
		},
	}
	var diagnostics []semanticir.Diagnostic

	if err := ctx.Err(); err != nil {
		return model, append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, "Rust translation cancelled: "+err.Error()))
	}
	if request.Language != semanticir.LanguageRust {
		diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, fmt.Sprintf("Rust frontend requires language %q, got %q", semanticir.LanguageRust, request.Language)))
	}
	if request.Kind != semanticir.ArtifactCode && request.Kind != semanticir.ArtifactTests {
		diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, fmt.Sprintf("Rust frontend accepts code or tests, got %q", request.Kind)))
	}
	if request.Artifact.Kind != request.Kind {
		diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, "artifact kind does not match frontend request kind"))
	}
	if err := semanticir.VerifyArtifact(request.Artifact, request.Source); err != nil {
		diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticStaleArtifact, err.Error()))
	}
	if len(request.Source) == 0 {
		diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, "Rust source is empty"))
	}
	diagnostics = append(diagnostics, validateRustOutcomeVocabulary(request)...)
	if !semanticir.HasErrors(diagnostics) {
		diagnostics = append(diagnostics, validateRustWorkspace(request)...)
	}
	if semanticir.HasErrors(diagnostics) {
		model.Coverage.Unsupported = diagnosticsToUnsupported(diagnostics)
		return model, diagnostics
	}
	functions, issues := parseRust(request.Source)
	var compilerOutput rustCompilerOutput
	for _, issue := range issues {
		code := semanticir.DiagnosticInvalidInput
		if strings.Contains(issue.Code, "UNSUPPORTED") || issue.Code == "RUST_UNSAFE" || issue.Code == "RUST_FFI" || issue.Code == "RUST_UNRESOLVED_MACRO" || issue.Code == "RUST_MUTATION" {
			code = semanticir.DiagnosticUnsupported
		}
		diagnostics = append(diagnostics, diagnostic(request.Artifact, issue.Span, code, issue.Code+": "+issue.Message))
	}
	if len(functions) == 0 {
		diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticIncomplete, "Rust artifact contains no translatable functions"))
	}
	if !semanticir.HasErrors(diagnostics) {
		var compilerDiagnostics []semanticir.Diagnostic
		compilerOutput, compilerDiagnostics = validateWithRustc(ctx, request, functions)
		diagnostics = append(diagnostics, compilerDiagnostics...)
	}
	if semanticir.HasErrors(diagnostics) {
		model.Coverage.Unsupported = diagnosticsToUnsupported(diagnostics)
		return model, deduplicateDiagnostics(diagnostics)
	}

	l := newLowerer(request, functions)
	for i := range functions {
		if err := ctx.Err(); err != nil {
			diagnostics = append(diagnostics, diagnostic(request.Artifact, functions[i].Span, semanticir.DiagnosticInvalidInput, "Rust translation cancelled: "+err.Error()))
			break
		}
		operation, tests := l.lowerFunction(functions[i])
		if !functions[i].IsTest && len(request.Operations) != 0 {
			if _, declared := requestOperation(request.Operations, operation.ID); !declared {
				continue
			}
		}
		model.Operations = append(model.Operations, operation)
		model.Tests = append(model.Tests, tests...)
	}
	diagnostics = append(diagnostics, l.diagnostics...)

	if !semanticir.HasErrors(diagnostics) {
		cases, rawCases, executionSeed, evalDiagnostics := enumerateBehavior(ctx, request, functions)
		model.Cases = cases
		model.RawReferenceCases = rawCases
		// Outcomes is the frozen observable alphabet only. Reference rows are
		// independently obtained as RawReferenceCases and joined to that
		// alphabet exclusively by semanticir.NormalizeReferenceCases.
		model.Outcomes = append([]semanticir.ObservableOutcome(nil), request.Outcomes...)
		diagnostics = append(diagnostics, evalDiagnostics...)
		populateOperationUniverses(&model)
		diagnostics = append(diagnostics, applyDeclaredRustScope(request, &model)...)
		if !semanticir.HasErrors(diagnostics) && request.Kind == semanticir.ArtifactCode {
			closure, closureDiagnostics := buildRustScopeClosure(ctx, request, functions, compilerOutput, model.Operations)
			model.ScopeClosure = closure
			diagnostics = append(diagnostics, closureDiagnostics...)
		}
		if !semanticir.HasErrors(diagnostics) && request.Kind == semanticir.ArtifactCode {
			evidence, evidenceDiagnostics := buildRustExhaustiveEvidence(request, compilerOutput, executionSeed)
			model.ExhaustiveEvidence = evidence
			diagnostics = append(diagnostics, evidenceDiagnostics...)
		}
	}
	if !semanticir.HasErrors(diagnostics) && request.Kind == semanticir.ArtifactTests {
		populateAcceptedOutcomes(&model)
		projection, runner, evidenceDiagnostics := buildRustTestEvidence(request, functions, compilerOutput, model.Tests)
		model.TestProjection = projection
		model.RunnerSelection = runner
		diagnostics = append(diagnostics, evidenceDiagnostics...)
	}

	model.Coverage.TotalConstructs = l.total
	model.Coverage.TranslatedConstructs = l.translated
	if request.Kind == semanticir.ArtifactTests && model.TestProjection != nil {
		// Test coverage is the closed compiler dependency graph. The source
		// lowering counter remains the coverage authority for code artifacts.
		model.Coverage.TotalConstructs = len(model.TestProjection.Constructs)
		model.Coverage.TranslatedConstructs = len(model.TestProjection.Constructs)
	}
	model.Coverage.Unsupported = diagnosticsToUnsupported(diagnostics)
	if semanticir.HasErrors(diagnostics) {
		model.Coverage.Status = semanticir.TranslationBlocked
		model.Coverage.Provenance = provenance(request.Artifact, whole, semanticir.TranslationUnsupported)
	} else {
		model.Coverage.Status = semanticir.TranslationComplete
		model.Coverage.Provenance = provenance(request.Artifact, whole, semanticir.TranslationTranslated)
	}
	return model, deduplicateDiagnostics(diagnostics)
}

func validateRustOutcomeVocabulary(request semanticir.FrontendRequest) []semanticir.Diagnostic {
	whole := wholeSpan(request.Source)
	if len(request.Operations) == 0 || len(request.Outcomes) == 0 {
		return []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticIncomplete, "Rust frontend requires the compiled operation/outcome vocabulary")}
	}
	outcomes := make(map[string]semanticir.ObservableOutcome, len(request.Outcomes))
	for _, outcome := range request.Outcomes {
		if outcome.ID == "" || outcome.ID != semanticir.OutcomeID(outcome) {
			return []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidReference, "Rust outcome vocabulary contains a non-canonical outcome")}
		}
		if _, duplicate := outcomes[outcome.ID]; duplicate {
			return []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidReference, "Rust outcome vocabulary repeats outcome "+outcome.ID)}
		}
		outcomes[outcome.ID] = outcome
	}
	var diagnostics []semanticir.Diagnostic
	for _, operation := range request.Operations {
		otherID := semanticir.OtherOutcome(operation.ID, provenance(request.Artifact, whole, semanticir.TranslationTranslated)).ID
		otherCount := 0
		seen := make(map[string]bool, len(operation.OutcomeIDs))
		for _, outcomeID := range operation.OutcomeIDs {
			outcome, exists := outcomes[outcomeID]
			if !exists || outcome.OperationID != operation.ID {
				diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidReference, fmt.Sprintf("Rust operation %s has an absent or cross-operation outcome %s", operation.ID, outcomeID)))
			}
			if seen[outcomeID] {
				diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidReference, fmt.Sprintf("Rust operation %s repeats outcome %s", operation.ID, outcomeID)))
			}
			seen[outcomeID] = true
			if outcomeID == otherID && exists && outcome.Kind == semanticir.OutcomeOther {
				otherCount++
			}
		}
		if otherCount != 1 {
			diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticIncomplete, fmt.Sprintf("Rust operation %s requires exactly one canonical other outcome complement", operation.ID)))
		}
	}
	return diagnostics
}

func applyDeclaredRustScope(request semanticir.FrontendRequest, model *semanticir.ArtifactModel) []semanticir.Diagnostic {
	if len(request.Operations) == 0 {
		return nil
	}
	var diagnostics []semanticir.Diagnostic
	seen := make(map[string]bool)
	for index := range model.Operations {
		operation := &model.Operations[index]
		if operation.Kind == semanticir.OperationTest {
			continue
		}
		declared, ok := requestOperation(request.Operations, operation.ID)
		if !ok {
			continue
		}
		seen[operation.ID] = true
		if !reflect.DeepEqual(operation.DomainIDs, declared.DomainIDs) {
			diagnostics = append(diagnostics, diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticInvalidReference, fmt.Sprintf("Rust operation %s parameter domains differ from compiled spec order", operation.ID)))
		}
		operation.OutcomeIDs = append([]string(nil), declared.OutcomeIDs...)
	}
	if request.Kind == semanticir.ArtifactCode {
		for _, declared := range request.Operations {
			if !seen[declared.ID] {
				diagnostics = append(diagnostics, diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticIncomplete, "Rust source omits requested operation "+declared.ID))
			}
		}
	}
	for _, behaviorCase := range model.Cases {
		declared, ok := requestOperation(request.Operations, behaviorCase.OperationID)
		if !ok {
			continue
		}
		for _, outcomeID := range behaviorCase.OutcomeIDs {
			if !containsString(declared.OutcomeIDs, outcomeID) {
				diagnostics = append(diagnostics, diagnostic(request.Artifact, wholeSpan(request.Source), semanticir.DiagnosticInvalidReference, fmt.Sprintf("compiler-observed outcome %s is outside operation %s", outcomeID, declared.ID)))
			}
		}
	}
	return diagnostics
}

type lowerer struct {
	request       semanticir.FrontendRequest
	functions     map[string]functionDecl
	returnTypes   map[string]semanticir.ValueType
	diagnostics   []semanticir.Diagnostic
	total         int
	translated    int
	currentInputs map[string]semanticir.Expression
}

func newLowerer(request semanticir.FrontendRequest, functions []functionDecl) *lowerer {
	l := &lowerer{
		request:     request,
		functions:   make(map[string]functionDecl, len(functions)),
		returnTypes: make(map[string]semanticir.ValueType, len(functions)),
	}
	for _, fn := range functions {
		if _, exists := l.functions[fn.Name]; exists {
			l.block(fn.Span, semanticir.DiagnosticDuplicateID, "duplicate Rust function "+fn.Name)
		}
		l.functions[fn.Name] = fn
		valueType, ok := rustValueType(fn.ReturnType)
		if ok {
			l.returnTypes[fn.Name] = valueType
		} else if inner, result := rustResultType(fn.ReturnType); result {
			l.returnTypes[fn.Name] = inner
		} else {
			l.returnTypes[fn.Name] = semanticir.TypeUnknown
		}
	}
	return l
}

func (l *lowerer) lowerFunction(fn functionDecl) (semanticir.Operation, []semanticir.TestModel) {
	l.total++
	kind := semanticir.OperationFunction
	if fn.IsTest {
		kind = semanticir.OperationTest
	}
	op := semanticir.Operation{ID: fn.Name, Kind: kind, Provenance: l.prov(fn.Span)}
	l.currentInputs = make(map[string]semanticir.Expression, len(fn.Parameters))
	for parameterIndex, param := range fn.Parameters {
		l.total++
		valueType, ok := rustValueType(param.Type)
		if !ok {
			l.block(param.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("parameter %s has unsupported Rust type %q", param.Name, param.Type))
			valueType = semanticir.TypeUnknown
		} else {
			l.translated++
		}
		domainID := findDomainID(l.request, fn.Name, param.Name)
		if !fn.IsTest && domainID == "" {
			declared, _ := requestOperation(l.request.Operations, fn.Name)
			available := make([]string, 0, len(l.request.FiniteDomains))
			for _, candidate := range l.request.FiniteDomains {
				available = append(available, candidate.ID)
			}
			l.block(param.Span, semanticir.DiagnosticMissingDomain, fmt.Sprintf("parameter %s.%s has no authoritative input-domain binding; finite domains=%v operation domain IDs=%v inputs=%+v", fn.Name, param.Name, available, declared.DomainIDs, declared.Inputs))
		} else if !fn.IsTest {
			declared, exists := requestOperation(l.request.Operations, fn.Name)
			if !exists || parameterIndex >= len(declared.Inputs) || declared.Inputs[parameterIndex].Name != param.Name || declared.Inputs[parameterIndex].Type != valueType {
				l.block(param.Span, semanticir.DiagnosticInvalidReference, fmt.Sprintf("Rust signature parameter %s.%s does not match authoritative typed input position %d", fn.Name, param.Name, parameterIndex))
			}
		}
		variable := semanticir.Variable{Name: param.Name, Type: valueType, DomainID: domainID, Provenance: l.prov(param.Span)}
		op.Inputs = append(op.Inputs, variable)
		if domainID != "" {
			op.DomainIDs = append(op.DomainIDs, domainID)
		}
		l.currentInputs[param.Name] = semanticir.Expression{Kind: semanticir.ExprVariable, Type: valueType, Name: param.Name, Provenance: l.prov(param.Span)}
	}
	bindings := cloneBindings(l.currentInputs)
	op.Body = l.lowerBlock(fn.Body, bindings, true)
	if !fn.IsTest {
		l.translated++
		return op, nil
	}
	assertions, operationID, conditions, predicate := l.lowerAssertions(fn, bindings)
	if len(assertions) == 0 {
		l.block(fn.Span, semanticir.DiagnosticIncomplete, fmt.Sprintf("test %s has no supported assertions", fn.Name))
	}
	test := semanticir.TestModel{
		ID:          fn.Name,
		Conditions:  conditions,
		OperationID: operationID,
		Assertions:  assertions,
		Predicate:   predicate,
		Provenance:  l.prov(fn.Span),
	}
	l.translated++
	return op, []semanticir.TestModel{test}
}

func (l *lowerer) lowerBlock(value block, bindings map[string]semanticir.Expression, terminalTail bool) []semanticir.Statement {
	var result []semanticir.Statement
	for _, stmt := range value.Statements {
		l.total++
		switch stmt.Kind {
		case statementLet:
			expr, ok := l.lowerExpression(stmt.Expr, bindings)
			if ok {
				if stmt.Mutable {
					result = append(result, semanticir.Statement{Kind: semanticir.StmtAssign, Target: stmt.Name, Value: &expr, Provenance: l.prov(stmt.Span)})
					bindings[stmt.Name] = semanticir.Expression{Kind: semanticir.ExprVariable, Type: expr.Type, Name: stmt.Name, Provenance: l.prov(stmt.Span)}
				} else {
					bindings[stmt.Name] = expr
				}
				l.translated++
			}
		case statementAssign:
			if _, declared := bindings[stmt.Name]; !declared {
				l.block(stmt.Span, semanticir.DiagnosticInvalidReference, "assignment target is not a declared local: "+stmt.Name)
				continue
			}
			expr, ok := l.lowerExpression(stmt.Expr, bindings)
			if ok {
				result = append(result, semanticir.Statement{Kind: semanticir.StmtAssign, Target: stmt.Name, Value: &expr, Provenance: l.prov(stmt.Span)})
				bindings[stmt.Name] = semanticir.Expression{Kind: semanticir.ExprVariable, Type: expr.Type, Name: stmt.Name, Provenance: l.prov(stmt.Span)}
				l.translated++
			}
		case statementFor:
			iterator, ok := l.lowerExpression(stmt.Expr, bindings)
			if !ok || stmt.Body == nil {
				continue
			}
			loopBindings := cloneBindings(bindings)
			loopBindings[stmt.Name] = semanticir.Expression{Kind: semanticir.ExprVariable, Type: semanticir.TypeInteger, Name: stmt.Name, Provenance: l.prov(stmt.Span)}
			body := l.lowerBlock(*stmt.Body, loopBindings, false)
			if len(body) == 0 {
				l.block(stmt.Span, semanticir.DiagnosticIncomplete, "bounded for-loop has no translated body")
				continue
			}
			result = append(result, semanticir.Statement{Kind: semanticir.StmtLoop, Target: stmt.Name, Iterator: &iterator, Then: body, Provenance: l.prov(stmt.Span)})
			l.translated++
		case statementReturn:
			result = append(result, l.lowerTerminal(stmt.Expr, bindings)...)
			l.translated++
		case statementExpr:
			statements, ok := l.lowerEffectExpression(stmt.Expr, bindings)
			if ok {
				result = append(result, statements...)
				l.translated++
			}
		}
	}
	if value.Tail != nil {
		l.total++
		if terminalTail {
			result = append(result, l.lowerTerminal(*value.Tail, bindings)...)
		} else {
			statements, _ := l.lowerEffectExpression(*value.Tail, bindings)
			result = append(result, statements...)
		}
		l.translated++
	} else if terminalTail && len(result) == 0 {
		unit := semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeUnit, Literal: &semanticir.Literal{Type: semanticir.TypeUnit}, Provenance: l.prov(value.Span)}
		result = append(result, semanticir.Statement{Kind: semanticir.StmtReturn, Value: &unit, Provenance: l.prov(value.Span)})
	}
	return result
}

func (l *lowerer) lowerTerminal(expr expression, bindings map[string]semanticir.Expression) []semanticir.Statement {
	switch expr.Kind {
	case expressionIf:
		return []semanticir.Statement{l.lowerIf(expr, bindings, true)}
	case expressionMatch:
		return []semanticir.Statement{l.lowerMatch(expr, bindings, true)}
	case expressionBlock:
		return l.lowerBlock(*expr.Then, cloneBindings(bindings), true)
	case expressionMacro:
		if expr.Text == "panic" {
			message, ok := l.panicMessage(expr, bindings)
			if !ok {
				return nil
			}
			return []semanticir.Statement{{Kind: semanticir.StmtRaise, ExceptionType: "panic", Message: message, Provenance: l.prov(expr.Span)}}
		}
		l.block(expr.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("unresolved macro %s!", expr.Text))
		return nil
	case expressionCall:
		if expr.Text == "Err" || expr.Text == "Result::Err" {
			if len(expr.Children) != 1 {
				l.block(expr.Span, semanticir.DiagnosticInvalidInput, "Err requires exactly one argument")
				return nil
			}
			value, ok := l.lowerExpression(expr.Children[0], bindings)
			if !ok {
				return nil
			}
			return []semanticir.Statement{{Kind: semanticir.StmtRaise, Value: &value, ExceptionType: "Result::Err", Provenance: l.prov(expr.Span)}}
		}
	}
	value, ok := l.lowerExpression(expr, bindings)
	if !ok {
		return nil
	}
	return []semanticir.Statement{{Kind: semanticir.StmtReturn, Value: &value, Provenance: l.prov(expr.Span)}}
}

func (l *lowerer) lowerEffectExpression(expr expression, bindings map[string]semanticir.Expression) ([]semanticir.Statement, bool) {
	switch expr.Kind {
	case expressionIf:
		return []semanticir.Statement{l.lowerIf(expr, bindings, false)}, true
	case expressionMatch:
		return []semanticir.Statement{l.lowerMatch(expr, bindings, false)}, true
	case expressionBlock:
		return l.lowerBlock(*expr.Then, cloneBindings(bindings), false), true
	case expressionMacro:
		if expr.Text == "panic" {
			message, ok := l.panicMessage(expr, bindings)
			return []semanticir.Statement{{Kind: semanticir.StmtRaise, ExceptionType: "panic", Message: message, Provenance: l.prov(expr.Span)}}, ok
		}
		if isAssertionMacro(expr.Text) {
			// Assertions are carried by TestModel; they are not effects in the
			// callable body vocabulary.
			return nil, true
		}
		l.block(expr.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("unresolved macro %s!", expr.Text))
		return nil, false
	case expressionCall:
		value, ok := l.lowerExpression(expr, bindings)
		if !ok {
			return nil, false
		}
		effect := semanticir.Effect{ID: fmt.Sprintf("call@%d", expr.Span.Start.Offset), Kind: semanticir.EffectCall, Target: expr.Text, Provenance: l.prov(expr.Span)}
		return []semanticir.Statement{{Kind: semanticir.StmtCall, Value: &value, Effects: []semanticir.Effect{effect}, Provenance: l.prov(expr.Span)}}, true
	default:
		l.block(expr.Span, semanticir.DiagnosticUnsupported, "value-only expression statement has no representable observable semantics")
		return nil, false
	}
}

func (l *lowerer) lowerIf(expr expression, bindings map[string]semanticir.Expression, terminal bool) semanticir.Statement {
	condition, ok := l.lowerExpression(expr.Children[0], bindings)
	if !ok {
		return semanticir.Statement{Kind: semanticir.StmtBranch, Provenance: l.prov(expr.Span)}
	}
	thenBody := l.lowerBlock(*expr.Then, cloneBindings(bindings), terminal)
	var elseBody []semanticir.Statement
	if expr.Else != nil {
		elseBody = l.lowerBlock(*expr.Else, cloneBindings(bindings), terminal)
	} else if terminal {
		l.block(expr.Span, semanticir.DiagnosticIncomplete, "value-producing if expression has no else branch")
	}
	return semanticir.Statement{Kind: semanticir.StmtBranch, Condition: &condition, Then: thenBody, Else: elseBody, Provenance: l.prov(expr.Span)}
}

func (l *lowerer) lowerMatch(expr expression, bindings map[string]semanticir.Expression, terminal bool) semanticir.Statement {
	subject, ok := l.lowerExpression(expr.Children[0], bindings)
	if !ok {
		return semanticir.Statement{Kind: semanticir.StmtBranch, Provenance: l.prov(expr.Span)}
	}
	return l.lowerMatchArms(subject, expr.Arms, bindings, terminal, expr.Span)
}

func (l *lowerer) lowerMatchArms(subject semanticir.Expression, arms []matchArm, bindings map[string]semanticir.Expression, terminal bool, span sourceSpan) semanticir.Statement {
	if len(arms) == 0 {
		l.block(span, semanticir.DiagnosticIncomplete, "match has no remaining arm")
		return semanticir.Statement{Kind: semanticir.StmtBranch, Provenance: l.prov(span)}
	}
	arm := arms[0]
	condition, armBindings, catchAll, ok := l.lowerPattern(subject, arm.Pattern, bindings, arm.Span)
	if !ok {
		return semanticir.Statement{Kind: semanticir.StmtBranch, Provenance: l.prov(arm.Span)}
	}
	if arm.Guard != nil {
		guard, guardOK := l.lowerExpression(*arm.Guard, armBindings)
		if !guardOK {
			return semanticir.Statement{Kind: semanticir.StmtBranch, Provenance: l.prov(arm.Span)}
		}
		if catchAll {
			condition = guard
			catchAll = false
		} else {
			condition = semanticir.Expression{Kind: semanticir.ExprBool, Type: semanticir.TypeBool, Operator: semanticir.OpAnd, Operands: []semanticir.Expression{condition, guard}, Provenance: l.prov(arm.Span)}
		}
	}
	thenBody := l.lowerArmValue(arm.Value, armBindings, terminal)
	if catchAll {
		// A catch-all is represented as a true branch, keeping Statement's
		// condition invariant intact.
		trueExpr := literalBool(true, l.prov(arm.Span))
		return semanticir.Statement{Kind: semanticir.StmtBranch, Condition: &trueExpr, Then: thenBody, Provenance: l.prov(arm.Span)}
	}
	var elseBody []semanticir.Statement
	if len(arms) > 1 {
		next := l.lowerMatchArms(subject, arms[1:], bindings, terminal, span)
		elseBody = []semanticir.Statement{next}
	} else {
		l.block(arm.Span, semanticir.DiagnosticIncomplete, "match has no catch-all arm; exhaustiveness is not established")
	}
	return semanticir.Statement{Kind: semanticir.StmtBranch, Condition: &condition, Then: thenBody, Else: elseBody, Provenance: l.prov(arm.Span)}
}

func (l *lowerer) lowerArmValue(value expression, bindings map[string]semanticir.Expression, terminal bool) []semanticir.Statement {
	if terminal {
		return l.lowerTerminal(value, bindings)
	}
	statements, _ := l.lowerEffectExpression(value, bindings)
	return statements
}

func (l *lowerer) lowerPattern(subject semanticir.Expression, pattern string, bindings map[string]semanticir.Expression, span sourceSpan) (semanticir.Expression, map[string]semanticir.Expression, bool, bool) {
	armBindings := cloneBindings(bindings)
	if pattern == "_" {
		return semanticir.Expression{}, armBindings, true, true
	}
	if variant, binding, ok := splitVariantPattern(pattern); ok {
		condition := semanticir.Expression{Kind: semanticir.ExprCall, Type: semanticir.TypeBool, Name: "ray::is_variant::" + variant, Operands: []semanticir.Expression{subject}, Provenance: l.prov(span)}
		if binding != "_" {
			armBindings[binding] = semanticir.Expression{Kind: semanticir.ExprCall, Type: semanticir.TypeUnknown, Name: "ray::variant_payload::" + variant, Operands: []semanticir.Expression{subject}, Provenance: l.prov(span)}
		}
		return condition, armBindings, false, true
	}
	if isRustIdentifier(pattern) && pattern != "true" && pattern != "false" {
		armBindings[pattern] = subject
		return semanticir.Expression{}, armBindings, true, true
	}
	patternExpr, ok := l.lowerExpression(expressionFromPattern(pattern, span), bindings)
	if !ok {
		return semanticir.Expression{}, armBindings, false, false
	}
	condition := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{subject, patternExpr}, Provenance: l.prov(span)}
	return condition, armBindings, false, true
}

func (l *lowerer) lowerExpression(expr expression, bindings map[string]semanticir.Expression) (semanticir.Expression, bool) {
	l.total++
	prov := l.prov(expr.Span)
	switch expr.Kind {
	case expressionIdentifier:
		if value, ok := bindings[expr.Text]; ok {
			value.Provenance = prov
			l.translated++
			return value, true
		}
		if expr.Text == "true" || expr.Text == "false" {
			l.translated++
			return literalBool(expr.Text == "true", prov), true
		}
		l.block(expr.Span, semanticir.DiagnosticInvalidReference, fmt.Sprintf("unresolved Rust name %q", expr.Text))
		return semanticir.Expression{}, false
	case expressionLiteral:
		literal, ok := parseRustLiteral(expr.Text)
		if !ok {
			l.block(expr.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("literal %q is outside the bounded scalar subset", expr.Text))
			return semanticir.Expression{}, false
		}
		l.translated++
		return semanticir.Expression{Kind: semanticir.ExprLiteral, Type: literal.Type, Literal: &literal, Provenance: prov}, true
	case expressionTuple:
		if expr.Text == "()" && len(expr.Children) == 0 {
			l.translated++
			literal := semanticir.Literal{Type: semanticir.TypeUnit}
			return semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeUnit, Literal: &literal, Provenance: prov}, true
		}
		l.block(expr.Span, semanticir.DiagnosticUnsupported, "non-unit tuples are not in Semantic IR")
		return semanticir.Expression{}, false
	case expressionUnary:
		operand, ok := l.lowerExpression(expr.Children[0], bindings)
		if !ok {
			return semanticir.Expression{}, false
		}
		op := mapOperator(expr.Text)
		typeOf := semanticir.TypeInteger
		if op == semanticir.OpNot {
			typeOf = semanticir.TypeBool
		}
		l.translated++
		return semanticir.Expression{Kind: semanticir.ExprUnary, Type: typeOf, Operator: op, Operands: []semanticir.Expression{operand}, Provenance: prov}, true
	case expressionBinary:
		left, leftOK := l.lowerExpression(expr.Children[0], bindings)
		right, rightOK := l.lowerExpression(expr.Children[1], bindings)
		if !leftOK || !rightOK {
			return semanticir.Expression{}, false
		}
		op := mapOperator(expr.Text)
		kind, typeOf := semanticir.ExprBinary, semanticir.TypeInteger
		if isComparison(expr.Text) {
			kind, typeOf = semanticir.ExprCompare, semanticir.TypeBool
		} else if expr.Text == "&&" || expr.Text == "||" {
			kind, typeOf = semanticir.ExprBool, semanticir.TypeBool
		}
		l.translated++
		return semanticir.Expression{Kind: kind, Type: typeOf, Operator: op, Operands: []semanticir.Expression{left, right}, Provenance: prov}, true
	case expressionRange:
		left, leftOK := l.lowerExpression(expr.Children[0], bindings)
		right, rightOK := l.lowerExpression(expr.Children[1], bindings)
		if !leftOK || !rightOK || left.Type != semanticir.TypeInteger || right.Type != semanticir.TypeInteger {
			l.block(expr.Span, semanticir.DiagnosticUnsupported, "bounded Rust range requires integer endpoints")
			return semanticir.Expression{}, false
		}
		l.translated++
		return semanticir.Expression{Kind: semanticir.ExprCall, Type: semanticir.TypeSequence, Name: "std::ops::Range" + expr.Text, Operands: []semanticir.Expression{left, right}, Provenance: prov}, true
	case expressionCall:
		if expr.Text != "Ok" && expr.Text != "Err" && expr.Text != "Result::Ok" && expr.Text != "Result::Err" {
			if _, exists := l.functions[expr.Text]; !exists {
				if l.request.Kind != semanticir.ArtifactTests || !containsString(l.request.EntryPoints, expr.Text) {
					l.block(expr.Span, semanticir.DiagnosticInvalidReference, fmt.Sprintf("call target %q is neither source-defined nor a declared entry point", expr.Text))
					return semanticir.Expression{}, false
				}
			}
		}
		operands := make([]semanticir.Expression, 0, len(expr.Children))
		for _, child := range expr.Children {
			operand, ok := l.lowerExpression(child, bindings)
			if !ok {
				return semanticir.Expression{}, false
			}
			operands = append(operands, operand)
		}
		typeOf := l.returnTypes[expr.Text]
		if typeOf == "" {
			typeOf = semanticir.TypeUnknown
		}
		if expr.Text == "Ok" || expr.Text == "Err" || expr.Text == "Result::Ok" || expr.Text == "Result::Err" {
			typeOf = semanticir.TypeUnknown
			if len(operands) == 1 {
				typeOf = operands[0].Type
			}
		}
		l.translated++
		return semanticir.Expression{Kind: semanticir.ExprCall, Type: typeOf, Name: expr.Text, Operands: operands, Provenance: prov}, true
	default:
		l.block(expr.Span, semanticir.DiagnosticUnsupported, "control-flow expression cannot be embedded in a scalar Semantic IR expression")
		return semanticir.Expression{}, false
	}
}

func (l *lowerer) panicMessage(expr expression, bindings map[string]semanticir.Expression) (string, bool) {
	if len(expr.Children) != 1 {
		l.block(expr.Span, semanticir.DiagnosticUnsupported, "panic! requires one static message in the bounded subset")
		return "", false
	}
	literal, ok := parseRustLiteral(expr.Children[0].Text)
	if expr.Children[0].Kind != expressionLiteral || !ok || literal.Type != semanticir.TypeString {
		l.block(expr.Span, semanticir.DiagnosticUnsupported, "panic! message must be a static string literal")
		return "", false
	}
	return literal.String, true
}

func (l *lowerer) block(span sourceSpan, code semanticir.DiagnosticCode, message string) {
	l.diagnostics = append(l.diagnostics, diagnostic(l.request.Artifact, span, code, message))
}

func (l *lowerer) prov(span sourceSpan) semanticir.Provenance {
	return provenance(l.request.Artifact, span, semanticir.TranslationTranslated)
}

func mapOperator(op string) semanticir.Operator {
	return map[string]semanticir.Operator{
		"!": semanticir.OpNot, "-": semanticir.OpNeg,
		"+": semanticir.OpAdd, "*": semanticir.OpMul, "/": semanticir.OpDiv, "%": semanticir.OpMod,
		"==": semanticir.OpEQ, "!=": semanticir.OpNE, "<": semanticir.OpLT, "<=": semanticir.OpLE, ">": semanticir.OpGT, ">=": semanticir.OpGE,
		"&&": semanticir.OpAnd, "||": semanticir.OpOr,
	}[op]
}

func isComparison(op string) bool {
	return op == "==" || op == "!=" || op == "<" || op == "<=" || op == ">" || op == ">="
}

func isAssertionMacro(name string) bool {
	return name == "assert" || name == "assert_eq" || name == "assert_ne"
}

func cloneBindings(source map[string]semanticir.Expression) map[string]semanticir.Expression {
	result := make(map[string]semanticir.Expression, len(source))
	for name, value := range source {
		result[name] = value
	}
	return result
}

func findDomainID(request semanticir.FrontendRequest, function, parameter string) string {
	operation, exists := requestOperation(request.Operations, function)
	if !exists {
		return ""
	}
	for index, input := range operation.Inputs {
		if input.Name == parameter {
			if input.DomainID != "" {
				return input.DomainID
			}
			if len(operation.DomainIDs) == len(operation.Inputs) {
				return operation.DomainIDs[index]
			}
			return ""
		}
	}
	return ""
}

func rustValueType(typeName string) (semanticir.ValueType, bool) {
	compact := strings.ReplaceAll(typeName, " ", "")
	compact = strings.TrimPrefix(compact, "&")
	if strings.HasPrefix(compact, "'") {
		if index := strings.Index(compact, "str"); index >= 0 {
			compact = compact[index:]
		}
	}
	switch compact {
	case "bool":
		return semanticir.TypeBool, true
	case "i8", "i16", "i32", "i64", "isize", "u8", "u16", "u32", "u64", "usize":
		return semanticir.TypeInteger, true
	case "str", "String":
		return semanticir.TypeString, true
	case "()":
		return semanticir.TypeUnit, true
	default:
		return semanticir.TypeUnknown, false
	}
}

func rustResultType(typeName string) (semanticir.ValueType, bool) {
	compact := strings.ReplaceAll(typeName, " ", "")
	if !strings.HasPrefix(compact, "Result<") || !strings.HasSuffix(compact, ">") {
		return semanticir.TypeUnknown, false
	}
	inner := compact[len("Result<") : len(compact)-1]
	parts := splitTopLevelString(inner, ',')
	if len(parts) != 2 {
		return semanticir.TypeUnknown, false
	}
	typeOf, ok := rustValueType(parts[0])
	return typeOf, ok
}

func parseRustLiteral(text string) (semanticir.Literal, bool) {
	if text == "true" || text == "false" {
		return semanticir.Literal{Type: semanticir.TypeBool, Bool: text == "true"}, true
	}
	if strings.HasPrefix(text, "\"") {
		value, err := strconv.Unquote(text)
		return semanticir.Literal{Type: semanticir.TypeString, String: value}, err == nil
	}
	if strings.HasPrefix(text, "'") {
		value, err := strconv.Unquote(text)
		return semanticir.Literal{Type: semanticir.TypeString, String: value}, err == nil
	}
	clean := strings.ReplaceAll(text, "_", "")
	for _, suffix := range []string{"isize", "usize", "i64", "u64", "i32", "u32", "i16", "u16", "i8", "u8"} {
		clean = strings.TrimSuffix(clean, suffix)
	}
	value, err := strconv.ParseInt(clean, 0, 64)
	return semanticir.Literal{Type: semanticir.TypeInteger, Integer: value}, err == nil
}

func literalBool(value bool, prov semanticir.Provenance) semanticir.Expression {
	literal := semanticir.Literal{Type: semanticir.TypeBool, Bool: value}
	return semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeBool, Literal: &literal, Provenance: prov}
}

func splitVariantPattern(pattern string) (string, string, bool) {
	open := strings.IndexByte(pattern, '(')
	if open <= 0 || !strings.HasSuffix(pattern, ")") {
		return "", "", false
	}
	variant := pattern[:open]
	binding := pattern[open+1 : len(pattern)-1]
	if !isRustIdentifier(variant) || !isRustIdentifier(binding) {
		return "", "", false
	}
	return variant, binding, true
}

func isRustIdentifier(value string) bool {
	if value == "_" {
		return true
	}
	for i, r := range value {
		if i == 0 && !isIdentStart(r) || i > 0 && !isIdentContinue(r) {
			return false
		}
	}
	return value != ""
}

func expressionFromPattern(pattern string, span sourceSpan) expression {
	kind := expressionLiteral
	if pattern == "true" || pattern == "false" {
		kind = expressionIdentifier
	}
	return expression{Kind: kind, Text: pattern, Span: span}
}

func splitTopLevelString(value string, separator byte) []string {
	depth := 0
	last := 0
	var result []string
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '<', '(', '[':
			depth++
		case '>', ')', ']':
			depth--
		default:
			if value[i] == separator && depth == 0 {
				result = append(result, value[last:i])
				last = i + 1
			}
		}
	}
	return append(result, value[last:])
}

func provenance(artifact semanticir.ArtifactRef, span sourceSpan, status semanticir.TranslationStatus) semanticir.Provenance {
	endLine, endColumn := span.End.Line, span.End.Column
	if span.End.Offset > span.Start.Offset && endColumn > 1 {
		endColumn--
	}
	location := semanticir.SourceLocation{Path: artifact.Path, StartLine: span.Start.Line, StartColumn: span.Start.Column, EndLine: endLine, EndColumn: endColumn}
	return semanticir.NewProvenance(artifact, location, status)
}

func wholeSpan(source []byte) sourceSpan {
	line, column := 1, 1
	for _, b := range source {
		if b == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	return sourceSpan{Start: sourcePos{Line: 1, Column: 1}, End: sourcePos{Offset: len(source), Line: line, Column: column}}
}

func diagnostic(artifact semanticir.ArtifactRef, span sourceSpan, code semanticir.DiagnosticCode, message string) semanticir.Diagnostic {
	return semanticir.Diagnostic{Severity: semanticir.SeverityError, Code: code, Message: message, Provenance: provenance(artifact, span, semanticir.TranslationUnsupported)}
}

func diagnosticsToUnsupported(diagnostics []semanticir.Diagnostic) []semanticir.UnsupportedConstruct {
	result := make([]semanticir.UnsupportedConstruct, 0, len(diagnostics))
	for _, item := range diagnostics {
		if item.Severity == semanticir.SeverityError {
			result = append(result, semanticir.UnsupportedConstruct{Kind: string(item.Code), Reason: item.Message, Provenance: item.Provenance})
		}
	}
	return result
}

func deduplicateDiagnostics(diagnostics []semanticir.Diagnostic) []semanticir.Diagnostic {
	seen := make(map[string]bool, len(diagnostics))
	result := make([]semanticir.Diagnostic, 0, len(diagnostics))
	for _, item := range diagnostics {
		key := fmt.Sprintf("%s\x00%s\x00%d\x00%d", item.Code, item.Message, item.Provenance.Location.StartLine, item.Provenance.Location.StartColumn)
		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}
	return result
}

func populateAcceptedOutcomes(model *semanticir.ArtifactModel) {
	outcomeByID := make(map[string]semanticir.ObservableOutcome, len(model.Outcomes))
	for _, outcome := range model.Outcomes {
		outcomeByID[outcome.ID] = outcome
	}
	byOperation := make(map[string][]semanticir.ObservableOutcome)
	for _, operation := range model.Operations {
		for _, outcomeID := range operation.OutcomeIDs {
			if outcome, ok := outcomeByID[outcomeID]; ok {
				byOperation[operation.ID] = append(byOperation[operation.ID], outcome)
			}
		}
	}
	for testIndex := range model.Tests {
		test := &model.Tests[testIndex]
		for _, assertion := range test.Assertions {
			for _, outcome := range byOperation[test.OperationID] {
				if assertionAcceptsOutcome(assertion, outcome) {
					test.AcceptedOutcomes = appendUnique(test.AcceptedOutcomes, outcome.ID)
				}
			}
		}
	}
}

func populateOperationUniverses(model *semanticir.ArtifactModel) {
	byOperation := make(map[string][]string)
	for _, behaviorCase := range model.Cases {
		for _, outcomeID := range behaviorCase.OutcomeIDs {
			byOperation[behaviorCase.OperationID] = appendUnique(byOperation[behaviorCase.OperationID], outcomeID)
		}
	}
	for index := range model.Operations {
		model.Operations[index].OutcomeIDs = byOperation[model.Operations[index].ID]
	}
}

func assertionAcceptsOutcome(assertion semanticir.Assertion, outcome semanticir.ObservableOutcome) bool {
	if assertion.Kind == semanticir.AssertRaises {
		return outcome.Kind == semanticir.OutcomeRaise && outcome.ExceptionType == assertion.ExceptionType && (assertion.Message == "" || outcome.Message == assertion.Message)
	}
	if outcome.Value == nil || assertion.Expected == nil || assertion.Expected.Literal == nil {
		return false
	}
	equal := reflect.DeepEqual(*outcome.Value, *assertion.Expected.Literal)
	if assertion.Kind == semanticir.AssertEqual {
		return equal
	}
	if assertion.Kind == semanticir.AssertNotEqual {
		return !equal
	}
	return false
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
