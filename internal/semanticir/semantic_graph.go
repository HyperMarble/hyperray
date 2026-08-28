package semanticir

import (
	"fmt"
	"reflect"
	"sort"
)

// CompilerNumericKind distinguishes mathematical integers from finite machine
// integers. Central lowering must never silently translate both as SMT Int.
type CompilerNumericKind string

const (
	CompilerNumericUnbounded CompilerNumericKind = "unbounded-integer"
	CompilerNumericBitVector CompilerNumericKind = "bit-vector"
)

type CompilerIntegerRangeKind string

const (
	CompilerRangeAll     CompilerIntegerRangeKind = "all"
	CompilerRangeBounded CompilerIntegerRangeKind = "bounded"
)

type CompilerOverflowBehavior string

const (
	CompilerOverflowUnbounded CompilerOverflowBehavior = "unbounded"
	CompilerOverflowWrap      CompilerOverflowBehavior = "wrap"
	CompilerOverflowPoison    CompilerOverflowBehavior = "poison"
	CompilerOverflowTrap      CompilerOverflowBehavior = "trap"
	CompilerOverflowChecked   CompilerOverflowBehavior = "checked"
)

type CompilerExceptionalBehavior string

const (
	CompilerExceptionalNone   CompilerExceptionalBehavior = "none"
	CompilerExceptionalPoison CompilerExceptionalBehavior = "poison"
	CompilerExceptionalTrap   CompilerExceptionalBehavior = "trap"
)

type CompilerDivisionRounding string

const (
	CompilerDivisionTruncateZero CompilerDivisionRounding = "truncate-zero"
	CompilerDivisionFloor        CompilerDivisionRounding = "floor"
)

// CompilerNumericSemantics freezes the source/compiler meaning of integer
// nodes. A bounded mathematical range is inclusive. Bit-vector range is
// derived from Width/Signed and therefore may not also carry bounds.
type CompilerNumericSemantics struct {
	ID         string                   `json:"id"`
	Kind       CompilerNumericKind      `json:"kind"`
	Width      int                      `json:"width"`
	Signed     bool                     `json:"signed"`
	Overflow   CompilerOverflowBehavior `json:"overflow"`
	Range      CompilerIntegerRangeKind `json:"range"`
	LowerBound *Literal                 `json:"lower_bound,omitempty"`
	UpperBound *Literal                 `json:"upper_bound,omitempty"`
}

type CompilerSemanticNodeKind string

const (
	CompilerNodeInput    CompilerSemanticNodeKind = "input"
	CompilerNodeConstant CompilerSemanticNodeKind = "constant"
	CompilerNodeAdd      CompilerSemanticNodeKind = "add"
	CompilerNodeSub      CompilerSemanticNodeKind = "sub"
	CompilerNodeMul      CompilerSemanticNodeKind = "mul"
	CompilerNodeDiv      CompilerSemanticNodeKind = "div"
	CompilerNodeMod      CompilerSemanticNodeKind = "mod"
	CompilerNodeEQ       CompilerSemanticNodeKind = "eq"
	CompilerNodeNE       CompilerSemanticNodeKind = "ne"
	CompilerNodeLT       CompilerSemanticNodeKind = "lt"
	CompilerNodeLE       CompilerSemanticNodeKind = "le"
	CompilerNodeGT       CompilerSemanticNodeKind = "gt"
	CompilerNodeGE       CompilerSemanticNodeKind = "ge"
	CompilerNodeAnd      CompilerSemanticNodeKind = "and"
	CompilerNodeOr       CompilerSemanticNodeKind = "or"
	CompilerNodeNot      CompilerSemanticNodeKind = "not"
	CompilerNodeSelect   CompilerSemanticNodeKind = "select"
	CompilerNodeReturn   CompilerSemanticNodeKind = "return"
	CompilerNodeSuccess  CompilerSemanticNodeKind = "success"
	CompilerNodeRaise    CompilerSemanticNodeKind = "raise"
	CompilerNodeEffect   CompilerSemanticNodeKind = "effect"
)

// CompilerSemanticNode is the closed data/terminal vocabulary accepted from
// language frontends. Operands are node IDs. Terminal EffectNodeIDs preserve
// the exact observable effect order.
type CompilerSemanticNode struct {
	ID               string                      `json:"id"`
	Kind             CompilerSemanticNodeKind    `json:"kind"`
	Type             ValueType                   `json:"type"`
	NumericID        string                      `json:"numeric_id"`
	InputName        string                      `json:"input_name"`
	Literal          *Literal                    `json:"literal,omitempty"`
	Operands         []string                    `json:"operands"`
	DivisionRounding CompilerDivisionRounding    `json:"division_rounding"`
	DivideByZero     CompilerExceptionalBehavior `json:"divide_by_zero"`
	ExceptionType    string                      `json:"exception_type"`
	Message          string                      `json:"message"`
	EffectKind       EffectKind                  `json:"effect_kind"`
	EffectTarget     string                      `json:"effect_target"`
	EffectNodeIDs    []string                    `json:"effect_node_ids"`
	CompilerNodeIDs  []string                    `json:"compiler_node_ids"`
	Provenance       Provenance                  `json:"provenance"`
}

