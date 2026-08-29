package cpp

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

func (l *lowerer) validateChangedRanges() bool {
	if len(l.request.ChangedRanges) == 0 {
		l.invalid(nil, "C++ code translation requires exact frozen changed source ranges")
		return false
	}
	valid := true
	for _, changed := range l.request.ChangedRanges {
		if changed.ArtifactID != l.request.Artifact.ID || changed.Path != l.request.Artifact.Path || changed.StartLine <= 0 || changed.EndLine < changed.StartLine {
			l.invalid(nil, fmt.Sprintf("changed range %q:%d-%d is outside the modeled C++ artifact", changed.Path, changed.StartLine, changed.EndLine))
			valid = false
			continue
		}
		if changed.Provenance.ArtifactID != l.request.Artifact.ID || changed.Provenance.ArtifactDigest != l.request.Artifact.Digest || changed.Provenance.Location.Path != changed.Path || changed.Provenance.Location.StartLine != changed.StartLine || changed.Provenance.Location.EndLine != changed.EndLine {
			l.invalid(nil, fmt.Sprintf("changed range %q:%d-%d has detached provenance", changed.Path, changed.StartLine, changed.EndLine))
			valid = false
		}
		body, ok := sourceLines(l.request.Source, changed.StartLine, changed.EndLine)
		if !ok || changed.SliceDigest != semanticir.DigestBytes(body) {
			l.invalid(nil, fmt.Sprintf("changed range %q:%d-%d slice digest is stale", changed.Path, changed.StartLine, changed.EndLine))
			valid = false
		}
	}
	return valid
}

func sourceLines(source []byte, startLine, endLine int) ([]byte, bool) {
	if startLine <= 0 || endLine < startLine {
		return nil, false
	}
	line, start := 1, -1
	for offset := 0; offset <= len(source); offset++ {
		if line == startLine && start < 0 {
			start = offset
		}
		if offset == len(source) || source[offset] == '\n' {
			if line == endLine {
				end := offset
				if offset < len(source) {
					end++
				}
				if start < 0 {
					return nil, false
				}
				return append([]byte(nil), source[start:end]...), true
			}
			line++
		}
	}
	return nil, false
}

type scopeDeclaration struct {
	operation loweredOperation
	record    semanticir.CompilerDeclaration
	symbol    string
}

