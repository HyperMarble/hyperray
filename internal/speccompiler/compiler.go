// Package speccompiler strictly compiles frozen Markdown behavior tables into
// semanticir. It rejects any ambiguity instead of lowering prose as a proof
// fact.
package speccompiler

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/specparser"
)

// Request binds the spec source to a frozen artifact.
//
// Instruction and Reference are both provenance artifacts because a benchmark
// instruction is a problem statement, not a specification: Harbor's task
// authoring rule is "don't leak the tests -- describe what done looks like,
// not how you'll check it", so the prompt is deliberately incomplete. Anchoring
// every requirement into it forces one of two defects -- omitting real behavior
// the prompt never stated, or citing a line that does not entail the row. The
// reference solution carries the mechanism, so a row may anchor there instead.
type Request struct {
	TaskID            string                 `json:"task_id"`
	Artifact          semanticir.ArtifactRef `json:"artifact"`
	Source            []byte                 `json:"source"`
	Instruction       semanticir.ArtifactRef `json:"instruction"`
	InstructionSource []byte                 `json:"instruction_source"`
	Reference         semanticir.ArtifactRef `json:"reference"`
	ReferenceSource   []byte                 `json:"reference_source"`
}

// anchorArtifacts is the set an `Evidence` cell may name. A bare span
// means the instruction, so specs written before reference anchors existed
// keep compiling unchanged.
type anchorArtifacts struct {
	instruction       semanticir.ArtifactRef
	instructionSource []byte
	reference         semanticir.ArtifactRef
	referenceSource   []byte
}

func (a anchorArtifacts) resolve(name string) (semanticir.ArtifactRef, []byte, error) {
	switch name {
	case "", "instruction":
		return a.instruction, a.instructionSource, nil
	case "reference":
		if len(a.referenceSource) == 0 {
			return semanticir.ArtifactRef{}, nil, fmt.Errorf("reference anchor used but no reference artifact was frozen")
		}
		return a.reference, a.referenceSource, nil
	default:
		return semanticir.ArtifactRef{}, nil, fmt.Errorf("unknown anchor artifact %q, want instruction or reference", name)
	}
}

const (
	HeaderID                = "ID"
	HeaderOperation         = "Operation"
	HeaderReachability      = "Reachability"
	HeaderRequiredOutcomes  = "Required outcomes"
	HeaderForbiddenOutcomes = "Forbidden outcomes"
	HeaderEffects           = "Effects"
	HeaderInvariants        = "Invariants"
	HeaderInputWitnesses    = "Input witnesses"
	HeaderEnforcedBy        = "Enforced by"
	HeaderEvidence          = "Evidence"
	HeaderConstraintReason  = "Constraint reason"
)

var (
	identifierPattern     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.:#-]*$`)
	normativePattern      = regexp.MustCompile(`(?i)\b(must|shall|required|forbidden|never|always)\b`)
	raisePattern          = regexp.MustCompile(`^raise\s+([A-Za-z_][A-Za-z0-9_.:]*)\s+containing\s+"([^"]*)"$`)
	sourceSpanPattern     = regexp.MustCompile(`^(\d+)(?::(\d+))?(?:-(\d+)(?::(\d+))?)?$`)
	anchorArtifactPattern = regexp.MustCompile(`^([A-Za-z][A-Za-z-]*):(.+)$`)
)

type rowDefinition struct {
	id                 string
	operation          string
	reachable          bool
	domains            []semanticir.Domain
	sets               [][]string
	required           []semanticir.ObservableOutcome
	forbidden          []semanticir.ObservableOutcome
	effects            []semanticir.Effect
	invariantIDs       []string
	inputWitnessSource string
	inputWitnesses     []map[string]semanticir.Literal
	testIDs            []string
	instructionSources []semanticir.Provenance
	constraintReason   string
	provenance         semanticir.Provenance
}

// resolveOutcomeSemantics makes effects part of required outcome identity,
// then resolves a bare forbidden terminal against the operation's complete
// candidate vocabulary. An effect-qualified forbidden outcome remains exact.
// The explicit "other outcome" form is the operation-scoped complement of
// every named exact terminal/effect trace and therefore never takes effects.
func resolveOutcomeSemantics(rows []rowDefinition, diagnostics []semanticir.Diagnostic) ([]rowDefinition, []semanticir.Diagnostic) {
	candidates := map[string]map[string]semanticir.ObservableOutcome{}
	for index := range rows {
		if !rows[index].reachable {
			continue
		}
		for outcomeIndex := range rows[index].required {
			outcome := rows[index].required[outcomeIndex]
			if outcome.Kind == semanticir.OutcomeOther || outcome.Kind == semanticir.OutcomeTimeout {
				if len(rows[index].effects) != 0 {
					diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticOverlapping, fmt.Sprintf("row %q cannot attach its Effects column to %s", rows[index].id, outcome.Kind), rows[index].provenance))
				}
				addCandidate(candidates, rows[index].operation, outcome)
				continue
			}
			if len(outcome.Effects) == 0 {
				outcome.Effects = append([]semanticir.Effect(nil), rows[index].effects...)
			} else if !effectsSubset(rows[index].effects, outcome.Effects) {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticOverlapping, fmt.Sprintf("row %q required outcome omits an effect required by its Effects column", rows[index].id), rows[index].provenance))
			}
			outcome.ID = semanticir.OutcomeID(outcome)
			rows[index].required[outcomeIndex] = outcome
			addCandidate(candidates, rows[index].operation, outcome)
		}
		for outcomeIndex := range rows[index].forbidden {
			outcome := rows[index].forbidden[outcomeIndex]
			if len(outcome.Effects) > 0 {
				outcome.ID = semanticir.OutcomeID(outcome)
				rows[index].forbidden[outcomeIndex] = outcome
				addCandidate(candidates, rows[index].operation, outcome)
			}
		}
	}
	for index := range rows {
		if !rows[index].reachable {
			continue
		}
		var resolved []semanticir.ObservableOutcome
		for _, pattern := range rows[index].forbidden {
			if len(pattern.Effects) > 0 {
				resolved = append(resolved, pattern)
				continue
			}
			var matches []semanticir.ObservableOutcome
			for _, candidate := range candidates[rows[index].operation] {
				if sameTerminal(candidate, pattern) {
					matches = append(matches, candidate)
				}
			}
			if len(matches) == 0 {
				pattern.ID = semanticir.OutcomeID(pattern)
				addCandidate(candidates, rows[index].operation, pattern)
				matches = append(matches, pattern)
			}
			sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
			resolved = append(resolved, matches...)
		}
		rows[index].forbidden = uniqueOutcomes(resolved)
	}
	return rows, diagnostics
}

