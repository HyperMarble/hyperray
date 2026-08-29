package speccompiler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

var inputIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type inputDeclaration struct {
	operationID string
	inputs      []semanticir.Variable
	provenance  semanticir.Provenance
}

type groundingDeclaration struct {
	key        string
	membership string
	witness    string
	provenance semanticir.Provenance
}

// bridgeDeclarations carries the Scope, Classify, and Observe directives that
// tie the finite model to the real system (proof-requirements.md, groups B
// and C). Scope names the finite real input set an operation covers, Classify
// names the executable that maps a real input to its row, and Observe names
// the executable that decides one string outcome label on real output.
type bridgeDeclarations struct {
	scopes      map[string]string
	classifiers map[string]string
	observers   map[string]map[string]string
}

type universeDeclaration struct {
	operationID string
	inputName   string
	values      []semanticir.Literal
	provenance  semanticir.Provenance
}

// parseSemanticDeclarations extracts the strict operation-input and label
// grounding language. These declarations are deliberately independent of
// Markdown tables so a relational category can range over several concrete
// inputs while remaining one finite semantic domain.
func parseSemanticDeclarations(source string, artifact semanticir.ArtifactRef) ([]inputDeclaration, []universeDeclaration, []groundingDeclaration, bridgeDeclarations, []semanticir.Diagnostic) {
	var inputs []inputDeclaration
	var universes []universeDeclaration
	var groundings []groundingDeclaration
	bridges := bridgeDeclarations{scopes: map[string]string{}, classifiers: map[string]string{}, observers: map[string]map[string]string{}}
	var diagnostics []semanticir.Diagnostic
	for index, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		provenance := rowProvenance(artifact, index+1)
		switch {
		case strings.HasPrefix(trimmed, "Inputs:"):
			declaration, err := parseInputDeclaration(trimmed, provenance)
			if err != nil {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("line %d Inputs declaration: %v", index+1, err), provenance))
				continue
			}
			inputs = append(inputs, declaration)
		case strings.HasPrefix(trimmed, "Grounding:"):
			declaration, err := parseGroundingDeclaration(trimmed, provenance)
			if err != nil {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("line %d Grounding declaration: %v", index+1, err), provenance))
				continue
			}
			groundings = append(groundings, declaration)
		case strings.HasPrefix(trimmed, "Scope:"):
			if err := parseBridgeAssignment(trimmed, "Scope:", bridges.scopes); err != nil {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("line %d Scope declaration: %v", index+1, err), provenance))
			}
		case strings.HasPrefix(trimmed, "Classify:"):
			if err := parseBridgeAssignment(trimmed, "Classify:", bridges.classifiers); err != nil {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("line %d Classify declaration: %v", index+1, err), provenance))
			}
		case strings.HasPrefix(trimmed, "Observe:"):
			if err := parseObserveDeclaration(trimmed, bridges.observers); err != nil {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("line %d Observe declaration: %v", index+1, err), provenance))
			}
		case strings.HasPrefix(trimmed, "Universe:"):
			declaration, err := parseUniverseDeclaration(trimmed, provenance)
			if err != nil {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("line %d Universe declaration: %v", index+1, err), provenance))
				continue
			}
			universes = append(universes, declaration)
		}
	}
	return inputs, universes, groundings, bridges, diagnostics
}

// parseBridgeAssignment handles `Scope: <op> = <text>.` and
// `Classify: <op> = command <text>.` -- one operation, one non-empty body.
func parseBridgeAssignment(line, prefix string, into map[string]string) error {
	body := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if !strings.HasSuffix(body, ".") {
		return fmt.Errorf("must end with a period")
	}
	body = strings.TrimSpace(strings.TrimSuffix(body, "."))
	operationID, text, found := strings.Cut(body, " = ")
	operationID = strings.TrimSpace(operationID)
	text = strings.TrimSpace(text)
	if prefix == "Classify:" {
		text = strings.TrimSpace(strings.TrimPrefix(text, "command"))
	}
	if !found || !identifierPattern.MatchString(operationID) || text == "" {
		return fmt.Errorf("want %s operation = %s<non-empty text>.", prefix, map[bool]string{true: "command ", false: ""}[prefix == "Classify:"])
	}
	if _, exists := into[operationID]; exists {
		return fmt.Errorf("operation %q declared twice", operationID)
	}
	into[operationID] = text
	return nil
}

// parseObserveDeclaration handles `Observe: <op>."<label>" = command <text>.`
func parseObserveDeclaration(line string, into map[string]map[string]string) error {
	body := strings.TrimSpace(strings.TrimPrefix(line, "Observe:"))
	if !strings.HasSuffix(body, ".") {
		return fmt.Errorf("must end with a period")
	}
	body = strings.TrimSpace(strings.TrimSuffix(body, "."))
	left, text, found := strings.Cut(body, " = command ")
	match := observeKeyPattern.FindStringSubmatch(strings.TrimSpace(left))
	text = strings.TrimSpace(text)
	if !found || match == nil || text == "" {
		return fmt.Errorf(`want Observe: operation."label" = command <non-empty text>.`)
	}
	operationID, label := match[1], match[2]
	if into[operationID] == nil {
		into[operationID] = map[string]string{}
	}
	if _, exists := into[operationID][label]; exists {
		return fmt.Errorf("label %q observed twice for operation %q", label, operationID)
	}
	into[operationID][label] = text
	return nil
}

var observeKeyPattern = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_.:#-]*)\."([^"]+)"$`)

