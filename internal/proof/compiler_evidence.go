package proof

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

// validateCompilerEvidence establishes the concrete reference semantics used
// by the finite proof.  Spec domains remain independently compiled axioms;
// reference authority comes from either exact replay over every point or one
// replay-bound typed compiler graph that central proof evaluates itself.
func (v *validator) validateCompilerEvidence() {
	seenOperations := make(map[string]bool)
	codeArtifacts := 0

	for artifactIndex := range v.task.Artifacts {
		artifact := &v.task.Artifacts[artifactIndex]
		if artifact.Kind != semanticir.ArtifactCode {
			seenEvidence := make(map[string]bool)
			for evidenceIndex := range artifact.CompilerEvidence {
				evidence := &artifact.CompilerEvidence[evidenceIndex]
				if strings.TrimSpace(evidence.ID) == "" || seenEvidence[evidence.ID] {
					v.add("invalid-compiler-evidence", fmt.Sprintf("artifact %q has an empty or duplicate compiler evidence ID %q", artifact.Artifact.ID, evidence.ID), &evidence.Provenance)
				}
				seenEvidence[evidence.ID] = true
				v.validateCompilerEvidenceRecord(artifact, &artifact.CompilerEvidence[evidenceIndex])
			}
			continue
		}
		codeArtifacts++
		v.validateScopeClosure(artifact)
		if len(artifact.CompilerEvidence) > 0 && len(artifact.ExhaustiveEvidence) > 0 {
			v.add("ambiguous-code-evidence", fmt.Sprintf("code artifact %q mixes compiler/model-checker and exhaustive-execution evidence", artifact.Artifact.ID), &artifact.Coverage.Provenance)
			continue
		}
		if len(artifact.ExhaustiveEvidence) > 0 {
			if v.validateExhaustiveExecutionEvidence(artifact) {
				for _, operation := range artifact.Operations {
					if operation.Kind == semanticir.OperationTest {
						continue
					}
					if seenOperations[operation.ID] {
						v.add("ambiguous-reference-operation", fmt.Sprintf("reference evidence defines operation %q more than once", operation.ID), &artifact.Coverage.Provenance)
					}
					seenOperations[operation.ID] = true
				}
			}
			continue
		}
		if len(artifact.CompilerEvidence) == 0 {
			v.add("missing-compiler-evidence", fmt.Sprintf("code artifact %q has no compiler-IR partition evidence", artifact.Artifact.ID), &artifact.Coverage.Provenance)
			continue
		}

		if len(artifact.CompilerEvidence) != 1 {
			v.add("ambiguous-compiler-evidence", fmt.Sprintf("code artifact %q has %d compiler semantic graphs; want exactly one", artifact.Artifact.ID, len(artifact.CompilerEvidence)), &artifact.Coverage.Provenance)
			continue
		}
		seenEvidence := make(map[string]bool)
		for evidenceIndex := range artifact.CompilerEvidence {
			evidence := &artifact.CompilerEvidence[evidenceIndex]
			v.validateCompilerEvidenceRecord(artifact, evidence)
			if strings.TrimSpace(evidence.ID) == "" || seenEvidence[evidence.ID] {
				v.add("invalid-compiler-evidence", fmt.Sprintf("code artifact %q has an empty or duplicate compiler evidence ID %q", artifact.Artifact.ID, evidence.ID), &evidence.Provenance)
			}
			seenEvidence[evidence.ID] = true
			if evidence.SemanticGraph == nil {
				continue
			}
			for _, root := range evidence.SemanticGraph.Operations {
				if _, wanted := targetOperationScope(v.task.Operations, root.OperationID); !wanted {
					v.add("invalid-compiler-operation", fmt.Sprintf("compiler evidence %q adds operation %q outside spec scope", evidence.ID, root.OperationID), &root.Provenance)
				} else if seenOperations[root.OperationID] {
					v.add("ambiguous-reference-operation", fmt.Sprintf("reference evidence defines operation %q more than once", root.OperationID), &root.Provenance)
				}
				seenOperations[root.OperationID] = true
			}
		}
	}
	if codeArtifacts == 0 {
		return
	}
	for _, operation := range v.task.Operations {
		if operation.Kind != semanticir.OperationTest && !seenOperations[operation.ID] {
			v.add("missing-reference-operation", fmt.Sprintf("reference evidence omits operation %q", operation.ID), &operation.Provenance)
		}
	}
}