type CompilerSemanticBlock struct {
	ID              string     `json:"id"`
	NodeIDs         []string   `json:"node_ids"`
	CompilerNodeIDs []string   `json:"compiler_node_ids"`
	Provenance      Provenance `json:"provenance"`
}

// CompilerControlEdge is a CFG edge. GuardNodeID is empty only for an
// unconditional edge. GuardValue selects the true/false branch.
type CompilerControlEdge struct {
	ID              string     `json:"id"`
	FromBlockID     string     `json:"from_block_id"`
	ToBlockID       string     `json:"to_block_id"`
	GuardNodeID     string     `json:"guard_node_id"`
	GuardValue      bool       `json:"guard_value"`
	CompilerNodeIDs []string   `json:"compiler_node_ids"`
	Provenance      Provenance `json:"provenance"`
}

type CompilerInputNode struct {
	InputName string `json:"input_name"`
	NodeID    string `json:"node_id"`
}

type CompilerOperationGraph struct {
	OperationID     string              `json:"operation_id"`
	EntryBlockID    string              `json:"entry_block_id"`
	Inputs          []CompilerInputNode `json:"inputs"`
	TerminalNodeIDs []string            `json:"terminal_node_ids"`
	Provenance      Provenance          `json:"provenance"`
}

type CompilerConstructKind string

const (
	CompilerConstructInput      CompilerConstructKind = "input"
	CompilerConstructConstant   CompilerConstructKind = "constant"
	CompilerConstructArithmetic CompilerConstructKind = "arithmetic"
	CompilerConstructComparison CompilerConstructKind = "comparison"
	CompilerConstructBoolean    CompilerConstructKind = "boolean"
	CompilerConstructControl    CompilerConstructKind = "control"
	CompilerConstructReturn     CompilerConstructKind = "return"
	CompilerConstructRaise      CompilerConstructKind = "raise"
	CompilerConstructEffect     CompilerConstructKind = "effect"
)

// CompilerConstructBinding is the complete decoded compiler-IR inventory.
// Every compiler node must bind to at least one typed semantic node, block, or
// control edge; calls/external operations have no accepted kind and block.
type CompilerConstructBinding struct {
	ID              string                `json:"id"`
	Kind            CompilerConstructKind `json:"kind"`
	Opcode          string                `json:"opcode"`
	SemanticNodeIDs []string              `json:"semantic_node_ids"`
	BlockIDs        []string              `json:"block_ids"`
	EdgeIDs         []string              `json:"edge_ids"`
	Provenance      Provenance            `json:"provenance"`
}

// CompilerSemanticGraph is the compiler-derived behavioral projection that
// central proof lowers itself. IR bytes and the hermetic derivation transcript
// are digest-bound; frontend-authored SMT predicates are not authority.
type CompilerSemanticGraph struct {
	SourceDigest        string                     `json:"source_digest"`
	WorkspaceTreeDigest string                     `json:"workspace_tree_digest"`
	Tool                ToolRef                    `json:"tool"`
	IRKind              CompilerIRKind             `json:"ir_kind"`
	IR                  []byte                     `json:"ir"`
	IRDigest            string                     `json:"ir_digest"`
	Environment         []EnvironmentVariable      `json:"environment"`
	EnvironmentDigest   string                     `json:"environment_digest"`
	DerivationSteps     []ProbeStep                `json:"derivation_steps"`
	DecoderSteps        []ProbeStep                `json:"decoder_steps"`
	DecoderOutput       []byte                     `json:"decoder_output"`
	DecoderOutputDigest string                     `json:"decoder_output_digest"`
	Numeric             []CompilerNumericSemantics `json:"numeric"`
	Nodes               []CompilerSemanticNode     `json:"nodes"`
	Blocks              []CompilerSemanticBlock    `json:"blocks"`
	Edges               []CompilerControlEdge      `json:"edges"`
	Operations          []CompilerOperationGraph   `json:"operations"`
	Constructs          []CompilerConstructBinding `json:"constructs"`
	Provenance          Provenance                 `json:"provenance"`
}