// validateBridges requires every operation the rows use to declare its Scope
// and Classify, and every string outcome label to declare an Observe. Without
// them the finite proof is about words with no tie to the real system.
func validateBridges(bridges bridgeDeclarations, rows []rowDefinition) []semanticir.Diagnostic {
	var diagnostics []semanticir.Diagnostic
	labels := map[string]map[string]bool{}
	operations := map[string]semanticir.Provenance{}
	var order []string
	for _, row := range rows {
		if _, seen := operations[row.operation]; !seen {
			operations[row.operation] = row.provenance
			order = append(order, row.operation)
		}
		for _, set := range [][]semanticir.ObservableOutcome{row.required, row.forbidden} {
			for _, outcome := range set {
				if outcome.Kind == semanticir.OutcomeReturn && outcome.Value != nil && outcome.Value.Type == semanticir.TypeString {
					if labels[row.operation] == nil {
						labels[row.operation] = map[string]bool{}
					}
					labels[row.operation][outcome.Value.String] = true
				}
			}
		}
	}
	for _, operationID := range order {
		provenance := operations[operationID]
		// v1 boundary: only an operation with string outcome labels needs its
		// bridges, because a label is a word until an observer defines it.
		// Concrete outcomes (booleans, numbers, unit, exceptions) carry their
		// own values; their runtime bridging arrives with the proof runner.
		if len(labels[operationID]) == 0 {
			continue
		}
		if bridges.scopes[operationID] == "" {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticMissingBridge, fmt.Sprintf("operation %q has no Scope declaration naming its finite real input set; add `Scope: %s = <the finite real inputs covered>.`", operationID, operationID), provenance))
		}
		if bridges.classifiers[operationID] == "" {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticMissingBridge, fmt.Sprintf("operation %q has no Classify declaration naming its row classifier; add `Classify: %s = command <executable mapping a real input to its row>.`", operationID, operationID), provenance))
		}
		var labelOrder []string
		for label := range labels[operationID] {
			labelOrder = append(labelOrder, label)
		}
		sort.Strings(labelOrder)
		for _, label := range labelOrder {
			if bridges.observers[operationID][label] == "" {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticMissingBridge, fmt.Sprintf("outcome label %q of operation %q has no Observe declaration; add `Observe: %s.%q = command <executable deciding the label on real output>.`", label, operationID, operationID, label), provenance))
			}
		}
	}
	// The reverse direction: an Observe naming an operation or label no row
	// uses is a dead declaration -- most often a typo -- and silently keeping
	// it lets the spec claim an observation nothing exercises.
	var observedOps []string
	for operationID := range bridges.observers {
		observedOps = append(observedOps, operationID)
	}
	sort.Strings(observedOps)
	for _, operationID := range observedOps {
		var observed []string
		for label := range bridges.observers[operationID] {
			observed = append(observed, label)
		}
		sort.Strings(observed)
		for _, label := range observed {
			if _, exists := operations[operationID]; !exists {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticMissingBridge, fmt.Sprintf("Observe declaration names operation %q, which no row uses", operationID), semanticir.Provenance{}))
				break
			}
			if !labels[operationID][label] {
				diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticMissingBridge, fmt.Sprintf("Observe declaration names label %q of operation %q, which no row's outcomes use", label, operationID), operations[operationID]))
			}
		}
	}
	return diagnostics
}

func parseUniverseDeclaration(line string, provenance semanticir.Provenance) (universeDeclaration, error) {
	var result universeDeclaration
	body := strings.TrimSpace(strings.TrimPrefix(line, "Universe:"))
	if !strings.HasSuffix(body, ".") {
		return result, fmt.Errorf("must end with a period")
	}
	body = strings.TrimSpace(strings.TrimSuffix(body, "."))
	left, right, found := strings.Cut(body, " = values ")
	parts := strings.Split(strings.TrimSpace(left), ".")
	if !found || len(parts) != 2 || !identifierPattern.MatchString(parts[0]) || !inputIdentifierPattern.MatchString(parts[1]) {
		return result, fmt.Errorf("want Universe: operation.input = values [<canonical JSON values>].")
	}
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(right)))
	decoder.UseNumber()
	var raw []any
	if err := decoder.Decode(&raw); err != nil || len(raw) == 0 {
		return result, fmt.Errorf("values must be a non-empty JSON array")
	}
	canonical, err := json.Marshal(raw)
	if err != nil || string(canonical) != strings.TrimSpace(right) {
		return result, fmt.Errorf("values must be compact canonical JSON")
	}
	seen := map[string]bool{}
	for _, value := range raw {
		literal, literalErr := literalFromJSON(value)
		if literalErr != nil {
			return result, literalErr
		}
		digest, _ := semanticir.Digest(literal)
		if seen[digest] {
			return result, fmt.Errorf("values repeat a runtime literal")
		}
		seen[digest] = true
		result.values = append(result.values, literal)
	}
	result.operationID, result.inputName, result.provenance = parts[0], parts[1], provenance
	return result, nil
}