func (v *validator) validateExhaustiveExecutionEvidence(artifact *semanticir.ArtifactModel) bool {
	startBlockers := len(v.blockers)
	for _, outcome := range artifact.Outcomes {
		if len(outcome.Effects) != 0 {
			v.add("untrusted-exhaustive-effect-observation", fmt.Sprintf("exact execution cannot certify effectful outcome %q without typed evidence proving that the observation captures the real effect trace", outcome.ID), &outcome.Provenance)
		}
	}
	diagnostics := semanticir.ValidateArtifactModel(*artifact)
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity != semanticir.SeverityError {
			continue
		}
		provenance := diagnostic.Provenance
		v.add("invalid-exhaustive-execution", fmt.Sprintf("%s: %s", diagnostic.Code, diagnostic.Message), &provenance)
	}
	for index := range artifact.ExhaustiveEvidence {
		evidence := &artifact.ExhaustiveEvidence[index]
		v.provenance(evidence.Provenance, fmt.Sprintf("exhaustive execution evidence %q", evidence.ID))
		if len(v.blockers) != startBlockers {
			continue
		}
		if err := replayExhaustiveExecution(v.ctx, v.task, artifact, *evidence); err != nil {
			v.add("exhaustive-replay-failed", err.Error(), &evidence.Provenance)
		}
	}
	return len(v.blockers) == startBlockers && len(artifact.ExhaustiveEvidence) == 1
}

func targetOperationScope(operations []semanticir.Operation, operationID string) (semanticir.Operation, bool) {
	for _, operation := range operations {
		if operation.ID == operationID && operation.Kind != semanticir.OperationTest {
			return operation, true
		}
	}
	return semanticir.Operation{}, false
}

func (v *validator) validateCompilerEvidenceRecord(artifact *semanticir.ArtifactModel, evidence *semanticir.CompilerEvidence) {
	label := fmt.Sprintf("compiler evidence %q", evidence.ID)
	v.validateCentralCompilerGraph(artifact, evidence)
	if strings.TrimSpace(evidence.ID) == "" {
		v.add("invalid-compiler-evidence", "compiler evidence ID is empty", &evidence.Provenance)
	}
	if evidence.Method != semanticir.CompilerEvidenceModelChecker || !digestPattern.MatchString(evidence.FormulaDerivationDigest) {
		v.add("unsupported-compiler-evidence-method", label+" is not typed compiler/model-checker entailment with a frozen formula derivation", &evidence.Provenance)
	}
	v.provenance(evidence.Provenance, label)
	v.requireProvenanceKind(evidence.Provenance, artifact.Kind, label)
	if evidence.SourceDigest != artifact.Artifact.Digest {
		v.add("stale-compiler-evidence", fmt.Sprintf("%s source digest does not match frozen artifact %q", label, artifact.Artifact.ID), &evidence.Provenance)
	}
	for name, digest := range map[string]string{
		"workspace tree": evidence.WorkspaceTreeDigest,
		"environment":    evidence.EnvironmentDigest,
		"emitted IR":     evidence.EmittedIRDigest,
		"harness":        evidence.HarnessDigest,
	} {
		if !digestPattern.MatchString(digest) {
			v.add("invalid-compiler-evidence", fmt.Sprintf("%s has an invalid %s digest", label, name), &evidence.Provenance)
		}
	}
	v.validateTool(evidence.Tool, label+" compiler")
	v.validateTool(evidence.Prover, label+" prover")
	if !v.environmentHasTool(evidence.Tool) || !v.environmentHasTool(evidence.Prover) {
		v.add("missing-tool-evidence", fmt.Sprintf("%s compiler/prover is not frozen in the environment model", label), &evidence.Provenance)
	}
	if err := verifyToolBinary(evidence.Tool); err != nil {
		v.add("stale-compiler-tool", fmt.Sprintf("%s compiler binary: %v", label, err), &evidence.Provenance)
	}
	if !v.environmentHasSnapshot(evidence.WorkspaceTreeDigest, evidence.EnvironmentDigest) {
		v.add("stale-compiler-evidence", fmt.Sprintf("%s workspace/environment digests match no frozen workspace command", label), &evidence.Provenance)
	}
	if len(evidence.Argv) == 0 {
		v.add("invalid-compiler-evidence", label+" has an empty invocation", &evidence.Provenance)
	}
	for _, argument := range evidence.Argv {
		if argument == "" {
			v.add("invalid-compiler-evidence", label+" invocation contains an empty argument", &evidence.Provenance)
			break
		}
	}
	if evidence.TotalConstructs <= 0 || evidence.TranslatedConstructs != evidence.TotalConstructs {
		v.add("incomplete-compiler-translation", fmt.Sprintf("%s compiler coverage is incomplete (%d/%d)", label, evidence.TranslatedConstructs, evidence.TotalConstructs), &evidence.Provenance)
	}
	wantedIR := map[semanticir.Language]semanticir.CompilerIRKind{
		semanticir.LanguagePython: semanticir.CompilerIRCPythonBytecode,
		semanticir.LanguageRust:   semanticir.CompilerIRRustMIR,
		semanticir.LanguageCPP:    semanticir.CompilerIRLLVM,
	}[artifact.Language]
	if wantedIR == "" || evidence.IRKind != wantedIR {
		v.add("invalid-compiler-evidence", fmt.Sprintf("%s IR kind %q does not match artifact language %q", label, evidence.IRKind, artifact.Language), &evidence.Provenance)
	}
	// Legacy frontend-authored SMT scopes/partitions/realization records are
	// intentionally ignored. The replayed typed graph is lowered and evaluated
	// centrally above; accepting a second semantic authority would make the
	// proof depend on circular evidence supplied by the frontend.
}