// CompilerDecodedSemantics is the complete typed result emitted by the
// compiler decoder.  The replay transcript must bind this entire value, not
// only the raw construct inventory: Nodes, CFG edges, numeric semantics, and
// operation roots are the behavior that the proof engine evaluates.
type CompilerDecodedSemantics struct {
	Numeric    []CompilerNumericSemantics `json:"numeric"`
	Nodes      []CompilerSemanticNode     `json:"nodes"`
	Blocks     []CompilerSemanticBlock    `json:"blocks"`
	Edges      []CompilerControlEdge      `json:"edges"`
	Operations []CompilerOperationGraph   `json:"operations"`
	Constructs []CompilerConstructBinding `json:"constructs"`
}

// DecodedSemantics returns the exact semantic projection whose canonical JSON
// must be reproduced by the frozen decoder during replay.
func (graph *CompilerSemanticGraph) DecodedSemantics() CompilerDecodedSemantics {
	if graph == nil {
		return CompilerDecodedSemantics{}
	}
	return CompilerDecodedSemantics{
		Numeric: graph.Numeric, Nodes: graph.Nodes, Blocks: graph.Blocks,
		Edges: graph.Edges, Operations: graph.Operations, Constructs: graph.Constructs,
	}
}

// CanonicalCompilerDecoderOutput encodes the complete typed projection used
// by central proof lowering.
func CanonicalCompilerDecoderOutput(graph *CompilerSemanticGraph) ([]byte, error) {
	return CanonicalJSON(graph.DecodedSemantics())
}

// ValidateCompilerSemanticGraph is the shared structural trust boundary used
// by frontends, proof, and certificates.
func ValidateCompilerSemanticGraph(model ArtifactModel, evidence CompilerEvidence) []Diagnostic {
	if evidence.SemanticGraph == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("compiler evidence %q has no typed semantic graph", evidence.ID), evidence.Provenance)}
	}
	graph := evidence.SemanticGraph
	var diagnostics []Diagnostic
	if graph.SourceDigest != evidence.SourceDigest || graph.WorkspaceTreeDigest != evidence.WorkspaceTreeDigest || graph.Tool != evidence.Tool || graph.IRKind != evidence.IRKind || graph.IRDigest != evidence.EmittedIRDigest || graph.IRDigest != DigestBytes(graph.IR) || len(graph.IR) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticStaleArtifact, "compiler semantic graph source/workspace/tool/IR binding differs from evidence", graph.Provenance))
	}
	if graph.EnvironmentDigest != evidence.EnvironmentDigest || validateExactEnvironment(graph.Environment, graph.EnvironmentDigest) != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "compiler semantic graph environment is not the exact evidence environment", graph.Provenance))
	}
	diagnostics = append(diagnostics, ValidateProbeSteps(graph.DerivationSteps, graph.Provenance)...)
	derivationBound := false
	for _, step := range graph.DerivationSteps {
		derivationBound = derivationBound || (step.Kind == ProbeStepRun && step.ExpectedStdoutDigest == graph.IRDigest)
	}
	if !derivationBound {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "compiler derivation transcript output does not equal exact graph IR bytes", graph.Provenance))
	}
	canonicalSemantics, semanticsErr := CanonicalCompilerDecoderOutput(graph)
	diagnostics = append(diagnostics, ValidateProbeSteps(graph.DecoderSteps, graph.Provenance)...)
	decoderBound := false
	for _, step := range graph.DecoderSteps {
		decoderBound = decoderBound || (step.Kind == ProbeStepRun && step.ExpectedStdoutDigest == graph.DecoderOutputDigest)
	}
	if semanticsErr != nil || !reflect.DeepEqual(canonicalSemantics, graph.DecoderOutput) || graph.DecoderOutputDigest != DigestBytes(graph.DecoderOutput) || !decoderBound {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "compiler decoder transcript is not the canonical complete typed semantic graph", graph.Provenance))
	}
	if validateFactSource(graph.Provenance, model.Artifact) != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "compiler semantic graph is not anchored to modeled source", graph.Provenance))
	}

	numeric := map[string]CompilerNumericSemantics{}
	for _, item := range graph.Numeric {
		if item.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "compiler numeric semantics ID is empty", graph.Provenance))
		} else if _, duplicate := numeric[item.ID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "compiler repeats numeric semantics "+item.ID, graph.Provenance))
		}
		numeric[item.ID] = item
		diagnostics = append(diagnostics, validateCompilerNumeric(item, graph.Provenance)...)
	}

	nodes := map[string]CompilerSemanticNode{}
	for _, node := range graph.Nodes {
		if node.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "compiler semantic node ID is empty", node.Provenance))
		} else if _, duplicate := nodes[node.ID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "compiler repeats semantic node "+node.ID, node.Provenance))
		}
		nodes[node.ID] = node
		if validateFactSource(node.Provenance, model.Artifact) != nil || len(node.CompilerNodeIDs) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "compiler semantic node lacks source/compiler binding", node.Provenance))
		}
	}
	for _, node := range graph.Nodes {
		diagnostics = append(diagnostics, validateCompilerSemanticNode(node, nodes, numeric)...)
	}
	diagnostics = append(diagnostics, validateNodeDAG(graph.Nodes)...)

	blocks := map[string]CompilerSemanticBlock{}
	nodeOwner := map[string]string{}
	for _, block := range graph.Blocks {
		if block.ID == "" || len(block.NodeIDs) == 0 || validateFactSource(block.Provenance, model.Artifact) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "compiler block lacks ID/nodes/source binding", block.Provenance))
		}
		if _, duplicate := blocks[block.ID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "compiler repeats block "+block.ID, block.Provenance))
		}
		blocks[block.ID] = block
		for _, nodeID := range block.NodeIDs {
			if _, exists := nodes[nodeID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("block %q refers to unknown node %q", block.ID, nodeID), block.Provenance))
			} else if owner, duplicate := nodeOwner[nodeID]; duplicate {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("node %q belongs to blocks %q and %q", nodeID, owner, block.ID), block.Provenance))
			}
			nodeOwner[nodeID] = block.ID
		}
	}
	for nodeID, node := range nodes {
		if _, exists := nodeOwner[nodeID]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("semantic node %q belongs to no block", nodeID), node.Provenance))
		}
	}

	edges := map[string]CompilerControlEdge{}
	outgoing := map[string][]CompilerControlEdge{}
	for _, edge := range graph.Edges {
		if edge.ID == "" || edge.FromBlockID == edge.ToBlockID || blocks[edge.FromBlockID].ID == "" || blocks[edge.ToBlockID].ID == "" || validateFactSource(edge.Provenance, model.Artifact) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "compiler control edge has invalid ID/block/source binding", edge.Provenance))
		}
		if _, duplicate := edges[edge.ID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "compiler repeats control edge "+edge.ID, edge.Provenance))
		}
		edges[edge.ID] = edge
		outgoing[edge.FromBlockID] = append(outgoing[edge.FromBlockID], edge)
		if edge.GuardNodeID != "" {
			guard, exists := nodes[edge.GuardNodeID]
			if !exists || guard.Type != TypeBool {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("edge %q guard is not a boolean node", edge.ID), edge.Provenance))
			}
		}
	}
	diagnostics = append(diagnostics, validateControlDAG(blocks, outgoing, graph.Provenance)...)
	diagnostics = append(diagnostics, validateCompilerOperations(model, graph, nodes, blocks, outgoing)...)
	diagnostics = append(diagnostics, validateCompilerConstructs(model, evidence, graph, nodes, blocks, edges)...)
	return diagnostics
}