func parseInputDeclaration(line string, provenance semanticir.Provenance) (inputDeclaration, error) {
	var result inputDeclaration
	body := strings.TrimSpace(strings.TrimPrefix(line, "Inputs:"))
	if !strings.HasSuffix(body, ".") {
		return result, fmt.Errorf("must end with a period")
	}
	body = strings.TrimSpace(strings.TrimSuffix(body, "."))
	open := strings.IndexByte(body, '(')
	if open <= 0 || !strings.HasSuffix(body, ")") || strings.Contains(body[open+1:len(body)-1], "(") {
		return result, fmt.Errorf("want Inputs: operation(name: type, ...).")
	}
	operationID := strings.TrimSpace(body[:open])
	if !identifierPattern.MatchString(operationID) {
		return result, fmt.Errorf("invalid operation ID %q", operationID)
	}
	result = inputDeclaration{operationID: operationID, provenance: provenance}
	fields := strings.TrimSpace(body[open+1 : len(body)-1])
	if fields == "" {
		return result, nil
	}
	seen := map[string]struct{}{}
	for _, raw := range strings.Split(fields, ",") {
		parts := strings.Split(raw, ":")
		if len(parts) != 2 {
			return inputDeclaration{}, fmt.Errorf("input %q must be name: type", strings.TrimSpace(raw))
		}
		name := strings.TrimSpace(parts[0])
		if !inputIdentifierPattern.MatchString(name) {
			return inputDeclaration{}, fmt.Errorf("invalid input name %q", name)
		}
		if _, duplicate := seen[name]; duplicate {
			return inputDeclaration{}, fmt.Errorf("duplicate input %q", name)
		}
		seen[name] = struct{}{}
		valueType := semanticir.ValueType(strings.TrimSpace(parts[1]))
		switch valueType {
		case semanticir.TypeBool, semanticir.TypeInteger, semanticir.TypeString:
		default:
			return inputDeclaration{}, fmt.Errorf("input %q has unsupported type %q", name, valueType)
		}
		result.inputs = append(result.inputs, semanticir.Variable{Name: name, Type: valueType, Provenance: provenance})
	}
	return result, nil
}

func parseGroundingDeclaration(line string, provenance semanticir.Provenance) (groundingDeclaration, error) {
	var result groundingDeclaration
	body := strings.TrimSpace(strings.TrimPrefix(line, "Grounding:"))
	if !strings.HasSuffix(body, ".") {
		return result, fmt.Errorf("must end with a period")
	}
	body = strings.TrimSpace(strings.TrimSuffix(body, "."))
	parts := strings.Split(body, " = when ")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return result, fmt.Errorf(`want Grounding: operation.domain."value" = when <boolean expression>; witness {<canonical JSON assignment>}.`)
	}
	rhs := strings.Split(parts[1], "; witness ")
	if len(rhs) != 2 || strings.TrimSpace(rhs[0]) == "" || strings.TrimSpace(rhs[1]) == "" {
		return result, fmt.Errorf("want exactly one membership expression and witness")
	}
	return groundingDeclaration{key: strings.TrimSpace(parts[0]), membership: strings.TrimSpace(rhs[0]), witness: strings.TrimSpace(rhs[1]), provenance: provenance}, nil
}

func applySemanticDeclarations(
	inputDeclarations []inputDeclaration,
	universeDeclarations []universeDeclaration,
	groundingDeclarations []groundingDeclaration,
	operations map[string]semanticir.Operation,
	domains []semanticir.Domain,
) ([]semanticir.Domain, map[string]semanticir.Operation, []semanticir.Diagnostic) {
	var diagnostics []semanticir.Diagnostic
	declaredInputs := map[string]inputDeclaration{}
	for _, declaration := range inputDeclarations {
		operation, exists := operations[declaration.operationID]
		if !exists {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidReference, fmt.Sprintf("Inputs declaration refers to unknown operation %q", declaration.operationID), declaration.provenance))
			continue
		}
		if _, duplicate := declaredInputs[declaration.operationID]; duplicate {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticDuplicateID, fmt.Sprintf("operation %q has more than one Inputs declaration", declaration.operationID), declaration.provenance))
			continue
		}
		declaredInputs[declaration.operationID] = declaration
		operation.Inputs = append([]semanticir.Variable(nil), declaration.inputs...)
		operations[declaration.operationID] = operation
	}
	for operationID, operation := range operations {
		if _, exists := declaredInputs[operationID]; !exists {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticIncomplete, fmt.Sprintf("operation %q has no explicit Inputs declaration", operationID), operation.Provenance))
		}
	}
	seenUniverses := map[string]bool{}
	for _, declaration := range universeDeclarations {
		operation, exists := operations[declaration.operationID]
		if !exists {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidReference, fmt.Sprintf("Universe declaration refers to unknown operation %q", declaration.operationID), declaration.provenance))
			continue
		}
		key := declaration.operationID + "\x00" + declaration.inputName
		if seenUniverses[key] {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticDuplicateID, fmt.Sprintf("duplicate Universe declaration for %s.%s", declaration.operationID, declaration.inputName), declaration.provenance))
			continue
		}
		seenUniverses[key] = true
		matched := false
		for index := range operation.Inputs {
			input := &operation.Inputs[index]
			if input.Name != declaration.inputName {
				continue
			}
			matched = true
			for _, literal := range declaration.values {
				if literal.Type != input.Type {
					diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("Universe %s.%s contains %q, want %q", declaration.operationID, declaration.inputName, literal.Type, input.Type), declaration.provenance))
					matched = false
					break
				}
			}
			if matched {
				input.Universe = append([]semanticir.Literal(nil), declaration.values...)
			}
			break
		}
		if !matched {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidReference, fmt.Sprintf("Universe declaration refers to unknown input %s.%s", declaration.operationID, declaration.inputName), declaration.provenance))
			continue
		}
		operations[declaration.operationID] = operation
	}

	type expectedGrounding struct {
		operationID string
		domainIndex int
		valueIndex  int
	}
	var expected []expectedGrounding
	for operationID, operation := range operations {
		for _, domainID := range operation.DomainIDs {
			for domainIndex := range domains {
				if domains[domainIndex].ID != domainID {
					continue
				}
				for valueIndex := range domains[domainIndex].Values {
					expected = append(expected, expectedGrounding{operationID: operationID, domainIndex: domainIndex, valueIndex: valueIndex})
				}
			}
		}
	}
	seen := map[string]struct{}{}
	for _, declaration := range groundingDeclarations {
		var matches []expectedGrounding
		for _, item := range expected {
			prefix := item.operationID + "." + domains[item.domainIndex].ID + "."
			if !strings.HasPrefix(declaration.key, prefix) {
				continue
			}
			encodedValue := strings.TrimPrefix(declaration.key, prefix)
			value, err := decodeCanonicalJSONString(encodedValue)
			if err == nil && value == domains[item.domainIndex].Values[item.valueIndex].ID {
				matches = append(matches, item)
			}
		}
		if len(matches) != 1 {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidReference, fmt.Sprintf("grounding key %q resolves to %d operation/domain/value triples; value must be a canonical JSON string", declaration.key, len(matches)), declaration.provenance))
			continue
		}
		item := matches[0]
		key := item.operationID + "\x00" + domains[item.domainIndex].ID + "\x00" + domains[item.domainIndex].Values[item.valueIndex].ID
		if _, duplicate := seen[key]; duplicate {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticDuplicateID, fmt.Sprintf("duplicate grounding for %q", declaration.key), declaration.provenance))
			continue
		}
		seen[key] = struct{}{}
		operation := operations[item.operationID]
		membership, err := parseMembershipExpression(declaration.membership, operation.Inputs, declaration.provenance)
		if err != nil {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("grounding %q membership: %v", declaration.key, err), declaration.provenance))
			continue
		}
		witness, err := parseCanonicalWitness(declaration.witness, operation.Inputs)
		if err != nil {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("grounding %q witness: %v", declaration.key, err), declaration.provenance))
			continue
		}
		if satisfied, err := evaluateBooleanExpression(membership, witness); err != nil || !satisfied {
			if err == nil {
				err = fmt.Errorf("assignment does not satisfy membership")
			}
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticUnreachable, fmt.Sprintf("grounding %q witness: %v", declaration.key, err), declaration.provenance))
			continue
		}
		value := &domains[item.domainIndex].Values[item.valueIndex]
		value.Groundings = append(value.Groundings, semanticir.GroundingAxiom{
			OperationID: item.operationID, Kind: semanticir.GroundingMembership, Membership: &membership,
			ConcreteWitness: witness, Provenance: declaration.provenance,
		})
	}
	for _, item := range expected {
		key := item.operationID + "\x00" + domains[item.domainIndex].ID + "\x00" + domains[item.domainIndex].Values[item.valueIndex].ID
		if _, exists := seen[key]; !exists {
			value := domains[item.domainIndex].Values[item.valueIndex]
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticIncomplete, fmt.Sprintf("operation %q domain %q value %q has no explicit Grounding declaration", item.operationID, domains[item.domainIndex].ID, value.ID), value.Provenance))
		}
	}
	return domains, operations, diagnostics
}

