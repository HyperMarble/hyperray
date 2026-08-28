package semanticir

import (
	"fmt"
	"reflect"
	"sort"
)

const maxConcreteQuantification = 100000

// TestConcreteInputsDigest canonically hashes a set of concrete input maps,
// independent of runner order, and rejects duplicates.
func TestConcreteInputsDigest(values []map[string]Literal) (string, error) {
	type item struct {
		Digest string             `json:"digest"`
		Inputs map[string]Literal `json:"inputs"`
	}
	items := make([]item, 0, len(values))
	seen := map[string]struct{}{}
	for _, inputs := range values {
		for name, literal := range inputs {
			if name == "" || ValidateLiteral(literal) != nil {
				return "", fmt.Errorf("concrete input map contains invalid input %q", name)
			}
		}
		digest, err := Digest(inputs)
		if err != nil {
			return "", err
		}
		if _, duplicate := seen[digest]; duplicate {
			return "", fmt.Errorf("concrete input set repeats %s", digest)
		}
		seen[digest] = struct{}{}
		items = append(items, item{digest, inputs})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Digest < items[j].Digest })
	return Digest(items)
}

func validateTestQuantification(task *Task, model ArtifactModel) []Diagnostic {
	if model.Kind != ArtifactTests || model.TestProjection == nil {
		return nil
	}
	projection := model.TestProjection
	var diagnostics []Diagnostic
	operations := map[string]Operation{}
	for _, operation := range task.Operations {
		operations[operation.ID] = operation
	}
	needed := map[string]BehaviorRef{}
	dependencyInputs := map[string][]map[string]Literal{}
	for _, dependency := range projection.Dependencies {
		key := behaviorKey(dependency.Behavior.OperationID, dependency.Behavior.Conditions)
		needed[key] = dependency.Behavior
		if dependency.Behavior.Inputs == nil || !reflect.DeepEqual(dependency.Inputs, dependency.Behavior.Inputs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test dependency does not identify one exact typed behavior point", dependency.Provenance))
		} else {
			dependencyInputs[key] = append(dependencyInputs[key], dependency.Behavior.Inputs)
		}
	}
	records := map[string]TestQuantificationEvidence{}
	for _, record := range projection.Quantification {
		key := behaviorKey(record.Behavior.OperationID, record.Behavior.Conditions)
		if _, duplicate := records[key]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "test projection repeats quantification for "+key, record.Provenance))
		}
		records[key] = record
		if record.Behavior.Inputs != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test quantification record must name a semantic category, not one concrete point", record.Provenance))
		}
		if validateFactSource(record.Provenance, model.Artifact) != nil || record.Result != ProofProved {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test quantification is not complete compiler-backed evidence", record.Provenance))
		}
		operation, exists := operations[record.Behavior.OperationID]
		if !exists || validateAssignment(record.Behavior.Conditions, operationDomainValues(operation, domainValueRegistry(task.Domains))) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test quantification refers to an unknown semantic behavior", record.Provenance))
			continue
		}
		digest, digestErr := TestConcreteInputsDigest(record.ConcreteInputs)
		if digestErr != nil || record.ConcreteInputsDigest != digest {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "test quantification concrete input set/digest is invalid", record.Provenance))
		}
		switch record.Kind {
		case TestQuantificationSingleton:
			exact, singleton := ExactGroundingInputs(operation, task.Domains, record.Behavior.Conditions)
			if !singleton || len(record.ConcreteInputs) != 1 || !reflect.DeepEqual(record.ConcreteInputs[0], exact) || record.CodeGraphDigest != "" || record.TestGraphDigest != "" {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test singleton quantification is not the exact selected-label input tuple", record.Provenance))
			}
		case TestQuantificationFiniteExhaustive:
			graph, graphDigest, graphErr := operationCompilerGraph(task, operation.ID)
			concrete, enumerationErr := enumerateFiniteCategory(operation, task.Domains, record.Behavior.Conditions, graph)
			if graphErr != nil || enumerationErr != nil || record.CodeGraphDigest != graphDigest || !sameConcreteInputSet(record.ConcreteInputs, concrete) || record.TestGraphDigest != "" {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test finite quantification does not exhaust the compiler-typed category input set", record.Provenance))
			}
		case TestQuantificationUniversalGraph:
			// The graph schema reserves this mode, but acceptance requires a
			// central graph-derived quantified claim. Opaque frontend predicates
			// are deliberately not a fallback.
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, "universal test quantification has no central graph-derived proof record", record.Provenance))
		default:
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, "test quantification kind is unsupported", record.Provenance))
		}
		allowed := concreteInputDigestSet(record.ConcreteInputs)
		for _, inputs := range uniqueConcreteInputs(dependencyInputs[key]) {
			digest, _ := Digest(inputs)
			if _, exists := allowed[digest]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test dependency point lies outside the declared semantic category", record.Provenance))
			}
		}
	}
	for key, behavior := range needed {
		if _, exists := records[key]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "test BehaviorRef has no quantification evidence for "+behaviorKey(behavior.OperationID, behavior.Conditions), behavior.Provenance))
		}
	}
	for key, record := range records {
		if _, exists := needed[key]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "test quantification has no compiler dependency for "+key, record.Provenance))
		}
	}
	return diagnostics
}