func (v *validator) validateCompilerOutcomeClosures(artifact *semanticir.ArtifactModel, evidence *semanticir.CompilerEvidence) {
	operations := make(map[string]semanticir.Operation)
	for _, operation := range artifact.Operations {
		if operation.Kind != semanticir.OperationTest {
			operations[operation.ID] = operation
		}
	}
	seen := map[string]bool{}
	for closureIndex := range evidence.OutcomeClosures {
		closure := &evidence.OutcomeClosures[closureIndex]
		label := fmt.Sprintf("compiler outcome closure %q", closure.OperationID)
		operation, exists := operations[closure.OperationID]
		if !exists || seen[closure.OperationID] {
			v.add("invalid-outcome-closure", label+" refers to an unknown or duplicated operation", &closure.Provenance)
			continue
		}
		seen[closure.OperationID] = true
		v.provenance(closure.Provenance, label)
		v.requireProvenanceKind(closure.Provenance, semanticir.ArtifactCode, label)
		if !digestPattern.MatchString(closure.BoundaryDigest) || closure.Totality != semanticir.ProofProved || closure.Disjointness != semanticir.ProofProved {
			v.add("unproved-outcome-closure", label+" has no frozen boundary/totality/disjointness proof", &closure.Provenance)
		}
		other := semanticir.OtherOutcome(operation.ID, closure.Provenance)
		if !containsString(operation.OutcomeIDs, other.ID) {
			v.add("incomplete-outcome-closure", label+" operation universe omits its canonical other outcome", &closure.Provenance)
		}
		wantedNamed := map[string]bool{}
		for _, outcomeID := range operation.OutcomeIDs {
			if outcomeID != other.ID {
				wantedNamed[outcomeID] = true
			}
		}
		memberships := make([]semanticir.CompilerPredicate, 0, len(closure.Declared)+1)
		declaredSeen := map[string]bool{}
		for declaredIndex := range closure.Declared {
			declared := &closure.Declared[declaredIndex]
			if !wantedNamed[declared.OutcomeID] || declaredSeen[declared.OutcomeID] {
				v.add("invalid-outcome-closure", fmt.Sprintf("%s has unknown or duplicate named outcome %q", label, declared.OutcomeID), &closure.Provenance)
			}
			declaredSeen[declared.OutcomeID] = true
			v.validateCompilerPredicate(declared.Predicate, evidence.Tool, evidence.EmittedIRDigest, label+" named outcome "+declared.OutcomeID, &closure.Provenance)
			memberships = append(memberships, declared.Predicate)
		}
		if len(declaredSeen) != len(wantedNamed) {
			v.add("incomplete-outcome-closure", fmt.Sprintf("%s binds %d named outcomes; want %d", label, len(declaredSeen), len(wantedNamed)), &closure.Provenance)
		}
		if len(closure.Complements) != 1 {
			v.add("incomplete-outcome-closure", label+" must have exactly one canonical other complement", &closure.Provenance)
		} else {
			complement := &closure.Complements[0]
			validKind := complement.Kind == semanticir.OutcomeComplementReturn || complement.Kind == semanticir.OutcomeComplementRaise || complement.Kind == semanticir.OutcomeComplementEffects || complement.Kind == semanticir.OutcomeComplementNontermination
			if complement.ID != other.ID || complement.Predicate.OutcomeID != other.ID || strings.TrimSpace(complement.Description) == "" || !validKind {
				v.add("invalid-outcome-closure", label+" complement is not the canonical typed other outcome", &closure.Provenance)
			}
			v.validateCompilerPredicate(complement.Predicate.Predicate, evidence.Tool, evidence.EmittedIRDigest, label+" other outcome", &closure.Provenance)
			memberships = append(memberships, complement.Predicate.Predicate)
		}
		scope, foundScope := compilerOperationScope(*evidence, operation.ID)
		if !foundScope {
			v.add("incomplete-outcome-closure", label+" has no compiler operation scope", &closure.Provenance)
			continue
		}
		totalClaim := semanticir.NewProofClaim(semanticir.ClaimTotality, compilerProofContext(*evidence), scope, memberships, nil)
		disjointClaim := semanticir.NewProofClaim(semanticir.ClaimDisjointness, compilerProofContext(*evidence), scope, memberships, nil)
		v.validateCompilerReplay(closure.TotalityProof, semanticir.SolverUNSAT, totalClaim, *evidence, label+" totality", &closure.Provenance)
		v.validateCompilerReplay(closure.DisjointnessProof, semanticir.SolverUNSAT, disjointClaim, *evidence, label+" disjointness", &closure.Provenance)
	}
	if len(seen) != len(operations) {
		v.add("incomplete-outcome-closure", fmt.Sprintf("compiler evidence %q proves outcome closure for %d operations; want %d", evidence.ID, len(seen), len(operations)), &evidence.Provenance)
	}
}

