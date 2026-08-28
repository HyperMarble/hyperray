package semanticir

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// ValidateFrontendRequest checks the frozen bytes, translator, complete
// compilation context, and compiled-spec scope supplied to a frontend.
func ValidateFrontendRequest(request FrontendRequest) []Diagnostic {
	provenance := NewProvenance(request.Artifact, SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, TranslationTranslated)
	var diagnostics []Diagnostic
	if strings.TrimSpace(request.TaskID) == "" {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "frontend request task ID is empty", provenance))
	}
	if err := VerifyArtifact(request.Artifact, request.Source); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticStaleArtifact, err.Error(), provenance))
	}
	if request.Kind != ArtifactCode && request.Kind != ArtifactTests {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("frontend request kind %q is not code or tests", request.Kind), provenance))
	}
	if !artifactSupportsRole(request.Artifact.Kind, request.Kind) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "frontend request role is incompatible with frozen artifact kind", provenance))
	}
	diagnostics = append(diagnostics, validateSourceRoleRanges(request.Artifact, request.Source, request.SourceRanges, request.Artifact.Kind == ArtifactSource)...)
	switch request.Language {
	case LanguagePython, LanguageRust, LanguageCPP:
	default:
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("frontend request language %q is unsupported", request.Language), provenance))
	}
	if err := validateToolRef(request.Translator); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, err.Error(), provenance))
	}
	if err := validateToolRef(request.Prover); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "frontend prover: "+err.Error(), provenance))
	}
	if request.Kind == ArtifactTests {
		if validateToolRef(request.Runner) != nil || request.RunnerCommand == nil || request.Configuration == nil || request.Configuration.Kind != ArtifactConfiguration || validateArtifactRef(*request.Configuration) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test frontend request lacks frozen runner/command/configuration", provenance))
		} else if request.RunnerCommand != nil {
			found := false
			for _, tool := range request.RunnerCommand.Tools {
				found = found || tool == request.Runner
			}
			if !found {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test runner command omits frozen runner tool", provenance))
			}
		}
	}
	if len(request.Operations) == 0 || len(request.Outcomes) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "frontend request lacks compiled operations or outcomes", provenance))
	}
	domainIDs := map[string]Domain{}
	for _, domain := range request.FiniteDomains {
		if domain.ID == "" || !ValidValueType(domain.Type) || len(domain.Values) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticNonFinite, fmt.Sprintf("frontend domain %q is empty or untyped", domain.ID), domain.Provenance))
			continue
		}
		if _, exists := domainIDs[domain.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("frontend repeats domain %q", domain.ID), domain.Provenance))
		}
		domainIDs[domain.ID] = domain
		seen := map[string]struct{}{}
		for _, value := range domain.Values {
			if value.ID == "" {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticNonFinite, fmt.Sprintf("frontend domain %q has an empty semantic label", domain.ID), value.Provenance))
				continue
			}
			if _, exists := seen[value.ID]; exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("frontend domain %q repeats value %q", domain.ID, value.ID), value.Provenance))
			}
			seen[value.ID] = struct{}{}
			if value.Value != nil && value.Value.Type != domain.Type {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("frontend domain %q value %q has type %q, want %q", domain.ID, value.ID, value.Value.Type, domain.Type), value.Provenance))
			} else if value.Value != nil {
				if err := ValidateLiteral(*value.Value); err != nil {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("frontend domain %q value %q: %v", domain.ID, value.ID, err), value.Provenance))
				}
			}
		}
	}
	outcomeIDs := map[string]struct{}{}
	for _, outcome := range request.Outcomes {
		if outcome.ID == "" || outcome.ID != OutcomeID(outcome) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("frontend scope outcome %q is not canonically identified", outcome.ID), outcome.Provenance))
		}
		if _, exists := outcomeIDs[outcome.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("frontend repeats outcome %q", outcome.ID), outcome.Provenance))
		}
		outcomeIDs[outcome.ID] = struct{}{}
	}
	operationIDs := map[string]Operation{}
	for _, operation := range request.Operations {
		if operation.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "frontend scope operation ID is empty", operation.Provenance))
		}
		if _, exists := operationIDs[operation.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("frontend repeats operation %q", operation.ID), operation.Provenance))
		}
		operationIDs[operation.ID] = operation
		for _, domainID := range operation.DomainIDs {
			if _, exists := domainIDs[domainID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("frontend operation %q uses unknown domain %q", operation.ID, domainID), operation.Provenance))
			}
		}
		for _, outcomeID := range operation.OutcomeIDs {
			if _, exists := outcomeIDs[outcomeID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("frontend operation %q uses unknown outcome %q", operation.ID, outcomeID), operation.Provenance))
			}
		}
	}
	for _, constraint := range request.Constraints {
		operation, exists := operationIDs[constraint.OperationID]
		if !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("frontend constraint %q uses unknown operation %q", constraint.ID, constraint.OperationID), constraint.Provenance))
			continue
		}
		values := map[string]map[string]struct{}{}
		for _, domainID := range operation.DomainIDs {
			members := map[string]struct{}{}
			for _, value := range domainIDs[domainID].Values {
				members[value.ID] = struct{}{}
			}
			values[domainID] = members
		}
		if constraint.ID == "" || strings.TrimSpace(constraint.Reason) == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "frontend constraint lacks ID/reason", constraint.Provenance))
		}
		if err := validateAssignment(constraint.Conditions, values); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("frontend constraint %q: %v", constraint.ID, err), constraint.Provenance))
		}
	}
	diagnostics = append(diagnostics, validateFrontendGroundings(request, operationIDs, domainIDs, provenance)...)
	diagnostics = append(diagnostics, validateWorkspace(request.Workspace, request.FocusArtifacts, provenance)...)
	diagnostics = append(diagnostics, validateChangedRanges(request.ChangedRanges, request.FocusArtifacts, provenance)...)
	return diagnostics
}