func decodeCanonicalJSONString(source string) (string, error) {
	var value string
	if len(source) < 2 || source[0] != '"' || source[len(source)-1] != '"' || json.Unmarshal([]byte(source), &value) != nil {
		return "", fmt.Errorf("not a JSON string")
	}
	canonical, _ := json.Marshal(value)
	if string(canonical) != source || value == "" {
		return "", fmt.Errorf("not a canonical non-empty JSON string")
	}
	return value, nil
}

func parseCanonicalWitness(source string, inputs []semanticir.Variable) (map[string]semanticir.Literal, error) {
	decoder := json.NewDecoder(strings.NewReader(source))
	decoder.UseNumber()
	var decoded map[string]any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("invalid JSON object: %w", err)
	}
	if decoder.More() {
		return nil, fmt.Errorf("trailing JSON content")
	}
	canonical, err := json.Marshal(decoded)
	if err != nil || !bytes.Equal(canonical, []byte(source)) {
		return nil, fmt.Errorf("must be compact canonical JSON with keys in lexical order")
	}
	if decoded == nil {
		return nil, fmt.Errorf("must be a JSON object")
	}
	inputTypes := map[string]semanticir.ValueType{}
	for _, input := range inputs {
		inputTypes[input.Name] = input.Type
	}
	if len(decoded) != len(inputTypes) {
		return nil, fmt.Errorf("must assign every operation input exactly once")
	}
	result := make(map[string]semanticir.Literal, len(decoded))
	for name, raw := range decoded {
		valueType, exists := inputTypes[name]
		if !exists {
			return nil, fmt.Errorf("assigns unknown input %q", name)
		}
		literal, err := literalFromJSON(raw)
		if err != nil {
			return nil, fmt.Errorf("input %q: %w", name, err)
		}
		if literal.Type != valueType {
			return nil, fmt.Errorf("input %q has type %q, want %q", name, literal.Type, valueType)
		}
		result[name] = literal
	}
	return result, nil
}

func parseCanonicalWitnessList(source string, inputs []semanticir.Variable) ([]map[string]semanticir.Literal, error) {
	decoder := json.NewDecoder(strings.NewReader(source))
	decoder.UseNumber()
	var decoded []any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("invalid JSON array: %w", err)
	}
	canonical, err := json.Marshal(decoded)
	if err != nil || !bytes.Equal(canonical, []byte(source)) {
		return nil, fmt.Errorf("must be a compact canonical JSON array with object keys in lexical order")
	}
	if decoded == nil || len(decoded) == 0 {
		return nil, fmt.Errorf("must contain at least one input assignment")
	}
	result := make([]map[string]semanticir.Literal, 0, len(decoded))
	for index, raw := range decoded {
		encoded, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		witness, err := parseCanonicalWitness(string(encoded), inputs)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", index, err)
		}
		result = append(result, witness)
	}
	return result, nil
}