func addCandidate(candidates map[string]map[string]semanticir.ObservableOutcome, operation string, outcome semanticir.ObservableOutcome) {
	if candidates[operation] == nil {
		candidates[operation] = map[string]semanticir.ObservableOutcome{}
	}
	candidates[operation][outcome.ID] = outcome
}

func uniqueOutcomes(outcomes []semanticir.ObservableOutcome) []semanticir.ObservableOutcome {
	seen := map[string]struct{}{}
	result := make([]semanticir.ObservableOutcome, 0, len(outcomes))
	for _, outcome := range outcomes {
		if _, exists := seen[outcome.ID]; exists {
			continue
		}
		seen[outcome.ID] = struct{}{}
		result = append(result, outcome)
	}
	return result
}

func sameTerminal(left, right semanticir.ObservableOutcome) bool {
	return left.Kind == right.Kind && reflect.DeepEqual(left.Value, right.Value) && left.ExceptionType == right.ExceptionType && left.Message == right.Message
}

func sameEffects(left, right []semanticir.Effect) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Kind != right[index].Kind || left[index].Target != right[index].Target || !reflect.DeepEqual(left[index].Value, right[index].Value) {
			return false
		}
	}
	return true
}

func effectsSubset(required, actual []semanticir.Effect) bool {
	used := make([]bool, len(actual))
	for _, want := range required {
		found := false
		for index, got := range actual {
			if !used[index] && sameEffects([]semanticir.Effect{want}, []semanticir.Effect{got}) {
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

// Compile translates a frozen strict spec. A nil task plus one or more error
// diagnostics is the only rejection form; callers must never use a partial IR.
func Compile(ctx context.Context, request Request) (*semanticir.Task, []semanticir.Diagnostic) {
	baseProvenance := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{
		Path:        request.Artifact.Path,
		StartLine:   1,
		StartColumn: 1,
	}, semanticir.TranslationTranslated)
	if err := ctx.Err(); err != nil {
		return nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticInvalidInput, err.Error(), baseProvenance)}
	}
	if strings.TrimSpace(request.TaskID) == "" {
		return nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticInvalidInput, "task ID is empty", baseProvenance)}
	}
	if request.Artifact.Kind != semanticir.ArtifactSpec {
		return nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("artifact %q has kind %q, want spec", request.Artifact.ID, request.Artifact.Kind), baseProvenance)}
	}
	if len(strings.TrimSpace(string(request.Source))) == 0 {
		return nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticInvalidInput, "spec is empty", baseProvenance)}
	}
	if err := semanticir.VerifyArtifact(request.Artifact, request.Source); err != nil {
		return nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticStaleArtifact, err.Error(), baseProvenance)}
	}
	if request.Instruction.Kind != semanticir.ArtifactInstruction {
		return nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("artifact %q has kind %q, want instruction", request.Instruction.ID, request.Instruction.Kind), baseProvenance)}
	}
	if err := semanticir.VerifyArtifact(request.Instruction, request.InstructionSource); err != nil {
		return nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticStaleArtifact, "instruction: "+err.Error(), baseProvenance)}
	}
	if len(request.ReferenceSource) != 0 {
		if err := semanticir.VerifyArtifact(request.Reference, request.ReferenceSource); err != nil {
			return nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticStaleArtifact, "reference: "+err.Error(), baseProvenance)}
		}
	}
	if line, text := proseOnlyRequirement(string(request.Source)); line > 0 {
		provenance := rowProvenance(request.Artifact, line)
		return nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticProseRequirement, fmt.Sprintf("line %d contains a graded requirement outside a strict table: %q", line, text), provenance)}
	}
	inputDeclarations, universeDeclarations, groundingDeclarations, bridgeDeclarations, declarationDiagnostics := parseSemanticDeclarations(string(request.Source), request.Artifact)

	tables, err := specparser.Parse(string(request.Source))
	if err != nil {
		return nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticInvalidInput, err.Error(), baseProvenance)}
	}
	if len(tables) == 0 {
		return nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticInvalidInput, "spec contains no behavior table", baseProvenance)}
	}

	diagnostics := append([]semanticir.Diagnostic(nil), declarationDiagnostics...)
	var domains []semanticir.Domain
	domainRegistry := map[string]semanticir.Domain{}
	var rows []rowDefinition
	anchors := anchorArtifacts{
		instruction:       request.Instruction,
		instructionSource: request.InstructionSource,
		reference:         request.Reference,
		referenceSource:   request.ReferenceSource,
	}
	rowIDs := map[string]struct{}{}
	for _, table := range tables {
		if err := ctx.Err(); err != nil {
			return nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticInvalidInput, err.Error(), baseProvenance)}
		}
		tableDomains, tableRows, tableDiagnostics := compileTable(request.Artifact, anchors, table, rowIDs)
		diagnostics = append(diagnostics, tableDiagnostics...)
		if semanticir.HasErrors(tableDiagnostics) {
			continue
		}
		for _, domain := range tableDomains {
			if existing, exists := domainRegistry[domain.ID]; exists {
				if !sameDomains([]semanticir.Domain{existing}, []semanticir.Domain{domain}) {
					diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticDuplicateID, fmt.Sprintf("domain %q is redeclared with different values", domain.ID), rowProvenance(request.Artifact, table.Line)))
				}
			} else {
				domainRegistry[domain.ID] = domain
				domains = append(domains, domain)
			}
		}
		rows = append(rows, tableRows...)
	}
	if semanticir.HasErrors(diagnostics) {
		return nil, diagnostics
	}
	rows, diagnostics = resolveOutcomeSemantics(rows, diagnostics)
	diagnostics = append(diagnostics, validateBridges(bridgeDeclarations, rows)...)
	diagnostics = append(diagnostics, warnQuantifierLabels(rows, request.InstructionSource)...)
	if semanticir.HasErrors(diagnostics) {
		return nil, diagnostics
	}
	operationsWithComplement := map[string]bool{}
	operationProvenance := map[string]semanticir.Provenance{}
	for _, row := range rows {
		if !row.reachable {
			continue
		}
		operationProvenance[row.operation] = row.provenance
		for _, outcome := range append(append([]semanticir.ObservableOutcome{}, row.required...), row.forbidden...) {
			if outcome.Kind == semanticir.OutcomeOther {
				operationsWithComplement[row.operation] = true
			}
		}
	}
	for operationID, provenance := range operationProvenance {
		if !operationsWithComplement[operationID] {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticIncomplete, fmt.Sprintf("operation %q does not explicitly declare other outcome in its closed outcome alphabet", operationID), provenance))
		}
	}
	if semanticir.HasErrors(diagnostics) {
		return nil, diagnostics
	}

	task := &semanticir.Task{
		ID:          request.TaskID,
		Instruction: request.Instruction,
		Reference:   request.Reference,
		Scopes:      bridgeDeclarations.scopes,
		Classifiers: bridgeDeclarations.classifiers,
		Observers:   bridgeDeclarations.observers,
		Spec:        request.Artifact,
		Domains:     domains,
		Provenance:  baseProvenance,
	}
	outcomes := map[string]semanticir.ObservableOutcome{}
	operations := map[string]semanticir.Operation{}
	invariants := map[string]semanticir.Invariant{}
	for _, row := range rows {
		for _, outcome := range append(append([]semanticir.ObservableOutcome{}, row.required...), row.forbidden...) {
			if previous, exists := outcomes[outcome.ID]; exists && !sameOutcome(previous, outcome) {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticDuplicateID, fmt.Sprintf("outcome ID collision %q", outcome.ID), row.provenance))
			} else if !exists {
				outcomes[outcome.ID] = outcome
			}
		}
		operation, exists := operations[row.operation]
		rowDomainIDs := domainIDs(row.domains)
		if !exists {
			operation = semanticir.Operation{ID: row.operation, Kind: semanticir.OperationCallable, DomainIDs: rowDomainIDs, Provenance: row.provenance}
		} else {
			operation.DomainIDs = unionStrings(operation.DomainIDs, rowDomainIDs)
		}
		if row.reachable {
			operation.OutcomeIDs = unionStrings(operation.OutcomeIDs, append(outcomeIDs(row.required), outcomeIDs(row.forbidden)...))
			for _, invariantID := range row.invariantIDs {
				if _, exists := invariants[invariantID]; !exists {
					invariants[invariantID] = semanticir.Invariant{
						ID: invariantID,
						Predicate: semanticir.Expression{
							Kind:       semanticir.ExprVariable,
							Type:       semanticir.TypeBool,
							Name:       invariantID,
							Provenance: row.provenance,
						},
						Provenance: row.provenance,
					}
				}
			}
		}
		operations[row.operation] = operation
	}
	domains, operations, declarationDiagnostics = applySemanticDeclarations(inputDeclarations, universeDeclarations, groundingDeclarations, operations, domains)
	diagnostics = append(diagnostics, declarationDiagnostics...)
	rows, declarationDiagnostics = applyRowInputWitnesses(rows, operations, domains)
	diagnostics = append(diagnostics, declarationDiagnostics...)
	if semanticir.HasErrors(diagnostics) {
		return nil, diagnostics
	}
	task.Domains = domains
	for _, domain := range domains {
		domainRegistry[domain.ID] = domain
	}
	task.Outcomes = sortedOutcomes(outcomes)
	task.Operations = sortedOperations(operations)
	task.Invariants = sortedInvariants(invariants)
	instructionModel, instructionErr := buildInstructionModel(request.Instruction, request.InstructionSource, rows)
	if instructionErr != nil {
		return nil, append(diagnostics, diagnostic(semanticir.DiagnosticInvalidProvenance, instructionErr.Error(), baseProvenance))
	}
	task.InstructionModel = instructionModel

	assignmentGroundings := map[string]semanticir.AssignmentGrounding{}
	for _, row := range rows {
		operation := operations[row.operation]
		assignments := expandScopedAssignments(selectCompilerDomains(domains, operation.DomainIDs), row.domains, row.sets)
		if len(assignments) == 0 {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticUnreachable, fmt.Sprintf("row %q matches no finite assignment", row.id), row.provenance))
			continue
		}
		for index, assignment := range assignments {
			id := expandedID(row.id, index, len(assignments))
			if row.reachable {
				groundingID := semanticir.AssignmentGroundingID(row.operation, assignment)
				grounding := semanticir.AssignmentGrounding{ID: groundingID, OperationID: row.operation, Conditions: assignment, Inputs: row.inputWitnesses[index], Provenance: row.provenance}
				if previous, exists := assignmentGroundings[groundingID]; exists && (!reflect.DeepEqual(previous.Conditions, grounding.Conditions) || !reflect.DeepEqual(previous.Inputs, grounding.Inputs)) {
					diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticOverlapping, fmt.Sprintf("semantic assignment %q has conflicting Input witnesses", groundingID), row.provenance))
					continue
				} else if !exists {
					assignmentGroundings[groundingID] = grounding
					task.Groundings = append(task.Groundings, grounding)
				}
				task.Requirements = append(task.Requirements, semanticir.RequirementCase{
					ID:                   id,
					Conditions:           assignment,
					OperationID:          row.operation,
					RequiredOutcomes:     outcomeIDs(row.required),
					ForbiddenOutcomes:    outcomeIDs(row.forbidden),
					Effects:              row.effects,
					InvariantIDs:         append([]string(nil), row.invariantIDs...),
					TestIDs:              append([]string(nil), row.testIDs...),
					GroundingID:          groundingID,
					InstructionClauseIDs: clauseIDsForSources(task.InstructionModel, row.instructionSources),
					InstructionSources:   append([]semanticir.Provenance(nil), row.instructionSources...),
					Evidence:             append([]semanticir.Provenance{row.provenance}, row.instructionSources...),
					Provenance:           row.provenance,
				})
			} else {
				task.Constraints = append(task.Constraints, semanticir.Constraint{
					ID:          id,
					Conditions:  assignment,
					OperationID: row.operation,
					Reason:      row.constraintReason,
					Provenance:  row.provenance,
				})
			}
		}
	}
	if semanticir.HasErrors(diagnostics) {
		return nil, diagnostics
	}
	coverage := semanticir.TranslationCoverage{
		Status:               semanticir.TranslationComplete,
		TotalConstructs:      len(domains) + len(rows),
		TranslatedConstructs: len(domains) + len(rows),
		Provenance:           baseProvenance,
	}
	task.Coverage = []semanticir.TranslationCoverage{coverage, task.InstructionModel.Coverage}
	var specDigestErr error
	task.SpecIRDigest, specDigestErr = semanticir.CanonicalSpecIRDigest(task)
	if specDigestErr != nil {
		return nil, append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, "canonical Spec IR digest: "+specDigestErr.Error(), baseProvenance))
	}
	diagnostics = append(diagnostics, task.ValidateSpec()...)
	if semanticir.HasErrors(diagnostics) {
		return nil, diagnostics
	}
	return task, diagnostics
}