func validateFrontendGroundings(request FrontendRequest, operations map[string]Operation, domains map[string]Domain, provenance Provenance) []Diagnostic {
	var diagnostics []Diagnostic
	byBehavior := map[string]AssignmentGrounding{}
	for _, grounding := range request.Groundings {
		operation, exists := operations[grounding.OperationID]
		if !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("frontend grounding %q refers to unknown operation %q", grounding.ID, grounding.OperationID), grounding.Provenance))
			continue
		}
		values := map[string]map[string]struct{}{}
		for _, domainID := range operation.DomainIDs {
			members := map[string]struct{}{}
			for _, value := range domains[domainID].Values {
				members[value.ID] = struct{}{}
			}
			values[domainID] = members
		}
		if err := validateAssignment(grounding.Conditions, values); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("frontend grounding %q: %v", grounding.ID, err), grounding.Provenance))
			continue
		}
		if grounding.ID != AssignmentGroundingID(grounding.OperationID, grounding.Conditions) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("frontend grounding %q is not canonically identified", grounding.ID), grounding.Provenance))
		}
		inputs := inputsByName(operation.Inputs)
		if len(grounding.Inputs) != len(inputs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("frontend grounding %q does not assign every operation input", grounding.ID), grounding.Provenance))
		}
		for name, literal := range grounding.Inputs {
			input, exists := inputs[name]
			if !exists || literal.Type != input.Type || ValidateLiteral(literal) != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("frontend grounding %q has invalid input %q", grounding.ID, name), grounding.Provenance))
			}
		}
		conjunction, err := GroundingConjunction(operation, request.FiniteDomains, grounding.Conditions, grounding.Provenance)
		if err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("frontend grounding %q: %v", grounding.ID, err), grounding.Provenance))
		} else if satisfied, evaluationErr := EvaluateGroundingMembership(conjunction, grounding.Inputs); evaluationErr != nil || !satisfied {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnreachable, fmt.Sprintf("frontend grounding %q does not satisfy selected labels", grounding.ID), grounding.Provenance))
		}
		key := behaviorKey(grounding.OperationID, grounding.Conditions)
		if _, duplicate := byBehavior[key]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("frontend scope repeats grounding for behavior %s", key), grounding.Provenance))
		}
		byBehavior[key] = grounding
	}
	excluded := map[string]struct{}{}
	for _, constraint := range request.Constraints {
		excluded[behaviorKey(constraint.OperationID, constraint.Conditions)] = struct{}{}
	}
	for _, operation := range operations {
		for _, assignment := range EnumerateAssignments(selectDomains(request.FiniteDomains, operation.DomainIDs)) {
			key := behaviorKey(operation.ID, assignment)
			if _, isExcluded := excluded[key]; isExcluded {
				if _, exists := byBehavior[key]; exists {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("frontend scope grounds excluded behavior %s", key), provenance))
				}
				continue
			}
			if _, exists := byBehavior[key]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("frontend scope omits outcome-free grounding for reachable behavior %s", key), provenance))
			}
		}
	}
	return diagnostics
}

