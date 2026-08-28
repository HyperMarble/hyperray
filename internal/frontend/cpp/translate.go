// Package cpp translates bounded C++ artifacts through pinned Clang evidence
// into Ray Semantic IR. Translation is deliberately fail-closed: AST data is
// used only for typed spans/advisory lowering, while executable meaning comes
// from compiler/model-checker or exhaustive compiled-execution evidence.
package cpp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/HyperMarble/ray/internal/semanticir"
)

const maxProvenLoopIterations = 10_000

// Version identifies this frontend and its lowering contract. The concrete
// Clang version used for a translation is returned in model diagnostics when
// Clang cannot run and should be recorded by the pipeline from its environment.
const Version = "cpp-clang-llvm-v2"

// Translate implements the common language frontend contract.
func Translate(ctx context.Context, request semanticir.FrontendRequest) (semanticir.ArtifactModel, []semanticir.Diagnostic) {
	l := newLowerer(request)
	if err := ctx.Err(); err != nil {
		l.invalid(nil, fmt.Sprintf("translation context: %v", err))
		return l.model(), l.diagnostics
	}
	if !l.validateRequest() {
		return l.model(), l.diagnostics
	}

	result, err := clangAST(ctx, request.Workspace, request.Translator.Path, l.compileDirectory, l.sourcePath, l.compileFlags, astDumpFilters(request))
	if err != nil {
		l.block(nil, "clang-ast", err.Error(), semanticir.DiagnosticUnsupported)
		return l.model(), l.diagnostics
	}
	l.clangVersion = result.Version
	l.compilerWidths = result.IntegerWidths
	l.root = result.Root
	l.llvmIR = result.LLVMIR
	l.assertions = scanAssertions(request.Source)
	l.discoverOperations()
	l.enumerateCases()
	l.buildTestModels()
	l.buildAuthoritativeEvidence(ctx)
	return l.model(), l.diagnostics
}

type loweredOperation struct {
	operation  semanticir.Operation
	node       *astNode
	body       *astNode
	asserts    []assertionCall
	resultType semanticir.ValueType
}

type lowerer struct {
	request            semanticir.FrontendRequest
	root               *astNode
	clangVersion       string
	operations         []loweredOperation
	outcomes           []semanticir.ObservableOutcome
	rawReferenceCases  []semanticir.RawReferenceCase
	cases              []semanticir.BehaviorCase
	tests              []semanticir.TestModel
	compilerEvidence   []semanticir.CompilerEvidence
	exhaustiveEvidence []semanticir.ExhaustiveExecutionEvidence
	scopeClosure       *semanticir.ScopeClosureEvidence
	testProjection     *semanticir.TestObservationProjection
	runnerSelection    *semanticir.RunnerSelectionEvidence
	diagnostics        []semanticir.Diagnostic
	unsupported        []semanticir.UnsupportedConstruct
	translated         int
	total              int
	assertions         []assertionCall
	entryFound         map[string]bool
	blockedKeys        map[string]bool
	compileFlags       []string
	compileDirectory   string
	sourcePath         string
	llvmIR             string
	compilerWidths     map[string]int
	integerBits        map[string]int
	currentOperation   *semanticir.Operation
	localExpressions   map[string]semanticir.Expression
	inliningCalls      map[string]bool
	declScopes         map[string][]string
	caseRawOutcomes    map[string]semanticir.ObservableOutcome
}

func newLowerer(request semanticir.FrontendRequest) *lowerer {
	return &lowerer{
		request:          request,
		entryFound:       make(map[string]bool),
		blockedKeys:      make(map[string]bool),
		integerBits:      make(map[string]int),
		localExpressions: make(map[string]semanticir.Expression),
		inliningCalls:    make(map[string]bool),
		declScopes:       make(map[string][]string),
		caseRawOutcomes:  make(map[string]semanticir.ObservableOutcome),
	}
}

func (l *lowerer) model() semanticir.ArtifactModel {
	status := semanticir.TranslationComplete
	if len(l.unsupported) > 0 || semanticir.HasErrors(l.diagnostics) {
		status = semanticir.TranslationBlocked
	}
	outcomes := append([]semanticir.ObservableOutcome(nil), l.outcomes...)
	if len(l.request.Outcomes) > 0 {
		outcomes = append([]semanticir.ObservableOutcome(nil), l.request.Outcomes...)
	}
	return semanticir.ArtifactModel{
		Artifact:           l.request.Artifact,
		Language:           semanticir.LanguageCPP,
		Kind:               l.request.Kind,
		Domains:            append([]semanticir.Domain(nil), l.request.FiniteDomains...),
		Groundings:         append([]semanticir.AssignmentGrounding(nil), l.request.Groundings...),
		Constraints:        append([]semanticir.Constraint(nil), l.request.Constraints...),
		Operations:         operationModels(l.operations),
		Outcomes:           outcomes,
		RawReferenceCases:  append([]semanticir.RawReferenceCase(nil), l.rawReferenceCases...),
		Cases:              append([]semanticir.BehaviorCase(nil), l.cases...),
		Invariants:         []semanticir.Invariant{},
		Tests:              append([]semanticir.TestModel(nil), l.tests...),
		CompilerEvidence:   append([]semanticir.CompilerEvidence(nil), l.compilerEvidence...),
		ExhaustiveEvidence: append([]semanticir.ExhaustiveExecutionEvidence(nil), l.exhaustiveEvidence...),
		ScopeClosure:       l.scopeClosure,
		TestProjection:     l.testProjection,
		RunnerSelection:    l.runnerSelection,
		Coverage: semanticir.TranslationCoverage{
			Status:               status,
			TotalConstructs:      l.total,
			TranslatedConstructs: l.translated,
			Unsupported:          append([]semanticir.UnsupportedConstruct(nil), l.unsupported...),
			Provenance:           l.provenance(nil, status),
		},
		Translator: l.request.Translator,
	}
}

func operationModels(operations []loweredOperation) []semanticir.Operation {
	result := make([]semanticir.Operation, 0, len(operations))
	for _, operation := range operations {
		result = append(result, operation.operation)
	}
	return result
}

func (l *lowerer) validateRequest() bool {
	valid := true
	if len(l.request.Options) != 0 {
		l.invalid(nil, "C++ frontend options are not semantic authority; all bounded mappings must come from typed operations, domains, constraints, and compiler evidence")
		valid = false
	}
	if l.request.Kind == semanticir.ArtifactCode && len(l.request.Constraints) != 0 {
		l.block(nil, "constraint-reachability", "C++ constraint exclusions require compiler-derived no-path evidence; the exhaustive free-function subset does not infer exclusions from Spec", semanticir.DiagnosticUnsupported)
		valid = false
	}
	if l.request.Kind == semanticir.ArtifactCode && !l.validateChangedRanges() {
		valid = false
	}
	if l.request.Language != semanticir.LanguageCPP {
		l.invalid(nil, fmt.Sprintf("C++ frontend received language %q", l.request.Language))
		valid = false
	}
	if l.request.Kind != semanticir.ArtifactCode && l.request.Kind != semanticir.ArtifactTests {
		l.invalid(nil, fmt.Sprintf("C++ frontend only translates code or tests, got %q", l.request.Kind))
		valid = false
	}
	if l.request.Artifact.Kind != l.request.Kind {
		l.invalid(nil, fmt.Sprintf("request kind %q does not match artifact kind %q", l.request.Kind, l.request.Artifact.Kind))
		valid = false
	}
	if l.request.Artifact.ID == "" || l.request.Artifact.Path == "" {
		l.invalid(nil, "artifact id and path are required")
		valid = false
	}
	if len(l.request.Source) == 0 {
		l.invalid(nil, "C++ source is empty")
		valid = false
	}
	sum := sha256.Sum256(l.request.Source)
	want := "sha256:" + hex.EncodeToString(sum[:])
	if l.request.Artifact.Digest == "" || l.request.Artifact.Digest != want {
		l.diagnostic(nil, semanticir.DiagnosticStaleArtifact, fmt.Sprintf("artifact digest mismatch: declared %q, source is %q", l.request.Artifact.Digest, want))
		valid = false
	}
	if !l.validateTranslator() {
		valid = false
	}
	if !l.validateWorkspaceCompilation() {
		valid = false
	}
	if !l.validateProver() {
		valid = false
	}
	return valid
}