func applyRowInputWitnesses(rows []rowDefinition, operations map[string]semanticir.Operation, domains []semanticir.Domain) ([]rowDefinition, []semanticir.Diagnostic) {
	var diagnostics []semanticir.Diagnostic
	domainByID := make(map[string]semanticir.Domain, len(domains))
	for _, domain := range domains {
		domainByID[domain.ID] = domain
	}
	for rowIndex := range rows {
		row := &rows[rowIndex]
		if !row.reachable {
			continue
		}
		operation := operations[row.operation]
		assignments := expandScopedAssignments(selectCompilerDomains(domains, operation.DomainIDs), row.domains, row.sets)
		witnesses, err := parseCanonicalWitnessList(row.inputWitnessSource, operation.Inputs)
		if err != nil {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticInvalidInput, fmt.Sprintf("row %q Input witnesses: %v", row.id, err), row.provenance))
			continue
		}
		if len(witnesses) != len(assignments) {
			diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticIncomplete, fmt.Sprintf("row %q has %d Input witnesses, want %d in expanded-assignment order", row.id, len(witnesses), len(assignments)), row.provenance))
			continue
		}
		valid := true
		for index, assignment := range assignments {
			for domainID, valueID := range assignment {
				domain := domainByID[domainID]
				var memberships []semanticir.Expression
				for _, value := range domain.Values {
					if value.ID != valueID {
						continue
					}
					for _, grounding := range value.Groundings {
						if grounding.OperationID == row.operation && grounding.Membership != nil {
							memberships = append(memberships, *grounding.Membership)
						}
					}
				}
				if len(memberships) != 1 {
					diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticIncomplete, fmt.Sprintf("row %q expansion %d label %s=%q has %d membership groundings", row.id, index, domainID, valueID, len(memberships)), row.provenance))
					valid = false
					continue
				}
				satisfied, evaluateErr := evaluateBooleanExpression(memberships[0], witnesses[index])
				if evaluateErr != nil || !satisfied {
					diagnostics = append(diagnostics, diagnostic(semanticir.DiagnosticUnreachable, fmt.Sprintf("row %q Input witnesses element %d does not satisfy expanded assignment %s=%q", row.id, index, domainID, valueID), row.provenance))
					valid = false
				}
			}
		}
		if valid {
			row.inputWitnesses = witnesses
		}
	}
	return rows, diagnostics
}

func literalFromJSON(value any) (semanticir.Literal, error) {
	switch typed := value.(type) {
	case nil:
		return semanticir.Literal{Type: semanticir.TypeOptional, Null: true}, nil
	case bool:
		return semanticir.Literal{Type: semanticir.TypeBool, Bool: typed}, nil
	case string:
		return semanticir.Literal{Type: semanticir.TypeString, String: typed}, nil
	case json.Number:
		integer, err := strconv.ParseInt(string(typed), 10, 64)
		if err != nil {
			return semanticir.Literal{}, fmt.Errorf("number is not a signed 64-bit integer")
		}
		return semanticir.Literal{Type: semanticir.TypeInteger, Integer: integer}, nil
	case []any:
		values := make([]semanticir.Literal, 0, len(typed))
		for _, element := range typed {
			literal, err := literalFromJSON(element)
			if err != nil {
				return semanticir.Literal{}, err
			}
			values = append(values, literal)
		}
		return semanticir.Literal{Type: semanticir.TypeSequence, Elements: &semanticir.LiteralElements{Values: values}}, nil
	case map[string]any:
		values := make(map[string]semanticir.Literal, len(typed))
		for key, element := range typed {
			literal, err := literalFromJSON(element)
			if err != nil {
				return semanticir.Literal{}, err
			}
			values[key] = literal
		}
		return semanticir.Literal{Type: semanticir.TypeRecord, Fields: &semanticir.LiteralFields{Values: values}}, nil
	default:
		return semanticir.Literal{}, fmt.Errorf("unsupported JSON value")
	}
}

type expressionTokenKind int

const (
	expressionEOF expressionTokenKind = iota
	expressionName
	expressionLiteral
	expressionLParen
	expressionRParen
	expressionEQ
	expressionNE
	expressionLT
	expressionLE
	expressionGT
	expressionGE
	expressionNot
	expressionAnd
	expressionOr
	expressionPlus
	expressionMinus
	expressionStar
)

type expressionToken struct {
	kind    expressionTokenKind
	text    string
	literal semanticir.Literal
}

type membershipParser struct {
	tokens     []expressionToken
	position   int
	inputs     map[string]semanticir.ValueType
	provenance semanticir.Provenance
}

func parseMembershipExpression(source string, inputs []semanticir.Variable, provenance semanticir.Provenance) (semanticir.Expression, error) {
	tokens, err := lexMembership(source)
	if err != nil {
		return semanticir.Expression{}, err
	}
	parser := membershipParser{tokens: tokens, inputs: map[string]semanticir.ValueType{}, provenance: provenance}
	for _, input := range inputs {
		parser.inputs[input.Name] = input.Type
	}
	expression, err := parser.parseOr()
	if err != nil {
		return semanticir.Expression{}, err
	}
	if parser.peek().kind != expressionEOF {
		return semanticir.Expression{}, fmt.Errorf("unexpected token %q", parser.peek().text)
	}
	if expression.Type != semanticir.TypeBool {
		return semanticir.Expression{}, fmt.Errorf("top-level membership is %q, want bool", expression.Type)
	}
	return expression, nil
}