func validateCompilerNumeric(item CompilerNumericSemantics, provenance Provenance) []Diagnostic {
	var diagnostics []Diagnostic
	switch item.Kind {
	case CompilerNumericUnbounded:
		if item.Width != 0 || !item.Signed || item.Overflow != CompilerOverflowUnbounded || (item.Range != CompilerRangeAll && item.Range != CompilerRangeBounded) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "unbounded numeric semantics has machine-width/overflow/range fields", provenance))
		}
		if item.Range == CompilerRangeBounded {
			if item.LowerBound == nil || item.UpperBound == nil || item.LowerBound.Type != TypeInteger || item.UpperBound.Type != TypeInteger || item.LowerBound.Integer > item.UpperBound.Integer {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "bounded integer semantics has invalid inclusive bounds", provenance))
			}
		} else if item.LowerBound != nil || item.UpperBound != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "unbounded range carries explicit bounds", provenance))
		}
	case CompilerNumericBitVector:
		if item.Width < 1 || item.Width > 64 || item.Range != CompilerRangeAll || item.LowerBound != nil || item.UpperBound != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "bit-vector numeric semantics has invalid width/range", provenance))
		}
		switch item.Overflow {
		case CompilerOverflowWrap, CompilerOverflowPoison, CompilerOverflowTrap, CompilerOverflowChecked:
		default:
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "bit-vector numeric semantics lacks explicit overflow behavior", provenance))
		}
	default:
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, "compiler numeric kind is unsupported", provenance))
	}
	return diagnostics
}