func (l *lowerer) validateProver() bool {
	declared := l.request.Prover
	if declared.Name == "" || declared.Path == "" || declared.Digest == "" || declared.Version == "" || !filepath.IsAbs(declared.Path) {
		l.invalid(nil, "C++ prover name, absolute path, sha256 digest, and version are required")
		return false
	}
	observed, err := pinnedZ3(context.Background(), l.request.Workspace)
	if err != nil {
		l.invalid(nil, fmt.Sprintf("resolve exact frozen C++ prover: %v", err))
		return false
	}
	if observed != declared {
		l.diagnostic(nil, semanticir.DiagnosticStaleArtifact, fmt.Sprintf("declared C++ prover %+v differs from frozen executable %+v", declared, observed))
		return false
	}
	return true
}

func (l *lowerer) validateTranslator() bool {
	tool := l.request.Translator
	if tool.Name == "" || tool.Path == "" || tool.Digest == "" || tool.Version == "" {
		l.invalid(nil, "translator name, absolute path, sha256 digest, and version are required")
		return false
	}
	if tool.Name != "clang++" {
		l.invalid(nil, fmt.Sprintf("C++ translator name must be clang++, got %q", tool.Name))
		return false
	}
	if !filepath.IsAbs(tool.Path) {
		l.invalid(nil, "translator path must be absolute")
		return false
	}
	bytes, err := os.ReadFile(tool.Path)
	if err != nil {
		l.invalid(nil, fmt.Sprintf("read pinned translator %q: %v", tool.Path, err))
		return false
	}
	sum := sha256.Sum256(bytes)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	if digest != tool.Digest {
		l.diagnostic(nil, semanticir.DiagnosticStaleArtifact, fmt.Sprintf("translator digest mismatch: declared %q, executable is %q", tool.Digest, digest))
		return false
	}
	actualVersion := clangVersion(context.Background(), l.request.Workspace, tool.Path)
	if actualVersion != tool.Version {
		l.diagnostic(nil, semanticir.DiagnosticStaleArtifact, fmt.Sprintf("translator version mismatch: declared %q, executable reports %q", tool.Version, actualVersion))
		return false
	}
	return true
}

func validateCompileFlags(flags []string) error {
	for index, flag := range flags {
		if flag == "" {
			return fmt.Errorf("compile.flags[%d] is empty", index)
		}
		if flag == "-c" || flag == "-E" || flag == "-S" || flag == "-o" || flag == "-load" || flag == "-plugin" || strings.HasPrefix(flag, "-fplugin") || strings.HasPrefix(flag, "-MJ") {
			return fmt.Errorf("compile flag %q may change compiler action or load uncontrolled code", flag)
		}
		if flag == "-Xclang" {
			if index+1 >= len(flags) {
				return fmt.Errorf("compile flag -Xclang has no argument")
			}
			next := flags[index+1]
			if next == "-load" || next == "-plugin" || strings.Contains(next, "ast-dump") {
				return fmt.Errorf("compile -Xclang argument %q conflicts with the pinned frontend", next)
			}
		}
		if !strings.HasPrefix(flag, "-") {
			valueForPriorFlag := index > 0 && (flags[index-1] == "-I" || flags[index-1] == "-isystem" || flags[index-1] == "-include" || flags[index-1] == "-iquote" || flags[index-1] == "-D" || flags[index-1] == "-U" || flags[index-1] == "-target" || flags[index-1] == "--target" || flags[index-1] == "-isysroot" || flags[index-1] == "--sysroot")
			if !valueForPriorFlag {
				return fmt.Errorf("compile flag vector contains an uncontrolled input path %q", flag)
			}
		}
	}
	return nil
}

func (l *lowerer) discoverOperations() {
	l.indexDeclarationScopes(l.root, nil)
	var visit func(*astNode, []string, bool)
	visit = func(node *astNode, scope []string, insideTemplate bool) {
		if node == nil {
			return
		}
		sourceOwned := l.sourceOwned(node)
		switch node.Kind {
		case "FunctionTemplateDecl", "ClassTemplateDecl", "ClassTemplatePartialSpecializationDecl", "VarTemplateDecl":
			if sourceOwned {
				l.block(node, "unexpanded-template", "templates are not part of the bounded C++ frontend")
			}
			insideTemplate = true
		case "NamespaceDecl", "CXXRecordDecl", "ClassTemplateSpecializationDecl":
			if node.Name != "" && !node.IsImplicit {
				scope = append(append([]string(nil), scope...), node.Name)
			}
		case "FunctionDecl", "CXXMethodDecl":
			if insideTemplate || node.IsImplicit || !sourceOwned {
				return
			}
			body := compoundBody(node)
			if body == nil { // A declaration without a definition has no behavior to lower.
				return
			}
			operationScope := scope
			if len(operationScope) == 0 && node.ParentDeclContextID != "" {
				operationScope = l.declScopes[node.ParentDeclContextID]
			}
			id := strings.Join(append(append([]string(nil), operationScope...), node.Name), "::")
			if len(operationScope) == 0 {
				if requested := l.requestedQualifiedOperation(node.Name); requested != "" {
					id = requested
				}
			}
			if id == "" {
				l.block(node, "unnamed-operation", "callable has no stable name")
				return
			}
			if l.request.Kind == semanticir.ArtifactTests {
				hasAssertions := len(l.assertionsWithin(node.sourceRange())) > 0
				if !hasAssertions && !l.explicitlySelected(id, node.Name) {
					return
				}
			} else if !l.selected(id, node.Name) {
				return
			}
			l.entryFound[id] = true
			l.entryFound[node.Name] = true
			l.lowerOperation(id, node, body)
			return
		case "CXXConstructorDecl", "CXXDestructorDecl", "CXXConversionDecl":
			if sourceOwned && compoundBody(node) != nil && !insideTemplate {
				l.block(node, node.Kind, "constructors, destructors, and conversion functions have implicit object semantics not present in the IR")
			}
			return
		}
		for _, child := range node.Inner {
			visit(child, scope, insideTemplate)
		}
	}
	visit(l.root, nil, false)

	if l.request.Kind == semanticir.ArtifactCode {
		for _, requested := range l.request.EntryPoints {
			if !l.entryFound[requested] {
				l.block(nil, "missing-entry-point", fmt.Sprintf("entry point %q was not defined in the frozen source", requested), semanticir.DiagnosticInvalidReference)
			}
		}
	}
	if len(l.operations) == 0 && len(l.unsupported) == 0 {
		l.block(nil, "empty-translation", "no bounded C++ function or method definitions were selected", semanticir.DiagnosticIncomplete)
	}
}

func (l *lowerer) requestedQualifiedOperation(name string) string {
	match := ""
	for _, entry := range l.request.EntryPoints {
		if shortName(entry) != name || !strings.Contains(entry, "::") {
			continue
		}
		if match != "" && match != entry {
			return ""
		}
		match = entry
	}
	return match
}

var cxxDefinitionName = regexp.MustCompile(`(?m)([A-Za-z_][A-Za-z0-9_]*)\s*\([^\n;{}]*\)\s*(?:const\s*)?\{`)

func astDumpFilters(request semanticir.FrontendRequest) []string {
	seen := make(map[string]bool)
	var filters []string
	add := func(filter string) {
		filter = strings.TrimSpace(filter)
		if filter != "" && !seen[filter] {
			seen[filter] = true
			filters = append(filters, filter)
		}
	}
	for _, entry := range request.EntryPoints {
		if index := strings.LastIndex(entry, "::"); index >= 0 {
			add(entry[:index+2])
		} else {
			add(entry)
		}
	}
	if request.Kind == semanticir.ArtifactTests || len(request.EntryPoints) == 0 {
		for _, match := range cxxDefinitionName.FindAllSubmatch(request.Source, -1) {
			name := string(match[1])
			switch name {
			case "if", "for", "while", "switch", "catch":
				continue
			default:
				add(name)
			}
		}
	}
	return filters
}