// ValidateTestObservationQuantification is the focused, task-aware validation
// boundary used by Test IR composition. It validates exact point identity and
// the category universe without requiring unrelated Task acceptance evidence.
func ValidateTestObservationQuantification(task *Task, model ArtifactModel) []Diagnostic {
	if task == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticInvalidInput, "test quantification task is nil", model.Coverage.Provenance)}
	}
	return validateTestQuantification(task, model)
}

// ConcreteBehaviorPoints returns D, the exact full-N-way reachable input/state
// universe compiled from spec.md. Singleton labels are used directly. A
// non-singleton semantic category is expanded only from each input's explicit
// frozen Universe and filtered by the category membership predicates.
func ConcreteBehaviorPoints(task *Task) ([]BehaviorRef, []Diagnostic) {
	if task == nil {
		return nil, []Diagnostic{errorDiagnostic(DiagnosticInvalidInput, "behavior-point task is nil", Provenance{})}
	}
	operations := map[string]Operation{}
	for _, operation := range task.Operations {
		operations[operation.ID] = operation
	}
	seenCategories := map[string]struct{}{}
	seenPoints := map[string]struct{}{}
	var points []BehaviorRef
	var diagnostics []Diagnostic
	for _, requirement := range task.Requirements {
		categoryKey := behaviorKey(requirement.OperationID, requirement.Conditions)
		if _, duplicate := seenCategories[categoryKey]; duplicate {
			continue
		}
		seenCategories[categoryKey] = struct{}{}
		operation, exists := operations[requirement.OperationID]
		if !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "reachable requirement refers to an unknown operation", requirement.Provenance))
			continue
		}
		concrete, err := ConcreteInputsForCategory(operation, task.Domains, requirement.Conditions)
		if err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticNonFinite, "reachable full-N-way assignment "+categoryKey+" is not a closed finite input/state set: "+err.Error(), requirement.Provenance))
			continue
		}
		for _, inputs := range concrete {
			point := BehaviorRef{OperationID: operation.ID, Conditions: requirement.Conditions, Inputs: inputs, Provenance: requirement.Provenance}
			key := BehaviorRefKey(point)
			if _, duplicate := seenPoints[key]; duplicate {
				continue
			}
			seenPoints[key] = struct{}{}
			points = append(points, point)
		}
	}
	sort.Slice(points, func(i, j int) bool { return BehaviorRefKey(points[i]) < BehaviorRefKey(points[j]) })
	return points, diagnostics
}

// ConcreteInputsForCategory expands one semantic assignment into its exact
// finite runtime tuples without consulting code or tests.
func ConcreteInputsForCategory(operation Operation, domains []Domain, assignment Assignment) ([]map[string]Literal, error) {
	if exact, singleton := ExactGroundingInputs(operation, domains, assignment); singleton {
		return []map[string]Literal{exact}, nil
	}
	if len(operation.Inputs) == 0 {
		if len(operation.DomainIDs) == 0 {
			return []map[string]Literal{{}}, nil
		}
		return nil, fmt.Errorf("zero-input operation has non-singleton semantic labels")
	}
	product := []map[string]Literal{{}}
	for _, input := range operation.Inputs {
		if len(input.Universe) == 0 {
			return nil, fmt.Errorf("input %q has no explicit finite Universe", input.Name)
		}
		if len(product) > maxConcreteQuantification/len(input.Universe) {
			return nil, fmt.Errorf("concrete input product exceeds %d", maxConcreteQuantification)
		}
		var next []map[string]Literal
		for _, existing := range product {
			for _, literal := range input.Universe {
				copy := make(map[string]Literal, len(existing)+1)
				for name, value := range existing {
					copy[name] = value
				}
				copy[input.Name] = literal
				next = append(next, copy)
			}
		}
		product = next
	}
	conjunction, err := GroundingConjunction(operation, domains, assignment, operation.Provenance)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]Literal, 0, len(product))
	for _, inputs := range product {
		match, evaluationErr := EvaluateGroundingMembership(conjunction, inputs)
		if evaluationErr != nil {
			return nil, evaluationErr
		}
		if match {
			result = append(result, inputs)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("category contains no runtime tuple")
	}
	sort.Slice(result, func(i, j int) bool {
		left, _ := Digest(result[i])
		right, _ := Digest(result[j])
		return left < right
	})
	return result, nil
}

func concreteInputDigestSet(values []map[string]Literal) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, inputs := range values {
		digest, _ := Digest(inputs)
		result[digest] = struct{}{}
	}
	return result
}

func domainValueRegistry(domains []Domain) map[string]map[string]struct{} {
	result := map[string]map[string]struct{}{}
	for _, domain := range domains {
		result[domain.ID] = domainValueIDs(domain)
	}
	return result
}