func compileTable(artifact semanticir.ArtifactRef, anchors anchorArtifacts, table specparser.Table, rowIDs map[string]struct{}) ([]semanticir.Domain, []rowDefinition, []semanticir.Diagnostic) {
	provenance := rowProvenance(artifact, table.Line)
	if strings.TrimSpace(table.Params) == "" {
		return nil, nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticMissingDomain, fmt.Sprintf("table %q has no Parameters declaration", table.Section), provenance)}
	}
	var parsedDomains []specparser.Domain
	if !isZeroDomainDeclaration(table.Params) {
		var unsupported string
		var err error
		parsedDomains, unsupported, err = specparser.ParseParams(table.Params)
		if err != nil {
			return nil, nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticNonFinite, err.Error(), provenance)}
		}
		if unsupported != "" {
			return nil, nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticNonFinite, fmt.Sprintf("domain %q is not an explicit finite value list", unsupported), provenance)}
		}
		if len(parsedDomains) == 0 {
			return nil, nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticMissingDomain, fmt.Sprintf("table %q declares no finite domains; write Parameters: none for a zero-argument operation", table.Section), provenance)}
		}
	}

	indices := map[string]int{}
	for index, header := range table.Columns {
		normalized := normalizeHeader(header)
		if _, exists := indices[normalized]; exists {
			return nil, nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticDuplicateID, fmt.Sprintf("table %q has duplicate column %q", table.Section, header), provenance)}
		}
		indices[normalized] = index
	}
	requiredHeaders := []string{HeaderID, HeaderOperation, HeaderReachability, HeaderRequiredOutcomes, HeaderForbiddenOutcomes, HeaderEffects, HeaderInvariants, HeaderInputWitnesses, HeaderEnforcedBy, HeaderEvidence, HeaderConstraintReason}
	for _, header := range requiredHeaders {
		if _, exists := indices[normalizeHeader(header)]; !exists {
			return nil, nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("table %q is missing required column %q", table.Section, header), provenance)}
		}
	}
	allowed := map[string]struct{}{}
	for _, header := range requiredHeaders {
		allowed[normalizeHeader(header)] = struct{}{}
	}
	for _, domain := range parsedDomains {
		allowed[normalizeHeader(domain.Name)] = struct{}{}
		if _, exists := indices[normalizeHeader(domain.Name)]; !exists {
			return nil, nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticMissingDomain, fmt.Sprintf("domain %q has no condition column", domain.Name), provenance)}
		}
	}
	for _, header := range table.Columns {
		if _, exists := allowed[normalizeHeader(header)]; !exists {
			return nil, nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("table %q has undeclared column %q", table.Section, header), provenance)}
		}
	}
	if len(table.Rows) == 0 {
		return nil, nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("table %q has no rows", table.Section), provenance)}
	}

	domains := make([]semanticir.Domain, 0, len(parsedDomains))
	for _, parsed := range parsedDomains {
		domain := semanticir.Domain{ID: parsed.Name, Type: semanticir.TypeString, Provenance: provenance}
		seen := map[string]struct{}{}
		for _, rawValue := range parsed.Values {
			value := strings.TrimSpace(rawValue)
			if value == "" {
				return nil, nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticNonFinite, fmt.Sprintf("domain %q contains an empty value", parsed.Name), provenance)}
			}
			if isWildcard(value) && !parsed.JSONQuoted[value] {
				return nil, nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticNonFinite, fmt.Sprintf("domain %q uses reserved wildcard value %q", parsed.Name, value), provenance)}
			}
			if _, exists := seen[value]; exists {
				return nil, nil, []semanticir.Diagnostic{diagnostic(semanticir.DiagnosticDuplicateID, fmt.Sprintf("domain %q contains duplicate value %q", parsed.Name, value), provenance)}
			}
			seen[value] = struct{}{}
			domain.Values = append(domain.Values, semanticir.DomainValue{ID: value, Provenance: provenance})
		}
		domains = append(domains, domain)
	}

	var diagnostics []semanticir.Diagnostic
	var rows []rowDefinition
	for rowIndex, cells := range table.Rows {
		rowLine := table.Line + 2 + rowIndex
		rowProvenance := rowProvenance(artifact, rowLine)
		id := cleanCell(cells[indices[normalizeHeader(HeaderID)]])
		if !identifierPattern.MatchString(id) {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("line %d has invalid requirement/constraint ID %q", rowLine, id), rowProvenance))
			continue
		}
		if _, exists := rowIDs[id]; exists {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticDuplicateID, fmt.Sprintf("duplicate row ID %q", id), rowProvenance))
			continue
		}
		rowIDs[id] = struct{}{}
		row := rowDefinition{id: id, domains: domains, provenance: rowProvenance}
		valid := true
		for _, parsed := range parsedDomains {
			values, valueErr := strictCellValues(cells[indices[normalizeHeader(parsed.Name)]], parsed)
			if valueErr != nil {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticUnreachable, fmt.Sprintf("row %q domain %q: %v", id, parsed.Name, valueErr), rowProvenance))
				valid = false
			}
			row.sets = append(row.sets, values)
		}
		if !valid {
			continue
		}
		reachability := strings.ToLower(cleanCell(cells[indices[normalizeHeader(HeaderReachability)]]))
		switch reachability {
		case "reachable":
			row.reachable = true
		case "excluded":
			row.reachable = false
		default:
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("row %q reachability is %q, want reachable or excluded", id, reachability), rowProvenance))
			continue
		}
		row.operation = cleanCell(cells[indices[normalizeHeader(HeaderOperation)]])
		if !identifierPattern.MatchString(row.operation) {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("row %q has invalid operation ID %q", id, row.operation), rowProvenance))
			continue
		}
		row.inputWitnessSource = strings.TrimSpace(cells[indices[normalizeHeader(HeaderInputWitnesses)]])
		if row.reachable {
			if row.inputWitnessSource == "" || strings.EqualFold(cleanCell(row.inputWitnessSource), "none") {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticIncomplete, fmt.Sprintf("row %q has no explicit Input witnesses", id), rowProvenance))
				continue
			}
		} else if !strings.EqualFold(cleanCell(row.inputWitnessSource), "none") {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("excluded row %q must set Input witnesses to none", id), rowProvenance))
			continue
		}
		row.constraintReason = cleanOptional(cells[indices[normalizeHeader(HeaderConstraintReason)]])
		if !row.reachable {
			if row.constraintReason == "" {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("excluded row %q has no constraint reason", id), rowProvenance))
				continue
			}
			if hasGradedExcludedCells(cells, indices) {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("excluded row %q must not declare operation/outcomes/effects/invariants/tests", id), rowProvenance))
				continue
			}
			rows = append(rows, row)
			continue
		}
		if row.constraintReason != "" {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticOverlapping, fmt.Sprintf("reachable row %q also declares a constraint reason", id), rowProvenance))
			continue
		}
		var parseErr error
		row.required, parseErr = parseOutcomes(cells[indices[normalizeHeader(HeaderRequiredOutcomes)]], row.operation, rowProvenance, false)
		if parseErr != nil {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticProseRequirement, fmt.Sprintf("row %q required outcomes: %v", id, parseErr), rowProvenance))
			continue
		}
		row.forbidden, parseErr = parseOutcomes(cells[indices[normalizeHeader(HeaderForbiddenOutcomes)]], row.operation, rowProvenance, true)
		if parseErr != nil {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticProseRequirement, fmt.Sprintf("row %q forbidden outcomes: %v", id, parseErr), rowProvenance))
			continue
		}
		row.effects, parseErr = parseEffects(cells[indices[normalizeHeader(HeaderEffects)]], rowProvenance)
		if parseErr != nil {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticProseRequirement, fmt.Sprintf("row %q effects: %v", id, parseErr), rowProvenance))
			continue
		}
		row.invariantIDs, parseErr = parseIDs(cells[indices[normalizeHeader(HeaderInvariants)]], true)
		if parseErr != nil {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticProseRequirement, fmt.Sprintf("row %q invariants: %v", id, parseErr), rowProvenance))
			continue
		}
		if len(row.invariantIDs) != 0 {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticProseRequirement, fmt.Sprintf("row %q names invariants without typed predicates/bindings; use none", id), rowProvenance))
			continue
		}
		row.testIDs, parseErr = parseIDs(cells[indices[normalizeHeader(HeaderEnforcedBy)]], true)
		if parseErr != nil {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("row %q enforced by: %v", id, parseErr), rowProvenance))
			continue
		}
		row.instructionSources, parseErr = parseInstructionSources(cells[indices[normalizeHeader(HeaderEvidence)]], anchors)
		if parseErr != nil {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidProvenance, fmt.Sprintf("row %q instruction source: %v", id, parseErr), rowProvenance))
			continue
		}
		rows = append(rows, row)
	}
	return domains, rows, diagnostics
}