// indexDeclarationScopes resolves out-of-line method definitions through
// Clang's parentDeclContextId. Pointer-shaped Clang IDs are used only for this
// within-one-AST join and never escape as stable evidence.
func (l *lowerer) indexDeclarationScopes(node *astNode, scope []string) {
	if node == nil {
		return
	}
	next := scope
	switch node.Kind {
	case "NamespaceDecl", "CXXRecordDecl", "ClassTemplateSpecializationDecl":
		if node.Name != "" && !node.IsImplicit {
			next = append(append([]string(nil), scope...), node.Name)
		}
	}
	if node.ID != "" {
		l.declScopes[node.ID] = append([]string(nil), next...)
	}
	for _, child := range node.Inner {
		l.indexDeclarationScopes(child, next)
	}
}

func (l *lowerer) selected(id, name string) bool {
	if len(l.request.EntryPoints) == 0 {
		return true
	}
	return l.explicitlySelected(id, name)
}

func (l *lowerer) explicitlySelected(id, name string) bool {
	for _, entry := range l.request.EntryPoints {
		if entry == id || entry == name {
			return true
		}
	}
	return false
}

func compoundBody(node *astNode) *astNode {
	for i := len(node.Inner) - 1; i >= 0; i-- {
		if node.Inner[i] != nil && node.Inner[i].Kind == "CompoundStmt" {
			return node.Inner[i]
		}
	}
	return nil
}

func (l *lowerer) lowerOperation(id string, node, body *astNode) {
	kind := semanticir.OperationFunction
	if node.Kind == "CXXMethodDecl" {
		kind = semanticir.OperationMethod
	}
	if l.request.Kind == semanticir.ArtifactTests {
		kind = semanticir.OperationTest
	}
	op := semanticir.Operation{ID: id, Kind: kind, Inputs: []semanticir.Variable{}, Body: []semanticir.Statement{}, Provenance: l.provenance(node, semanticir.TranslationTranslated)}
	op.DomainIDs = []string{}
	op.OutcomeIDs = []string{}
	declared, hasDeclared := l.declaredOperation(id)
	if hasDeclared {
		op.OutcomeIDs = append([]string(nil), declared.OutcomeIDs...)
		if len(declared.Inputs) != 0 {
			op.DomainIDs = append([]string(nil), declared.DomainIDs...)
		}
	}
	strictInputs := hasDeclared && len(declared.Inputs) != 0
	for _, child := range node.Inner {
		if child == nil || child.Kind != "ParmVarDecl" {
			continue
		}
		typeName, ok := l.valueType(child.Type.QualType)
		if !ok || typeName == semanticir.TypeUnit {
			l.block(child, "unsupported-parameter-type", fmt.Sprintf("parameter %q has unsupported C++ type %q", child.Name, child.Type.QualType))
			continue
		}
		domainID := ""
		if strictInputs {
			matched := false
			for _, input := range declared.Inputs {
				if input.Name != child.Name {
					continue
				}
				matched = true
				domainID = input.DomainID
				if input.Type != typeName {
					l.block(child, "operation-input-type", fmt.Sprintf("operation %q input %q compiler type %q differs from declared type %q", id, child.Name, typeName, input.Type), semanticir.DiagnosticInvalidReference)
				}
				break
			}
			if !matched {
				l.block(child, "operation-input", fmt.Sprintf("operation %q compiler parameter %q is absent from the declared input list", id, child.Name), semanticir.DiagnosticInvalidReference)
			}
		} else {
			domainID = l.domainFor(id, child.Name)
			if domainID == "" {
				l.block(child, "missing-finite-domain", fmt.Sprintf("parameter %s.%s has no finite domain", id, child.Name), semanticir.DiagnosticMissingDomain)
			}
		}
		op.Inputs = append(op.Inputs, semanticir.Variable{Name: child.Name, Type: typeName, DomainID: domainID, Provenance: l.provenance(child, semanticir.TranslationTranslated)})
		l.recordIntegerWidth(op.Inputs[len(op.Inputs)-1].Provenance, child.Type.QualType)
		if domainID != "" && !strictInputs {
			op.DomainIDs = append(op.DomainIDs, domainID)
		}
		l.accept()
	}

	operationAssertions := l.assertionsWithin(node.sourceRange())
	l.currentOperation = &op
	l.localExpressions = make(map[string]semanticir.Expression)
	bodyStatements := l.lowerCompound(body, operationAssertions)
	l.currentOperation = nil
	l.localExpressions = make(map[string]semanticir.Expression)
	op.Body = bodyStatements
	if strictInputs && len(op.Inputs) != len(declared.Inputs) {
		l.block(node, "operation-inputs", fmt.Sprintf("operation %q compiler parameters do not exactly match the declared input list", id), semanticir.DiagnosticInvalidReference)
	}
	if hasDeclared && !sameStrings(op.DomainIDs, declared.DomainIDs) {
		l.block(node, "operation-domain-scope", fmt.Sprintf("operation %q compiler-derived domains %v differ from declared local domains %v", id, op.DomainIDs, declared.DomainIDs), semanticir.DiagnosticInvalidReference)
	}
	resultType, ok := l.valueType(functionReturnType(node.Type.QualType))
	if !ok {
		l.block(node, "unsupported-return-type", fmt.Sprintf("operation %q has unsupported return type in %q", id, node.Type.QualType))
	}
	l.accept()
	if err := l.validateLLVMOperation(node, op.Body); err != nil {
		l.block(node, "llvm-evidence", fmt.Sprintf("operation %q is not justified by pinned Clang LLVM IR: %v", id, err), semanticir.DiagnosticIncomplete)
	}
	l.operations = append(l.operations, loweredOperation{operation: op, node: node, body: body, asserts: operationAssertions, resultType: resultType})
}