func validateWorkspace(workspace WorkspaceRef, focus []ArtifactRef, provenance Provenance) []Diagnostic {
	var diagnostics []Diagnostic
	if workspace.ID == "" || workspace.Root == "" || workspace.WorkingDirectory == "" || !ValidDigest(workspace.TreeDigest) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "frontend workspace lacks ID/root/workdir/tree digest", provenance))
	}
	if !workspace.ClearEnvironment || !workspace.KillProcessGroup {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "frontend workspace lacks clear-environment/process-group policy", workspace.Provenance))
	}
	if err := validateExactEnvironment(workspace.Environment, workspace.EnvironmentDigest); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "frontend workspace environment: "+err.Error(), workspace.Provenance))
	}
	switch workspace.State {
	case WorkspaceBaseOldTests, WorkspaceBaseNewTests, WorkspaceSolutionNewTests:
	default:
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("frontend workspace has invalid state %q", workspace.State), workspace.Provenance))
	}
	if workspace.BuildCommand == "" && workspace.CompilationDatabase == nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "frontend workspace has neither build command nor compilation database", workspace.Provenance))
	}
	if err := validateProvenance(workspace.Provenance); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "frontend workspace: "+err.Error(), workspace.Provenance))
	}
	entries := map[string]ArtifactRef{}
	for _, entry := range workspace.Entries {
		if entry.Path == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "frontend workspace has an empty entry path", entry.Provenance))
		}
		if _, exists := entries[entry.Path]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("frontend workspace repeats entry path %q", entry.Path), entry.Provenance))
		}
		entries[entry.Path] = entry.Artifact
		if err := validateArtifactRef(entry.Artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "frontend workspace entry: "+err.Error(), entry.Provenance))
		}
		if err := validateFactSource(entry.Provenance, entry.Artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "frontend workspace entry: "+err.Error(), entry.Provenance))
		}
	}
	if len(workspace.Entries) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "frontend workspace has no frozen entries", workspace.Provenance))
	}
	for _, artifact := range focus {
		found := false
		for _, entry := range workspace.Entries {
			if entry.Artifact == artifact {
				found = true
				break
			}
		}
		if !found {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("focus artifact %q is absent from the frozen workspace", artifact.ID), provenance))
		}
	}
	if len(focus) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "frontend request has no focused artifacts", provenance))
	}
	if workspace.CompilationDatabase != nil {
		found := false
		for _, entry := range workspace.Entries {
			if entry.Artifact == *workspace.CompilationDatabase {
				found = true
			}
		}
		if !found {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "compilation database is absent from frozen workspace entries", workspace.Provenance))
		}
	}
	return diagnostics
}

