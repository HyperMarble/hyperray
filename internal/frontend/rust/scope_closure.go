package rust

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/HyperMarble/ray/internal/semanticir"
)

func buildRustScopeClosure(ctx context.Context, request semanticir.FrontendRequest, functions []functionDecl, compiler rustCompilerOutput, operations []semanticir.Operation) (*semanticir.ScopeClosureEvidence, []semanticir.Diagnostic) {
	whole := wholeSpan(request.Source)
	block := func(span sourceSpan, message string) (*semanticir.ScopeClosureEvidence, []semanticir.Diagnostic) {
		return nil, []semanticir.Diagnostic{diagnostic(request.Artifact, span, semanticir.DiagnosticIncomplete, message)}
	}
	if len(request.ChangedRanges) == 0 {
		return block(whole, "Rust patch-scope closure requires exact frozen changed source ranges")
	}
	if compiler.MIR == "" {
		return block(whole, "Rust patch-scope closure has no emitted MIR")
	}
	for _, entry := range request.Workspace.Entries {
		if entry.Artifact.Kind == semanticir.ArtifactCode && entry.Artifact != request.Artifact {
			return block(whole, "Rust patch-scope closure cannot omit another code artifact from the frozen workspace")
		}
	}
	for _, changed := range request.ChangedRanges {
		if changed.ArtifactID != request.Artifact.ID || changed.Path != request.Artifact.Path {
			return block(whole, "Rust patch-scope closure cannot map a changed range outside the focused artifact")
		}
		body, exact := rustChangedRangeBytes(request.Source, changed.StartLine, changed.EndLine)
		if !exact || semanticir.DigestBytes(body) != changed.SliceDigest {
			return block(whole, fmt.Sprintf("Rust changed range %d-%d does not match its frozen slice digest", changed.StartLine, changed.EndLine))
		}
	}

	irDigest := semanticir.DigestBytes([]byte(compiler.MIR))
	declarationByName := make(map[string]semanticir.CompilerDeclaration, len(functions))
	functionByName := make(map[string]functionDecl, len(functions))
	declarations := make([]semanticir.CompilerDeclaration, 0, len(functions))
	for _, function := range functions {
		section, exists := mirFunction(compiler.MIR, function.Name)
		if !exists {
			return block(function.Span, "Rust patch-scope closure cannot bind declaration "+function.Name+" to one MIR body")
		}
		nodeID := "mir:declaration:" + function.Name + ":" + strings.TrimPrefix(semanticir.DigestBytes([]byte(section)), "sha256:")[:16]
		prov := provenance(request.Artifact, function.Span, semanticir.TranslationTranslated)
		declaration := semanticir.CompilerDeclaration{
			ID: "rust-declaration::" + function.Name, QualifiedName: function.Name, Artifact: request.Artifact,
			Location: prov.Location, CompilerNodeIDs: []string{nodeID}, Changed: rustFunctionChanged(request.ChangedRanges, function), Provenance: prov,
		}
		declarationByName[function.Name] = declaration
		functionByName[function.Name] = function
		declarations = append(declarations, declaration)
	}

	var edges []semanticir.ResolvedCallEdge
	for _, caller := range functions {
		section, _ := mirFunction(compiler.MIR, caller.Name)
		body := section
		if newline := strings.IndexByte(body, '\n'); newline >= 0 {
			body = body[newline+1:]
		}
		calls := rustSourceCalls(caller.Body)
		callsByTarget := make(map[string][]expression)
		for _, call := range calls {
			if _, local := declarationByName[call.Text]; local {
				callsByTarget[call.Text] = append(callsByTarget[call.Text], call)
			}
		}
		for callee, sourceCalls := range callsByTarget {
			mirNodes := rustMIRCallNodes(caller.Name, callee, body)
			if len(mirNodes) != len(sourceCalls) {
				return block(caller.Span, fmt.Sprintf("Rust MIR/source call mapping for %s -> %s is not one-to-one (%d compiler calls, %d source calls)", caller.Name, callee, len(mirNodes), len(sourceCalls)))
			}
			for index, call := range sourceCalls {
				prov := provenance(request.Artifact, call.Span, semanticir.TranslationTranslated)
				edges = append(edges, semanticir.ResolvedCallEdge{
					CallerDeclarationID: declarationByName[caller.Name].ID, CalleeDeclarationID: declarationByName[callee].ID,
					Location: prov.Location, CompilerNodeID: mirNodes[index], Provenance: prov,
				})
			}
		}
		// Any call to a source-owned declaration visible in MIR but absent from
		// the strict source mapping is an omission and therefore blocks closure.
		for callee := range functionByName {
			if len(rustMIRCallNodes(caller.Name, callee, body)) != len(callsByTarget[callee]) {
				return block(caller.Span, "Rust patch-scope closure found an unmapped MIR call edge "+caller.Name+" -> "+callee)
			}
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		left := edges[i].CallerDeclarationID + "\x00" + edges[i].CalleeDeclarationID + "\x00" + edges[i].CompilerNodeID
		right := edges[j].CallerDeclarationID + "\x00" + edges[j].CalleeDeclarationID + "\x00" + edges[j].CompilerNodeID
		return left < right
	})

	impactedSet := make(map[string]bool)
	for _, declaration := range declarations {
		impactedSet[declaration.ID] = declaration.Changed
	}
	changed := true
	for changed {
		changed = false
		for _, edge := range edges {
			if impactedSet[edge.CalleeDeclarationID] && !impactedSet[edge.CallerDeclarationID] {
				impactedSet[edge.CallerDeclarationID] = true
				changed = true
			}
		}
	}
	var impacted []string
	for declarationID, included := range impactedSet {
		if included {
			impacted = append(impacted, declarationID)
		}
	}
	sort.Strings(impacted)
	var owners []semanticir.OperationOwner
	for _, operation := range operations {
		if operation.Kind == semanticir.OperationTest {
			continue
		}
		declaration, exists := declarationByName[operation.ID]
		if !exists || !impactedSet[declaration.ID] {
			return block(whole, "Rust requested operation is outside the exact changed-declaration/caller closure: "+operation.ID)
		}
		owners = append(owners, semanticir.OperationOwner{OperationID: operation.ID, DeclarationID: declaration.ID})
	}

	prover, proverArgv, proverEnvironment, proverDiagnostics := pinnedRustProver(ctx, request)
	if semanticir.HasErrors(proverDiagnostics) {
		return nil, proverDiagnostics
	}
	sources := []semanticir.ArtifactRef{request.Artifact}
	prov := provenance(request.Artifact, whole, semanticir.TranslationTranslated)
	evidence := semanticir.ScopeClosureEvidence{
		SourceArtifacts: sources, WorkspaceTreeDigest: request.Workspace.TreeDigest, Compiler: request.Translator, Prover: prover,
		CompilerIRDigest: irDigest, ChangedRanges: append([]semanticir.ChangedSourceRange(nil), request.ChangedRanges...),
		Declarations: declarations, CallEdges: edges, ImpactedDeclarationIDs: impacted, OperationOwners: owners,
		Completeness: semanticir.ProofProved, Complete: true, Provenance: prov,
	}
	graphDigest, err := semanticir.ScopeClosureGraphDigest(evidence)
	if err != nil {
		return block(whole, "digest Rust patch-scope graph: "+err.Error())
	}
	sourceDigest, err := semanticir.Digest(sources)
	if err != nil {
		return block(whole, "digest Rust patch-scope sources: "+err.Error())
	}
	var nodeIDs []string
	for _, declaration := range declarations {
		nodeIDs = append(nodeIDs, declaration.CompilerNodeIDs...)
	}
	for _, edge := range edges {
		nodeIDs = append(nodeIDs, edge.CompilerNodeID)
	}
	sort.Strings(nodeIDs)
	// The strict MIR/source reconciliation above has enumerated every local
	// declaration and every source-owned local call. Its omission predicate is
	// therefore the empty disjunction, i.e. false; Z3 proves it unreachable.
	scope := rustCompilerPredicate(request.Translator, irDigest, "(declare-const ray_scope_omission Bool)", "false", nodeIDs)
	context := semanticir.CompilerProofContext{SourceDigest: sourceDigest, WorkspaceTreeDigest: request.Workspace.TreeDigest, EmittedIRDigest: irDigest, HarnessDigest: graphDigest, Compiler: request.Translator}
	claim := semanticir.NewProofClaim(semanticir.ClaimScopeClosure, context, scope, nil, nil)
	proof, proofErr := replayRustClaim(ctx, prover, proverArgv, proverEnvironment, claim, semanticir.SolverUNSAT)
	if proofErr != nil {
		return block(whole, "prove Rust patch-scope closure: "+proofErr.Error())
	}
	evidence.CompletenessProof = proof
	return &evidence, nil
}