func (l *lowerer) declaredOperation(id string) (semanticir.Operation, bool) {
	for _, operation := range l.request.Operations {
		if operation.ID == id {
			return operation, true
		}
	}
	return semanticir.Operation{}, false
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (l *lowerer) domainFor(operationID, parameter string) string {
	for _, operation := range l.request.Operations {
		if operation.ID != operationID {
			continue
		}
		for _, input := range operation.Inputs {
			if input.Name == parameter && l.hasDomain(input.DomainID) {
				return input.DomainID
			}
		}
	}
	for _, id := range []string{operationID + "." + parameter, parameter} {
		if l.hasDomain(id) {
			return id
		}
	}
	return ""
}

func (l *lowerer) hasDomain(id string) bool {
	for _, domain := range l.request.FiniteDomains {
		if domain.ID == id && len(domain.Values) > 0 {
			return true
		}
	}
	return false
}

func (l *lowerer) lowerCompound(node *astNode, assertions []assertionCall) []semanticir.Statement {
	if node == nil || node.Kind != "CompoundStmt" {
		l.block(node, "expected-compound-statement", "function body is not a compound statement")
		return nil
	}
	statements := l.lowerStatementSequence(node.Inner, node, assertions)
	l.accept()
	return statements
}

// lowerStatementSequence expands an immutable scalar conditional initializer
// into ordinary IR branches spanning its remaining lexical scope. This keeps
// the shared expression vocabulary closed while preserving exact C++ ?: path
// semantics and Clang declaration identity.
func (l *lowerer) lowerStatementSequence(nodes []*astNode, compound *astNode, assertions []assertionCall) []semanticir.Statement {
	statements := make([]semanticir.Statement, 0, len(nodes))
	for index, child := range nodes {
		if child == nil {
			continue
		}
		if assertion := l.assertionForExpansion(child, assertions); assertion != nil {
			if l.request.Kind == semanticir.ArtifactCode {
				statements = append(statements, l.lowerCodeAssertion(*assertion, compound)...)
			}
			continue
		}
		declaration, conditional, ok := scalarConditionalDeclaration(child)
		if ok {
			condition, conditionOK := l.lowerExpression(conditional.Inner[0])
			thenValue, thenOK := l.lowerExpression(conditional.Inner[1])
			elseValue, elseOK := l.lowerExpression(conditional.Inner[2])
			declaredType, typeOK := l.valueType(declaration.Type.QualType)
			if !conditionOK || !thenOK || !elseOK || !typeOK || condition.Type != semanticir.TypeBool || thenValue.Type != declaredType || elseValue.Type != declaredType {
				l.block(child, "conditional-local", fmt.Sprintf("conditional local %q does not have one exact bool condition and matching scalar branches", declaration.Name))
				return statements
			}
			saved := cloneLocalExpressions(l.localExpressions)
			l.bindLocalExpression(declaration, thenValue)
			thenBody := l.lowerStatementSequence(nodes[index+1:], compound, assertions)
			l.localExpressions = cloneLocalExpressions(saved)
			l.bindLocalExpression(declaration, elseValue)
			elseBody := l.lowerStatementSequence(nodes[index+1:], compound, assertions)
			l.localExpressions = saved
			statements = append(statements, semanticir.Statement{
				Kind: semanticir.StmtBranch, Condition: &condition, Then: thenBody, Else: elseBody,
				Effects: []semanticir.Effect{}, Provenance: l.provenance(child, semanticir.TranslationTranslated),
			})
			l.accept()
			return statements
		}
		statements = append(statements, l.lowerStatement(child, assertions)...)
	}
	return statements
}

func scalarConditionalDeclaration(node *astNode) (*astNode, *astNode, bool) {
	if node == nil || node.Kind != "DeclStmt" || len(node.Inner) != 1 || node.Inner[0] == nil || node.Inner[0].Kind != "VarDecl" {
		return nil, nil, false
	}
	declaration := node.Inner[0]
	if strings.Contains(declaration.Type.QualType, "*") || len(declaration.Inner) != 1 {
		return nil, nil, false
	}
	initializer := unwrapExpression(declaration.Inner[0])
	if initializer == nil || (initializer.Kind != "ConditionalOperator" && initializer.Kind != "BinaryConditionalOperator") || len(initializer.Inner) != 3 {
		return nil, nil, false
	}
	return declaration, initializer, true
}

func cloneLocalExpressions(values map[string]semanticir.Expression) map[string]semanticir.Expression {
	copy := make(map[string]semanticir.Expression, len(values))
	for name, value := range values {
		copy[name] = value
	}
	return copy
}

func (l *lowerer) lowerStatement(node *astNode, assertions []assertionCall) []semanticir.Statement {
	if node == nil {
		return nil
	}
	if l.isAssertionExpansion(node, assertions) {
		return nil
	}
	switch node.Kind {
	case "CompoundStmt":
		return l.lowerCompound(node, assertions)
	case "IfStmt":
		return l.lowerIf(node, assertions)
	case "SwitchStmt":
		return l.lowerSwitch(node, assertions)
	case "ReturnStmt":
		if len(node.Inner) == 1 {
			conditional := unwrapExpression(node.Inner[0])
			if conditional != nil && (conditional.Kind == "ConditionalOperator" || conditional.Kind == "BinaryConditionalOperator") {
				return l.lowerConditionalReturn(node, conditional)
			}
		}
		stmt := semanticir.Statement{Kind: semanticir.StmtReturn, Then: []semanticir.Statement{}, Else: []semanticir.Statement{}, Effects: []semanticir.Effect{}, Provenance: l.provenance(node, semanticir.TranslationTranslated)}
		if len(node.Inner) > 1 {
			l.block(node, "return-shape", "return statement has multiple values")
			return nil
		}
		if len(node.Inner) == 1 {
			value, ok := l.lowerExpression(node.Inner[0])
			if !ok {
				return nil
			}
			stmt.Value = &value
		}
		l.accept()
		return []semanticir.Statement{stmt}
	case "CXXThrowExpr":
		stmt := l.lowerThrow(node)
		return []semanticir.Statement{stmt}
	case "ExprWithCleanups":
		if len(node.Inner) == 1 && node.Inner[0] != nil && node.Inner[0].Kind == "CXXThrowExpr" {
			return l.lowerStatement(node.Inner[0], assertions)
		}
		l.block(node, node.Kind, "cleanup-bearing expression statement has unsupported lifetime semantics")
		return nil
	case "CallExpr", "CXXMemberCallExpr":
		call, ok := l.lowerExpression(node)
		if !ok {
			return nil
		}
		effect := semanticir.Effect{ID: l.effectID(node, call.Name), Kind: semanticir.EffectCall, Target: call.Name, Provenance: l.provenance(node, semanticir.TranslationTranslated)}
		l.accept()
		return []semanticir.Statement{{Kind: semanticir.StmtCall, Value: &call, Then: []semanticir.Statement{}, Else: []semanticir.Statement{}, Effects: []semanticir.Effect{effect}, Provenance: l.provenance(node, semanticir.TranslationTranslated)}}
	case "CXXOperatorCallExpr":
		if statements, ok := l.lowerOutputStatement(node); ok {
			return statements
		}
		l.block(node, node.Kind, "overloaded operator statement is not a recognized ordered output operation")
		return nil
	case "NullStmt":
		l.accept()
		return nil
	case "GCCAsmStmt", "MSAsmStmt":
		l.block(node, "inline-assembly", "inline assembly has no portable bounded semantics")
		return nil
	case "CaseStmt", "DefaultStmt", "BreakStmt", "ContinueStmt", "GotoStmt", "IndirectGotoStmt", "LabelStmt":
		l.block(node, node.Kind, "unstructured or fallthrough control flow cannot be represented by the bounded IR")
		return nil
	case "ForStmt":
		return l.lowerProvenFor(node, assertions)
	case "WhileStmt", "DoStmt", "CXXForRangeStmt":
		l.block(node, node.Kind, "loop does not have a mechanically proven finite scalar for-loop bound")
		return nil
	case "DeclStmt":
		return l.lowerLocalDeclaration(node)
	case "BinaryOperator":
		if node.Opcode == "=" {
			if l.lowerLocalAssignment(node) {
				return nil
			}
			return l.lowerAssignment(node)
		}
		l.block(node, node.Kind, "expression statement is not an exact state assignment")
		return nil
	case "CompoundAssignOperator":
		if l.lowerLocalCompoundAssignment(node) {
			return nil
		}
		l.block(node, node.Kind, "compound mutation is not a loop-local scalar binding")
		return nil
	case "UnaryOperator":
		if l.lowerLocalIncrement(node) {
			return nil
		}
		l.block(node, node.Kind, "mutation is not a loop-local scalar increment/decrement")
		return nil
	default:
		l.block(node, node.Kind, "C++ statement is not supported by the bounded frontend")
		return nil
	}
}

// lowerProvenFor unrolls the deliberately narrow C++ loop form whose exact
// initializer, condition, body mutations, and increment can all be evaluated
// as scalar expressions at translation time. Parameter-dependent bounds,
// breaks/continues, effects, and loops exceeding the explicit cap block.
// Authoritative compiled execution subsequently checks the unrolled relation
// against the pinned Clang binary for every exact input tuple.
func (l *lowerer) lowerProvenFor(node *astNode, assertions []assertionCall) []semanticir.Statement {
	if len(node.Inner) != 5 || node.Inner[0] == nil || node.Inner[0].Kind != "DeclStmt" || node.Inner[1] == nil || node.Inner[1].Kind != "" || node.Inner[2] == nil || node.Inner[3] == nil || node.Inner[4] == nil {
		l.block(node, "for-loop-shape", "for loop must have one scalar initializer, no condition variable, one condition, one increment, and one body")
		return nil
	}
	if !staticLoopBodySupported(node.Inner[4]) {
		l.block(node.Inner[4], "for-loop-body", "bounded for-loop body contains control flow, effects, calls, terminals, or non-local mutation")
		return nil
	}
	outerKeys := make(map[string]bool, len(l.localExpressions))
	for key := range l.localExpressions {
		outerKeys[key] = true
	}
	l.lowerStatement(node.Inner[0], assertions)
	if semanticir.HasErrors(l.diagnostics) {
		return nil
	}
	for iteration := 0; ; iteration++ {
		if iteration >= maxProvenLoopIterations {
			l.block(node, "for-loop-bound", fmt.Sprintf("for loop did not terminate within the proven cap of %d iterations", maxProvenLoopIterations), semanticir.DiagnosticNonFinite)
			return nil
		}
		condition, ok := l.lowerExpression(node.Inner[2])
		if !ok || condition.Type != semanticir.TypeBool {
			if ok {
				l.block(node.Inner[2], "for-loop-condition", "for-loop condition is not boolean")
			}
			return nil
		}
		value, err := l.eval(condition, map[string]runtimeValue{}, map[string]loweredOperation{}, 0)
		if err != nil || value.typeName != semanticir.TypeBool {
			l.block(node.Inner[2], "for-loop-bound", fmt.Sprintf("for-loop condition is not a compile-time decidable finite bound: %v", err), semanticir.DiagnosticNonFinite)
			return nil
		}
		if !value.b {
			break
		}
		iterationKeys := make(map[string]bool, len(l.localExpressions))
		for key := range l.localExpressions {
			iterationKeys[key] = true
		}
		if statements := l.lowerStatement(node.Inner[4], assertions); len(statements) != 0 {
			l.block(node.Inner[4], "for-loop-body", "bounded loop produced runtime statements instead of a pure scalar unrolling")
			return nil
		}
		for key := range l.localExpressions {
			if !iterationKeys[key] {
				delete(l.localExpressions, key)
			}
		}
		if statements := l.lowerStatement(node.Inner[3], assertions); len(statements) != 0 {
			l.block(node.Inner[3], "for-loop-increment", "bounded loop increment produced runtime statements")
			return nil
		}
		if semanticir.HasErrors(l.diagnostics) {
			return nil
		}
	}
	for key := range l.localExpressions {
		if !outerKeys[key] {
			delete(l.localExpressions, key)
		}
	}
	l.accept()
	return nil
}

func staticLoopBodySupported(node *astNode) bool {
	if node == nil {
		return false
	}
	switch node.Kind {
	case "CompoundStmt":
		for _, child := range node.Inner {
			if !staticLoopBodySupported(child) {
				return false
			}
		}
		return true
	case "DeclStmt", "CompoundAssignOperator", "UnaryOperator", "NullStmt":
		return true
	case "BinaryOperator":
		return node.Opcode == "=" && len(node.Inner) == 2 && node.Inner[0] != nil && node.Inner[0].Kind == "DeclRefExpr"
	default:
		return false
	}
}

func (l *lowerer) lowerConditionalReturn(statement, conditional *astNode) []semanticir.Statement {
	if conditional == nil || len(conditional.Inner) != 3 {
		l.block(statement, "conditional-return", "conditional return does not have exactly three operands")
		return nil
	}
	condition, conditionOK := l.lowerExpression(conditional.Inner[0])
	thenValue, thenOK := l.lowerExpression(conditional.Inner[1])
	elseValue, elseOK := l.lowerExpression(conditional.Inner[2])
	if !conditionOK || !thenOK || !elseOK || condition.Type != semanticir.TypeBool || thenValue.Type != elseValue.Type {
		l.block(statement, "conditional-return", "conditional return does not have one exact bool condition and matching result branches")
		return nil
	}
	provenance := l.provenance(statement, semanticir.TranslationTranslated)
	thenReturn := semanticir.Statement{Kind: semanticir.StmtReturn, Value: &thenValue, Then: []semanticir.Statement{}, Else: []semanticir.Statement{}, Effects: []semanticir.Effect{}, Provenance: l.provenance(conditional.Inner[1], semanticir.TranslationTranslated)}
	elseReturn := semanticir.Statement{Kind: semanticir.StmtReturn, Value: &elseValue, Then: []semanticir.Statement{}, Else: []semanticir.Statement{}, Effects: []semanticir.Effect{}, Provenance: l.provenance(conditional.Inner[2], semanticir.TranslationTranslated)}
	l.accept()
	return []semanticir.Statement{{Kind: semanticir.StmtBranch, Condition: &condition, Then: []semanticir.Statement{thenReturn}, Else: []semanticir.Statement{elseReturn}, Effects: []semanticir.Effect{}, Provenance: provenance}}
}

func (l *lowerer) lowerIf(node *astNode, assertions []assertionCall) []semanticir.Statement {
	if len(node.Inner) < 2 || len(node.Inner) > 3 {
		l.block(node, "if-shape", "if statements with init declarations or condition variables are unsupported")
		return nil
	}
	condition, ok := l.lowerExpression(node.Inner[0])
	if !ok {
		return nil
	}
	if condition.Type != semanticir.TypeBool {
		// Clang inserts IntegralToBoolean casts; lowerExpression preserves the
		// semantic bool type for that cast. Reaching here means it was not safe.
		l.block(node.Inner[0], "non-boolean-condition", "branch condition did not lower to bool")
		return nil
	}
	thenBody := l.lowerStatement(node.Inner[1], assertions)
	elseBody := []semanticir.Statement{}
	if len(node.Inner) == 3 {
		elseBody = l.lowerStatement(node.Inner[2], assertions)
	}
	stmt := semanticir.Statement{Kind: semanticir.StmtBranch, Condition: &condition, Then: thenBody, Else: elseBody, Effects: []semanticir.Effect{}, Provenance: l.provenance(node, semanticir.TranslationTranslated)}
	l.accept()
	return []semanticir.Statement{stmt}
}

func (l *lowerer) lowerSwitch(node *astNode, assertions []assertionCall) []semanticir.Statement {
	if len(node.Inner) != 2 || node.Inner[1] == nil || node.Inner[1].Kind != "CompoundStmt" {
		l.block(node, "switch-shape", "switch init declarations and condition variables are unsupported")
		return nil
	}
	selector, ok := l.lowerExpression(node.Inner[0])
	if !ok {
		return nil
	}
	labels := node.Inner[1].Inner
	elseBody := []semanticir.Statement{}
	type switchCase struct {
		value *astNode
		body  []*astNode
		node  *astNode
	}
	cases := make([]switchCase, 0, len(labels))
	for _, label := range labels {
		if label == nil {
			continue
		}
		switch label.Kind {
		case "CaseStmt":
			if len(label.Inner) < 2 {
				l.block(label, "empty-case", "empty/fallthrough switch cases are unsupported")
				continue
			}
			if label.Inner[1].Kind == "CaseStmt" {
				l.block(label, "fallthrough-case", "fallthrough switch cases are unsupported")
				continue
			}
			cases = append(cases, switchCase{value: label.Inner[0], body: label.Inner[1:], node: label})
		case "DefaultStmt":
			if len(label.Inner) != 1 {
				l.block(label, "default-shape", "default switch label has unsupported shape")
				continue
			}
			elseBody = l.lowerStatement(label.Inner[0], assertions)
		default:
			l.block(label, label.Kind, "switch body contains a statement outside a case/default label")
		}
	}
	for i := len(cases) - 1; i >= 0; i-- {
		literal, ok := l.lowerExpression(cases[i].value)
		if !ok {
			continue
		}
		condition := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{selector, literal}, Provenance: l.provenance(cases[i].node, semanticir.TranslationTranslated)}
		thenBody := make([]semanticir.Statement, 0)
		for _, statement := range cases[i].body {
			thenBody = append(thenBody, l.lowerStatement(statement, assertions)...)
		}
		branch := semanticir.Statement{Kind: semanticir.StmtBranch, Condition: &condition, Then: thenBody, Else: elseBody, Effects: []semanticir.Effect{}, Provenance: l.provenance(cases[i].node, semanticir.TranslationTranslated)}
		elseBody = []semanticir.Statement{branch}
		l.accept()
	}
	l.accept()
	return elseBody
}