func (l *lowerer) buildScopeClosure(ctx context.Context) (semanticir.ScopeClosureEvidence, error) {
	irDigest := semanticir.DigestBytes([]byte(l.llvmIR))
	prover := l.request.Prover
	declarations := make([]scopeDeclaration, 0, len(l.operations))
	byOperation, bySymbol := map[string]scopeDeclaration{}, map[string]scopeDeclaration{}
	for _, operation := range l.operations {
		if operation.operation.Kind == semanticir.OperationTest {
			continue
		}
		symbol := llvmSymbol(l.llvmIR, operation.node.MangledName, operation.node.Name)
		body, ok := llvmFunctionBody(l.llvmIR, symbol)
		if !ok {
			return semanticir.ScopeClosureEvidence{}, fmt.Errorf("operation %q has no exact LLVM declaration body", operation.operation.ID)
		}
		location := operation.operation.Provenance.Location
		changed := false
		for _, changedRange := range l.request.ChangedRanges {
			if changedRange.ArtifactID == l.request.Artifact.ID && location.StartLine <= changedRange.EndLine && (location.EndLine == 0 || location.EndLine >= changedRange.StartLine) {
				changed = true
			}
		}
		identity := struct {
			Artifact string
			Name     string
			Symbol   string
			Location semanticir.SourceLocation
		}{l.request.Artifact.Digest, operation.operation.ID, symbol, location}
		identityDigest, _ := semanticir.Digest(identity)
		record := semanticir.CompilerDeclaration{
			ID: "cpp-decl:" + strings.TrimPrefix(identityDigest, "sha256:"), QualifiedName: operation.operation.ID,
			Artifact: l.request.Artifact, Location: location,
			CompilerNodeIDs: []string{"@" + symbol, "@" + symbol + ":body:" + semanticir.DigestBytes([]byte(body))},
			Changed:         changed, Provenance: operation.operation.Provenance,
		}
		item := scopeDeclaration{operation: operation, record: record, symbol: symbol}
		declarations = append(declarations, item)
		byOperation[operation.operation.ID], bySymbol[symbol] = item, item
	}
	for _, changedRange := range l.request.ChangedRanges {
		covered := false
		for _, declaration := range declarations {
			location := declaration.record.Location
			if declaration.record.Changed && location.StartLine <= changedRange.EndLine && (location.EndLine == 0 || location.EndLine >= changedRange.StartLine) {
				covered = true
				break
			}
		}
		if !covered {
			return semanticir.ScopeClosureEvidence{}, fmt.Errorf("changed range %s:%d-%d is not covered by a selected Clang declaration", changedRange.Path, changedRange.StartLine, changedRange.EndLine)
		}
	}

	defined := llvmDefinedFunctionBodies(l.llvmIR)
	for callerSymbol, body := range defined {
		for calleeSymbol := range bySymbol {
			if !llvmBodyCalls(body, calleeSymbol) {
				continue
			}
			if _, selected := bySymbol[callerSymbol]; !selected {
				return semanticir.ScopeClosureEvidence{}, fmt.Errorf("LLVM caller %q of selected declaration %q is outside the exact entry-point scope", callerSymbol, calleeSymbol)
			}
		}
	}

	var edges []semanticir.ResolvedCallEdge
	for _, caller := range declarations {
		calls := collectOperationCalls(caller.operation.operation.Body)
		for _, call := range calls {
			callee, exists := byOperation[call.Name]
			if !exists {
				matches := []scopeDeclaration{}
				for id, candidate := range byOperation {
					if shortName(id) == call.Name {
						matches = append(matches, candidate)
					}
				}
				if len(matches) != 1 {
					return semanticir.ScopeClosureEvidence{}, fmt.Errorf("call %q in %q has no unique selected Clang declaration", call.Name, caller.operation.operation.ID)
				}
				callee = matches[0]
			}
			edgeNode := "@" + caller.symbol + ":call:@" + callee.symbol + ":" + semanticir.DigestBytes([]byte(fmt.Sprintf("%+v", call.Provenance.Location)))
			edges = append(edges, semanticir.ResolvedCallEdge{
				CallerDeclarationID: caller.record.ID, CalleeDeclarationID: callee.record.ID,
				Location: call.Provenance.Location, CompilerNodeID: edgeNode, Provenance: call.Provenance,
			})
		}
	}

	callers := map[string][]string{}
	for _, edge := range edges {
		callers[edge.CalleeDeclarationID] = append(callers[edge.CalleeDeclarationID], edge.CallerDeclarationID)
	}
	impactedSet, queue := map[string]bool{}, []string{}
	for _, declaration := range declarations {
		if declaration.record.Changed {
			impactedSet[declaration.record.ID] = true
			queue = append(queue, declaration.record.ID)
		}
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, caller := range callers[current] {
			if !impactedSet[caller] {
				impactedSet[caller] = true
				queue = append(queue, caller)
			}
		}
	}
	var impacted []string
	for id := range impactedSet {
		impacted = append(impacted, id)
	}
	sort.Strings(impacted)
	var records []semanticir.CompilerDeclaration
	var owners []semanticir.OperationOwner
	var compilerNodes []string
	for _, declaration := range declarations {
		records = append(records, declaration.record)
		compilerNodes = append(compilerNodes, declaration.record.CompilerNodeIDs...)
		if !impactedSet[declaration.record.ID] {
			return semanticir.ScopeClosureEvidence{}, fmt.Errorf("selected operation %q is outside the changed-declaration/caller closure", declaration.operation.operation.ID)
		}
		owners = append(owners, semanticir.OperationOwner{OperationID: declaration.operation.operation.ID, DeclarationID: declaration.record.ID})
	}
	for _, edge := range edges {
		compilerNodes = append(compilerNodes, edge.CompilerNodeID)
	}
	sort.Strings(compilerNodes)
	sourceArtifacts := append([]semanticir.ArtifactRef(nil), l.request.FocusArtifacts...)
	closure := semanticir.ScopeClosureEvidence{
		SourceArtifacts: sourceArtifacts, WorkspaceTreeDigest: l.request.Workspace.TreeDigest,
		Compiler: l.request.Translator, Prover: prover, CompilerIRDigest: irDigest,
		ChangedRanges: append([]semanticir.ChangedSourceRange(nil), l.request.ChangedRanges...),
		Declarations:  records, CallEdges: edges, ImpactedDeclarationIDs: impacted, OperationOwners: owners,
		Completeness: semanticir.ProofProved, Complete: true, Provenance: l.provenance(nil, semanticir.TranslationTranslated),
	}
	graphDigest, err := semanticir.ScopeClosureGraphDigest(closure)
	if err != nil {
		return semanticir.ScopeClosureEvidence{}, fmt.Errorf("digest scope-closure graph: %v", err)
	}
	sourceDigest, err := semanticir.Digest(sourceArtifacts)
	if err != nil {
		return semanticir.ScopeClosureEvidence{}, fmt.Errorf("digest scope-closure sources: %v", err)
	}
	context := semanticir.CompilerProofContext{
		SourceDigest: sourceDigest, WorkspaceTreeDigest: l.request.Workspace.TreeDigest,
		EmittedIRDigest: irDigest, HarnessDigest: graphDigest, Compiler: l.request.Translator,
	}
	scope := compilerPredicate(l.request.Translator, irDigest, nil, []byte("false"), compilerNodes)
	claim := semanticir.NewProofClaim(semanticir.ClaimScopeClosure, context, scope, nil, nil)
	proof, result, err := replaySMTProof(ctx, prover, l.request.Workspace, claim)
	if err != nil || result != semanticir.SolverUNSAT {
		return semanticir.ScopeClosureEvidence{}, fmt.Errorf("prove complete compiler call closure: %v (%s)", err, result)
	}
	closure.CompletenessProof = proof
	return closure, nil
}

