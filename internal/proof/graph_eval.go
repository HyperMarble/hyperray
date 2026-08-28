package proof

import (
	"fmt"
	"math/big"
	"reflect"

	"github.com/HyperMarble/ray/internal/semanticir"
)

type concreteGraphEvaluator struct {
	graph    *semanticir.CompilerSemanticGraph
	root     semanticir.CompilerOperationGraph
	inputs   map[string]semanticir.Literal
	nodes    map[string]semanticir.CompilerSemanticNode
	blocks   map[string]semanticir.CompilerSemanticBlock
	outgoing map[string][]semanticir.CompilerControlEdge
	numeric  map[string]semanticir.CompilerNumericSemantics
	memo     map[string]semanticir.Literal
}

// validateCentralCodePointOutcomes independently evaluates the replayed typed
// compiler graph at every concrete BehaviorCase input. Category-level legacy
// realization SMT is audit evidence only; it cannot justify copying one
// sampled outcome across distinct concrete points.
func (v *validator) validateCentralCodePointOutcomes(artifact *semanticir.ArtifactModel, graph *semanticir.CompilerSemanticGraph) bool {
	start := len(v.blockers)
	operations := make(map[string]semanticir.Operation)
	for _, operation := range v.task.Operations {
		operations[operation.ID] = operation
	}
	for _, behaviorCase := range artifact.Cases {
		operation, exists := operations[behaviorCase.OperationID]
		if !exists || behaviorCase.Inputs == nil || len(behaviorCase.OutcomeIDs) != 1 {
			continue
		}
		raw, err := evaluateCompilerGraphPoint(graph, operation, behaviorCase.Inputs)
		if err != nil {
			v.add("unsupported-concrete-code-derivation", fmt.Sprintf("code case %q: %v", behaviorCase.ID, err), &behaviorCase.Provenance)
			continue
		}
		outcomeID, err := semanticir.ClassifyRawOutcome(operation, raw, behaviorCase.Provenance)
		if err != nil || outcomeID != behaviorCase.OutcomeIDs[0] {
			v.add("stale-concrete-code-point", fmt.Sprintf("code case %q declares %q but central graph evaluation classifies %q: %v", behaviorCase.ID, behaviorCase.OutcomeIDs[0], outcomeID, err), &behaviorCase.Provenance)
		}
	}
	return len(v.blockers) == start
}

func evaluateCompilerGraphPoint(graph *semanticir.CompilerSemanticGraph, operation semanticir.Operation, inputs map[string]semanticir.Literal) (semanticir.RawOutcomeTrace, error) {
	var root *semanticir.CompilerOperationGraph
	evaluator := concreteGraphEvaluator{
		graph: graph, inputs: inputs, nodes: map[string]semanticir.CompilerSemanticNode{}, blocks: map[string]semanticir.CompilerSemanticBlock{},
		outgoing: map[string][]semanticir.CompilerControlEdge{}, numeric: map[string]semanticir.CompilerNumericSemantics{}, memo: map[string]semanticir.Literal{},
	}
	for _, node := range graph.Nodes {
		evaluator.nodes[node.ID] = node
	}
	for _, block := range graph.Blocks {
		evaluator.blocks[block.ID] = block
	}
	for _, edge := range graph.Edges {
		evaluator.outgoing[edge.FromBlockID] = append(evaluator.outgoing[edge.FromBlockID], edge)
	}
	for _, numeric := range graph.Numeric {
		evaluator.numeric[numeric.ID] = numeric
	}
	for index := range graph.Operations {
		if graph.Operations[index].OperationID == operation.ID {
			root = &graph.Operations[index]
		}
	}
	if root == nil {
		return semanticir.RawOutcomeTrace{}, fmt.Errorf("compiler graph omits operation %q", operation.ID)
	}
	evaluator.root = *root
	if len(inputs) != len(operation.Inputs) {
		return semanticir.RawOutcomeTrace{}, fmt.Errorf("input point assigns %d values; want %d", len(inputs), len(operation.Inputs))
	}
	for _, input := range operation.Inputs {
		literal, exists := inputs[input.Name]
		if !exists || literal.Type != input.Type || semanticir.ValidateLiteral(literal) != nil {
			return semanticir.RawOutcomeTrace{}, fmt.Errorf("input point has no valid %q value", input.Name)
		}
	}
	blockID := root.EntryBlockID
	for steps := 0; steps <= len(graph.Blocks); steps++ {
		block, exists := evaluator.blocks[blockID]
		if !exists {
			return semanticir.RawOutcomeTrace{}, fmt.Errorf("control flow enters missing block %q", blockID)
		}
		edges := evaluator.outgoing[blockID]
		if len(edges) == 0 {
			for _, nodeID := range block.NodeIDs {
				node := evaluator.nodes[nodeID]
				if node.Kind == semanticir.CompilerNodeReturn || node.Kind == semanticir.CompilerNodeSuccess || node.Kind == semanticir.CompilerNodeRaise {
					return evaluator.terminal(node)
				}
			}
			return semanticir.RawOutcomeTrace{}, fmt.Errorf("terminal block %q has no terminal node", blockID)
		}
		selected := ""
		for _, edge := range edges {
			matches := edge.GuardNodeID == ""
			if edge.GuardNodeID != "" {
				guard, err := evaluator.value(edge.GuardNodeID)
				if err != nil || guard.Type != semanticir.TypeBool {
					return semanticir.RawOutcomeTrace{}, fmt.Errorf("evaluate edge %q guard: %v", edge.ID, err)
				}
				matches = guard.Bool == edge.GuardValue
			}
			if matches {
				if selected != "" {
					return semanticir.RawOutcomeTrace{}, fmt.Errorf("multiple CFG edges match concrete point in block %q", blockID)
				}
				selected = edge.ToBlockID
			}
		}
		if selected == "" {
			return semanticir.RawOutcomeTrace{}, fmt.Errorf("no CFG edge matches concrete point in block %q", blockID)
		}
		blockID = selected
	}
	return semanticir.RawOutcomeTrace{}, fmt.Errorf("compiler CFG does not terminate within its acyclic block bound")
}