func isZeroDomainDeclaration(raw string) bool {
	text := strings.TrimSpace(raw)
	if index := strings.Index(text, "Parameters:"); index >= 0 {
		text = strings.TrimSpace(text[index+len("Parameters:"):])
	}
	text = strings.TrimSpace(strings.TrimSuffix(text, "."))
	return strings.EqualFold(cleanCell(text), "none")
}

func parseOutcomes(cell, operationID string, provenance semanticir.Provenance, allowNone bool) ([]semanticir.ObservableOutcome, error) {
	if allowNone && strings.EqualFold(cleanOptional(cell), "none") {
		return nil, nil
	}
	items := splitList(cell)
	if len(items) == 0 {
		return nil, fmt.Errorf("empty outcome set")
	}
	var outcomes []semanticir.ObservableOutcome
	seen := map[string]struct{}{}
	for _, item := range items {
		outcome, err := parseOutcome(item, operationID, provenance)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[outcome.ID]; exists {
			return nil, fmt.Errorf("duplicate outcome %q", item)
		}
		seen[outcome.ID] = struct{}{}
		outcomes = append(outcomes, outcome)
	}
	return outcomes, nil
}

func strictCellValues(cell string, domain specparser.Domain) ([]string, error) {
	raw := strings.TrimSpace(cell)
	if raw == "" {
		return nil, fmt.Errorf("empty condition cell; use any or — explicitly")
	}
	if isWildcard(cleanCell(raw)) {
		return append([]string(nil), domain.Values...), nil
	}
	declared := make(map[string]struct{}, len(domain.Values))
	for _, value := range domain.Values {
		declared[value] = struct{}{}
	}
	var result []string
	seen := map[string]struct{}{}
	tokens, _, err := specparser.ParseValueList(raw)
	if err != nil {
		return nil, err
	}
	for _, token := range tokens {
		if _, exists := declared[token]; !exists {
			return nil, fmt.Errorf("value %q is undeclared", token)
		}
		if _, exists := seen[token]; exists {
			return nil, fmt.Errorf("value %q is repeated", token)
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	return result, nil
}

func parseOutcome(raw, operationID string, provenance semanticir.Provenance) (semanticir.ObservableOutcome, error) {
	text, effectText := splitOutcomeEffectSuffix(strings.TrimSpace(raw))
	outcome := semanticir.ObservableOutcome{OperationID: operationID, Provenance: provenance}
	switch {
	case text == "other outcome":
		if effectText != "" {
			return outcome, fmt.Errorf("other outcome is the complete complement and cannot declare effects")
		}
		return semanticir.OtherOutcome(operationID, provenance), nil
	case text == "success":
		outcome.Kind = semanticir.OutcomeSuccess
	case text == "timeout":
		if effectText != "" {
			return outcome, fmt.Errorf("timeout cannot declare effects")
		}
		outcome.Kind = semanticir.OutcomeTimeout
	case text == "return" || text == "return unit":
		outcome.Kind = semanticir.OutcomeReturn
		outcome.Value = &semanticir.Literal{Type: semanticir.TypeUnit}
	case strings.HasPrefix(text, "return "):
		literal, err := parseLiteral(strings.TrimSpace(strings.TrimPrefix(text, "return ")))
		if err != nil {
			return outcome, err
		}
		outcome.Kind = semanticir.OutcomeReturn
		outcome.Value = &literal
	case raisePattern.MatchString(text):
		parts := raisePattern.FindStringSubmatch(text)
		outcome.Kind = semanticir.OutcomeRaise
		outcome.ExceptionType = parts[1]
		outcome.Message = parts[2]
	default:
		return outcome, fmt.Errorf("%q is prose, want timeout, other outcome, success, return <typed literal>, or raise <Type> containing \"message\"", text)
	}
	if effectText != "" {
		effects, err := parseEffectItems(splitCommaList(effectText), provenance)
		if err != nil {
			return outcome, err
		}
		outcome.Effects = effects
	}
	outcome.ID = semanticir.OutcomeID(outcome)
	return outcome, nil
}

func splitOutcomeEffectSuffix(text string) (string, string) {
	inQuote := false
	for index := 0; index+6 <= len(text); index++ {
		if text[index] == '"' && (index == 0 || text[index-1] != '\\') {
			inQuote = !inQuote
			continue
		}
		if !inQuote && strings.HasPrefix(text[index:], " with ") {
			return strings.TrimSpace(text[:index]), strings.TrimSpace(text[index+6:])
		}
	}
	return text, ""
}

func splitCommaList(text string) []string {
	var result []string
	for _, item := range strings.Split(text, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseLiteral(text string) (semanticir.Literal, error) {
	if text == "null" {
		return semanticir.Literal{Type: semanticir.TypeOptional, Null: true}, nil
	}
	if text == "true" || text == "false" {
		return semanticir.Literal{Type: semanticir.TypeBool, Bool: text == "true"}, nil
	}
	if integer, err := strconv.ParseInt(text, 10, 64); err == nil {
		return semanticir.Literal{Type: semanticir.TypeInteger, Integer: integer}, nil
	}
	if len(text) >= 2 && strings.HasPrefix(text, "\"") && strings.HasSuffix(text, "\"") {
		unquoted, err := strconv.Unquote(text)
		if err != nil {
			return semanticir.Literal{}, fmt.Errorf("invalid string literal %q", text)
		}
		return semanticir.Literal{Type: semanticir.TypeString, String: unquoted}, nil
	}
	return semanticir.Literal{}, fmt.Errorf("%q is not null, a bool, integer, quoted string, or unit literal", text)
}

func parseEffects(cell string, provenance semanticir.Provenance) ([]semanticir.Effect, error) {
	clean := cleanOptional(cell)
	if clean == "" || strings.EqualFold(clean, "none") {
		return nil, nil
	}
	return parseEffectItems(splitList(clean), provenance)
}

func parseEffectItems(items []string, provenance semanticir.Provenance) ([]semanticir.Effect, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("empty effect set")
	}
	var effects []semanticir.Effect
	seen := map[string]struct{}{}
	for index, item := range items {
		parts := strings.SplitN(item, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("effect %q must be kind:target or kind:target=literal", item)
		}
		kind := semanticir.EffectKind(strings.ToLower(strings.TrimSpace(parts[0])))
		switch kind {
		case semanticir.EffectRead, semanticir.EffectWrite, semanticir.EffectCall, semanticir.EffectOutput:
		default:
			return nil, fmt.Errorf("effect %q has unsupported kind %q", item, kind)
		}
		target := strings.TrimSpace(parts[1])
		var value *semanticir.Expression
		if separator := strings.Index(target, "="); separator >= 0 {
			literalText := strings.TrimSpace(target[separator+1:])
			target = strings.TrimSpace(target[:separator])
			literal, err := parseLiteral(literalText)
			if err != nil {
				return nil, fmt.Errorf("effect %q value: %v", item, err)
			}
			value = &semanticir.Expression{Kind: semanticir.ExprLiteral, Type: literal.Type, Literal: &literal, Provenance: provenance}
		}
		if target == "" {
			return nil, fmt.Errorf("effect %q has an empty target", item)
		}
		identity, _ := semanticir.Digest(struct {
			Kind   semanticir.EffectKind `json:"kind"`
			Target string                `json:"target"`
			Value  any                   `json:"value,omitempty"`
		}{kind, target, value})
		id := fmt.Sprintf("effect-%d-%s", index+1, strings.TrimPrefix(identity, "sha256:")[:12])
		key := fmt.Sprintf("%s\x00%s\x00%s", kind, target, identity)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate effect %q", item)
		}
		seen[key] = struct{}{}
		effects = append(effects, semanticir.Effect{
			ID:         id,
			Kind:       kind,
			Target:     target,
			Value:      value,
			Provenance: provenance,
		})
	}
	return effects, nil
}

func parseIDs(cell string, allowNone bool) ([]string, error) {
	clean := cleanOptional(cell)
	if allowNone && (clean == "" || strings.EqualFold(clean, "none")) {
		return nil, nil
	}
	ids := splitList(clean)
	if len(ids) == 0 {
		return nil, fmt.Errorf("empty ID set")
	}
	seen := map[string]struct{}{}
	for _, id := range ids {
		if !identifierPattern.MatchString(id) {
			return nil, fmt.Errorf("invalid ID %q", id)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("duplicate ID %q", id)
		}
		seen[id] = struct{}{}
	}
	return ids, nil
}

func parseInstructionSources(cell string, artifacts anchorArtifacts) ([]semanticir.Provenance, error) {
	items := splitList(cell)
	if len(items) == 0 {
		return nil, fmt.Errorf("empty source anchor set")
	}
	var sources []semanticir.Provenance
	for _, item := range items {
		item = strings.TrimSpace(item)
		name := ""
		if match := anchorArtifactPattern.FindStringSubmatch(item); match != nil {
			name, item = match[1], match[2]
		}
		instruction, source, err := artifacts.resolve(name)
		if err != nil {
			return nil, err
		}
		lines := strings.Split(string(source), "\n")
		maxLine := len(lines)
		match := sourceSpanPattern.FindStringSubmatch(item)
		if match == nil {
			return nil, fmt.Errorf("invalid span %q, want line, line:column, or start-end, optionally prefixed with instruction: or reference:", item)
		}
		startLine, _ := strconv.Atoi(match[1])
		startColumn := 1
		if match[2] != "" {
			startColumn, _ = strconv.Atoi(match[2])
		}
		endLine := startLine
		if match[3] != "" {
			endLine, _ = strconv.Atoi(match[3])
		}
		if startLine < 1 || startLine > maxLine || endLine < startLine || endLine > maxLine {
			return nil, fmt.Errorf("span %q is outside %s's %d lines", item, instruction.ID, maxLine)
		}
		endColumn := len(lines[endLine-1])
		if match[4] != "" {
			endColumn, _ = strconv.Atoi(match[4])
		}
		if startColumn < 1 || startColumn > len(lines[startLine-1])+1 || endColumn < 1 || endColumn > len(lines[endLine-1]) || (endLine == startLine && endColumn < startColumn) {
			return nil, fmt.Errorf("span %q is outside %s's %d lines", item, instruction.ID, maxLine)
		}
		sources = append(sources, semanticir.NewProvenance(instruction, semanticir.SourceLocation{
			Path:        instruction.Path,
			StartLine:   startLine,
			StartColumn: startColumn,
			EndLine:     endLine,
			EndColumn:   endColumn,
		}, semanticir.TranslationTranslated))
	}
	return sources, nil
}

func proseOnlyRequirement(source string) (int, string) {
	insideFence := false
	for index, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			insideFence = !insideFence
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "|") || strings.Contains(trimmed, "Parameters:") {
			continue
		}
		if insideFence || normativePattern.MatchString(trimmed) {
			if normativePattern.MatchString(trimmed) {
				return index + 1, trimmed
			}
		}
	}
	return 0, ""
}

