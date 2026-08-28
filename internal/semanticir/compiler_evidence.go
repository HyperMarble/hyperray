package semanticir

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// ValidateReplayableProof checks the immutable replay contract. The proof
// engine additionally executes Prover.Path with Argv, Query on stdin, and the
// declared timeout/environment, then requires byte-identical SolverOutput.
func ValidateReplayableProof(proof ReplayableProof, expected SolverResult, provenance Provenance) []Diagnostic {
	var diagnostics []Diagnostic
	if proof.Logic != ProofLogicSMTLIB2 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("unsupported proof logic %q", proof.Logic), provenance))
	}
	if len(bytes.TrimSpace(proof.Query)) == 0 || proof.QueryDigest != DigestBytes(proof.Query) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "replayable proof query/digest is empty or mismatched", provenance))
	}
	canonicalQuery, err := CanonicalProofQuery(proof.Claim)
	if err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "replayable proof claim: "+err.Error(), provenance))
	} else if !bytes.Equal(proof.Query, canonicalQuery) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "replayable proof query is not the canonical query for its typed claim", provenance))
	}
	if len(bytes.TrimSpace(proof.SolverOutput)) == 0 || proof.SolverOutputDigest != DigestBytes(proof.SolverOutput) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "replayable proof output/digest is empty or mismatched", provenance))
	}
	if err := validateToolRef(proof.Prover); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "replayable proof prover: "+err.Error(), provenance))
	}
	if proof.WorkingDirectory == "" || !proof.ClearEnvironment || !proof.KillProcessGroup || proof.TimeoutMillis <= 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "replayable proof lacks workdir/clear-environment/process-group/timeout policy", provenance))
	}
	if err := validateExactEnvironment(proof.Environment, proof.EnvironmentDigest); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "replayable proof environment: "+err.Error(), provenance))
	}
	for _, argument := range proof.Argv {
		if argument == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "replayable proof argv contains an empty argument", provenance))
			break
		}
	}
	fields := strings.Fields(string(proof.SolverOutput))
	if proof.Result != expected || len(fields) == 0 || SolverResult(fields[0]) != proof.Result {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("replayable proof result/output is %q, want %q", proof.Result, expected), provenance))
	}
	wantSubjects := proofClaimSubjects(proof.Claim)
	seenSubjects := map[string]struct{}{}
	for _, digest := range proof.SubjectDigests {
		if !ValidDigest(digest) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "replayable proof has an invalid subject digest", provenance))
		}
		if _, exists := seenSubjects[digest]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "replayable proof repeats a subject digest", provenance))
		}
		seenSubjects[digest] = struct{}{}
	}
	if !sameStringSet(proof.SubjectDigests, wantSubjects) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "replayable proof subjects differ from its typed claim", provenance))
	}
	return diagnostics
}

func validateExactEnvironment(environment []EnvironmentVariable, digest string) error {
	previousName := ""
	for index, variable := range environment {
		if variable.Name == "" || strings.ContainsRune(variable.Name, '=') || strings.ContainsRune(variable.Name, '\x00') || strings.ContainsRune(variable.Value, '\x00') {
			return fmt.Errorf("invalid variable name %q", variable.Name)
		}
		if index > 0 && variable.Name <= previousName {
			return fmt.Errorf("variables are not strictly name-sorted and unique")
		}
		previousName = variable.Name
	}
	calculated, err := Digest(environment)
	if err != nil || digest != calculated {
		return fmt.Errorf("digest does not match explicit variables")
	}
	return nil
}