func (l *lowerer) lowerThrow(node *astNode) semanticir.Statement {
	stmt := semanticir.Statement{Kind: semanticir.StmtRaise, Then: []semanticir.Statement{}, Else: []semanticir.Statement{}, Effects: []semanticir.Effect{}, Provenance: l.provenance(node, semanticir.TranslationTranslated)}
	if len(node.Inner) == 0 {
		l.block(node, "rethrow", "rethrow depends on ambient exception state")
		return stmt
	}
	if len(node.Inner) != 1 {
		l.block(node, "throw-shape", "throw expression has unsupported shape")
		return stmt
	}
	valueNode := unwrapExpression(node.Inner[0])
	stmt.ExceptionType = normalizedExceptionType(valueNode.Type.QualType)
	if stmt.ExceptionType == "" {
		stmt.ExceptionType = normalizedExceptionType(node.Inner[0].Type.QualType)
	}
	if messageNode := throwableMessageNode(valueNode); messageNode != nil {
		if expression, ok := l.lowerExpression(messageNode); ok && expression.Type == semanticir.TypeString {
			stmt.Value = &expression
		} else {
			l.block(node, "exception-message", "exception message is not an exact bounded string expression")
		}
	} else if expr, ok := l.lowerThrowable(valueNode); ok {
		stmt.Value = &expr
	}
	l.accept()
	return stmt
}