// ValidateArtifactScope rejects a frontend result that changes the compiled
// spec vocabulary supplied in its request. Code models must cover every
// requested operation; test models may add OperationTest nodes but may not add
// non-test operations.
func ValidateArtifactScope(request FrontendRequest, model ArtifactModel) []Diagnostic {
	var diagnostics []Diagnostic
	provenance := model.Coverage.Provenance
	if model.Artifact != request.Artifact || model.Language != request.Language || model.Kind != request.Kind || model.Translator != request.Translator {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "frontend result identity differs from its frozen request", provenance))
	}
	if !reflect.DeepEqual(model.SourceRanges, request.SourceRanges) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "frontend result source-role ranges differ from its frozen request", provenance))
	}
	if !sameDomainVocabulary(request.FiniteDomains, model.Domains) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "frontend result domain vocabulary differs from compiled spec scope", provenance))
	}
	if !sameAssignmentGroundings(request.Groundings, model.Groundings) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "frontend result assignment groundings differ from compiled outcome-free scope", provenance))
	}
	if !sameConstraints(request.Constraints, model.Constraints) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "frontend result constraints differ from compiled spec scope", provenance))
	}
	if !sameOutcomeVocabulary(request.Outcomes, model.Outcomes) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "frontend result outcome vocabulary differs from compiled spec scope", provenance))
	}
	for _, evidence := range model.CompilerEvidence {
		if evidence.SourceDigest != request.Artifact.Digest || evidence.WorkspaceTreeDigest != request.Workspace.TreeDigest {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticStaleArtifact, fmt.Sprintf("compiler evidence %q differs from request source/workspace", evidence.ID), evidence.Provenance))
		}
		if evidence.Tool != request.Translator {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("compiler evidence %q tool differs from request translator", evidence.ID), evidence.Provenance))
		}
		if evidence.Prover != request.Prover {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("compiler evidence %q prover differs from request prover", evidence.ID), evidence.Provenance))
		}
	}
	requested := map[string]Operation{}
	for _, operation := range request.Operations {
		requested[operation.ID] = operation
	}
	seen := map[string]struct{}{}
	for _, operation := range model.Operations {
		if operation.Kind == OperationTest {
			continue
		}
		declared, exists := requested[operation.ID]
		if !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("frontend result adds operation %q outside compiled spec scope", operation.ID), operation.Provenance))
			continue
		}
		seen[operation.ID] = struct{}{}
		if !sameOrderedStrings(declared.DomainIDs, operation.DomainIDs) || !sameStringSet(declared.OutcomeIDs, operation.OutcomeIDs) || !sameOperationInputs(declared.Inputs, operation.Inputs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("frontend operation %q domain/outcome scope differs from compiled spec", operation.ID), operation.Provenance))
		}
	}
	if request.Kind == ArtifactCode {
		normalizedCases, normalizationDiagnostics := NormalizeReferenceCases(request, model.RawReferenceCases)
		diagnostics = append(diagnostics, normalizationDiagnostics...)
		if len(model.RawReferenceCases) == 0 || !reflect.DeepEqual(normalizedCases, model.Cases) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "frontend reference cases are not the central normalization of independently extracted raw outcomes", provenance))
		}
		for operationID := range requested {
			if _, exists := seen[operationID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("frontend result omits requested operation %q", operationID), provenance))
			}
		}
		if len(model.CompilerEvidence) > 0 {
			diagnostics = append(diagnostics, validateCompilerScope(request.FiniteDomains, request.Constraints, request.Operations, model.CompilerEvidence, provenance)...)
		}
		if model.ScopeClosure == nil || !reflect.DeepEqual(request.ChangedRanges, model.ScopeClosure.ChangedRanges) || model.ScopeClosure.WorkspaceTreeDigest != request.Workspace.TreeDigest || model.ScopeClosure.Prover != request.Prover {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "frontend patch-scope closure differs from exact request ranges/workspace", provenance))
		}
	}
	if request.Kind == ArtifactTests && model.RunnerSelection != nil && (request.RunnerCommand == nil || request.Configuration == nil || model.RunnerSelection.Verifier != request.Runner || (request.RunnerCommand != nil && !reflect.DeepEqual(model.RunnerSelection.Command, *request.RunnerCommand)) || (request.Configuration != nil && model.RunnerSelection.Configuration != *request.Configuration)) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "frontend test runner selection differs from request", provenance))
	}
	return diagnostics
}