func validateCompilerPredicate(predicate CompilerPredicate, tool ToolRef, irDigest string, provenance Provenance) []Diagnostic {
	var diagnostics []Diagnostic
	if predicate.Logic != ProofLogicSMTLIB2 || len(bytes.TrimSpace(predicate.Formula)) == 0 || predicate.FormulaDigest != DigestBytes(predicate.Formula) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "compiler predicate formula/logic/digest is invalid", provenance))
	}
	if predicate.DeclarationsDigest != DigestBytes(predicate.Declarations) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "compiler predicate declaration digest is invalid", provenance))
	} else if _, err := canonicalDeclarations(ProofClaim{Kind: ClaimReachability, Scope: predicate, Memberships: []CompilerPredicate{predicate}}); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "compiler predicate declarations: "+err.Error(), provenance))
	}
	if predicate.Tool != tool || predicate.IRDigest != irDigest || !ValidDigest(predicate.IRDigest) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "compiler predicate is not bound to declared compiler/IR", provenance))
	}
	if len(predicate.CompilerNodeIDs) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "compiler predicate has no compiler nodes", provenance))
	}
	for _, nodeID := range predicate.CompilerNodeIDs {
		if nodeID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "compiler predicate has an empty node ID", provenance))
		}
	}
	return diagnostics
}

func proofHasSubjects(proof ReplayableProof, required ...string) bool {
	seen := map[string]struct{}{}
	for _, digest := range proof.SubjectDigests {
		seen[digest] = struct{}{}
	}
	for _, digest := range required {
		if _, exists := seen[digest]; !exists {
			return false
		}
	}
	return true
}

func proofSubjectsExactly(proof ReplayableProof, required ...string) bool {
	return len(proof.SubjectDigests) == len(required) && proofHasSubjects(proof, required...)
}

func validateCompilerEvidence(model ArtifactModel) []Diagnostic {
	var diagnostics []Diagnostic
	seen := map[string]struct{}{}
	for _, evidence := range model.CompilerEvidence {
		if evidence.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "compiler evidence ID is empty", evidence.Provenance))
		} else if _, exists := seen[evidence.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("duplicate compiler evidence ID %q", evidence.ID), evidence.Provenance))
		}
		seen[evidence.ID] = struct{}{}
		diagnostics = append(diagnostics, ValidateCompilerSemanticGraph(model, evidence)...)
		graphDigest, graphDigestErr := CompilerSemanticGraphDigest(evidence.SemanticGraph)
		if evidence.Method != CompilerEvidenceModelChecker || graphDigestErr != nil || evidence.FormulaDerivationDigest != graphDigest {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("compiler evidence %q is not bound to its exact typed semantic graph", evidence.ID), evidence.Provenance))
		}
		if err := validateFactSource(evidence.Provenance, model.Artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "compiler evidence: "+err.Error(), evidence.Provenance))
		}
		if evidence.SourceDigest != model.Artifact.Digest {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticStaleArtifact, fmt.Sprintf("compiler evidence %q source digest differs from modeled artifact", evidence.ID), evidence.Provenance))
		}
		for name, digest := range map[string]string{
			"workspace tree": evidence.WorkspaceTreeDigest,
			"environment":    evidence.EnvironmentDigest,
			"emitted IR":     evidence.EmittedIRDigest,
			"harness":        evidence.HarnessDigest,
		} {
			if !ValidDigest(digest) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("compiler evidence %q has invalid %s digest", evidence.ID, name), evidence.Provenance))
			}
		}
		if err := validateToolRef(evidence.Tool); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "compiler evidence tool: "+err.Error(), evidence.Provenance))
		}
		if err := validateToolRef(evidence.Prover); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "compiler evidence prover: "+err.Error(), evidence.Provenance))
		}
		if len(evidence.Argv) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("compiler evidence %q has empty argv", evidence.ID), evidence.Provenance))
		}
		for _, argument := range evidence.Argv {
			if argument == "" {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("compiler evidence %q argv contains an empty argument", evidence.ID), evidence.Provenance))
				break
			}
		}
		if evidence.TotalConstructs <= 0 || evidence.TranslatedConstructs != evidence.TotalConstructs {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("compiler evidence %q construct coverage is incomplete", evidence.ID), evidence.Provenance))
		}
		wantKind := map[Language]CompilerIRKind{LanguagePython: CompilerIRCPythonBytecode, LanguageRust: CompilerIRRustMIR, LanguageCPP: CompilerIRLLVM}[model.Language]
		if evidence.IRKind != wantKind {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("compiler evidence %q IR kind %q does not match language %q", evidence.ID, evidence.IRKind, model.Language), evidence.Provenance))
		}
		// OperationScopes, Partitions, BehaviorProofs, and OutcomeClosures are
		// legacy audit fields.  They are deliberately not proof authority: all
		// accepted predicates and point outcomes are centrally derived from the
		// replay-bound CompilerSemanticGraph.
	}
	return diagnostics
}