func validateCompilerSemanticNode(node CompilerSemanticNode, nodes map[string]CompilerSemanticNode, numeric map[string]CompilerNumericSemantics) []Diagnostic {
	var diagnostics []Diagnostic
	operands := make([]CompilerSemanticNode, 0, len(node.Operands))
	for _, operandID := range node.Operands {
		operand, exists := nodes[operandID]
		if !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("semantic node %q refers to unknown operand %q", node.ID, operandID), node.Provenance))
		}
		operands = append(operands, operand)
	}
	integerNode := node.Type == TypeInteger
	if integerNode {
		if _, exists := numeric[node.NumericID]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("integer node %q has unknown numeric semantics %q", node.ID, node.NumericID), node.Provenance))
		}
	} else if node.NumericID != "" {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("non-integer node %q carries numeric semantics", node.ID), node.Provenance))
	}
	invalid := func(message string) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("semantic node %q %s", node.ID, message), node.Provenance))
	}
	switch node.Kind {
	case CompilerNodeInput:
		if node.InputName == "" || len(node.Operands) != 0 || node.Literal != nil {
			invalid("has invalid input fields")
		}
	case CompilerNodeConstant:
		if node.Literal == nil || len(node.Operands) != 0 || node.Type != node.Literal.Type || ValidateLiteral(*node.Literal) != nil {
			invalid("has invalid constant fields")
		}
	case CompilerNodeAdd, CompilerNodeSub, CompilerNodeMul:
		if len(operands) != 2 || !integerNode || operands[0].Type != TypeInteger || operands[1].Type != TypeInteger || operands[0].NumericID != node.NumericID || operands[1].NumericID != node.NumericID {
			invalid("arithmetic operands/result do not share integer numeric semantics")
		}
	case CompilerNodeDiv, CompilerNodeMod:
		if len(operands) != 2 || !integerNode || operands[0].Type != TypeInteger || operands[1].Type != TypeInteger || operands[0].NumericID != node.NumericID || operands[1].NumericID != node.NumericID || (node.DivisionRounding != CompilerDivisionTruncateZero && node.DivisionRounding != CompilerDivisionFloor) || (node.DivideByZero != CompilerExceptionalPoison && node.DivideByZero != CompilerExceptionalTrap) {
			invalid("division/modulo semantics are incomplete or type-incompatible")
		}
	case CompilerNodeEQ, CompilerNodeNE, CompilerNodeLT, CompilerNodeLE, CompilerNodeGT, CompilerNodeGE:
		if len(operands) != 2 || node.Type != TypeBool || operands[0].Type != operands[1].Type || (operands[0].Type == TypeInteger && operands[0].NumericID != operands[1].NumericID) || (node.Kind != CompilerNodeEQ && node.Kind != CompilerNodeNE && operands[0].Type != TypeInteger) {
			invalid("comparison operands/result are type-incompatible")
		}
	case CompilerNodeAnd, CompilerNodeOr:
		if len(operands) != 2 || node.Type != TypeBool || operands[0].Type != TypeBool || operands[1].Type != TypeBool {
			invalid("boolean operands/result are not bool")
		}
	case CompilerNodeNot:
		if len(operands) != 1 || node.Type != TypeBool || operands[0].Type != TypeBool {
			invalid("not operand/result is not bool")
		}
	case CompilerNodeSelect:
		if len(operands) != 3 || operands[0].Type != TypeBool || operands[1].Type != node.Type || operands[2].Type != node.Type || (node.Type == TypeInteger && (operands[1].NumericID != node.NumericID || operands[2].NumericID != node.NumericID)) {
			invalid("select condition/branches are type-incompatible")
		}
	case CompilerNodeReturn:
		if len(operands) != 1 || operands[0].Type != node.Type || (node.Type == TypeInteger && operands[0].NumericID != node.NumericID) || node.ExceptionType != "" || node.EffectKind != "" {
			invalid("return terminal has invalid fields")
		}
	case CompilerNodeSuccess:
		if len(operands) != 0 || node.Type != TypeUnit || node.ExceptionType != "" || node.EffectKind != "" {
			invalid("success terminal has invalid fields")
		}
	case CompilerNodeRaise:
		if len(operands) != 0 || node.Type != TypeUnit || node.ExceptionType == "" || node.EffectKind != "" {
			invalid("raise terminal has invalid fields")
		}
	case CompilerNodeEffect:
		if len(operands) > 1 || node.Type != TypeUnit || node.EffectTarget == "" || len(node.EffectNodeIDs) != 0 {
			invalid("effect node has invalid fields")
		}
		switch node.EffectKind {
		case EffectRead, EffectWrite, EffectCall, EffectOutput:
		default:
			invalid("has unsupported effect kind")
		}
	default:
		invalid("has unsupported kind")
	}
	if node.Kind != CompilerNodeDiv && node.Kind != CompilerNodeMod && (node.DivisionRounding != "" || node.DivideByZero != "") {
		invalid("carries division-only semantics")
	}
	if node.Kind != CompilerNodeReturn && node.Kind != CompilerNodeSuccess && node.Kind != CompilerNodeRaise && len(node.EffectNodeIDs) != 0 {
		invalid("carries terminal-only ordered effects")
	}
	for _, effectID := range node.EffectNodeIDs {
		if effect, exists := nodes[effectID]; !exists || effect.Kind != CompilerNodeEffect {
			invalid("refers to a non-effect ordered effect node")
		}
	}
	return diagnostics
}