func throwableMessageNode(node *astNode) *astNode {
	if node == nil {
		return nil
	}
	if node.Kind == "CXXConstructExpr" || node.Kind == "CXXTemporaryObjectExpr" {
		for _, child := range node.Inner {
			candidate := unwrapExpression(child)
			if candidate != nil && (strings.Contains(candidate.Type.QualType, "string") || candidate.Kind == "StringLiteral" || candidate.Kind == "CXXOperatorCallExpr") {
				return child
			}
		}
	}
	for _, child := range node.Inner {
		if candidate := throwableMessageNode(child); candidate != nil {
			return candidate
		}
	}
	return nil
}

func (l *lowerer) lowerThrowable(node *astNode) (semanticir.Expression, bool) {
	if node == nil {
		return semanticir.Expression{}, false
	}
	if node.Kind == "CXXConstructExpr" || node.Kind == "CXXTemporaryObjectExpr" || node.Kind == "CXXFunctionalCastExpr" {
		operands := make([]semanticir.Expression, 0)
		for _, child := range node.Inner {
			if expr, ok := l.lowerExpression(child); ok {
				operands = append(operands, expr)
			}
		}
		return semanticir.Expression{Kind: semanticir.ExprCall, Type: semanticir.TypeUnknown, Name: normalizedExceptionType(node.Type.QualType), Operands: operands, Provenance: l.provenance(node, semanticir.TranslationTranslated)}, true
	}
	return l.lowerExpression(node)
}

func (l *lowerer) lowerExpression(node *astNode) (semanticir.Expression, bool) {
	if node == nil {
		return semanticir.Expression{}, false
	}
	prov := l.provenance(node, semanticir.TranslationTranslated)
	switch node.Kind {
	case "ImplicitCastExpr":
		if len(node.Inner) != 1 {
			l.block(node, "implicit-cast-shape", "implicit cast has unsupported shape")
			return semanticir.Expression{}, false
		}
		if node.CastKind == "PointerToBoolean" {
			if expression, ok := l.localPointerTruth(node.Inner[0]); ok {
				expression.Provenance = prov
				l.accept()
				return expression, true
			}
			l.block(node, "uncontrolled-pointer", "pointer-to-bool conversion has no exact frozen lookup binding")
			return semanticir.Expression{}, false
		}
		expr, ok := l.lowerExpression(node.Inner[0])
		if !ok {
			return semanticir.Expression{}, false
		}
		if node.CastKind == "IntegralToBoolean" || node.Type.QualType == "bool" {
			expr.Type = semanticir.TypeBool
		}
		expr.Provenance = prov
		// Clang normally wraps scalar reads in an ImplicitCastExpr (for
		// example LValueToRValue).  A substituted immutable local therefore
		// acquires the wrapper's source span here.  Preserve the compiler's
		// exact integer width at that new provenance key so checked arithmetic
		// cannot silently lose its UB bound during substitution.
		l.recordIntegerWidth(prov, node.Type.QualType)
		l.accept()
		return expr, true
	case "ParenExpr", "ExprWithCleanups", "MaterializeTemporaryExpr", "CXXBindTemporaryExpr", "ConstantExpr", "FullExpr":
		if len(node.Inner) != 1 {
			l.block(node, node.Kind, "expression wrapper has unsupported shape")
			return semanticir.Expression{}, false
		}
		expr, ok := l.lowerExpression(node.Inner[0])
		if ok {
			expr.Provenance = prov
			l.recordIntegerWidth(prov, node.Type.QualType)
			l.accept()
		}
		return expr, ok
	case "IntegerLiteral", "CharacterLiteral":
		value, err := strconv.ParseInt(string(node.Value), 0, 64)
		if err != nil {
			l.block(node, "integer-literal", fmt.Sprintf("integer literal %q is outside signed 64-bit bounded IR", node.Value))
			return semanticir.Expression{}, false
		}
		l.accept()
		return semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: value}, Operands: []semanticir.Expression{}, Provenance: prov}, true
	case "CXXBoolLiteralExpr":
		value := node.Value == "true" || node.Value == "1"
		l.accept()
		return semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeBool, Literal: &semanticir.Literal{Type: semanticir.TypeBool, Bool: value}, Operands: []semanticir.Expression{}, Provenance: prov}, true
	case "StringLiteral":
		value, err := strconv.Unquote(string(node.Value))
		if err != nil {
			value = strings.Trim(string(node.Value), "\"")
		}
		l.accept()
		return semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeString, Literal: &semanticir.Literal{Type: semanticir.TypeString, String: value}, Operands: []semanticir.Expression{}, Provenance: prov}, true
	case "DeclRefExpr":
		if node.ReferencedDecl.Kind == "EnumConstantDecl" {
			l.accept()
			return semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeString, Literal: &semanticir.Literal{Type: semanticir.TypeString, String: node.ReferencedDecl.Name}, Operands: []semanticir.Expression{}, Provenance: prov}, true
		}
		name := node.ReferencedDecl.Name
		if name == "" {
			name = node.Name
		}
		if expression, exists := l.localExpressions[node.ReferencedDecl.ID]; exists {
			l.accept()
			return expression, true
		}
		if expression, exists := l.localExpressions[name]; exists {
			l.accept()
			return expression, true
		}
		typeName, ok := l.valueType(node.Type.QualType)
		if !ok {
			l.block(node, "variable-type", fmt.Sprintf("name %q has unsupported type %q", name, node.Type.QualType))
			return semanticir.Expression{}, false
		}
		l.accept()
		return semanticir.Expression{Kind: semanticir.ExprVariable, Type: typeName, Name: name, Operands: []semanticir.Expression{}, Provenance: prov}, true
	case "MemberExpr":
		if isImplicitThisMember(node) && node.Type.QualType != "<bound member function type>" {
			return l.lowerStateMember(node)
		}
		if node.IsArrow {
			l.block(node, "pointer-member-access", "pointer member access has uncontrolled lifetime and aliasing semantics")
			return semanticir.Expression{}, false
		}
		typeName, ok := l.valueType(node.Type.QualType)
		if !ok {
			typeName = semanticir.TypeUnknown
		}
		l.accept()
		return semanticir.Expression{Kind: semanticir.ExprVariable, Type: typeName, Name: node.Name, Operands: []semanticir.Expression{}, Provenance: prov}, true
	case "UnaryOperator":
		if len(node.Inner) != 1 {
			l.block(node, "unary-shape", "unary expression has unsupported shape")
			return semanticir.Expression{}, false
		}
		operand, ok := l.lowerExpression(node.Inner[0])
		if !ok {
			return semanticir.Expression{}, false
		}
		var operator semanticir.Operator
		switch node.Opcode {
		case "!":
			operator = semanticir.OpNot
		case "-":
			operator = semanticir.OpNeg
		case "+":
			return operand, true
		case "*", "&", "++", "--":
			l.block(node, "uncontrolled-ub", fmt.Sprintf("unary operator %q has pointer/lifetime or mutation semantics not bounded by the request", node.Opcode))
			return semanticir.Expression{}, false
		default:
			l.block(node, "unary-operator", fmt.Sprintf("unary operator %q is unsupported", node.Opcode))
			return semanticir.Expression{}, false
		}
		typeName, _ := l.valueType(node.Type.QualType)
		l.accept()
		expr := semanticir.Expression{Kind: semanticir.ExprUnary, Type: typeName, Operator: operator, Operands: []semanticir.Expression{operand}, Provenance: prov}
		l.recordIntegerWidth(prov, node.Type.QualType)
		return expr, true
	case "BinaryOperator":
		return l.lowerBinary(node)
	case "CallExpr", "CXXMemberCallExpr":
		return l.lowerCall(node)
	case "CXXOperatorCallExpr":
		return l.lowerOperatorCall(node)
	case "CXXConstructExpr", "CXXTemporaryObjectExpr":
		operands := make([]semanticir.Expression, 0, len(node.Inner))
		for _, child := range node.Inner {
			expr, ok := l.lowerExpression(child)
			if !ok {
				return semanticir.Expression{}, false
			}
			operands = append(operands, expr)
		}
		typeName, _ := l.valueType(node.Type.QualType)
		l.accept()
		return semanticir.Expression{Kind: semanticir.ExprCall, Type: typeName, Name: normalizedExceptionType(node.Type.QualType), Operands: operands, Provenance: prov}, true
	case "ArraySubscriptExpr", "CXXNewExpr", "CXXDeleteExpr", "UnaryExprOrTypeTraitExpr":
		l.block(node, "uncontrolled-ub", fmt.Sprintf("%s has memory/layout semantics not bounded by the request", node.Kind))
		return semanticir.Expression{}, false
	case "CStyleCastExpr", "CXXStaticCastExpr", "CXXReinterpretCastExpr", "CXXConstCastExpr", "CXXDynamicCastExpr":
		l.block(node, node.Kind, "explicit C++ casts may truncate or alter object identity and are not represented in the IR")
		return semanticir.Expression{}, false
	case "ConditionalOperator", "BinaryConditionalOperator", "InitListExpr", "LambdaExpr", "StmtExpr", "CXXThisExpr", "CXXTypeidExpr":
		l.block(node, node.Kind, "expression is outside the closed bounded IR vocabulary")
		return semanticir.Expression{}, false
	default:
		l.block(node, node.Kind, "C++ expression is not supported by the bounded frontend")
		return semanticir.Expression{}, false
	}
}