func validateOutcomeClosures(model ArtifactModel, evidence CompilerEvidence) []Diagnostic {
	var diagnostics []Diagnostic
	operations := map[string]Operation{}
	for _, operation := range model.Operations {
		if operation.Kind != OperationTest {
			operations[operation.ID] = operation
		}
	}
	scopes := map[string]CompilerPredicate{}
	for _, scope := range evidence.OperationScopes {
		scopes[scope.OperationID] = scope.ScopePredicate
	}
	seen := map[string]struct{}{}
	for _, closure := range evidence.OutcomeClosures {
		operation, exists := operations[closure.OperationID]
		if !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "outcome closure refers to unknown operation", closure.Provenance))
			continue
		}
		if _, duplicate := seen[closure.OperationID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "compiler evidence repeats outcome closure "+closure.OperationID, closure.Provenance))
		}
		seen[closure.OperationID] = struct{}{}
		if !ValidDigest(closure.BoundaryDigest) || closure.Totality != ProofProved || closure.Disjointness != ProofProved {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "outcome closure lacks boundary/totality/disjointness proof", closure.Provenance))
		}
		other := OtherOutcome(operation.ID, closure.Provenance)
		if len(closure.Complements) != 1 || closure.Complements[0].ID != other.ID || closure.Complements[0].Predicate.OutcomeID != other.ID || strings.TrimSpace(closure.Complements[0].Description) == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "outcome closure must contain the canonical per-operation other complement", closure.Provenance))
		}
		declaredIDs := make([]string, 0)
		for _, id := range operation.OutcomeIDs {
			if id != other.ID {
				declaredIDs = append(declaredIDs, id)
			}
		}
		sort.Strings(declaredIDs)
		gotIDs := make([]string, 0, len(closure.Declared))
		memberships := make([]CompilerPredicate, 0, len(closure.Declared)+1)
		for _, item := range closure.Declared {
			gotIDs = append(gotIDs, item.OutcomeID)
			memberships = append(memberships, item.Predicate)
			diagnostics = append(diagnostics, validateCompilerPredicate(item.Predicate, evidence.Tool, evidence.EmittedIRDigest, closure.Provenance)...)
		}
		sort.Strings(gotIDs)
		if !reflect.DeepEqual(gotIDs, declaredIDs) || hasDuplicateStrings(gotIDs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "outcome closure declared predicates differ from operation named outcomes", closure.Provenance))
		}
		if len(closure.Complements) == 1 {
			memberships = append(memberships, closure.Complements[0].Predicate.Predicate)
			diagnostics = append(diagnostics, validateCompilerPredicate(closure.Complements[0].Predicate.Predicate, evidence.Tool, evidence.EmittedIRDigest, closure.Provenance)...)
		}
		context := proofContextFor(evidence)
		scope := scopes[operation.ID]
		totalClaim := NewProofClaim(ClaimTotality, context, scope, memberships, nil)
		disjointClaim := NewProofClaim(ClaimDisjointness, context, scope, memberships, nil)
		if !reflect.DeepEqual(closure.TotalityProof.Claim, totalClaim) || closure.TotalityProof.QueryDigest == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "outcome closure totality claim differs from exact operation partition", closure.Provenance))
		}
		if !reflect.DeepEqual(closure.DisjointnessProof.Claim, disjointClaim) || closure.DisjointnessProof.QueryDigest == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "outcome closure disjointness claim differs from exact operation partition", closure.Provenance))
		}
		diagnostics = append(diagnostics, ValidateReplayableProof(closure.TotalityProof, SolverUNSAT, closure.Provenance)...)
		diagnostics = append(diagnostics, ValidateReplayableProof(closure.DisjointnessProof, SolverUNSAT, closure.Provenance)...)
	}
	if len(seen) != len(operations) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "compiler evidence does not prove outcome closure for every operation", evidence.Provenance))
	}
	return diagnostics
}