func hasGradedExcludedCells(cells []string, indices map[string]int) bool {
	for _, header := range []string{HeaderRequiredOutcomes, HeaderForbiddenOutcomes, HeaderEffects, HeaderInvariants, HeaderEnforcedBy, HeaderEvidence} {
		value := cleanOptional(cells[indices[normalizeHeader(header)]])
		if value != "" && !strings.EqualFold(value, "none") {
			return true
		}
	}
	return false
}

func expandAssignments(domains []semanticir.Domain, sets [][]string) []semanticir.Assignment {
	assignments := []semanticir.Assignment{{}}
	for index, domain := range domains {
		var next []semanticir.Assignment
		for _, assignment := range assignments {
			for _, value := range sets[index] {
				copyAssignment := make(semanticir.Assignment, len(assignment)+1)
				for key, existing := range assignment {
					copyAssignment[key] = existing
				}
				copyAssignment[domain.ID] = value
				next = append(next, copyAssignment)
			}
		}
		assignments = next
	}
	return assignments
}

func expandScopedAssignments(operationDomains, rowDomains []semanticir.Domain, rowSets [][]string) []semanticir.Assignment {
	setsByDomain := make(map[string][]string, len(rowDomains))
	for index, domain := range rowDomains {
		setsByDomain[domain.ID] = rowSets[index]
	}
	sets := make([][]string, 0, len(operationDomains))
	for _, domain := range operationDomains {
		if values, exists := setsByDomain[domain.ID]; exists {
			sets = append(sets, values)
			continue
		}
		values := make([]string, 0, len(domain.Values))
		for _, value := range domain.Values {
			values = append(values, value.ID)
		}
		sets = append(sets, values)
	}
	return expandAssignments(operationDomains, sets)
}