type operationCall struct {
	Name       string
	Provenance semanticir.Provenance
}

func collectOperationCalls(statements []semanticir.Statement) []operationCall {
	var calls []operationCall
	var expression func(*semanticir.Expression)
	expression = func(value *semanticir.Expression) {
		if value == nil {
			return
		}
		if value.Kind == semanticir.ExprCall {
			calls = append(calls, operationCall{Name: value.Name, Provenance: value.Provenance})
		}
		for index := range value.Operands {
			expression(&value.Operands[index])
		}
	}
	for index := range statements {
		statement := &statements[index]
		expression(statement.Condition)
		expression(statement.Value)
		calls = append(calls, collectOperationCalls(statement.Then)...)
		calls = append(calls, collectOperationCalls(statement.Else)...)
	}
	return calls
}

func llvmDefinedFunctionBodies(module string) map[string]string {
	result := map[string]string{}
	lines := strings.Split(module, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "define ") {
			continue
		}
		at := strings.Index(trimmed, "@")
		if at < 0 {
			continue
		}
		rest := trimmed[at+1:]
		name := ""
		if strings.HasPrefix(rest, `"`) {
			if end := strings.Index(rest[1:], `"(`); end >= 0 {
				name = rest[1 : end+1]
			}
		} else if end := strings.IndexByte(rest, '('); end >= 0 {
			name = rest[:end]
		}
		if name == "" {
			continue
		}
		if body, ok := llvmFunctionBody(module, name); ok {
			result[name] = body
		}
	}
	return result
}

func llvmBodyCalls(body, symbol string) bool {
	pattern := `(?m)\b(?:call|invoke)\b[^\n@]*@(?:"` + regexp.QuoteMeta(symbol) + `"|` + regexp.QuoteMeta(symbol) + `)\s*\(`
	return regexp.MustCompile(pattern).MatchString(body)
}