func proofContextFor(evidence CompilerEvidence) CompilerProofContext {
	return CompilerProofContext{
		SourceDigest: evidence.SourceDigest, WorkspaceTreeDigest: evidence.WorkspaceTreeDigest,
		EmittedIRDigest: evidence.EmittedIRDigest, HarnessDigest: evidence.HarnessDigest,
		Compiler: evidence.Tool,
	}
}

func samePredicates(left, right []CompilerPredicate) bool {
	if len(left) != len(right) {
		return false
	}
	used := make([]bool, len(right))
	for _, candidate := range left {
		found := false
		for index, other := range right {
			if !used[index] && reflect.DeepEqual(candidate, other) {
				used[index], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func claimMatches(proof ReplayableProof, kind ProofClaimKind, evidence CompilerEvidence, scope CompilerPredicate, memberships []CompilerPredicate) bool {
	claim := proof.Claim
	return claim.Kind == kind && reflect.DeepEqual(claim.Context, proofContextFor(evidence)) &&
		reflect.DeepEqual(claim.Scope, scope) && samePredicates(claim.Memberships, memberships) && len(claim.Outcomes) == 0 && claim.LeftPass == nil && claim.RightPass == nil
}

func validatePartitionEvidence(partition DomainPartitionEvidence, artifact ArtifactRef, evidence CompilerEvidence) []Diagnostic {
	var diagnostics []Diagnostic
	if err := validateFactSource(partition.Provenance, artifact); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "domain partition: "+err.Error(), partition.Provenance))
	}
	if partition.OperationID == "" || partition.DomainID == "" || !ValidDigest(partition.ScopePredicateDigest) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "domain partition lacks operation/domain/scope predicate digest", partition.Provenance))
	}
	diagnostics = append(diagnostics, validateCompilerPredicate(partition.ScopePredicate, evidence.Tool, evidence.EmittedIRDigest, partition.Provenance)...)
	if partition.ScopePredicateDigest != partition.ScopePredicate.FormulaDigest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "domain partition scope digest differs from embedded predicate", partition.Provenance))
	}
	foundOperationScope := false
	for _, scope := range evidence.OperationScopes {
		if scope.OperationID == partition.OperationID && reflect.DeepEqual(scope.ScopePredicate, partition.ScopePredicate) {
			foundOperationScope = true
			break
		}
	}
	if !foundOperationScope {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "domain partition does not use its operation's declared compiler scope", partition.Provenance))
	}
	if partition.Totality != ProofProved || !ValidDigest(partition.TotalityProofDigest) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "domain partition totality is not proved with immutable evidence", partition.Provenance))
	}
	if partition.TotalityProofDigest != partition.TotalityProof.QueryDigest || partition.TotalityProof.Prover != evidence.Prover {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "domain partition totality proof is not bound to declared prover/query", partition.Provenance))
	}
	memberships := make([]CompilerPredicate, 0, len(partition.Labels))
	for _, label := range partition.Labels {
		memberships = append(memberships, label.MembershipPredicate)
	}
	if !claimMatches(partition.TotalityProof, ClaimTotality, evidence, partition.ScopePredicate, memberships) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "domain partition totality proof claim differs from its scope/labels", partition.Provenance))
	}
	diagnostics = append(diagnostics, ValidateReplayableProof(partition.TotalityProof, SolverUNSAT, partition.Provenance)...)
	if partition.Disjointness != ProofProved || !ValidDigest(partition.DisjointnessProofDigest) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "domain partition disjointness is not proved with immutable evidence", partition.Provenance))
	}
	if partition.DisjointnessProofDigest != partition.DisjointnessProof.QueryDigest || partition.DisjointnessProof.Prover != evidence.Prover {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "domain partition disjointness proof is not bound to declared prover/query", partition.Provenance))
	}
	if !claimMatches(partition.DisjointnessProof, ClaimDisjointness, evidence, partition.ScopePredicate, memberships) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "domain partition disjointness proof claim differs from its scope/labels", partition.Provenance))
	}
	diagnostics = append(diagnostics, ValidateReplayableProof(partition.DisjointnessProof, SolverUNSAT, partition.Provenance)...)
	seenLabels := map[string]struct{}{}
	for _, label := range partition.Labels {
		if err := validateFactSource(label.Provenance, artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "label path: "+err.Error(), label.Provenance))
		}
		if label.ValueID == "" || !ValidDigest(label.PredicateDigest) || len(label.CompilerNodeIDs) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "label path lacks value/predicate/compiler nodes", label.Provenance))
		}
		diagnostics = append(diagnostics, validateCompilerPredicate(label.MembershipPredicate, evidence.Tool, evidence.EmittedIRDigest, label.Provenance)...)
		if label.PredicateDigest != label.MembershipPredicate.FormulaDigest || !reflect.DeepEqual(label.CompilerNodeIDs, label.MembershipPredicate.CompilerNodeIDs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("label %q digest/nodes differ from embedded membership predicate", label.ValueID), label.Provenance))
		}
		if _, exists := seenLabels[label.ValueID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("domain partition repeats label %q", label.ValueID), label.Provenance))
		}
		seenLabels[label.ValueID] = struct{}{}
		if !ValidDigest(label.ReachabilityProofDigest) || (label.Reachability != ProofProved && label.Reachability != ProofRefuted) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("label %q reachability is neither proved nor refuted with immutable evidence", label.ValueID), label.Provenance))
		} else if label.Reachability == ProofProved {
			if label.ConcreteWitness == nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("reachable label %q has no concrete witness", label.ValueID), label.Provenance))
			} else if err := ValidateLiteral(*label.ConcreteWitness); err != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("label %q concrete witness: %v", label.ValueID, err), label.Provenance))
			} else if digest, err := Digest(*label.ConcreteWitness); err != nil || label.WitnessDigest != digest {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("label %q concrete witness digest does not match", label.ValueID), label.Provenance))
			}
		} else if label.Reachability == ProofRefuted && (label.WitnessDigest != "" || label.ConcreteWitness != nil) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("unreachable label %q also carries a witness", label.ValueID), label.Provenance))
		}
		expectedResult := SolverSAT
		expectedClaim := ClaimReachability
		if label.Reachability == ProofRefuted {
			expectedResult = SolverUNSAT
			expectedClaim = ClaimUnreachability
		}
		if label.ReachabilityProofDigest != label.ReachabilityProof.QueryDigest || label.ReachabilityProof.Prover != evidence.Prover {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("label %q reachability proof is not bound to declared prover/query", label.ValueID), label.Provenance))
		}
		if !claimMatches(label.ReachabilityProof, expectedClaim, evidence, partition.ScopePredicate, []CompilerPredicate{label.MembershipPredicate}) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("label %q reachability proof claim differs from its scope/membership", label.ValueID), label.Provenance))
		}
		diagnostics = append(diagnostics, ValidateReplayableProof(label.ReachabilityProof, expectedResult, label.Provenance)...)
		for _, nodeID := range label.CompilerNodeIDs {
			if nodeID == "" {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("label %q has an empty compiler node ID", label.ValueID), label.Provenance))
			}
		}
	}
	seenExclusions := map[string]struct{}{}
	for _, exclusion := range partition.Exclusions {
		if err := validateFactSource(exclusion.Provenance, artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "constraint path: "+err.Error(), exclusion.Provenance))
		}
		if exclusion.ConstraintID == "" || exclusion.Result != ProofProved || !ValidDigest(exclusion.ProofDigest) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "constraint exclusion is not proved with immutable evidence", exclusion.Provenance))
		}
		if exclusion.ProofDigest != exclusion.Proof.QueryDigest || exclusion.Proof.Prover != evidence.Prover {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("constraint %q proof is not bound to declared prover/query", exclusion.ConstraintID), exclusion.Provenance))
		}
		diagnostics = append(diagnostics, ValidateReplayableProof(exclusion.Proof, SolverUNSAT, exclusion.Provenance)...)
		if exclusion.Proof.Claim.Kind != ClaimExclusion || !reflect.DeepEqual(exclusion.Proof.Claim.Context, proofContextFor(evidence)) || !reflect.DeepEqual(exclusion.Proof.Claim.Scope, partition.ScopePredicate) || len(exclusion.Proof.Claim.Outcomes) != 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("constraint %q proof claim differs from its compiler scope", exclusion.ConstraintID), exclusion.Provenance))
		}
		if _, exists := seenExclusions[exclusion.ConstraintID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("domain partition repeats constraint %q", exclusion.ConstraintID), exclusion.Provenance))
		}
		seenExclusions[exclusion.ConstraintID] = struct{}{}
	}
	return diagnostics
}