func selectCompilerDomains(registry []semanticir.Domain, ids []string) []semanticir.Domain {
	byID := make(map[string]semanticir.Domain, len(registry))
	for _, domain := range registry {
		byID[domain.ID] = domain
	}
	result := make([]semanticir.Domain, 0, len(ids))
	for _, id := range ids {
		if domain, exists := byID[id]; exists {
			result = append(result, domain)
		}
	}
	return result
}

func buildInstructionModel(artifact semanticir.ArtifactRef, source []byte, rows []rowDefinition) (semanticir.InstructionModel, error) {
	model := semanticir.InstructionModel{Artifact: artifact}
	seen := map[string]struct{}{}
	for _, row := range rows {
		for _, provenance := range row.instructionSources {
			// A reference-anchored row cites the solution, not the prompt.
			// Slicing instruction bytes at its line numbers would read
			// unrelated text, so it contributes no instruction clause.
			if provenance.ArtifactID != artifact.ID {
				continue
			}
			key := locationKey(provenance.Location)
			if _, exists := seen[key]; exists {
				continue
			}
			selected, err := sourceSlice(source, provenance.Location)
			if err != nil {
				return semanticir.InstructionModel{}, err
			}
			sliceDigest := semanticir.DigestBytes(selected)
			identity, _ := semanticir.Digest(struct {
				Location semanticir.SourceLocation `json:"location"`
				Digest   string                    `json:"digest"`
			}{provenance.Location, sliceDigest})
			model.Clauses = append(model.Clauses, semanticir.InstructionClause{
				ID:          "instruction-clause-" + strings.TrimPrefix(identity, "sha256:")[:16],
				Span:        provenance.Location,
				SliceDigest: sliceDigest,
				Provenance:  provenance,
			})
			seen[key] = struct{}{}
		}
	}
	provenance := semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	model.Coverage = semanticir.TranslationCoverage{
		Status:               semanticir.TranslationComplete,
		TotalConstructs:      len(model.Clauses),
		TranslatedConstructs: len(model.Clauses),
		Provenance:           provenance,
	}
	return model, nil
}