func lexMembership(source string) ([]expressionToken, error) {
	var tokens []expressionToken
	for index := 0; index < len(source); {
		if unicode.IsSpace(rune(source[index])) {
			index++
			continue
		}
		switch source[index] {
		case '(':
			tokens = append(tokens, expressionToken{kind: expressionLParen, text: "("})
			index++
			continue
		case ')':
			tokens = append(tokens, expressionToken{kind: expressionRParen, text: ")"})
			index++
			continue
		case '=':
			if index+1 >= len(source) || source[index+1] != '=' {
				return nil, fmt.Errorf("use == for equality")
			}
			tokens = append(tokens, expressionToken{kind: expressionEQ, text: "=="})
			index += 2
			continue
		case '!':
			if index+1 >= len(source) || source[index+1] != '=' {
				return nil, fmt.Errorf("unsupported !; use not or !=")
			}
			tokens = append(tokens, expressionToken{kind: expressionNE, text: "!="})
			index += 2
			continue
		case '<', '>':
			kind := expressionLT
			if source[index] == '>' {
				kind = expressionGT
			}
			text := source[index : index+1]
			if index+1 < len(source) && source[index+1] == '=' {
				text = source[index : index+2]
				if kind == expressionLT {
					kind = expressionLE
				} else {
					kind = expressionGE
				}
				index++
			}
			tokens = append(tokens, expressionToken{kind: kind, text: text})
			index++
			continue
		case '+', '-', '*':
			kinds := map[byte]expressionTokenKind{'+': expressionPlus, '-': expressionMinus, '*': expressionStar}
			tokens = append(tokens, expressionToken{kind: kinds[source[index]], text: source[index : index+1]})
			index++
			continue
		case '"':
			start := index
			index++
			escaped := false
			for index < len(source) {
				if escaped {
					escaped = false
					index++
					continue
				}
				if source[index] == '\\' {
					escaped = true
					index++
					continue
				}
				if source[index] == '"' {
					index++
					break
				}
				index++
			}
			if index > len(source) || source[index-1] != '"' {
				return nil, fmt.Errorf("unterminated JSON string literal")
			}
			encoded := source[start:index]
			value, err := decodeCanonicalJSONString(encoded)
			if err != nil {
				return nil, fmt.Errorf("invalid string literal %q", encoded)
			}
			tokens = append(tokens, expressionToken{kind: expressionLiteral, text: encoded, literal: semanticir.Literal{Type: semanticir.TypeString, String: value}})
			continue
		}
		if source[index] >= '0' && source[index] <= '9' {
			start := index
			for index < len(source) && source[index] >= '0' && source[index] <= '9' {
				index++
			}
			integer, err := strconv.ParseInt(source[start:index], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("integer literal is outside signed 64-bit range")
			}
			tokens = append(tokens, expressionToken{kind: expressionLiteral, text: source[start:index], literal: semanticir.Literal{Type: semanticir.TypeInteger, Integer: integer}})
			continue
		}
		if unicode.IsLetter(rune(source[index])) || source[index] == '_' {
			start := index
			for index < len(source) && (unicode.IsLetter(rune(source[index])) || unicode.IsDigit(rune(source[index])) || source[index] == '_') {
				index++
			}
			word := source[start:index]
			switch word {
			case "not":
				tokens = append(tokens, expressionToken{kind: expressionNot, text: word})
			case "and":
				tokens = append(tokens, expressionToken{kind: expressionAnd, text: word})
			case "or":
				tokens = append(tokens, expressionToken{kind: expressionOr, text: word})
			case "true":
				tokens = append(tokens, expressionToken{kind: expressionLiteral, text: word, literal: semanticir.Literal{Type: semanticir.TypeBool, Bool: true}})
			case "false":
				tokens = append(tokens, expressionToken{kind: expressionLiteral, text: word, literal: semanticir.Literal{Type: semanticir.TypeBool}})
			case "null":
				tokens = append(tokens, expressionToken{kind: expressionLiteral, text: word, literal: semanticir.Literal{Type: semanticir.TypeOptional, Null: true}})
			default:
				tokens = append(tokens, expressionToken{kind: expressionName, text: word})
			}
			continue
		}
		return nil, fmt.Errorf("unsupported token starting at %q", source[index:])
	}
	return append(tokens, expressionToken{kind: expressionEOF}), nil
}

func (parser *membershipParser) peek() expressionToken { return parser.tokens[parser.position] }
func (parser *membershipParser) take() expressionToken {
	token := parser.peek()
	parser.position++
	return token
}

func (parser *membershipParser) parseOr() (semanticir.Expression, error) {
	left, err := parser.parseAnd()
	for err == nil && parser.peek().kind == expressionOr {
		parser.take()
		var right semanticir.Expression
		right, err = parser.parseAnd()
		if err == nil {
			if left.Type != semanticir.TypeBool || right.Type != semanticir.TypeBool {
				return semanticir.Expression{}, fmt.Errorf("or operands must be bool")
			}
			left = semanticir.Expression{Kind: semanticir.ExprBool, Type: semanticir.TypeBool, Operator: semanticir.OpOr, Operands: []semanticir.Expression{left, right}, Provenance: parser.provenance}
		}
	}
	return left, err
}

