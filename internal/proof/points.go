package proof

import (
	"fmt"
	"sort"

	"github.com/HyperMarble/ray/internal/semanticir"
)

const maxConcretePointsPerCategory = 100000

// concreteInputsForCategory returns the complete finite set of concrete
// operation inputs denoted by one semantic assignment. Exact groundings are
// singletons. Non-singleton categories are enumerated only from the typed
// compiler graph's closed input universe; an unbounded or unsupported input
// sort is proof-blocking rather than represented by one sampled witness.
func concreteInputsForCategory(task *semanticir.Task, operation semanticir.Operation, assignment semanticir.Assignment) ([]map[string]semanticir.Literal, error) {
	if concrete, err := semanticir.ConcreteInputsForCategory(operation, task.Domains, assignment); err == nil {
		result := make([]map[string]semanticir.Literal, len(concrete))
		for index := range concrete {
			result[index] = cloneInputs(concrete[index])
		}
		return result, nil
	}
	graph, err := uniqueOperationSemanticGraph(task, operation.ID)
	if err != nil {
		return nil, err
	}
	return enumerateConcreteCategory(task, operation, assignment, graph)
}

func uniqueOperationSemanticGraph(task *semanticir.Task, operationID string) (*semanticir.CompilerSemanticGraph, error) {
	var found *semanticir.CompilerSemanticGraph
	for artifactIndex := range task.Artifacts {
		artifact := &task.Artifacts[artifactIndex]
		if artifact.Kind != semanticir.ArtifactCode {
			continue
		}
		for evidenceIndex := range artifact.CompilerEvidence {
			graph := artifact.CompilerEvidence[evidenceIndex].SemanticGraph
			if graph == nil {
				continue
			}
			for _, root := range graph.Operations {
				if root.OperationID != operationID {
					continue
				}
				if found != nil {
					return nil, fmt.Errorf("operation %q has multiple compiler semantic graphs", operationID)
				}
				found = graph
			}
		}
	}
	if found == nil {
		return nil, fmt.Errorf("semantic category for operation %q is not singleton and has no typed compiler graph finite universe", operationID)
	}
	return found, nil
}

func enumerateConcreteCategory(task *semanticir.Task, operation semanticir.Operation, assignment semanticir.Assignment, graph *semanticir.CompilerSemanticGraph) ([]map[string]semanticir.Literal, error) {
	var root *semanticir.CompilerOperationGraph
	nodes := make(map[string]semanticir.CompilerSemanticNode, len(graph.Nodes))
	numeric := make(map[string]semanticir.CompilerNumericSemantics, len(graph.Numeric))
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
		return nil, fmt.Errorf("compiler graph omits operation %q", operation.ID)
	}
	bindingNodes := make(map[string]semanticir.CompilerSemanticNode, len(root.Inputs))
	for _, binding := range root.Inputs {
		node, exists := nodes[binding.NodeID]
		if !exists {
			return nil, fmt.Errorf("compiler graph input %q refers to missing node %q", binding.InputName, binding.NodeID)
		}
		bindingNodes[binding.InputName] = node
	}
	values := make([][]semanticir.Literal, len(operation.Inputs))
	for index, input := range operation.Inputs {
		node, exists := bindingNodes[input.Name]
		if !exists || node.Type != input.Type {
			return nil, fmt.Errorf("compiler graph has no type-exact input node for %q", input.Name)
		}
		switch input.Type {
		case semanticir.TypeBool:
			values[index] = []semanticir.Literal{{Type: semanticir.TypeBool}, {Type: semanticir.TypeBool, Bool: true}}
		case semanticir.TypeInteger:
			semantics, exists := numeric[node.NumericID]
			if !exists {
				return nil, fmt.Errorf("compiler graph integer input %q lacks numeric semantics", input.Name)
			}
			lower, upper, finite := concreteIntegerBounds(semantics)
			if !finite || upper < lower || uint64(upper-lower)+1 > maxConcretePointsPerCategory {
				return nil, fmt.Errorf("compiler graph input %q does not have an exhaustively enumerable finite integer universe", input.Name)
			}
			for value := lower; ; value++ {
				values[index] = append(values[index], semanticir.Literal{Type: semanticir.TypeInteger, Integer: value})
				if value == upper {
					break
				}
			}
		default:
			return nil, fmt.Errorf("compiler graph input %q type %q has no finite central enumeration", input.Name, input.Type)
		}
	}
	product := []map[string]semanticir.Literal{{}}
	for index, input := range operation.Inputs {
		if len(values[index]) == 0 || len(product) > maxConcretePointsPerCategory/len(values[index]) {
			return nil, fmt.Errorf("concrete input universe for operation %q exceeds %d points", operation.ID, maxConcretePointsPerCategory)
		}
		next := make([]map[string]semanticir.Literal, 0, len(product)*len(values[index]))
		for _, existing := range product {
			for _, literal := range values[index] {
				candidate := cloneInputs(existing)
				candidate[input.Name] = literal
				next = append(next, candidate)
			}
		}
		product = next
	}
	conjunction, err := semanticir.GroundingConjunction(operation, task.Domains, assignment, operation.Provenance)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]semanticir.Literal, 0, len(product))
	for _, inputs := range product {
		matches, evaluationErr := semanticir.EvaluateGroundingMembership(conjunction, inputs)
		if evaluationErr != nil {
			return nil, fmt.Errorf("evaluate concrete category membership: %w", evaluationErr)
		}
		if matches {
			result = append(result, inputs)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("semantic category for operation %q has no concrete input point", operation.ID)
	}
	sort.Slice(result, func(i, j int) bool {
		left, _ := semanticir.Digest(result[i])
		right, _ := semanticir.Digest(result[j])
		return left < right
	})
	return result, nil
}

func concreteIntegerBounds(item semanticir.CompilerNumericSemantics) (int64, int64, bool) {
	if item.Kind == semanticir.CompilerNumericUnbounded && item.Range == semanticir.CompilerRangeBounded && item.LowerBound != nil && item.UpperBound != nil {
		return item.LowerBound.Integer, item.UpperBound.Integer, true
	}
	if item.Kind != semanticir.CompilerNumericBitVector || item.Width <= 0 || item.Width > 16 {
		return 0, 0, false
	}
	if item.Signed {
		half := int64(1) << (item.Width - 1)
		return -half, half - 1, true
	}
	return 0, (int64(1) << item.Width) - 1, true
}

func cloneInputs(inputs map[string]semanticir.Literal) map[string]semanticir.Literal {
	if inputs == nil {
		return nil
	}
	result := make(map[string]semanticir.Literal, len(inputs))
	for name, literal := range inputs {
		result[name] = literal
	}
	return result
}

func inputPointKey(inputs map[string]semanticir.Literal) string {
	if inputs == nil {
		return ""
	}
	digest, err := semanticir.Digest(inputs)
	if err != nil {
		return "invalid-inputs"
	}
	return digest
}

func concreteCaseKey(operationID string, domainIDs []string, assignment semanticir.Assignment, inputs map[string]semanticir.Literal) caseKey {
	return caseKey{operation: operationID, assignment: canonicalAssignment(domainIDs, assignment), inputs: inputPointKey(inputs)}
}

func finiteCaseKey(model *finiteModel, value finiteCase) caseKey {
	return concreteCaseKey(value.operation, model.operationDomains[value.operation], value.conditions, value.inputs)
}