func validateNodeDAG(nodes []CompilerSemanticNode) []Diagnostic {
	byID := map[string]CompilerSemanticNode{}
	for _, node := range nodes {
		byID[node.ID] = node
	}
	state := map[string]uint8{}
	var diagnostics []Diagnostic
	var visit func(string)
	visit = func(id string) {
		if state[id] == 1 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, "compiler semantic data graph contains a cycle at "+id, byID[id].Provenance))
			return
		}
		if state[id] == 2 {
			return
		}
		state[id] = 1
		for _, operand := range byID[id].Operands {
			if _, ok := byID[operand]; ok {
				visit(operand)
			}
		}
		state[id] = 2
	}
	for id := range byID {
		visit(id)
	}
	return diagnostics
}

func validateControlDAG(blocks map[string]CompilerSemanticBlock, outgoing map[string][]CompilerControlEdge, provenance Provenance) []Diagnostic {
	state := map[string]uint8{}
	var diagnostics []Diagnostic
	var visit func(string)
	visit = func(id string) {
		if state[id] == 1 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, "compiler control graph contains a cycle at "+id, blocks[id].Provenance))
			return
		}
		if state[id] == 2 {
			return
		}
		state[id] = 1
		for _, edge := range outgoing[id] {
			visit(edge.ToBlockID)
		}
		state[id] = 2
	}
	for id := range blocks {
		visit(id)
	}
	return diagnostics
}

func validateCompilerOperations(model ArtifactModel, graph *CompilerSemanticGraph, nodes map[string]CompilerSemanticNode, blocks map[string]CompilerSemanticBlock, outgoing map[string][]CompilerControlEdge) []Diagnostic {
	var diagnostics []Diagnostic
	declared := map[string]Operation{}
	for _, operation := range model.Operations {
		if operation.Kind != OperationTest {
			declared[operation.ID] = operation
		}
	}
	seen := map[string]struct{}{}
	blockOwner := map[string]string{}
	for _, root := range graph.Operations {
		operation, exists := declared[root.OperationID]
		if !exists || blocks[root.EntryBlockID].ID == "" || validateFactSource(root.Provenance, model.Artifact) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "compiler operation graph has invalid operation/entry/source binding", root.Provenance))
			continue
		}
		if _, duplicate := seen[root.OperationID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "compiler repeats operation graph "+root.OperationID, root.Provenance))
		}
		seen[root.OperationID] = struct{}{}
		inputs := map[string]string{}
		for _, binding := range root.Inputs {
			node, ok := nodes[binding.NodeID]
			if !ok || node.Kind != CompilerNodeInput || node.InputName != binding.InputName {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "compiler operation input binding is invalid", root.Provenance))
			}
			if _, duplicate := inputs[binding.InputName]; duplicate {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "compiler repeats operation input "+binding.InputName, root.Provenance))
			}
			inputs[binding.InputName] = binding.NodeID
		}
		if len(inputs) != len(operation.Inputs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "compiler operation input mapping is incomplete", root.Provenance))
		}
		for _, input := range operation.Inputs {
			node := nodes[inputs[input.Name]]
			if node.ID == "" || node.Type != input.Type {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "compiler operation input type differs from frozen spec input "+input.Name, root.Provenance))
			}
		}
		reachable := map[string]struct{}{}
		var walk func(string)
		walk = func(blockID string) {
			if _, ok := reachable[blockID]; ok {
				return
			}
			reachable[blockID] = struct{}{}
			for _, edge := range outgoing[blockID] {
				walk(edge.ToBlockID)
			}
		}
		walk(root.EntryBlockID)
		for blockID := range reachable {
			if owner, shared := blockOwner[blockID]; shared {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("compiler block %q is shared by operations %q and %q", blockID, owner, root.OperationID), root.Provenance))
			}
			blockOwner[blockID] = root.OperationID
		}
		diagnostics = append(diagnostics, validateOperationCFG(root, reachable, nodes, blocks, outgoing)...)
		terminals := map[string]struct{}{}
		for blockID := range reachable {
			terminalCount := 0
			for _, nodeID := range blocks[blockID].NodeIDs {
				switch nodes[nodeID].Kind {
				case CompilerNodeReturn, CompilerNodeSuccess, CompilerNodeRaise:
					terminals[nodeID] = struct{}{}
					terminalCount++
				}
			}
			if len(outgoing[blockID]) == 0 && terminalCount != 1 {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "compiler terminal block must contain exactly one terminal", blocks[blockID].Provenance))
			}
			if len(outgoing[blockID]) != 0 && terminalCount != 0 {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "compiler nonterminal block contains a terminal", blocks[blockID].Provenance))
			}
		}
		want := map[string]struct{}{}
		for _, id := range root.TerminalNodeIDs {
			want[id] = struct{}{}
		}
		if !reflect.DeepEqual(terminals, want) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "compiler operation terminal inventory differs from reachable CFG terminals", root.Provenance))
		}
	}
	if len(seen) != len(declared) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "compiler semantic graph does not cover every modeled operation", graph.Provenance))
	}
	if len(blockOwner) != len(blocks) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "compiler semantic graph contains blocks outside every operation root", graph.Provenance))
	}
	return diagnostics
}

