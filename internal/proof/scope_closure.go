package proof

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

// validateScopeClosure proves that the author-declared operation vocabulary
// is the exact compiler-derived closure of changed declarations and their
// transitive callers. Without it, omitting one affected callable would make
// every downstream finite proof vacuously incomplete.
func (v *validator) validateScopeClosure(artifact *semanticir.ArtifactModel) {
	evidence := artifact.ScopeClosure
	if evidence == nil {
		v.add("missing-scope-closure", fmt.Sprintf("code artifact %q has no compiler-derived patch/caller scope closure", artifact.Artifact.ID), &artifact.Coverage.Provenance)
		return
	}
	label := fmt.Sprintf("code artifact %q scope closure", artifact.Artifact.ID)
	startBlockers := len(v.blockers)
	v.provenance(evidence.Provenance, label)
	v.requireProvenanceKind(evidence.Provenance, semanticir.ArtifactCode, label)
	if !evidence.Complete || evidence.Completeness != semanticir.ProofProved || !digestPattern.MatchString(evidence.WorkspaceTreeDigest) || !digestPattern.MatchString(evidence.CompilerIRDigest) {
		v.add("incomplete-scope-closure", label+" is incomplete or unproved", &evidence.Provenance)
	}
	if evidence.Compiler != artifact.Translator {
		v.add("stale-scope-closure", label+" compiler differs from the artifact translator", &evidence.Provenance)
	}
	v.validateTool(evidence.Compiler, label+" compiler")
	v.validateTool(evidence.Prover, label+" prover")
	if !v.environmentHasTool(evidence.Compiler) || !v.environmentHasTool(evidence.Prover) {
		v.add("missing-tool-evidence", label+" compiler/prover is absent from the frozen environment", &evidence.Provenance)
	}
	if err := verifyToolBinary(evidence.Compiler); err != nil {
		v.add("stale-scope-tool", fmt.Sprintf("%s compiler: %v", label, err), &evidence.Provenance)
	}
	if !v.environmentHasSnapshot(evidence.WorkspaceTreeDigest, evidence.CompletenessProof.EnvironmentDigest) {
		v.add("stale-scope-closure", label+" workspace/environment matches no frozen command", &evidence.Provenance)
	}

	sources := make(map[string]semanticir.ArtifactRef, len(evidence.SourceArtifacts))
	for _, source := range evidence.SourceArtifacts {
		frozen, exists := v.artifacts[source.ID]
		if !exists || frozen != source || (source.Kind != semanticir.ArtifactCode && source.Kind != semanticir.ArtifactConfiguration) || sources[source.ID].ID != "" {
			v.add("invalid-scope-source", fmt.Sprintf("%s has a missing, stale, duplicate, or invalid-kind source %q", label, source.ID), &evidence.Provenance)
		}
		sources[source.ID] = source
	}
	if sources[artifact.Artifact.ID] != artifact.Artifact {
		v.add("incomplete-scope-source", label+" omits its modeled code artifact", &evidence.Provenance)
	}

	declarations := make(map[string]semanticir.CompilerDeclaration, len(evidence.Declarations))
	changed := make(map[string]bool)
	for index := range evidence.Declarations {
		declaration := &evidence.Declarations[index]
		source, sourceExists := sources[declaration.Artifact.ID]
		if strings.TrimSpace(declaration.ID) == "" || strings.TrimSpace(declaration.QualifiedName) == "" || !sourceExists || source != declaration.Artifact || len(declaration.CompilerNodeIDs) == 0 || declarations[declaration.ID].ID != "" {
			v.add("invalid-scope-declaration", fmt.Sprintf("%s declaration %q is incomplete, duplicated, or outside its sources", label, declaration.ID), &declaration.Provenance)
		}
		v.provenance(declaration.Provenance, label+" declaration "+declaration.ID)
		seenNodes := map[string]bool{}
		for _, nodeID := range declaration.CompilerNodeIDs {
			if strings.TrimSpace(nodeID) == "" || seenNodes[nodeID] {
				v.add("invalid-scope-declaration", fmt.Sprintf("%s declaration %q has empty or duplicate compiler node IDs", label, declaration.ID), &declaration.Provenance)
			}
			seenNodes[nodeID] = true
		}
		declarations[declaration.ID] = *declaration
		changed[declaration.ID] = declaration.Changed
	}
	for index := range evidence.ChangedRanges {
		changedRange := &evidence.ChangedRanges[index]
		source, exists := sources[changedRange.ArtifactID]
		v.provenance(changedRange.Provenance, fmt.Sprintf("%s changed range %d", label, index))
		if !exists || source.Path != changedRange.Path || changedRange.StartLine <= 0 || changedRange.EndLine < changedRange.StartLine || !digestPattern.MatchString(changedRange.SliceDigest) {
			v.add("invalid-scope-range", fmt.Sprintf("%s changed range %d is not bound to frozen source bytes", label, index), &changedRange.Provenance)
		}
		covered := false
		for _, declaration := range evidence.Declarations {
			covered = covered || declaration.Changed && declaration.Artifact.ID == changedRange.ArtifactID && declaration.Location.StartLine <= changedRange.EndLine && (declaration.Location.EndLine == 0 || declaration.Location.EndLine >= changedRange.StartLine)
		}
		if !covered {
			v.add("incomplete-scope-range", fmt.Sprintf("%s changed range %d has no changed declaration", label, index), &changedRange.Provenance)
		}
	}

	callers := make(map[string][]string)
	seenEdges := map[string]bool{}
	for index := range evidence.CallEdges {
		edge := &evidence.CallEdges[index]
		_, callerExists := declarations[edge.CallerDeclarationID]
		_, calleeExists := declarations[edge.CalleeDeclarationID]
		key := edge.CallerDeclarationID + "\x00" + edge.CalleeDeclarationID + "\x00" + edge.CompilerNodeID
		v.provenance(edge.Provenance, fmt.Sprintf("%s call edge %d", label, index))
		if !callerExists || !calleeExists || strings.TrimSpace(edge.CompilerNodeID) == "" || seenEdges[key] {
			v.add("invalid-scope-call-edge", fmt.Sprintf("%s call edge %d is unresolved or duplicated", label, index), &edge.Provenance)
		}
		seenEdges[key] = true
		callers[edge.CalleeDeclarationID] = append(callers[edge.CalleeDeclarationID], edge.CallerDeclarationID)
	}
	wantedImpacted := map[string]bool{}
	var queue []string
	for declarationID, isChanged := range changed {
		if isChanged {
			wantedImpacted[declarationID] = true
			queue = append(queue, declarationID)
		}
	}
	for len(queue) != 0 {
		declarationID := queue[0]
		queue = queue[1:]
		for _, caller := range callers[declarationID] {
			if !wantedImpacted[caller] {
				wantedImpacted[caller] = true
				queue = append(queue, caller)
			}
		}
	}
	wantIDs := make([]string, 0, len(wantedImpacted))
	for declarationID := range wantedImpacted {
		wantIDs = append(wantIDs, declarationID)
	}
	sort.Strings(wantIDs)
	gotIDs := append([]string(nil), evidence.ImpactedDeclarationIDs...)
	sort.Strings(gotIDs)
	if !reflect.DeepEqual(gotIDs, wantIDs) || hasDuplicateStrings(gotIDs) {
		v.add("incomplete-scope-closure", label+" impacted declarations are not the exact transitive caller closure", &evidence.Provenance)
	}
	wantedOperations := map[string]bool{}
	for _, operation := range artifact.Operations {
		if operation.Kind != semanticir.OperationTest {
			wantedOperations[operation.ID] = true
		}
	}
	seenOwners := map[string]bool{}
	for _, owner := range evidence.OperationOwners {
		if !wantedOperations[owner.OperationID] || seenOwners[owner.OperationID] || declarations[owner.DeclarationID].ID == "" || !wantedImpacted[owner.DeclarationID] {
			v.add("invalid-scope-owner", fmt.Sprintf("%s has invalid or duplicate owner for operation %q", label, owner.OperationID), &evidence.Provenance)
		}
		seenOwners[owner.OperationID] = true
	}
	for operationID := range wantedOperations {
		if !seenOwners[operationID] {
			v.add("incomplete-scope-closure", fmt.Sprintf("%s omits operation owner %q", label, operationID), &evidence.Provenance)
		}
	}

	graphDigest, graphErr := semanticir.ScopeClosureGraphDigest(*evidence)
	sourceDigest, sourceErr := semanticir.Digest(evidence.SourceArtifacts)
	context := semanticir.CompilerProofContext{SourceDigest: sourceDigest, WorkspaceTreeDigest: evidence.WorkspaceTreeDigest, EmittedIRDigest: evidence.CompilerIRDigest, HarnessDigest: graphDigest, Compiler: evidence.Compiler}
	v.validateCompilerPredicate(evidence.CompletenessProof.Claim.Scope, evidence.Compiler, evidence.CompilerIRDigest, label+" omission predicate", &evidence.Provenance)
	wantClaim := semanticir.NewProofClaim(semanticir.ClaimScopeClosure, context, evidence.CompletenessProof.Claim.Scope, nil, nil)
	if graphErr != nil || sourceErr != nil || evidence.CompletenessProof.Prover != evidence.Prover || !reflect.DeepEqual(evidence.CompletenessProof.Claim, wantClaim) {
		v.add("noncanonical-scope-proof", label+" proof differs from the exact graph/source/omission obligation", &evidence.Provenance)
	}
	if len(v.blockers) != startBlockers {
		return
	}
	query, queryErr := semanticir.CanonicalProofQuery(wantClaim)
	if queryErr != nil || !bytes.Equal(evidence.CompletenessProof.Query, query) {
		v.add("noncanonical-scope-proof", label+" query is not centrally reconstructed from its typed obligation", &evidence.Provenance)
		return
	}
	if err := Replay(v.ctx, evidence.CompletenessProof, semanticir.SolverUNSAT, v.task.Environment); err != nil {
		v.add("scope-proof-replay-failed", fmt.Sprintf("%s: %v", label, err), &evidence.Provenance)
	}
}