func clauseIDsForSources(model semanticir.InstructionModel, sources []semanticir.Provenance) []string {
	byLocation := make(map[string]string, len(model.Clauses))
	for _, clause := range model.Clauses {
		byLocation[locationKey(clause.Span)] = clause.ID
	}
	ids := make([]string, 0, len(sources))
	for _, source := range sources {
		if id := byLocation[locationKey(source.Location)]; id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func sourceSlice(source []byte, location semanticir.SourceLocation) ([]byte, error) {
	lines := strings.Split(string(source), "\n")
	if location.StartLine < 1 || location.EndLine < location.StartLine || location.EndLine > len(lines) {
		return nil, fmt.Errorf("invalid instruction source span")
	}
	selected := append([]string(nil), lines[location.StartLine-1:location.EndLine]...)
	start := location.StartColumn - 1
	if start < 0 || start > len(selected[0]) {
		return nil, fmt.Errorf("instruction start column is outside source line")
	}
	selected[0] = selected[0][start:]
	if location.EndColumn > 0 {
		last := len(selected) - 1
		end := location.EndColumn
		if location.StartLine == location.EndLine {
			end -= start
		}
		if end < 0 || end > len(selected[last]) {
			return nil, fmt.Errorf("instruction end column is outside source line")
		}
		selected[last] = selected[last][:end]
	}
	return []byte(strings.Join(selected, "\n")), nil
}

func locationKey(location semanticir.SourceLocation) string {
	return fmt.Sprintf("%s:%d:%d-%d:%d", location.Path, location.StartLine, location.StartColumn, location.EndLine, location.EndColumn)
}

func outcomeIDs(outcomes []semanticir.ObservableOutcome) []string {
	ids := make([]string, 0, len(outcomes))
	for _, outcome := range outcomes {
		ids = append(ids, outcome.ID)
	}
	sort.Strings(ids)
	return ids
}

func sortedOutcomes(values map[string]semanticir.ObservableOutcome) []semanticir.ObservableOutcome {
	keys := sortedKeys(values)
	result := make([]semanticir.ObservableOutcome, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

func sortedOperations(values map[string]semanticir.Operation) []semanticir.Operation {
	keys := sortedKeys(values)
	result := make([]semanticir.Operation, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

func sortedInvariants(values map[string]semanticir.Invariant) []semanticir.Invariant {
	keys := sortedKeys(values)
	result := make([]semanticir.Invariant, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sameDomains(left, right []semanticir.Domain) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].ID != right[index].ID || left[index].Type != right[index].Type || len(left[index].Values) != len(right[index].Values) {
			return false
		}
		for valueIndex := range left[index].Values {
			if left[index].Values[valueIndex].ID != right[index].Values[valueIndex].ID || !reflect.DeepEqual(left[index].Values[valueIndex].Value, right[index].Values[valueIndex].Value) {
				return false
			}
		}
	}
	return true
}

func domainIDs(domains []semanticir.Domain) []string {
	ids := make([]string, 0, len(domains))
	for _, domain := range domains {
		ids = append(ids, domain.ID)
	}
	return ids
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

func unionStrings(left, right []string) []string {
	seen := make(map[string]struct{}, len(left)+len(right))
	result := make([]string, 0, len(left)+len(right))
	for _, values := range [][]string{left, right} {
		for _, value := range values {
			if _, exists := seen[value]; !exists {
				seen[value] = struct{}{}
				result = append(result, value)
			}
		}
	}
	return result
}

func sameOutcome(left, right semanticir.ObservableOutcome) bool {
	if left.Kind != right.Kind || left.ExceptionType != right.ExceptionType || left.Message != right.Message {
		return false
	}
	if left.Value == nil || right.Value == nil {
		return left.Value == nil && right.Value == nil
	}
	return reflect.DeepEqual(left.Value, right.Value)
}

func expandedID(base string, index, total int) string {
	if total == 1 {
		return base
	}
	return fmt.Sprintf("%s.%d", base, index+1)
}

func normalizeHeader(header string) string {
	return strings.ToLower(strings.Join(strings.Fields(cleanCell(header)), " "))
}

func cleanCell(cell string) string {
	return strings.TrimSpace(strings.ReplaceAll(cell, "`", ""))
}

func cleanOptional(cell string) string {
	clean := cleanCell(cell)
	if clean == "—" || clean == "-" || clean == "--" {
		return ""
	}
	return clean
}

func splitList(cell string) []string {
	clean := cleanOptional(cell)
	if clean == "" {
		return nil
	}
	var result []string
	for _, item := range strings.Split(clean, ";") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func isWildcard(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized == "any" || normalized == "—" || normalized == "-" || normalized == "--"
}

func rowProvenance(artifact semanticir.ArtifactRef, line int) semanticir.Provenance {
	return semanticir.NewProvenance(artifact, semanticir.SourceLocation{
		Path:        artifact.Path,
		StartLine:   line,
		StartColumn: 1,
	}, semanticir.TranslationTranslated)
}

func diagnostic(code semanticir.DiagnosticCode, message string, provenance semanticir.Provenance) semanticir.Diagnostic {
	return semanticir.Diagnostic{Severity: semanticir.SeverityError, Code: code, Message: message, Provenance: provenance}
}

var quantifierPattern = regexp.MustCompile(`(?i)\b(every|any|all|regardless)\b`)

// warnQuantifierLabels flags a lone boolean row whose contract anchor
// quantifies. "Every horizon over one" is a domain over horizons; written as
// one true/false aspect the dimension vanishes from the finite model, and no
// later stage can ask whether each value is enforced. Warning severity: the
// anchor text may quantify over something the row legitimately does not
// range over, so the author decides -- but now with the line named.
func warnQuantifierLabels(rows []rowDefinition, instructionSource []byte) []semanticir.Diagnostic {
	var diagnostics []semanticir.Diagnostic
	lines := strings.Split(string(instructionSource), "\n")
	for _, row := range rows {
		if !row.reachable || len(row.required) != 1 || row.required[0].Kind != semanticir.OutcomeReturn {
			continue
		}
		value := row.required[0].Value
		if value == nil || value.Type != semanticir.TypeBool {
			continue
		}
		for _, source := range row.instructionSources {
			start, end := source.Location.StartLine, source.Location.EndLine
			if end < start {
				end = start
			}
			for line := start; line <= end && line <= len(lines); line++ {
				if match := quantifierPattern.FindString(lines[line-1]); match != "" {
					diagnostics = append(diagnostics, semanticir.Diagnostic{
						Severity:   semanticir.SeverityWarning,
						Code:       semanticir.DiagnosticQuantifierAsLabel,
						Message:    fmt.Sprintf("row %q is a lone boolean anchored to contract text containing %q (line %d); a quantified variable is a domain -- consider `Universe:` values ranging over it", row.id, match, line),
						Provenance: row.provenance,
					})
					line = end + 1
					break
				}
			}
		}
	}
	return diagnostics
}