func uniqueConcreteInputs(values []map[string]Literal) []map[string]Literal {
	result := make([]map[string]Literal, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		digest, _ := Digest(value)
		if _, exists := seen[digest]; exists {
			continue
		}
		seen[digest] = struct{}{}
		result = append(result, value)
	}
	return result
}

func sameConcreteInputSet(left, right []map[string]Literal) bool {
	leftDigest, leftErr := TestConcreteInputsDigest(left)
	rightDigest, rightErr := TestConcreteInputsDigest(right)
	return leftErr == nil && rightErr == nil && leftDigest == rightDigest
}

func operationCompilerGraph(task *Task, operationID string) (*CompilerSemanticGraph, string, error) {
	var found *CompilerSemanticGraph
	for _, artifact := range task.Artifacts {
		if artifact.Kind != ArtifactCode {
			continue
		}
		for _, evidence := range artifact.CompilerEvidence {
			if evidence.SemanticGraph == nil {
				continue
			}
			for _, operation := range evidence.SemanticGraph.Operations {
				if operation.OperationID == operationID {
					if found != nil {
						return nil, "", fmt.Errorf("operation %q has multiple compiler semantic graphs", operationID)
					}
					found = evidence.SemanticGraph
				}
			}
		}
	}
	if found == nil {
		return nil, "", fmt.Errorf("operation %q has no compiler semantic graph", operationID)
	}
	digest, err := CompilerSemanticGraphDigest(found)
	return found, digest, err
}

func enumerateFiniteCategory(operation Operation, domains []Domain, assignment Assignment, graph *CompilerSemanticGraph) ([]map[string]Literal, error) {
	if concrete, err := ConcreteInputsForCategory(operation, domains, assignment); err == nil {
		return concrete, nil
	}
	if graph == nil {
		return nil, fmt.Errorf("compiler graph is nil")
	}
	var root *CompilerOperationGraph
	nodes := map[string]CompilerSemanticNode{}
	numeric := map[string]CompilerNumericSemantics{}
	for index := range graph.Operations {
		if graph.Operations[index].OperationID == operation.ID {
			root = &graph.Operations[index]
		}
	}
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}
	for _, item := range graph.Numeric {
		numeric[item.ID] = item
	}
	if root == nil {
		return nil, fmt.Errorf("compiler graph omits operation")
	}
	bindingNodes := map[string]CompilerSemanticNode{}
	for _, binding := range root.Inputs {
		bindingNodes[binding.InputName] = nodes[binding.NodeID]
	}
	values := make([][]Literal, 0, len(operation.Inputs))
	for _, input := range operation.Inputs {
		node := bindingNodes[input.Name]
		var members []Literal
		switch input.Type {
		case TypeBool:
			members = []Literal{{Type: TypeBool}, {Type: TypeBool, Bool: true}}
		case TypeInteger:
			semantics, exists := numeric[node.NumericID]
			if !exists {
				return nil, fmt.Errorf("integer input %q lacks numeric semantics", input.Name)
			}
			lower, upper, ok := finiteIntegerBounds(semantics)
			if !ok || upper < lower || uint64(upper-lower)+1 > maxConcreteQuantification {
				return nil, fmt.Errorf("integer input %q range is not bounded small-finite", input.Name)
			}
			for value := lower; ; value++ {
				members = append(members, Literal{Type: TypeInteger, Integer: value})
				if value == upper {
					break
				}
			}
		default:
			return nil, fmt.Errorf("input %q type %q has no centrally enumerable finite universe", input.Name, input.Type)
		}
		values = append(values, members)
	}
	product := []map[string]Literal{{}}
	for index, input := range operation.Inputs {
		if len(product) > maxConcreteQuantification/len(values[index]) {
			return nil, fmt.Errorf("concrete input product exceeds %d", maxConcreteQuantification)
		}
		var next []map[string]Literal
		for _, existing := range product {
			for _, value := range values[index] {
				copy := map[string]Literal{}
				for name, item := range existing {
					copy[name] = item
				}
				copy[input.Name] = value
				next = append(next, copy)
			}
		}
		product = next
	}
	conjunction, err := GroundingConjunction(operation, domains, assignment, operation.Provenance)
	if err != nil {
		return nil, err
	}
	var result []map[string]Literal
	for _, inputs := range product {
		match, evaluationErr := EvaluateGroundingMembership(conjunction, inputs)
		if evaluationErr != nil {
			return nil, evaluationErr
		}
		if match {
			result = append(result, inputs)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("category has no concrete inputs")
	}
	return result, nil
}

func finiteIntegerBounds(item CompilerNumericSemantics) (int64, int64, bool) {
	if item.Kind == CompilerNumericUnbounded && item.Range == CompilerRangeBounded && item.LowerBound != nil && item.UpperBound != nil {
		return item.LowerBound.Integer, item.UpperBound.Integer, true
	}
	if item.Kind != CompilerNumericBitVector || item.Width > 16 {
		return 0, 0, false
	}
	if item.Signed {
		half := int64(1) << (item.Width - 1)
		return -half, half - 1, true
	}
	return 0, (int64(1) << item.Width) - 1, true
}