func (v *validator) validateCompilerPartition(partition semanticir.DomainPartitionEvidence, domain semanticir.Domain, evidence semanticir.CompilerEvidence) {
	label := fmt.Sprintf("compiler partition %q/%q", partition.OperationID, partition.DomainID)
	if strings.TrimSpace(partition.OperationID) == "" || strings.TrimSpace(partition.DomainID) == "" || !digestPattern.MatchString(partition.ScopePredicateDigest) {
		v.add("invalid-compiler-partition", label+" lacks an operation, domain, or scope predicate digest", &partition.Provenance)
	}
	if partition.Totality != semanticir.ProofProved || !digestPattern.MatchString(partition.TotalityProofDigest) {
		v.add("compiler-partition-unproved", label+" totality is not proved with immutable evidence", &partition.Provenance)
	}
	if partition.Disjointness != semanticir.ProofProved || !digestPattern.MatchString(partition.DisjointnessProofDigest) {
		v.add("compiler-partition-unproved", label+" disjointness is not proved with immutable evidence", &partition.Provenance)
	}
	v.validateCompilerPredicate(partition.ScopePredicate, evidence.Tool, evidence.EmittedIRDigest, label+" scope", &partition.Provenance)
	if partition.ScopePredicateDigest != partition.ScopePredicate.FormulaDigest {
		v.add("invalid-compiler-partition", label+" scope digest differs from its embedded predicate", &partition.Provenance)
	}
	operationScope, foundScope := compilerOperationScope(evidence, partition.OperationID)
	if !foundScope || !reflect.DeepEqual(operationScope, partition.ScopePredicate) {
		v.add("invalid-compiler-partition", label+" does not use its operation's exact declared compiler scope", &partition.Provenance)
	}
	wantedLabels := make(map[string]bool, len(domain.Values))
	for _, value := range domain.Values {
		wantedLabels[value.ID] = true
	}
	seenLabels := make(map[string]bool)
	memberships := make([]semanticir.CompilerPredicate, 0, len(partition.Labels))
	for pathIndex := range partition.Labels {
		path := &partition.Labels[pathIndex]
		if !wantedLabels[path.ValueID] {
			v.add("invalid-compiler-label", fmt.Sprintf("%s maps unknown label %q", label, path.ValueID), &path.Provenance)
		}
		if seenLabels[path.ValueID] {
			v.add("ambiguous-compiler-label", fmt.Sprintf("%s maps label %q more than once", label, path.ValueID), &path.Provenance)
		}
		seenLabels[path.ValueID] = true
		if !digestPattern.MatchString(path.PredicateDigest) || len(path.CompilerNodeIDs) == 0 {
			v.add("invalid-compiler-label", fmt.Sprintf("%s label %q lacks a predicate digest or compiler nodes", label, path.ValueID), &path.Provenance)
		}
		v.validateCompilerPredicate(path.MembershipPredicate, evidence.Tool, evidence.EmittedIRDigest, fmt.Sprintf("%s label %q membership", label, path.ValueID), &path.Provenance)
		if path.PredicateDigest != path.MembershipPredicate.FormulaDigest || !reflect.DeepEqual(path.CompilerNodeIDs, path.MembershipPredicate.CompilerNodeIDs) {
			v.add("invalid-compiler-label", fmt.Sprintf("%s label %q digest/nodes differ from its embedded membership predicate", label, path.ValueID), &path.Provenance)
		}
		memberships = append(memberships, path.MembershipPredicate)
		seenNodes := make(map[string]bool)
		for _, nodeID := range path.CompilerNodeIDs {
			if strings.TrimSpace(nodeID) == "" || seenNodes[nodeID] {
				v.add("invalid-compiler-label", fmt.Sprintf("%s label %q has an empty or duplicate compiler node ID", label, path.ValueID), &path.Provenance)
			}
			seenNodes[nodeID] = true
		}
		if path.Reachability != semanticir.ProofProved || !digestPattern.MatchString(path.ReachabilityProofDigest) || path.ConcreteWitness == nil || !digestPattern.MatchString(path.WitnessDigest) {
			v.add("compiler-label-unproved", fmt.Sprintf("%s label %q is not proved reachable with an immutable witness", label, path.ValueID), &path.Provenance)
		} else {
			witnessDigest, err := semanticir.Digest(*path.ConcreteWitness)
			if err != nil || witnessDigest != path.WitnessDigest {
				v.add("compiler-label-unproved", fmt.Sprintf("%s label %q concrete witness digest is invalid", label, path.ValueID), &path.Provenance)
			}
			if err := semanticir.ValidateLiteral(*path.ConcreteWitness); err != nil {
				v.add("compiler-label-unproved", fmt.Sprintf("%s label %q concrete witness is invalid: %v", label, path.ValueID, err), &path.Provenance)
			}
		}
		if path.ReachabilityProofDigest != path.ReachabilityProof.QueryDigest {
			v.add("invalid-compiler-proof", fmt.Sprintf("%s label %q proof digest differs from the replay query", label, path.ValueID), &path.Provenance)
		}
		wantClaim := semanticir.NewProofClaim(semanticir.ClaimReachability, compilerProofContext(evidence), partition.ScopePredicate, []semanticir.CompilerPredicate{path.MembershipPredicate}, nil)
		v.validateCompilerReplay(path.ReachabilityProof, semanticir.SolverSAT, wantClaim, evidence, fmt.Sprintf("%s label %q reachability", label, path.ValueID), &path.Provenance)
		if path.ConcreteWitness != nil {
			// The concrete compiler witness need not equal a semantic label literal,
			// but it must at least have the declared finite domain type.
			if path.ConcreteWitness.Type != domain.Type {
				v.add("invalid-compiler-label", fmt.Sprintf("%s label %q witness type %q differs from domain type %q", label, path.ValueID, path.ConcreteWitness.Type, domain.Type), &path.Provenance)
			}
		}
	}
	for valueID := range wantedLabels {
		if !seenLabels[valueID] {
			v.add("missing-compiler-label", fmt.Sprintf("%s omits label %q", label, valueID), &partition.Provenance)
		}
	}
	for exclusionIndex := range partition.Exclusions {
		exclusion := &partition.Exclusions[exclusionIndex]
		if strings.TrimSpace(exclusion.ConstraintID) == "" || exclusion.Result != semanticir.ProofProved || !digestPattern.MatchString(exclusion.ProofDigest) {
			v.add("compiler-exclusion-unproved", fmt.Sprintf("%s exclusion %q is not proved unreachable", label, exclusion.ConstraintID), &exclusion.Provenance)
		}
		if exclusion.ProofDigest != exclusion.Proof.QueryDigest {
			v.add("invalid-compiler-proof", fmt.Sprintf("%s exclusion %q proof digest differs from the replay query", label, exclusion.ConstraintID), &exclusion.Provenance)
		}
		claim, exact := v.compilerExclusionClaim(evidence, exclusion.ConstraintID)
		if !exact {
			v.add("invalid-compiler-proof", fmt.Sprintf("%s exclusion %q cannot be reconstructed from its exact assignment", label, exclusion.ConstraintID), &exclusion.Provenance)
			continue
		}
		v.validateCompilerReplay(exclusion.Proof, semanticir.SolverUNSAT, claim, evidence, fmt.Sprintf("%s exclusion %q", label, exclusion.ConstraintID), &exclusion.Provenance)
	}
	totalityClaim := semanticir.NewProofClaim(semanticir.ClaimTotality, compilerProofContext(evidence), partition.ScopePredicate, memberships, nil)
	disjointnessClaim := semanticir.NewProofClaim(semanticir.ClaimDisjointness, compilerProofContext(evidence), partition.ScopePredicate, memberships, nil)
	if partition.TotalityProofDigest != partition.TotalityProof.QueryDigest {
		v.add("invalid-compiler-proof", label+" totality proof digest differs from the replay query", &partition.Provenance)
	}
	if partition.DisjointnessProofDigest != partition.DisjointnessProof.QueryDigest {
		v.add("invalid-compiler-proof", label+" disjointness proof digest differs from the replay query", &partition.Provenance)
	}
	v.validateCompilerReplay(partition.TotalityProof, semanticir.SolverUNSAT, totalityClaim, evidence, label+" totality", &partition.Provenance)
	v.validateCompilerReplay(partition.DisjointnessProof, semanticir.SolverUNSAT, disjointnessClaim, evidence, label+" disjointness", &partition.Provenance)
}