func (e *concreteGraphEvaluator) terminal(node semanticir.CompilerSemanticNode) (semanticir.RawOutcomeTrace, error) {
	trace := semanticir.RawOutcomeTrace{Effects: []semanticir.RawEffectTrace{}}
	switch node.Kind {
	case semanticir.CompilerNodeReturn:
		value, err := e.value(node.Operands[0])
		if err != nil {
			return trace, err
		}
		trace.Kind, trace.Value = semanticir.OutcomeReturn, &value
	case semanticir.CompilerNodeSuccess:
		trace.Kind = semanticir.OutcomeSuccess
	case semanticir.CompilerNodeRaise:
		trace.Kind, trace.ExceptionType, trace.Message = semanticir.OutcomeRaise, node.ExceptionType, node.Message
	default:
		return trace, fmt.Errorf("node %q is not a terminal", node.ID)
	}
	for _, effectID := range node.EffectNodeIDs {
		effect := e.nodes[effectID]
		raw := semanticir.RawEffectTrace{Kind: effect.EffectKind, Target: effect.EffectTarget}
		if len(effect.Operands) == 1 {
			value, err := e.value(effect.Operands[0])
			if err != nil {
				return trace, err
			}
			raw.Value = &value
		}
		trace.Effects = append(trace.Effects, raw)
	}
	return trace, semanticir.ValidateRawOutcomeTrace(trace)
}

func (e *concreteGraphEvaluator) value(id string) (semanticir.Literal, error) {
	if cached, exists := e.memo[id]; exists {
		return cached, nil
	}
	node, exists := e.nodes[id]
	if !exists {
		return semanticir.Literal{}, fmt.Errorf("missing value node %q", id)
	}
	operands := make([]semanticir.Literal, len(node.Operands))
	for index, operandID := range node.Operands {
		value, err := e.value(operandID)
		if err != nil {
			return semanticir.Literal{}, err
		}
		operands[index] = value
	}
	var result semanticir.Literal
	var err error
	switch node.Kind {
	case semanticir.CompilerNodeInput:
		result, exists = e.inputs[node.InputName]
		if !exists {
			err = fmt.Errorf("input node %q has no concrete value", node.ID)
		}
	case semanticir.CompilerNodeConstant:
		result = *node.Literal
	case semanticir.CompilerNodeAdd, semanticir.CompilerNodeSub, semanticir.CompilerNodeMul:
		result, err = e.integerArithmetic(node, operands[0], operands[1])
	case semanticir.CompilerNodeEQ, semanticir.CompilerNodeNE:
		equal := reflect.DeepEqual(operands[0], operands[1])
		if node.Kind == semanticir.CompilerNodeNE {
			equal = !equal
		}
		result = semanticir.Literal{Type: semanticir.TypeBool, Bool: equal}
	case semanticir.CompilerNodeLT, semanticir.CompilerNodeLE, semanticir.CompilerNodeGT, semanticir.CompilerNodeGE:
		var comparison int
		comparison, err = e.integerCompare(node, operands[0], operands[1])
		if err == nil {
			matched := map[semanticir.CompilerSemanticNodeKind]bool{
				semanticir.CompilerNodeLT: comparison < 0, semanticir.CompilerNodeLE: comparison <= 0,
				semanticir.CompilerNodeGT: comparison > 0, semanticir.CompilerNodeGE: comparison >= 0,
			}[node.Kind]
			result = semanticir.Literal{Type: semanticir.TypeBool, Bool: matched}
		}
	case semanticir.CompilerNodeAnd:
		result = semanticir.Literal{Type: semanticir.TypeBool, Bool: operands[0].Bool && operands[1].Bool}
	case semanticir.CompilerNodeOr:
		result = semanticir.Literal{Type: semanticir.TypeBool, Bool: operands[0].Bool || operands[1].Bool}
	case semanticir.CompilerNodeNot:
		result = semanticir.Literal{Type: semanticir.TypeBool, Bool: !operands[0].Bool}
	case semanticir.CompilerNodeSelect:
		if operands[0].Bool {
			result = operands[1]
		} else {
			result = operands[2]
		}
	default:
		err = fmt.Errorf("node %q kind %q is not a concrete value", node.ID, node.Kind)
	}
	if err == nil {
		if result.Type != node.Type || semanticir.ValidateLiteral(result) != nil {
			err = fmt.Errorf("node %q evaluated to invalid type %q, want %q", node.ID, result.Type, node.Type)
		} else {
			e.memo[id] = result
		}
	}
	return result, err
}