func (parser *membershipParser) parseAnd() (semanticir.Expression, error) {
	left, err := parser.parseNot()
	for err == nil && parser.peek().kind == expressionAnd {
		parser.take()
		var right semanticir.Expression
		right, err = parser.parseNot()
		if err == nil {
			if left.Type != semanticir.TypeBool || right.Type != semanticir.TypeBool {
				return semanticir.Expression{}, fmt.Errorf("and operands must be bool")
			}
			left = semanticir.Expression{Kind: semanticir.ExprBool, Type: semanticir.TypeBool, Operator: semanticir.OpAnd, Operands: []semanticir.Expression{left, right}, Provenance: parser.provenance}
		}
	}
	return left, err
}

func (parser *membershipParser) parseNot() (semanticir.Expression, error) {
	if parser.peek().kind == expressionNot {
		parser.take()
		operand, err := parser.parseNot()
		if err != nil {
			return semanticir.Expression{}, err
		}
		if operand.Type != semanticir.TypeBool {
			return semanticir.Expression{}, fmt.Errorf("not operand must be bool")
		}
		return semanticir.Expression{Kind: semanticir.ExprUnary, Type: semanticir.TypeBool, Operator: semanticir.OpNot, Operands: []semanticir.Expression{operand}, Provenance: parser.provenance}, nil
	}
	return parser.parseComparison()
}

func (parser *membershipParser) parseComparison() (semanticir.Expression, error) {
	left, err := parser.parseAdditive()
	if err != nil {
		return semanticir.Expression{}, err
	}
	token := parser.peek()
	operators := map[expressionTokenKind]semanticir.Operator{
		expressionEQ: semanticir.OpEQ, expressionNE: semanticir.OpNE, expressionLT: semanticir.OpLT,
		expressionLE: semanticir.OpLE, expressionGT: semanticir.OpGT, expressionGE: semanticir.OpGE,
	}
	operator, exists := operators[token.kind]
	if !exists {
		return left, nil
	}
	parser.take()
	right, err := parser.parseAdditive()
	if err != nil {
		return semanticir.Expression{}, err
	}
	if left.Type != right.Type {
		return semanticir.Expression{}, fmt.Errorf("comparison mixes %q and %q", left.Type, right.Type)
	}
	if operator != semanticir.OpEQ && operator != semanticir.OpNE && left.Type != semanticir.TypeInteger {
		return semanticir.Expression{}, fmt.Errorf("ordered comparison requires integer operands")
	}
	return semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: operator, Operands: []semanticir.Expression{left, right}, Provenance: parser.provenance}, nil
}

func (parser *membershipParser) parseAdditive() (semanticir.Expression, error) {
	left, err := parser.parseMultiplicative()
	for err == nil && (parser.peek().kind == expressionPlus || parser.peek().kind == expressionMinus) {
		token := parser.take()
		var right semanticir.Expression
		right, err = parser.parseMultiplicative()
		if err == nil {
			if left.Type != semanticir.TypeInteger || right.Type != semanticir.TypeInteger {
				return semanticir.Expression{}, fmt.Errorf("arithmetic operands must be integer")
			}
			operator := semanticir.OpAdd
			if token.kind == expressionMinus {
				operator = semanticir.OpSub
			}
			left = semanticir.Expression{Kind: semanticir.ExprBinary, Type: semanticir.TypeInteger, Operator: operator, Operands: []semanticir.Expression{left, right}, Provenance: parser.provenance}
		}
	}
	return left, err
}

func (parser *membershipParser) parseMultiplicative() (semanticir.Expression, error) {
	left, err := parser.parseArithmeticUnary()
	for err == nil && parser.peek().kind == expressionStar {
		parser.take()
		var right semanticir.Expression
		right, err = parser.parseArithmeticUnary()
		if err == nil {
			if left.Type != semanticir.TypeInteger || right.Type != semanticir.TypeInteger {
				return semanticir.Expression{}, fmt.Errorf("arithmetic operands must be integer")
			}
			left = semanticir.Expression{Kind: semanticir.ExprBinary, Type: semanticir.TypeInteger, Operator: semanticir.OpMul, Operands: []semanticir.Expression{left, right}, Provenance: parser.provenance}
		}
	}
	return left, err
}

func (parser *membershipParser) parseArithmeticUnary() (semanticir.Expression, error) {
	if parser.peek().kind == expressionMinus {
		parser.take()
		operand, err := parser.parseArithmeticUnary()
		if err != nil {
			return semanticir.Expression{}, err
		}
		if operand.Type != semanticir.TypeInteger {
			return semanticir.Expression{}, fmt.Errorf("unary - operand must be integer")
		}
		return semanticir.Expression{Kind: semanticir.ExprUnary, Type: semanticir.TypeInteger, Operator: semanticir.OpNeg, Operands: []semanticir.Expression{operand}, Provenance: parser.provenance}, nil
	}
	return parser.parsePrimary()
}

func (parser *membershipParser) parsePrimary() (semanticir.Expression, error) {
	token := parser.take()
	switch token.kind {
	case expressionName:
		valueType, exists := parser.inputs[token.text]
		if !exists {
			return semanticir.Expression{}, fmt.Errorf("unknown input %q", token.text)
		}
		return semanticir.Expression{Kind: semanticir.ExprVariable, Type: valueType, Name: token.text, Provenance: parser.provenance}, nil
	case expressionLiteral:
		literal := token.literal
		return semanticir.Expression{Kind: semanticir.ExprLiteral, Type: literal.Type, Literal: &literal, Provenance: parser.provenance}, nil
	case expressionLParen:
		expression, err := parser.parseOr()
		if err != nil {
			return semanticir.Expression{}, err
		}
		if parser.peek().kind != expressionRParen {
			return semanticir.Expression{}, fmt.Errorf("missing closing parenthesis")
		}
		parser.take()
		return expression, nil
	default:
		return semanticir.Expression{}, fmt.Errorf("unexpected token %q", token.text)
	}
}