func hasDuplicateStrings(values []string) bool {
	for index := 1; index < len(values); index++ {
		if values[index] == values[index-1] {
			return true
		}
	}
	return false
}

func (v *validator) scopeClosureTranscript() []semanticir.ScopeClosureEvidence {
	var result []semanticir.ScopeClosureEvidence
	for _, artifact := range v.task.Artifacts {
		if artifact.Kind != semanticir.ArtifactCode || artifact.ScopeClosure == nil {
			continue
		}
		encoded, err := semanticir.CanonicalJSON(artifact.ScopeClosure)
		if err != nil {
			continue
		}
		var cloned semanticir.ScopeClosureEvidence
		if json.Unmarshal(encoded, &cloned) == nil {
			result = append(result, cloned)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Provenance.ArtifactID < result[j].Provenance.ArtifactID })
	return result
}

func (v *validator) exhaustiveEvidenceTranscript() []semanticir.ExhaustiveExecutionEvidence {
	var result []semanticir.ExhaustiveExecutionEvidence
	for _, artifact := range v.task.Artifacts {
		for _, evidence := range artifact.ExhaustiveEvidence {
			encoded, err := semanticir.CanonicalJSON(evidence)
			if err != nil {
				continue
			}
			var cloned semanticir.ExhaustiveExecutionEvidence
			if json.Unmarshal(encoded, &cloned) == nil {
				result = append(result, cloned)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Provenance.ArtifactID != result[j].Provenance.ArtifactID {
			return result[i].Provenance.ArtifactID < result[j].Provenance.ArtifactID
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func proofEvidenceDigest(value any) string {
	digest, err := semanticir.Digest(value)
	if err != nil {
		return ""
	}
	return digest
}