func (e *concreteGraphEvaluator) integerArithmetic(node semanticir.CompilerSemanticNode, left, right semanticir.Literal) (semanticir.Literal, error) {
	semantics, exists := e.numeric[node.NumericID]
	if !exists {
		return semanticir.Literal{}, fmt.Errorf("arithmetic node %q has no numeric semantics", node.ID)
	}
	a, b := big.NewInt(left.Integer), big.NewInt(right.Integer)
	value := new(big.Int)
	switch node.Kind {
	case semanticir.CompilerNodeAdd:
		value.Add(a, b)
	case semanticir.CompilerNodeSub:
		value.Sub(a, b)
	case semanticir.CompilerNodeMul:
		value.Mul(a, b)
	}
	if semantics.Kind == semanticir.CompilerNumericUnbounded {
		if !value.IsInt64() {
			return semanticir.Literal{}, fmt.Errorf("unbounded integer node %q result exceeds the closed Literal range", node.ID)
		}
		return semanticir.Literal{Type: semanticir.TypeInteger, Integer: value.Int64()}, nil
	}
	if semantics.Kind != semanticir.CompilerNumericBitVector || semantics.Overflow != semanticir.CompilerOverflowWrap || semantics.Width <= 0 || semantics.Width > 64 {
		return semanticir.Literal{}, fmt.Errorf("arithmetic node %q has unsupported machine overflow semantics", node.ID)
	}
	modulus := new(big.Int).Lsh(big.NewInt(1), uint(semantics.Width))
	value.Mod(value, modulus)
	if semantics.Signed && value.Bit(semantics.Width-1) == 1 {
		value.Sub(value, modulus)
	}
	if !value.IsInt64() {
		return semanticir.Literal{}, fmt.Errorf("machine integer node %q result cannot be represented by Literal", node.ID)
	}
	return semanticir.Literal{Type: semanticir.TypeInteger, Integer: value.Int64()}, nil
}

func (e *concreteGraphEvaluator) integerCompare(node semanticir.CompilerSemanticNode, left, right semanticir.Literal) (int, error) {
	if left.Type != semanticir.TypeInteger || right.Type != semanticir.TypeInteger {
		return 0, fmt.Errorf("ordered comparison %q is not integer-valued", node.ID)
	}
	semantics, exists := e.numeric[e.nodes[node.Operands[0]].NumericID]
	if !exists || semantics.Kind == semanticir.CompilerNumericUnbounded || semantics.Signed {
		return big.NewInt(left.Integer).Cmp(big.NewInt(right.Integer)), nil
	}
	if semantics.Width <= 0 || semantics.Width > 63 {
		return 0, fmt.Errorf("unsigned comparison %q width cannot be represented exactly", node.ID)
	}
	mask := (uint64(1) << uint(semantics.Width)) - 1
	a, b := uint64(left.Integer)&mask, uint64(right.Integer)&mask
	if a < b {
		return -1, nil
	}
	if a > b {
		return 1, nil
	}
	return 0, nil
}