func evaluateBooleanExpression(expression semanticir.Expression, witness map[string]semanticir.Literal) (bool, error) {
	value, err := evaluateGroundingExpression(expression, witness)
	if err != nil {
		return false, err
	}
	if value.Type != semanticir.TypeBool {
		return false, fmt.Errorf("expression did not evaluate to bool")
	}
	return value.Bool, nil
}

func evaluateGroundingExpression(expression semanticir.Expression, witness map[string]semanticir.Literal) (semanticir.Literal, error) {
	switch expression.Kind {
	case semanticir.ExprLiteral:
		return *expression.Literal, nil
	case semanticir.ExprVariable:
		value, exists := witness[expression.Name]
		if !exists {
			return semanticir.Literal{}, fmt.Errorf("witness omits input %q", expression.Name)
		}
		return value, nil
	case semanticir.ExprUnary:
		value, err := evaluateGroundingExpression(expression.Operands[0], witness)
		if err != nil {
			return semanticir.Literal{}, err
		}
		if expression.Operator == semanticir.OpNot && value.Type == semanticir.TypeBool {
			return semanticir.Literal{Type: semanticir.TypeBool, Bool: !value.Bool}, nil
		}
		if expression.Operator == semanticir.OpNeg && value.Type == semanticir.TypeInteger {
			if value.Integer == math.MinInt64 {
				return semanticir.Literal{}, fmt.Errorf("signed 64-bit overflow")
			}
			return semanticir.Literal{Type: semanticir.TypeInteger, Integer: -value.Integer}, nil
		}
		return semanticir.Literal{}, fmt.Errorf("invalid unary operand")
	case semanticir.ExprBinary:
		left, err := evaluateGroundingExpression(expression.Operands[0], witness)
		if err != nil {
			return semanticir.Literal{}, err
		}
		right, err := evaluateGroundingExpression(expression.Operands[1], witness)
		if err != nil {
			return semanticir.Literal{}, err
		}
		if left.Type != semanticir.TypeInteger || right.Type != semanticir.TypeInteger {
			return semanticir.Literal{}, fmt.Errorf("invalid arithmetic operands")
		}
		var value int64
		switch expression.Operator {
		case semanticir.OpAdd:
			if (right.Integer > 0 && left.Integer > math.MaxInt64-right.Integer) || (right.Integer < 0 && left.Integer < math.MinInt64-right.Integer) {
				return semanticir.Literal{}, fmt.Errorf("signed 64-bit overflow")
			}
			value = left.Integer + right.Integer
		case semanticir.OpSub:
			if (right.Integer < 0 && left.Integer > math.MaxInt64+right.Integer) || (right.Integer > 0 && left.Integer < math.MinInt64+right.Integer) {
				return semanticir.Literal{}, fmt.Errorf("signed 64-bit overflow")
			}
			value = left.Integer - right.Integer
		case semanticir.OpMul:
			if left.Integer == 0 || right.Integer == 0 {
				value = 0
				break
			}
			if (left.Integer == math.MinInt64 && right.Integer == -1) || (right.Integer == math.MinInt64 && left.Integer == -1) {
				return semanticir.Literal{}, fmt.Errorf("signed 64-bit overflow")
			}
			value = left.Integer * right.Integer
			if value/right.Integer != left.Integer {
				return semanticir.Literal{}, fmt.Errorf("signed 64-bit overflow")
			}
		default:
			return semanticir.Literal{}, fmt.Errorf("unsupported arithmetic")
		}
		return semanticir.Literal{Type: semanticir.TypeInteger, Integer: value}, nil
	case semanticir.ExprBool:
		left, err := evaluateGroundingExpression(expression.Operands[0], witness)
		if err != nil {
			return semanticir.Literal{}, err
		}
		right, err := evaluateGroundingExpression(expression.Operands[1], witness)
		if err != nil {
			return semanticir.Literal{}, err
		}
		if left.Type != semanticir.TypeBool || right.Type != semanticir.TypeBool {
			return semanticir.Literal{}, fmt.Errorf("invalid boolean operands")
		}
		if expression.Operator == semanticir.OpAnd {
			return semanticir.Literal{Type: semanticir.TypeBool, Bool: left.Bool && right.Bool}, nil
		}
		return semanticir.Literal{Type: semanticir.TypeBool, Bool: left.Bool || right.Bool}, nil
	case semanticir.ExprCompare:
		left, err := evaluateGroundingExpression(expression.Operands[0], witness)
		if err != nil {
			return semanticir.Literal{}, err
		}
		right, err := evaluateGroundingExpression(expression.Operands[1], witness)
		if err != nil {
			return semanticir.Literal{}, err
		}
		var result bool
		switch expression.Operator {
		case semanticir.OpEQ:
			result = reflect.DeepEqual(left, right)
		case semanticir.OpNE:
			result = !reflect.DeepEqual(left, right)
		case semanticir.OpLT:
			result = left.Integer < right.Integer
		case semanticir.OpLE:
			result = left.Integer <= right.Integer
		case semanticir.OpGT:
			result = left.Integer > right.Integer
		case semanticir.OpGE:
			result = left.Integer >= right.Integer
		default:
			return semanticir.Literal{}, fmt.Errorf("unsupported comparison")
		}
		return semanticir.Literal{Type: semanticir.TypeBool, Bool: result}, nil
	default:
		return semanticir.Literal{}, fmt.Errorf("unsupported expression kind %q", expression.Kind)
	}
}