func compilerOperationScope(evidence semanticir.CompilerEvidence, operationID string) (semanticir.CompilerPredicate, bool) {
	for _, scope := range evidence.OperationScopes {
		if scope.OperationID == operationID {
			return scope.ScopePredicate, true
		}
	}
	return semanticir.CompilerPredicate{}, false
}

func (v *validator) compilerExclusionClaim(evidence semanticir.CompilerEvidence, constraintID string) (semanticir.ProofClaim, bool) {
	var constraint semanticir.Constraint
	found := false
	for _, candidate := range v.task.Constraints {
		if candidate.ID == constraintID {
			constraint, found = candidate, true
			break
		}
	}
	if !found {
		return semanticir.ProofClaim{}, false
	}
	scope, found := compilerOperationScope(evidence, constraint.OperationID)
	if !found {
		return semanticir.ProofClaim{}, false
	}
	operation := v.operations[constraint.OperationID]
	memberships := make([]semanticir.CompilerPredicate, 0, len(operation.DomainIDs))
	for _, domainID := range operation.DomainIDs {
		valueID, exists := constraint.Conditions[domainID]
		if !exists {
			return semanticir.ProofClaim{}, false
		}
		foundPredicate := false
		for _, partition := range evidence.Partitions {
			if partition.OperationID != constraint.OperationID || partition.DomainID != domainID {
				continue
			}
			for _, path := range partition.Labels {
				if path.ValueID == valueID {
					memberships = append(memberships, path.MembershipPredicate)
					foundPredicate = true
					break
				}
			}
		}
		if !foundPredicate {
			return semanticir.ProofClaim{}, false
		}
	}
	return semanticir.NewProofClaim(semanticir.ClaimExclusion, compilerProofContext(evidence), scope, memberships, nil), true
}