func validateBehaviorProofs(model ArtifactModel, evidence CompilerEvidence) []Diagnostic {
	var diagnostics []Diagnostic
	caseByID := map[string]BehaviorCase{}
	for _, behaviorCase := range model.Cases {
		caseByID[behaviorCase.ID] = behaviorCase
	}
	partitionPredicates := map[string]map[string]CompilerPredicate{}
	operationScopes := map[string]CompilerPredicate{}
	for _, scope := range evidence.OperationScopes {
		operationScopes[scope.OperationID] = scope.ScopePredicate
	}
	for _, partition := range evidence.Partitions {
		key := partition.OperationID + "\x00" + partition.DomainID
		partitionPredicates[key] = map[string]CompilerPredicate{}
		if previous, exists := operationScopes[partition.OperationID]; exists && !reflect.DeepEqual(previous, partition.ScopePredicate) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("operation %q has differing compiler scope predicates", partition.OperationID), partition.Provenance))
		}
		for _, label := range partition.Labels {
			partitionPredicates[key][label.ValueID] = label.MembershipPredicate
		}
	}
	seen := map[string]struct{}{}
	for _, proof := range evidence.BehaviorProofs {
		behaviorCase, exists := caseByID[proof.BehaviorCaseID]
		if !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("behavior proof refers to unknown case %q", proof.BehaviorCaseID), proof.Provenance))
			continue
		}
		if _, exists := seen[proof.BehaviorCaseID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("compiler evidence repeats behavior proof %q", proof.BehaviorCaseID), proof.Provenance))
		}
		seen[proof.BehaviorCaseID] = struct{}{}
		if err := validateFactSource(proof.Provenance, model.Artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "behavior proof: "+err.Error(), proof.Provenance))
		}
		if BehaviorRefKey(proof.Behavior) != BehaviorCaseKey(behaviorCase) || !sameStringSet(proof.OutcomeIDs, behaviorCase.OutcomeIDs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("behavior proof %q differs from modeled case", proof.BehaviorCaseID), proof.Provenance))
		}
		var wantPredicates []string
		var claimMemberships []CompilerPredicate
		for domainID, valueID := range behaviorCase.Conditions {
			predicate := partitionPredicates[behaviorCase.OperationID+"\x00"+domainID][valueID]
			wantPredicates = append(wantPredicates, predicate.FormulaDigest)
			claimMemberships = append(claimMemberships, predicate)
		}
		if !sameStringSet(wantPredicates, proof.CategoryPredicateDigests) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("behavior proof %q category predicates differ from assignment", proof.BehaviorCaseID), proof.Provenance))
		}
		claim := proof.RealizationProof.Claim
		claimOutcomeIDs := make([]string, 0, len(claim.Outcomes))
		for _, outcome := range claim.Outcomes {
			claimOutcomeIDs = append(claimOutcomeIDs, outcome.OutcomeID)
			diagnostics = append(diagnostics, validateCompilerPredicate(outcome.Predicate, evidence.Tool, evidence.EmittedIRDigest, proof.Provenance)...)
		}
		if proof.RealizationProof.Prover != evidence.Prover || claim.Kind != ClaimRealization || !reflect.DeepEqual(claim.Context, proofContextFor(evidence)) || !reflect.DeepEqual(claim.Scope, operationScopes[behaviorCase.OperationID]) || !samePredicates(claim.Memberships, claimMemberships) || !sameStringSet(claimOutcomeIDs, behaviorCase.OutcomeIDs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("behavior proof %q claim differs from compiler scope/category/outcomes", proof.BehaviorCaseID), proof.Provenance))
		}
		diagnostics = append(diagnostics, ValidateReplayableProof(proof.RealizationProof, SolverUNSAT, proof.Provenance)...)
	}
	if len(seen) != len(caseByID) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("compiler evidence proves %d behavior cases, want %d", len(seen), len(caseByID)), evidence.Provenance))
	}
	return diagnostics
}