func (l *lowerer) lowerBinary(node *astNode) (semanticir.Expression, bool) {
	if len(node.Inner) != 2 {
		l.block(node, "binary-shape", "binary expression does not have two operands")
		return semanticir.Expression{}, false
	}
	left, okLeft := l.lowerExpression(node.Inner[0])
	right, okRight := l.lowerExpression(node.Inner[1])
	if !okLeft || !okRight {
		return semanticir.Expression{}, false
	}
	var operator semanticir.Operator
	var kind semanticir.ExpressionKind
	switch node.Opcode {
	case "+":
		operator, kind = semanticir.OpAdd, semanticir.ExprBinary
	case "-":
		operator, kind = semanticir.OpSub, semanticir.ExprBinary
	case "*":
		operator, kind = semanticir.OpMul, semanticir.ExprBinary
	case "/":
		operator, kind = semanticir.OpDiv, semanticir.ExprBinary
	case "%":
		operator, kind = semanticir.OpMod, semanticir.ExprBinary
	case "==":
		operator, kind = semanticir.OpEQ, semanticir.ExprCompare
	case "!=":
		operator, kind = semanticir.OpNE, semanticir.ExprCompare
	case "<":
		operator, kind = semanticir.OpLT, semanticir.ExprCompare
	case "<=":
		operator, kind = semanticir.OpLE, semanticir.ExprCompare
	case ">":
		operator, kind = semanticir.OpGT, semanticir.ExprCompare
	case ">=":
		operator, kind = semanticir.OpGE, semanticir.ExprCompare
	case "&&":
		operator, kind = semanticir.OpAnd, semanticir.ExprBool
	case "||":
		operator, kind = semanticir.OpOr, semanticir.ExprBool
	case "<<", ">>":
		l.block(node, "uncontrolled-ub", fmt.Sprintf("shift operator %q requires a proven width/count bound not present in the request", node.Opcode))
		return semanticir.Expression{}, false
	default:
		l.block(node, "binary-operator", fmt.Sprintf("binary operator %q is unsupported", node.Opcode))
		return semanticir.Expression{}, false
	}
	typeName, ok := l.valueType(node.Type.QualType)
	if !ok {
		l.block(node, "binary-type", fmt.Sprintf("binary result type %q is unsupported", node.Type.QualType))
		return semanticir.Expression{}, false
	}
	l.accept()
	prov := l.provenance(node, semanticir.TranslationTranslated)
	expr := semanticir.Expression{Kind: kind, Type: typeName, Operator: operator, Operands: []semanticir.Expression{left, right}, Provenance: prov}
	l.recordIntegerWidth(prov, node.Type.QualType)
	return expr, true
}

func (l *lowerer) lowerCall(node *astNode) (semanticir.Expression, bool) {
	if len(node.Inner) == 0 {
		l.block(node, "call-shape", "call has no target")
		return semanticir.Expression{}, false
	}
	name := memberCallTarget(node)
	if name == "" {
		l.block(node, "indirect-call", "indirect/function-pointer calls are not bounded")
		return semanticir.Expression{}, false
	}
	if unsafeCall(name) {
		l.block(node, "uncontrolled-ub", fmt.Sprintf("call to %q has uncontrolled memory/process semantics", name))
		return semanticir.Expression{}, false
	}
	if len(node.Inner) == 1 && node.Type.QualType == "bool" {
		if expression, ok := l.inlinePureBooleanMethod(shortName(name), node); ok {
			return expression, true
		}
	}
	operands := make([]semanticir.Expression, 0, len(node.Inner)-1)
	for _, child := range node.Inner[1:] {
		expr, ok := l.lowerExpression(child)
		if !ok {
			return semanticir.Expression{}, false
		}
		operands = append(operands, expr)
	}
	typeName, ok := l.valueType(node.Type.QualType)
	if !ok {
		l.block(node, "call-result-type", fmt.Sprintf("call %q returns unsupported type %q", name, node.Type.QualType))
		return semanticir.Expression{}, false
	}
	l.accept()
	return semanticir.Expression{Kind: semanticir.ExprCall, Type: typeName, Name: name, Operands: operands, Provenance: l.provenance(node, semanticir.TranslationTranslated)}, true
}