func (v *validator) validateCompilerPredicate(predicate semanticir.CompilerPredicate, tool semanticir.ToolRef, irDigest, label string, provenance *semanticir.Provenance) {
	if predicate.Logic != semanticir.ProofLogicSMTLIB2 || len(bytes.TrimSpace(predicate.Formula)) == 0 || predicate.FormulaDigest != semanticir.DigestBytes(predicate.Formula) {
		v.add("invalid-compiler-predicate", label+" has invalid logic/formula/digest", provenance)
	}
	if predicate.DeclarationsDigest != semanticir.DigestBytes(predicate.Declarations) {
		v.add("invalid-compiler-predicate", label+" has an invalid declarations digest", provenance)
	}
	if predicate.Tool != tool || predicate.IRDigest != irDigest || !digestPattern.MatchString(predicate.IRDigest) {
		v.add("invalid-compiler-predicate", label+" is not bound to the frozen compiler and IR", provenance)
	}
	if len(predicate.CompilerNodeIDs) == 0 {
		v.add("invalid-compiler-predicate", label+" has no compiler node IDs", provenance)
	}
	seen := map[string]bool{}
	for _, nodeID := range predicate.CompilerNodeIDs {
		if strings.TrimSpace(nodeID) == "" || seen[nodeID] {
			v.add("invalid-compiler-predicate", label+" has an empty or duplicate compiler node ID", provenance)
		}
		seen[nodeID] = true
	}
}

func (v *validator) validateCompilerReplay(replay semanticir.ReplayableProof, expected semanticir.SolverResult, claim semanticir.ProofClaim, evidence semanticir.CompilerEvidence, label string, provenance *semanticir.Provenance) {
	if !reflect.DeepEqual(replay.Claim, claim) {
		v.add("noncanonical-compiler-proof", label+" typed claim differs from the exact obligation", provenance)
		return
	}
	canonical, err := semanticir.CanonicalProofQuery(claim)
	if err != nil || !bytes.Equal(replay.Query, canonical) {
		v.add("noncanonical-compiler-proof", fmt.Sprintf("%s query is not centrally reconstructed from its typed obligation", label), provenance)
		return
	}
	if replay.Prover != evidence.Prover || replay.EnvironmentDigest != evidence.EnvironmentDigest {
		v.add("stale-compiler-proof", label+" is not bound to the declared prover/environment", provenance)
		return
	}
	if err := Replay(v.ctx, replay, expected, v.task.Environment); err != nil {
		v.add("compiler-proof-replay-failed", fmt.Sprintf("%s: %v", label, err), provenance)
	}
}

func compilerProofContext(evidence semanticir.CompilerEvidence) semanticir.CompilerProofContext {
	return semanticir.CompilerProofContext{
		SourceDigest: evidence.SourceDigest, WorkspaceTreeDigest: evidence.WorkspaceTreeDigest,
		EmittedIRDigest: evidence.EmittedIRDigest, HarnessDigest: evidence.HarnessDigest, Compiler: evidence.Tool,
	}
}