// validateCompilerScope establishes exact operation ownership from the typed
// compiler graphs. Domain membership and impossible-case constraints belong
// to frozen Spec IR; central proof evaluates those axioms over D instead of
// accepting duplicated frontend-authored partition formulas.
func validateCompilerScope(_ []Domain, _ []Constraint, operations []Operation, evidence []CompilerEvidence, provenance Provenance) []Diagnostic {
	var diagnostics []Diagnostic
	wanted := map[string]struct{}{}
	for _, operation := range operations {
		if operation.Kind != OperationTest {
			wanted[operation.ID] = struct{}{}
		}
	}
	seen := map[string]struct{}{}
	for _, record := range evidence {
		if record.SemanticGraph == nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("compiler evidence %q has no typed semantic graph", record.ID), record.Provenance))
			continue
		}
		for _, root := range record.SemanticGraph.Operations {
			if _, exists := wanted[root.OperationID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("compiler semantic graph adds operation %q outside the modeled scope", root.OperationID), root.Provenance))
				continue
			}
			if _, duplicate := seen[root.OperationID]; duplicate {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("compiler semantic graph repeats operation %q", root.OperationID), root.Provenance))
			}
			seen[root.OperationID] = struct{}{}
		}
	}
	for operationID := range wanted {
		if _, exists := seen[operationID]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("compiler semantic graph omits operation %q", operationID), provenance))
		}
	}
	return diagnostics
}