func (l *lowerer) inlinePureBooleanMethod(name string, call *astNode) (semanticir.Expression, bool) {
	if l.inliningCalls[name] {
		l.block(call, "recursive-helper", fmt.Sprintf("recursive helper %q cannot be finitely inlined", name))
		return semanticir.Expression{}, false
	}
	var matches []*astNode
	var visit func(*astNode)
	visit = func(node *astNode) {
		if node == nil {
			return
		}
		if node.Kind == "CXXMethodDecl" && node.Name == name && l.sourceOwned(node) && compoundBody(node) != nil {
			matches = append(matches, node)
			return
		}
		for _, child := range node.Inner {
			visit(child)
		}
	}
	visit(l.root)
	if len(matches) != 1 {
		return semanticir.Expression{}, false
	}
	body := compoundBody(matches[0])
	if body == nil || len(body.Inner) != 1 || body.Inner[0] == nil || body.Inner[0].Kind != "ReturnStmt" || len(body.Inner[0].Inner) != 1 {
		l.block(call, "impure-helper", fmt.Sprintf("boolean helper %q is not a single resolved return expression", name))
		return semanticir.Expression{}, false
	}
	l.inliningCalls[name] = true
	expression, ok := l.lowerExpression(body.Inner[0].Inner[0])
	delete(l.inliningCalls, name)
	if !ok || expression.Type != semanticir.TypeBool {
		return semanticir.Expression{}, false
	}
	// The call site remains the primary provenance while recursively lowered
	// operands retain the helper-definition evidence.
	expression.Provenance = l.provenance(call, semanticir.TranslationTranslated)
	l.accept()
	return expression, true
}

func referencedName(node *astNode) string {
	if node == nil {
		return ""
	}
	if node.ReferencedDecl.Name != "" {
		return node.ReferencedDecl.Name
	}
	if node.Name != "" {
		return node.Name
	}
	for _, child := range node.Inner {
		if name := referencedName(child); name != "" {
			return name
		}
	}
	return ""
}

func unsafeCall(name string) bool {
	name = strings.TrimPrefix(name, "::")
	switch name {
	case "memcpy", "memmove", "memset", "strcpy", "strcat", "sprintf", "system", "abort", "exit", "malloc", "free", "realloc":
		return true
	default:
		return false
	}
}

func (l *lowerer) valueType(qual string) (semanticir.ValueType, bool) {
	q := strings.TrimSpace(qual)
	isReference := strings.HasSuffix(q, "&")
	if isReference {
		q = strings.TrimSpace(strings.TrimSuffix(q, "&"))
	}
	q = strings.TrimPrefix(q, "const ")
	q = strings.TrimSuffix(q, " const")
	q = strings.TrimSpace(q)
	if q == "void" {
		return semanticir.TypeUnit, true
	}
	if q == "bool" {
		return semanticir.TypeBool, true
	}
	if strings.Contains(q, "basic_string") || q == "std::string" || q == "string" {
		return semanticir.TypeString, true
	}
	if isReference || strings.ContainsAny(q, "*&[") {
		return semanticir.TypeUnknown, false
	}
	for _, integer := range []string{"signed char", "short", "short int", "signed short", "signed short int", "int", "signed", "signed int", "long", "long int", "signed long", "signed long int", "long long", "long long int", "signed long long", "signed long long int"} {
		if q == integer {
			return semanticir.TypeInteger, true
		}
	}
	return semanticir.TypeUnknown, false
}

func (l *lowerer) recordIntegerWidth(provenance semanticir.Provenance, qual string) {
	if width := l.integerWidth(qual); width > 0 {
		l.integerBits[provenanceKey(provenance)] = width
	}
}

func (l *lowerer) integerWidth(qual string) int {
	q := strings.TrimSpace(qual)
	q = strings.TrimPrefix(q, "const ")
	q = strings.TrimSuffix(q, " const")
	switch q {
	case "signed char":
		q = "signed char"
	case "short", "short int", "signed short", "signed short int":
		q = "short"
	case "int", "signed", "signed int":
		q = "int"
	case "long", "long int", "signed long", "signed long int":
		q = "long"
	case "long long", "long long int", "signed long long", "signed long long int":
		q = "long long"
	default:
		return 0
	}
	return l.compilerWidths[q]
}

func provenanceKey(provenance semanticir.Provenance) string {
	location := provenance.Location
	return fmt.Sprintf("%s:%d:%d:%d:%d", provenance.ArtifactID, location.StartLine, location.StartColumn, location.EndLine, location.EndColumn)
}

func normalizedExceptionType(qual string) string {
	qual = strings.TrimSpace(qual)
	qual = strings.TrimPrefix(qual, "class ")
	qual = strings.TrimPrefix(qual, "struct ")
	return qual
}

func functionReturnType(qual string) string {
	if before, _, ok := strings.Cut(qual, " ("); ok {
		return strings.TrimSpace(before)
	}
	if before, _, ok := strings.Cut(qual, "("); ok {
		return strings.TrimSpace(before)
	}
	return strings.TrimSpace(qual)
}

func unwrapExpression(node *astNode) *astNode {
	for node != nil && len(node.Inner) == 1 {
		switch node.Kind {
		case "ImplicitCastExpr", "ExprWithCleanups", "MaterializeTemporaryExpr", "CXXBindTemporaryExpr", "CXXFunctionalCastExpr":
			node = node.Inner[0]
		default:
			return node
		}
	}
	return node
}

func firstStringLiteral(node *astNode) string {
	if node == nil {
		return ""
	}
	if node.Kind == "StringLiteral" {
		value, err := strconv.Unquote(string(node.Value))
		if err == nil {
			return value
		}
		return strings.Trim(string(node.Value), "\"")
	}
	for _, child := range node.Inner {
		if value := firstStringLiteral(child); value != "" {
			return value
		}
	}
	return ""
}

func (l *lowerer) sourceOwned(node *astNode) bool {
	if node == nil {
		return false
	}
	r := node.sourceRange()
	if r.Begin.Offset < 0 || r.End.Offset < r.Begin.Offset || r.End.Offset >= len(l.request.Source) {
		return false
	}
	file := r.Begin.File
	return file == "" || file == l.request.Artifact.Path || samePath(file, l.sourcePath)
}

func (l *lowerer) provenance(node *astNode, status semanticir.TranslationStatus) semanticir.Provenance {
	location := semanticir.SourceLocation{Path: l.request.Artifact.Path}
	if node == nil {
		location.StartLine, location.StartColumn = 1, 1
		location.EndLine, location.EndColumn = offsetLineColumn(l.request.Source, len(l.request.Source))
	} else {
		r := node.sourceRange()
		location.StartLine, location.StartColumn = offsetLineColumn(l.request.Source, r.Begin.Offset)
		end := r.End.Offset + r.End.TokLen - 1
		if end < r.Begin.Offset {
			end = r.Begin.Offset
		}
		location.EndLine, location.EndColumn = offsetLineColumn(l.request.Source, end)
	}
	return semanticir.NewProvenance(l.request.Artifact, location, status)
}

func offsetLineColumn(source []byte, offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	line, column := 1, 1
	for _, b := range source[:offset] {
		if b == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	return line, column
}

func (l *lowerer) accept() {
	l.total++
	l.translated++
}

func (l *lowerer) block(node *astNode, kind, reason string, codes ...semanticir.DiagnosticCode) {
	location := l.provenance(node, semanticir.TranslationUnsupported)
	key := fmt.Sprintf("%s:%d:%d:%s", kind, location.Location.StartLine, location.Location.StartColumn, reason)
	if l.blockedKeys[key] {
		return
	}
	l.blockedKeys[key] = true
	l.total++
	l.unsupported = append(l.unsupported, semanticir.UnsupportedConstruct{Kind: kind, Reason: reason, Provenance: location})
	code := semanticir.DiagnosticUnsupported
	if len(codes) > 0 {
		code = codes[0]
	}
	l.diagnostics = append(l.diagnostics, semanticir.Diagnostic{Severity: semanticir.SeverityError, Code: code, Message: reason, Provenance: location})
}

func (l *lowerer) invalid(node *astNode, reason string) {
	l.diagnostic(node, semanticir.DiagnosticInvalidInput, reason)
}

func (l *lowerer) diagnostic(node *astNode, code semanticir.DiagnosticCode, reason string) {
	l.diagnostics = append(l.diagnostics, semanticir.Diagnostic{Severity: semanticir.SeverityError, Code: code, Message: reason, Provenance: l.provenance(node, semanticir.TranslationUnsupported)})
}

func (l *lowerer) effectID(node *astNode, target string) string {
	r := node.sourceRange()
	return fmt.Sprintf("%s:call:%s:%d", l.request.Artifact.ID, target, r.Begin.Offset)
}