func (v *validator) validateCompilerBehaviorProofs(artifact *semanticir.ArtifactModel, evidence *semanticir.CompilerEvidence) {
	cases := make(map[string]semanticir.BehaviorCase, len(artifact.Cases))
	for _, behaviorCase := range artifact.Cases {
		cases[behaviorCase.ID] = behaviorCase
		if len(behaviorCase.OutcomeIDs) != 1 {
			v.add("nondeterministic-code-model", fmt.Sprintf("code case %q fixes %d outcomes; v0.1 requires exactly one", behaviorCase.ID, len(behaviorCase.OutcomeIDs)), &behaviorCase.Provenance)
		}
	}
	partitions := make(map[string]semanticir.DomainPartitionEvidence)
	for _, partition := range evidence.Partitions {
		partitions[compilerPartitionKey(partition.OperationID, partition.DomainID)] = partition
	}
	seen := map[string]bool{}
	for proofIndex := range evidence.BehaviorProofs {
		behaviorProof := &evidence.BehaviorProofs[proofIndex]
		label := fmt.Sprintf("compiler behavior proof %q", behaviorProof.BehaviorCaseID)
		v.provenance(behaviorProof.Provenance, label)
		v.requireProvenanceKind(behaviorProof.Provenance, artifact.Kind, label)
		behaviorCase, exists := cases[behaviorProof.BehaviorCaseID]
		if !exists || seen[behaviorProof.BehaviorCaseID] {
			v.add("invalid-compiler-behavior-proof", label+" is unknown or duplicated", &behaviorProof.Provenance)
			continue
		}
		if len(behaviorCase.OutcomeIDs) != 1 {
			continue
		}
		seen[behaviorProof.BehaviorCaseID] = true
		operation := v.operations[behaviorCase.OperationID]
		proofAssignment := canonicalAssignment(v.operationDomains[behaviorCase.OperationID], behaviorProof.Behavior.Conditions)
		caseAssignment := canonicalAssignment(v.operationDomains[behaviorCase.OperationID], behaviorCase.Conditions)
		if semanticir.BehaviorRefKey(behaviorProof.Behavior) != semanticir.BehaviorCaseKey(behaviorCase) || !sameDomainSet(operation.DomainIDs, behaviorProof.Behavior.Conditions) || proofAssignment != caseAssignment || !reflect.DeepEqual(behaviorProof.OutcomeIDs, behaviorCase.OutcomeIDs) {
			v.add("invalid-compiler-behavior-proof", label+" differs from the frozen behavior case", &behaviorProof.Provenance)
		}
		scope, foundScope := compilerOperationScope(*evidence, behaviorCase.OperationID)
		if !foundScope {
			v.add("missing-compiler-scope", label+" has no declared operation compiler scope", &behaviorProof.Provenance)
		}
		var memberships []semanticir.CompilerPredicate
		var categoryDigests []string
		for _, domainID := range operation.DomainIDs {
			partition, ok := partitions[compilerPartitionKey(operation.ID, domainID)]
			if !ok {
				v.add("invalid-compiler-behavior-proof", fmt.Sprintf("%s lacks partition %q", label, domainID), &behaviorProof.Provenance)
				continue
			}
			if foundScope && !reflect.DeepEqual(scope, partition.ScopePredicate) {
				v.add("ambiguous-compiler-scope", label+" combines domain partitions with different operation scopes", &behaviorProof.Provenance)
			}
			valueID := behaviorCase.Conditions[domainID]
			found := false
			for _, path := range partition.Labels {
				if path.ValueID == valueID {
					memberships = append(memberships, path.MembershipPredicate)
					categoryDigests = append(categoryDigests, path.PredicateDigest)
					found = true
					break
				}
			}
			if !found {
				v.add("invalid-compiler-behavior-proof", fmt.Sprintf("%s has no category predicate for %s=%s", label, domainID, valueID), &behaviorProof.Provenance)
			}
		}
		sort.Strings(categoryDigests)
		gotDigests := append([]string(nil), behaviorProof.CategoryPredicateDigests...)
		sort.Strings(gotDigests)
		if !reflect.DeepEqual(gotDigests, categoryDigests) {
			v.add("invalid-compiler-behavior-proof", label+" category predicates differ from the case assignment", &behaviorProof.Provenance)
		}
		claim := behaviorProof.RealizationProof.Claim
		if !foundScope || claim.Kind != semanticir.ClaimRealization || claim.Context != compilerProofContext(*evidence) || !reflect.DeepEqual(claim.Scope, scope) || !sameCompilerPredicates(claim.Memberships, memberships) || !sameCompilerOutcomes(claim.Outcomes, behaviorCase.OutcomeIDs) {
			v.add("invalid-compiler-behavior-proof", label+" typed realization claim differs from the exact case obligation", &behaviorProof.Provenance)
			continue
		}
		for _, outcome := range claim.Outcomes {
			v.validateCompilerPredicate(outcome.Predicate, evidence.Tool, evidence.EmittedIRDigest, fmt.Sprintf("%s outcome %q", label, outcome.OutcomeID), &behaviorProof.Provenance)
		}
		wantClaim := semanticir.NewProofClaim(semanticir.ClaimRealization, compilerProofContext(*evidence), scope, memberships, claim.Outcomes)
		v.validateCompilerReplay(behaviorProof.RealizationProof, semanticir.SolverUNSAT, wantClaim, *evidence, label, &behaviorProof.Provenance)
	}
	if len(seen) != len(cases) {
		v.add("missing-compiler-behavior-proof", fmt.Sprintf("compiler evidence %q proves %d behavior cases, want %d", evidence.ID, len(seen), len(cases)), &evidence.Provenance)
	}
}