func rustFunctionChanged(ranges []semanticir.ChangedSourceRange, function functionDecl) bool {
	for _, changed := range ranges {
		if changed.StartLine <= function.Span.End.Line && changed.EndLine >= function.Span.Start.Line {
			return true
		}
	}
	return false
}

func rustChangedRangeBytes(source []byte, startLine, endLine int) ([]byte, bool) {
	if startLine <= 0 || endLine < startLine {
		return nil, false
	}
	starts := []int{0}
	for index, value := range source {
		if value == '\n' {
			starts = append(starts, index+1)
		}
	}
	if startLine > len(starts) || endLine > len(starts) {
		return nil, false
	}
	start := starts[startLine-1]
	end := len(source)
	if endLine < len(starts) {
		end = starts[endLine]
	}
	return append([]byte(nil), source[start:end]...), true
}

func rustMIRCallNodes(caller, callee, body string) []string {
	var nodes []string
	needle := callee + "("
	for lineIndex, line := range strings.Split(body, "\n") {
		for offset := 0; ; {
			found := strings.Index(line[offset:], needle)
			if found < 0 {
				break
			}
			column := offset + found
			identity := fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%s", caller, callee, lineIndex+2, column+1, line)
			nodes = append(nodes, "mir:call:"+caller+":"+callee+":"+strings.TrimPrefix(semanticir.DigestBytes([]byte(identity)), "sha256:")[:16])
			offset = column + len(needle)
		}
	}
	return nodes
}

func rustSourceCalls(value block) []expression {
	var calls []expression
	var visitExpression func(expression)
	var visitBlock func(block)
	visitExpression = func(expr expression) {
		if expr.Kind == expressionCall {
			calls = append(calls, expr)
		}
		for _, child := range expr.Children {
			visitExpression(child)
		}
		if expr.Then != nil {
			visitBlock(*expr.Then)
		}
		if expr.Else != nil {
			visitBlock(*expr.Else)
		}
		for _, arm := range expr.Arms {
			if arm.Guard != nil {
				visitExpression(*arm.Guard)
			}
			visitExpression(arm.Value)
		}
	}
	visitBlock = func(body block) {
		for _, statement := range body.Statements {
			visitExpression(statement.Expr)
		}
		if body.Tail != nil {
			visitExpression(*body.Tail)
		}
	}
	visitBlock(value)
	return calls
}