// validateLegacyCompilerScope is retained only to decode historical evidence
// files. Production validation never calls it.
func validateLegacyCompilerScope(domains []Domain, constraints []Constraint, operations []Operation, evidence []CompilerEvidence, provenance Provenance) []Diagnostic {
	var diagnostics []Diagnostic
	domainByID := map[string]Domain{}
	for _, domain := range domains {
		domainByID[domain.ID] = domain
	}
	wantPartitions := map[string]Domain{}
	operationByID := map[string]Operation{}
	for _, operation := range operations {
		operationByID[operation.ID] = operation
		for _, domainID := range operation.DomainIDs {
			wantPartitions[operation.ID+"\x00"+domainID] = domainByID[domainID]
		}
	}
	seenPartitions := map[string]struct{}{}
	seenOperationScopes := map[string]struct{}{}
	seenConstraints := map[string]ConstraintPathEvidence{}
	type exclusionRecord struct {
		path     ConstraintPathEvidence
		evidence CompilerEvidence
		scope    CompilerPredicate
	}
	var exclusionRecords []exclusionRecord
	predicateByAssignment := map[string]CompilerPredicate{}
	constraintByID := map[string]Constraint{}
	for _, constraint := range constraints {
		constraintByID[constraint.ID] = constraint
	}
	for _, record := range evidence {
		for _, scope := range record.OperationScopes {
			if _, exists := operationByID[scope.OperationID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("compiler evidence adds operation scope %q outside spec scope", scope.OperationID), scope.Provenance))
				continue
			}
			if _, exists := seenOperationScopes[scope.OperationID]; exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("compiler evidence repeats operation scope %q", scope.OperationID), scope.Provenance))
			}
			seenOperationScopes[scope.OperationID] = struct{}{}
		}
		for _, partition := range record.Partitions {
			key := partition.OperationID + "\x00" + partition.DomainID
			domain, exists := wantPartitions[key]
			if !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("compiler evidence adds partition %s/%s outside spec scope", partition.OperationID, partition.DomainID), partition.Provenance))
				continue
			}
			if _, exists := seenPartitions[key]; exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("compiler evidence repeats partition %s/%s", partition.OperationID, partition.DomainID), partition.Provenance))
			}
			seenPartitions[key] = struct{}{}
			wantLabels := map[string]struct{}{}
			for _, value := range domain.Values {
				wantLabels[value.ID] = struct{}{}
			}
			if len(partition.Labels) != len(wantLabels) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("compiler partition %s/%s maps %d labels, want %d", partition.OperationID, partition.DomainID, len(partition.Labels), len(wantLabels)), partition.Provenance))
			}
			for _, label := range partition.Labels {
				predicateByAssignment[partition.OperationID+"\x00"+partition.DomainID+"\x00"+label.ValueID] = label.MembershipPredicate
				if _, exists := wantLabels[label.ValueID]; !exists {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("compiler partition %s/%s maps unknown label %q", partition.OperationID, partition.DomainID, label.ValueID), label.Provenance))
				}
				if label.Reachability == ProofRefuted {
					exclusionIDs := map[string]struct{}{}
					for _, exclusion := range partition.Exclusions {
						exclusionIDs[exclusion.ConstraintID] = struct{}{}
					}
					fullyExcluded := true
					operation := operationByID[partition.OperationID]
					for _, assignment := range EnumerateAssignments(selectDomains(domains, operation.DomainIDs)) {
						if assignment[partition.DomainID] != label.ValueID {
							continue
						}
						found := false
						for constraintID, constraint := range constraintByID {
							if constraint.OperationID == partition.OperationID && reflect.DeepEqual(constraint.Conditions, assignment) {
								_, found = exclusionIDs[constraintID]
								if found {
									break
								}
							}
						}
						if !found {
							fullyExcluded = false
							break
						}
					}
					if !fullyExcluded {
						diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("unreachable label %q lacks a matching proved exclusion", label.ValueID), label.Provenance))
					}
				}
			}
			for _, exclusion := range partition.Exclusions {
				exclusionRecords = append(exclusionRecords, exclusionRecord{path: exclusion, evidence: record, scope: partition.ScopePredicate})
				if previous, exists := seenConstraints[exclusion.ConstraintID]; exists && (previous.Result != exclusion.Result || previous.ProofDigest != exclusion.ProofDigest) {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("compiler evidence gives conflicting exclusion proofs for %q", exclusion.ConstraintID), exclusion.Provenance))
				}
				seenConstraints[exclusion.ConstraintID] = exclusion
			}
		}
	}
	for key := range wantPartitions {
		if _, exists := seenPartitions[key]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("compiler evidence omits operation/domain partition %q", key), provenance))
		}
	}
	for operationID := range operationByID {
		if _, exists := seenOperationScopes[operationID]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("compiler evidence omits operation scope %q", operationID), provenance))
		}
	}
	for _, constraint := range constraints {
		if _, exists := seenConstraints[constraint.ID]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("compiler evidence omits exclusion proof %q", constraint.ID), provenance))
		}
	}
	for _, record := range exclusionRecords {
		constraint, exists := constraintByID[record.path.ConstraintID]
		if !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("compiler evidence proves unknown exclusion %q", record.path.ConstraintID), record.path.Provenance))
			continue
		}
		memberships := make([]CompilerPredicate, 0, len(constraint.Conditions))
		for domainID, valueID := range constraint.Conditions {
			predicate, exists := predicateByAssignment[constraint.OperationID+"\x00"+domainID+"\x00"+valueID]
			if !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("constraint %q lacks membership predicate for %s=%s", constraint.ID, domainID, valueID), record.path.Provenance))
				continue
			}
			memberships = append(memberships, predicate)
		}
		if !claimMatches(record.path.Proof, ClaimExclusion, record.evidence, record.scope, memberships) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("constraint %q proof claim differs from its exact assignment", constraint.ID), record.path.Provenance))
		}
	}
	return diagnostics
}