func sameCompilerOutcomes(outcomes []semanticir.CompilerOutcomePredicate, outcomeIDs []string) bool {
	if len(outcomes) != len(outcomeIDs) {
		return false
	}
	wanted := make(map[string]bool, len(outcomeIDs))
	for _, outcomeID := range outcomeIDs {
		wanted[outcomeID] = true
	}
	for _, outcome := range outcomes {
		if !wanted[outcome.OutcomeID] {
			return false
		}
		delete(wanted, outcome.OutcomeID)
	}
	return len(wanted) == 0
}

func sameCompilerPredicates(left, right []semanticir.CompilerPredicate) bool {
	if len(left) != len(right) {
		return false
	}
	used := make([]bool, len(right))
	for _, wanted := range left {
		found := false
		for index, candidate := range right {
			if !used[index] && reflect.DeepEqual(wanted, candidate) {
				used[index] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (v *validator) environmentHasTool(wanted semanticir.ToolRef) bool {
	if v.task.Environment == nil {
		return false
	}
	for _, tool := range v.task.Environment.Tools {
		if tool == wanted {
			return true
		}
	}
	return false
}

func (v *validator) environmentHasSnapshot(treeDigest, environmentDigest string) bool {
	if v.task.Environment == nil {
		return false
	}
	for _, command := range v.task.Environment.Commands {
		if command.TreeDigest == treeDigest && command.EnvironmentDigest == environmentDigest {
			return true
		}
	}
	return false
}

func compilerPartitionKey(operationID, domainID string) string {
	return operationID + "/" + domainID
}

func (v *validator) compilerEvidenceTranscript() []semanticir.CompilerEvidence {
	var result []semanticir.CompilerEvidence
	for _, artifact := range v.task.Artifacts {
		for _, evidence := range artifact.CompilerEvidence {
			result = append(result, cloneCompilerEvidence(evidence))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Provenance.ArtifactID != result[j].Provenance.ArtifactID {
			return result[i].Provenance.ArtifactID < result[j].Provenance.ArtifactID
		}
		return result[i].ID < result[j].ID
	})
	for i := range result {
		sort.Slice(result[i].OperationScopes, func(a, b int) bool {
			return result[i].OperationScopes[a].OperationID < result[i].OperationScopes[b].OperationID
		})
		sort.Slice(result[i].Partitions, func(a, b int) bool {
			left, right := result[i].Partitions[a], result[i].Partitions[b]
			if left.OperationID != right.OperationID {
				return left.OperationID < right.OperationID
			}
			return left.DomainID < right.DomainID
		})
		for j := range result[i].Partitions {
			partition := &result[i].Partitions[j]
			sort.Slice(partition.Labels, func(a, b int) bool { return partition.Labels[a].ValueID < partition.Labels[b].ValueID })
			sort.Slice(partition.Exclusions, func(a, b int) bool {
				return partition.Exclusions[a].ConstraintID < partition.Exclusions[b].ConstraintID
			})
		}
		sort.Slice(result[i].BehaviorProofs, func(a, b int) bool {
			return result[i].BehaviorProofs[a].BehaviorCaseID < result[i].BehaviorProofs[b].BehaviorCaseID
		})
	}
	return result
}

func cloneCompilerEvidence(evidence semanticir.CompilerEvidence) semanticir.CompilerEvidence {
	encoded, err := semanticir.CanonicalJSON(evidence)
	if err != nil {
		return semanticir.CompilerEvidence{}
	}
	var result semanticir.CompilerEvidence
	if err := json.Unmarshal(encoded, &result); err != nil {
		return semanticir.CompilerEvidence{}
	}
	return result
}

func compilerEvidenceDigest(evidence []semanticir.CompilerEvidence) string {
	encoded, err := json.Marshal(evidence)
	if err != nil {
		// CompilerEvidence has no unsupported JSON values; retain a closed
		// sentinel if that contract ever changes so callers never see success
		// accompanied by an unbound transcript.
		return ""
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:])
}