func validateOperationCFG(root CompilerOperationGraph, reachable map[string]struct{}, nodes map[string]CompilerSemanticNode, blocks map[string]CompilerSemanticBlock, outgoing map[string][]CompilerControlEdge) []Diagnostic {
	var diagnostics []Diagnostic
	// Dominators over the acyclic reachable CFG.
	dominators := map[string]map[string]struct{}{}
	all := map[string]struct{}{}
	for blockID := range reachable {
		all[blockID] = struct{}{}
	}
	for blockID := range reachable {
		if blockID == root.EntryBlockID {
			dominators[blockID] = map[string]struct{}{blockID: {}}
		} else {
			copy := map[string]struct{}{}
			for id := range all {
				copy[id] = struct{}{}
			}
			dominators[blockID] = copy
		}
	}
	predecessors := map[string][]string{}
	for from := range reachable {
		for _, edge := range outgoing[from] {
			if _, ok := reachable[edge.ToBlockID]; ok {
				predecessors[edge.ToBlockID] = append(predecessors[edge.ToBlockID], from)
			}
		}
	}
	changed := true
	for changed {
		changed = false
		for blockID := range reachable {
			if blockID == root.EntryBlockID {
				continue
			}
			next := map[string]struct{}{blockID: {}}
			if len(predecessors[blockID]) > 0 {
				for id := range dominators[predecessors[blockID][0]] {
					next[id] = struct{}{}
				}
				for _, pred := range predecessors[blockID][1:] {
					for id := range next {
						if id == blockID {
							continue
						}
						if _, ok := dominators[pred][id]; !ok {
							delete(next, id)
						}
					}
				}
			}
			if !reflect.DeepEqual(next, dominators[blockID]) {
				dominators[blockID] = next
				changed = true
			}
		}
	}
	nodeBlock := map[string]string{}
	nodeIndex := map[string]int{}
	for blockID := range reachable {
		for index, nodeID := range blocks[blockID].NodeIDs {
			nodeBlock[nodeID] = blockID
			nodeIndex[nodeID] = index
		}
	}
	dominatesUse := func(definition, useBlock string, useIndex int) bool {
		defBlock, ok := nodeBlock[definition]
		if !ok {
			return false
		}
		if defBlock == useBlock {
			return nodeIndex[definition] < useIndex
		}
		_, ok = dominators[useBlock][defBlock]
		return ok
	}
	for blockID := range reachable {
		block := blocks[blockID]
		for index, nodeID := range block.NodeIDs {
			for _, operand := range nodes[nodeID].Operands {
				if !dominatesUse(operand, blockID, index) {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("operand %q does not dominate use %q", operand, nodeID), nodes[nodeID].Provenance))
				}
			}
		}
		edges := outgoing[blockID]
		terminal := false
		for _, nodeID := range block.NodeIDs {
			switch nodes[nodeID].Kind {
			case CompilerNodeReturn, CompilerNodeSuccess, CompilerNodeRaise:
				terminal = true
			}
		}
		if terminal {
			continue
		}
		validShape := false
		if len(edges) == 1 && edges[0].GuardNodeID == "" {
			validShape = true
		}
		if len(edges) == 2 && edges[0].GuardNodeID != "" && edges[0].GuardNodeID == edges[1].GuardNodeID && edges[0].GuardValue != edges[1].GuardValue && edges[0].ToBlockID != edges[1].ToBlockID {
			validShape = true
		}
		if !validShape {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "nonterminal compiler block lacks one unconditional edge or one complementary boolean branch", block.Provenance))
		}
		for _, edge := range edges {
			if edge.GuardNodeID != "" && !dominatesUse(edge.GuardNodeID, blockID, len(block.NodeIDs)) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "compiler branch guard does not dominate its edge", edge.Provenance))
			}
		}
	}
	return diagnostics
}