func artifactSupportsRole(artifactKind, role ArtifactKind) bool {
	return artifactKind == role || artifactKind == ArtifactSource && (role == ArtifactCode || role == ArtifactTests)
}

func sameOperationInputs(left, right []Variable) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Name != right[index].Name || left[index].Type != right[index].Type || left[index].DomainID != right[index].DomainID {
			return false
		}
	}
	return true
}

func sameOrderedStrings(left, right []string) bool {
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

func validateSourceRoleRanges(artifact ArtifactRef, source []byte, ranges []SourceRoleRange, required bool) []Diagnostic {
	provenance := NewProvenance(artifact, SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1}, TranslationTranslated)
	if required && len(ranges) == 0 {
		return []Diagnostic{errorDiagnostic(DiagnosticIncomplete, "role-neutral source artifact has no exact code/test byte ranges", provenance)}
	}
	values := append([]SourceRoleRange(nil), ranges...)
	sort.Slice(values, func(i, j int) bool {
		if values[i].StartByte == values[j].StartByte {
			return values[i].EndByte < values[j].EndByte
		}
		return values[i].StartByte < values[j].StartByte
	})
	var diagnostics []Diagnostic
	previousEnd := -1
	for _, value := range values {
		if value.ArtifactID != artifact.ID || value.Path != artifact.Path || value.StartByte < 0 || value.EndByte <= value.StartByte || value.EndByte > len(source) || value.StartByte < previousEnd || !ValidDigest(value.SliceDigest) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "source-role byte range is invalid, overlapping, or outside frozen bytes", value.Provenance))
			continue
		}
		if value.SliceDigest != DigestBytes(source[value.StartByte:value.EndByte]) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticStaleArtifact, "source-role byte range digest differs from frozen bytes", value.Provenance))
		}
		if err := validateFactSource(value.Provenance, artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "source-role byte range: "+err.Error(), value.Provenance))
		}
		previousEnd = value.EndByte
	}
	return diagnostics
}

func validateModelSourceRoleRanges(model ArtifactModel) []Diagnostic {
	if model.Artifact.Kind == ArtifactSource && len(model.SourceRanges) == 0 {
		return []Diagnostic{errorDiagnostic(DiagnosticIncomplete, "role-neutral source model has no exact code/test byte ranges", model.Coverage.Provenance)}
	}
	var diagnostics []Diagnostic
	values := append([]SourceRoleRange(nil), model.SourceRanges...)
	sort.Slice(values, func(i, j int) bool { return values[i].StartByte < values[j].StartByte })
	previousEnd := -1
	for _, value := range values {
		if value.ArtifactID != model.Artifact.ID || value.Path != model.Artifact.Path || value.StartByte < 0 || value.EndByte <= value.StartByte || value.StartByte < previousEnd || !ValidDigest(value.SliceDigest) || validateFactSource(value.Provenance, model.Artifact) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "artifact model source-role range is malformed or overlapping", value.Provenance))
		}
		previousEnd = value.EndByte
	}
	return diagnostics
}

func sameOutcomeVocabulary(left, right []ObservableOutcome) bool {
	if len(left) != len(right) {
		return false
	}
	byID := make(map[string]ObservableOutcome, len(left))
	for _, outcome := range left {
		if _, exists := byID[outcome.ID]; exists {
			return false
		}
		byID[outcome.ID] = outcome
	}
	seen := map[string]struct{}{}
	for _, outcome := range right {
		declared, exists := byID[outcome.ID]
		if !exists || !sameOutcomeSemantics(declared, outcome) {
			return false
		}
		if _, exists := seen[outcome.ID]; exists {
			return false
		}
		seen[outcome.ID] = struct{}{}
	}
	return true
}
