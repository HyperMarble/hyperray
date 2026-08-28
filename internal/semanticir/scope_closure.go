package semanticir

import (
	"fmt"
	"reflect"
	"sort"
)

type scopeClosureGraph struct {
	ChangedRanges          []ChangedSourceRange  `json:"changed_ranges"`
	Declarations           []CompilerDeclaration `json:"declarations"`
	CallEdges              []ResolvedCallEdge    `json:"call_edges"`
	ImpactedDeclarationIDs []string              `json:"impacted_declaration_ids"`
	OperationOwners        []OperationOwner      `json:"operation_owners"`
}

// ScopeClosureGraphDigest binds the complete typed declaration/call graph
// projection used to compute the transitive impacted-caller closure.
func ScopeClosureGraphDigest(evidence ScopeClosureEvidence) (string, error) {
	return Digest(scopeClosureGraph{evidence.ChangedRanges, evidence.Declarations, evidence.CallEdges, evidence.ImpactedDeclarationIDs, evidence.OperationOwners})
}

func validateScopeClosure(model ArtifactModel) []Diagnostic {
	if model.Kind != ArtifactCode {
		if model.ScopeClosure != nil {
			return []Diagnostic{errorDiagnostic(DiagnosticInvalidInput, "non-code artifact declares patch-scope closure evidence", model.Coverage.Provenance)}
		}
		return nil
	}
	if model.ScopeClosure == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticIncomplete, "code artifact has no compiler-derived patch-scope closure evidence", model.Coverage.Provenance)}
	}
	evidence := model.ScopeClosure
	var diagnostics []Diagnostic
	if !evidence.Complete || evidence.Completeness != ProofProved || evidence.WorkspaceTreeDigest == "" || !ValidDigest(evidence.WorkspaceTreeDigest) || !ValidDigest(evidence.CompilerIRDigest) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "patch-scope closure is incomplete or unproved", evidence.Provenance))
	}
	if evidence.Compiler != model.Translator || validateToolRef(evidence.Compiler) != nil || validateToolRef(evidence.Prover) != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "patch-scope closure compiler/prover identity is invalid", evidence.Provenance))
	}
	artifacts := map[string]ArtifactRef{}
	for _, artifact := range evidence.SourceArtifacts {
		if validateArtifactRef(artifact) != nil || (artifact.Kind != ArtifactCode && artifact.Kind != ArtifactSource && artifact.Kind != ArtifactConfiguration) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "patch-scope closure has an invalid source artifact", evidence.Provenance))
		}
		if _, exists := artifacts[artifact.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "patch-scope closure repeats source artifact "+artifact.ID, evidence.Provenance))
		}
		artifacts[artifact.ID] = artifact
	}
	if source, exists := artifacts[model.Artifact.ID]; !exists || source != model.Artifact {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "patch-scope closure omits modeled source artifact", evidence.Provenance))
	}
	declarations := map[string]CompilerDeclaration{}
	changed := map[string]struct{}{}
	for _, declaration := range evidence.Declarations {
		artifact, exists := artifacts[declaration.Artifact.ID]
		if declaration.ID == "" || declaration.QualifiedName == "" || !exists || artifact != declaration.Artifact || len(declaration.CompilerNodeIDs) == 0 || validateFactSource(declaration.Provenance, declaration.Artifact) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "patch-scope declaration is incomplete or not frozen", declaration.Provenance))
		}
		if _, exists := declarations[declaration.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "patch-scope repeats declaration "+declaration.ID, declaration.Provenance))
		}
		declarations[declaration.ID] = declaration
		if declaration.Changed {
			changed[declaration.ID] = struct{}{}
		}
	}
	for _, changedRange := range evidence.ChangedRanges {
		artifact, exists := artifacts[changedRange.ArtifactID]
		if changedRange.Path == "" || changedRange.StartLine <= 0 || changedRange.EndLine < changedRange.StartLine || !ValidDigest(changedRange.SliceDigest) || !exists || artifact.Path != changedRange.Path || validateFactSource(changedRange.Provenance, artifact) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "patch-scope changed range is invalid or not frozen", changedRange.Provenance))
		}
		covered := false
		for _, declaration := range evidence.Declarations {
			covered = covered || declaration.Changed && declaration.Artifact.ID == changedRange.ArtifactID && declaration.Location.StartLine <= changedRange.EndLine && (declaration.Location.EndLine == 0 || declaration.Location.EndLine >= changedRange.StartLine)
		}
		if !covered {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "changed range has no changed compiler declaration", changedRange.Provenance))
		}
	}
	callers := map[string][]string{}
	edgeIDs := map[string]struct{}{}
	for _, edge := range evidence.CallEdges {
		_, callerOK := declarations[edge.CallerDeclarationID]
		_, calleeOK := declarations[edge.CalleeDeclarationID]
		key := edge.CallerDeclarationID + "\x00" + edge.CalleeDeclarationID + "\x00" + edge.CompilerNodeID
		if !callerOK || !calleeOK || edge.CompilerNodeID == "" || validateProvenance(edge.Provenance) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "patch-scope call edge is unresolved or unanchored", edge.Provenance))
		}
		if _, exists := edgeIDs[key]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "patch-scope repeats a resolved call edge", edge.Provenance))
		}
		edgeIDs[key] = struct{}{}
		callers[edge.CalleeDeclarationID] = append(callers[edge.CalleeDeclarationID], edge.CallerDeclarationID)
	}
	wantImpacted := map[string]struct{}{}
	queue := make([]string, 0, len(changed))
	for id := range changed {
		wantImpacted[id] = struct{}{}
		queue = append(queue, id)
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, caller := range callers[id] {
			if _, exists := wantImpacted[caller]; !exists {
				wantImpacted[caller] = struct{}{}
				queue = append(queue, caller)
			}
		}
	}
	wantIDs := make([]string, 0, len(wantImpacted))
	for id := range wantImpacted {
		wantIDs = append(wantIDs, id)
	}
	sort.Strings(wantIDs)
	gotIDs := append([]string(nil), evidence.ImpactedDeclarationIDs...)
	sort.Strings(gotIDs)
	if !reflect.DeepEqual(gotIDs, wantIDs) || hasDuplicateStrings(gotIDs) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "patch-scope impacted declarations are not the exact transitive caller closure", evidence.Provenance))
	}
	ownerOps := map[string]struct{}{}
	for _, owner := range evidence.OperationOwners {
		declaration, exists := declarations[owner.DeclarationID]
		_, impacted := wantImpacted[owner.DeclarationID]
		if owner.OperationID == "" || !exists || !impacted || declaration.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "operation owner is absent from impacted declaration closure", evidence.Provenance))
		}
		if _, duplicate := ownerOps[owner.OperationID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "patch-scope repeats operation owner "+owner.OperationID, evidence.Provenance))
		}
		ownerOps[owner.OperationID] = struct{}{}
	}
	for _, operation := range model.Operations {
		if operation.Kind != OperationTest {
			if _, exists := ownerOps[operation.ID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "patch-scope closure omits operation owner "+operation.ID, operation.Provenance))
			}
		}
	}
	graphDigest, graphErr := ScopeClosureGraphDigest(*evidence)
	sourceDigest, sourceErr := Digest(evidence.SourceArtifacts)
	context := CompilerProofContext{SourceDigest: sourceDigest, WorkspaceTreeDigest: evidence.WorkspaceTreeDigest, EmittedIRDigest: evidence.CompilerIRDigest, HarnessDigest: graphDigest, Compiler: evidence.Compiler}
	claim := NewProofClaim(ClaimScopeClosure, context, evidence.CompletenessProof.Claim.Scope, nil, nil)
	if graphErr != nil || sourceErr != nil || evidence.CompletenessProof.Prover != evidence.Prover || !reflect.DeepEqual(evidence.CompletenessProof.Claim, claim) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "patch-scope completeness proof differs from exact graph/source binding", evidence.Provenance))
	}
	diagnostics = append(diagnostics, ValidateReplayableProof(evidence.CompletenessProof, SolverUNSAT, evidence.Provenance)...)
	return diagnostics
}

func validateChangedRanges(ranges []ChangedSourceRange, focus []ArtifactRef, provenance Provenance) []Diagnostic {
	artifacts := map[string]ArtifactRef{}
	for _, artifact := range focus {
		artifacts[artifact.ID] = artifact
	}
	var diagnostics []Diagnostic
	for _, item := range ranges {
		artifact, exists := artifacts[item.ArtifactID]
		if !exists || item.Path != artifact.Path || item.StartLine <= 0 || item.EndLine < item.StartLine || !ValidDigest(item.SliceDigest) || validateFactSource(item.Provenance, artifact) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("changed source range %q:%d-%d is invalid", item.Path, item.StartLine, item.EndLine), item.Provenance))
		}
	}
	if len(ranges) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "frontend request has no exact changed source ranges", provenance))
	}
	return diagnostics
}