func validateCompilerConstructs(model ArtifactModel, evidence CompilerEvidence, graph *CompilerSemanticGraph, nodes map[string]CompilerSemanticNode, blocks map[string]CompilerSemanticBlock, edges map[string]CompilerControlEdge) []Diagnostic {
	var diagnostics []Diagnostic
	seen := map[string]struct{}{}
	for _, construct := range graph.Constructs {
		if construct.ID == "" || construct.Opcode == "" || validateFactSource(construct.Provenance, model.Artifact) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "compiler construct binding lacks ID/opcode/source", construct.Provenance))
		}
		if _, duplicate := seen[construct.ID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "compiler repeats construct "+construct.ID, construct.Provenance))
		}
		seen[construct.ID] = struct{}{}
		switch construct.Kind {
		case CompilerConstructInput, CompilerConstructConstant, CompilerConstructArithmetic, CompilerConstructComparison, CompilerConstructBoolean, CompilerConstructControl, CompilerConstructReturn, CompilerConstructRaise, CompilerConstructEffect:
		default:
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, "compiler construct has unsupported/unclassified kind", construct.Provenance))
		}
		switch construct.Opcode {
		case "input", "constant", "add", "sub", "mul", "div", "mod", "eq", "ne", "lt", "le", "gt", "ge", "and", "or", "not", "select", "branch", "jump", "return", "success", "raise", "effect":
		default:
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, "compiler construct has unknown normalized opcode "+construct.Opcode, construct.Provenance))
		}
		if len(construct.SemanticNodeIDs)+len(construct.BlockIDs)+len(construct.EdgeIDs) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "compiler construct is not bound into semantic graph", construct.Provenance))
		}
		for _, id := range construct.SemanticNodeIDs {
			if _, ok := nodes[id]; !ok {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "compiler construct refers to unknown semantic node "+id, construct.Provenance))
			}
		}
		for _, id := range construct.BlockIDs {
			if _, ok := blocks[id]; !ok {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "compiler construct refers to unknown block "+id, construct.Provenance))
			}
		}
		for _, id := range construct.EdgeIDs {
			if _, ok := edges[id]; !ok {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "compiler construct refers to unknown edge "+id, construct.Provenance))
			}
		}
	}
	if len(graph.Constructs) == 0 || len(graph.Constructs) != evidence.TotalConstructs || evidence.TranslatedConstructs != evidence.TotalConstructs {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "compiler construct inventory is not exact and complete", graph.Provenance))
	}
	// Every frontend-declared compiler-node binding must occur in the exact
	// decoded inventory, preventing graph nodes from citing omitted constructs.
	for _, node := range graph.Nodes {
		for _, id := range node.CompilerNodeIDs {
			if _, ok := seen[id]; !ok {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "semantic node cites compiler construct outside inventory "+id, node.Provenance))
			}
		}
	}
	for _, block := range graph.Blocks {
		for _, id := range block.CompilerNodeIDs {
			if _, ok := seen[id]; !ok {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "block cites compiler construct outside inventory "+id, block.Provenance))
			}
		}
	}
	for _, edge := range graph.Edges {
		for _, id := range edge.CompilerNodeIDs {
			if _, ok := seen[id]; !ok {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "edge cites compiler construct outside inventory "+id, edge.Provenance))
			}
		}
	}
	return diagnostics
}

// CompilerSemanticGraphDigest binds the typed graph used by central lowering.
func CompilerSemanticGraphDigest(graph *CompilerSemanticGraph) (string, error) {
	if graph == nil {
		return "", fmt.Errorf("compiler semantic graph is nil")
	}
	return Digest(graph)
}

// SortedCompilerOperationIDs is a small integration helper for proof/cert.
func SortedCompilerOperationIDs(graph *CompilerSemanticGraph) []string {
	if graph == nil {
		return nil
	}
	ids := make([]string, 0, len(graph.Operations))
	for _, operation := range graph.Operations {
		ids = append(ids, operation.OperationID)
	}
	sort.Strings(ids)
	return ids
}
