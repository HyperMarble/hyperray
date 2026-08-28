package proof

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/semanticir"
)

// centralCompilerPredicates are reconstructed from the typed semantic graph.
// Legacy frontend-authored CompilerPredicate bytes are accepted only when
// they are byte-identical to these values.
type centralCompilerPredicates struct {
	scopes      map[string]semanticir.CompilerPredicate
	memberships map[string]semanticir.CompilerPredicate
	outcomes    map[string]semanticir.CompilerOutcomePredicate
}

type graphOperationLowerer struct {
	evidence     *semanticir.CompilerEvidence
	graph        *semanticir.CompilerSemanticGraph
	operation    semanticir.Operation
	root         semanticir.CompilerOperationGraph
	nodes        map[string]semanticir.CompilerSemanticNode
	blocks       map[string]semanticir.CompilerSemanticBlock
	outgoing     map[string][]semanticir.CompilerControlEdge
	numeric      map[string]semanticir.CompilerNumericSemantics
	inputByName  map[string]string
	inputSymbols map[string]string
	declarations []byte
	compilerIDs  []string
	nodeMemo     map[string]graphSMTValue
	nodeErr      map[string]error
}

type graphSMTValue struct {
	expression string
	typeID     semanticir.ValueType
	numericID  string
}

type graphTerminalPath struct {
	condition string
	node      semanticir.CompilerSemanticNode
}

func (v *validator) validateCentralCompilerGraph(artifact *semanticir.ArtifactModel, evidence *semanticir.CompilerEvidence) *centralCompilerPredicates {
	startBlockers := len(v.blockers)
	if evidence.SemanticGraph == nil {
		v.add("opaque-compiler-derivation", fmt.Sprintf("compiler evidence %q has no typed compiler semantic graph", evidence.ID), &evidence.Provenance)
		return nil
	}
	for _, diagnostic := range semanticir.ValidateCompilerSemanticGraph(*artifact, *evidence) {
		if diagnostic.Severity == semanticir.SeverityError {
			provenance := diagnostic.Provenance
			v.add("invalid-compiler-semantic-graph", fmt.Sprintf("%s: %s", diagnostic.Code, diagnostic.Message), &provenance)
		}
	}
	graphDigest, digestErr := semanticir.CompilerSemanticGraphDigest(evidence.SemanticGraph)
	if digestErr != nil || evidence.FormulaDerivationDigest != graphDigest {
		v.add("stale-compiler-semantic-graph", "compiler formula derivation digest is not the exact typed semantic graph digest", &evidence.Provenance)
	}
	if !v.environmentHasTool(evidence.SemanticGraph.Tool) || !v.environmentHasSnapshot(evidence.SemanticGraph.WorkspaceTreeDigest, evidence.SemanticGraph.EnvironmentDigest) {
		v.add("stale-compiler-semantic-graph", "compiler semantic graph tool/workspace/environment is not frozen in the task environment", &evidence.SemanticGraph.Provenance)
	}
	// Structure alone is not execution evidence. Until executor exposes a
	// derivation-only isolated replay, this binding is checked again below and
	// proof remains fail-closed if no step directly binds the exact IR bytes.
	outputBound := false
	for _, step := range evidence.SemanticGraph.DerivationSteps {
		if step.ExpectedStdoutDigest == evidence.SemanticGraph.IRDigest {
			outputBound = true
		}
		for _, output := range step.Outputs {
			if output.AfterDigest == evidence.SemanticGraph.IRDigest {
				outputBound = true
			}
		}
	}
	if !outputBound {
		v.add("unbound-compiler-semantic-graph", "compiler semantic graph IR bytes are not an exact derivation-step output", &evidence.SemanticGraph.Provenance)
	}
	if len(v.blockers) != startBlockers {
		return nil
	}
	central, err := lowerCompilerSemanticGraph(v.task, artifact, evidence)
	if err != nil {
		v.add("unsupported-compiler-semantic-graph", err.Error(), &evidence.SemanticGraph.Provenance)
		return nil
	}
	if !v.replayCentralCompilerGraph(artifact, evidence.SemanticGraph) {
		return nil
	}
	if artifact.Kind == semanticir.ArtifactCode && !v.validateCentralCodePointOutcomes(artifact, evidence.SemanticGraph) {
		return nil
	}
	return central
}

func (v *validator) replayCentralCompilerGraph(artifact *semanticir.ArtifactModel, graph *semanticir.CompilerSemanticGraph) bool {
	if artifact == nil || graph == nil || v.task.Environment == nil {
		v.add("compiler-derivation-replay-failed", "compiler semantic graph replay lacks artifact, graph, or frozen environment", nil)
		return false
	}
	workspaceID := ""
	for _, command := range v.task.Environment.Commands {
		if command.State == semanticir.WorkspaceSolutionNewTests && command.TreeDigest == graph.WorkspaceTreeDigest && command.EnvironmentDigest == graph.EnvironmentDigest {
			workspaceID = command.WorkspaceID
			break
		}
	}
	if workspaceID == "" {
		v.add("compiler-derivation-replay-failed", "compiler semantic graph matches no frozen solution+new-tests workspace/environment", &graph.Provenance)
		return false
	}
	workingDirectory := ""
	for _, steps := range [][]semanticir.ProbeStep{graph.DerivationSteps, graph.DecoderSteps} {
		for _, step := range steps {
			if step.WorkingDirectory != "" {
				workingDirectory = step.WorkingDirectory
				break
			}
		}
		if workingDirectory != "" {
			break
		}
	}
	root, err := exhaustiveWorkspaceRoot(workingDirectory, artifact.Artifact, graph.WorkspaceTreeDigest)
	if err != nil {
		v.add("compiler-derivation-replay-failed", err.Error(), &graph.Provenance)
		return false
	}
	plan := executor.DerivationReplayPlan{
		ID: "proof-compiler-" + artifact.Artifact.ID,
		Workspace: executor.ProbeWorkspace{
			ID: workspaceID, Root: root, State: semanticir.WorkspaceSolutionNewTests,
			TreeSHA256: graph.WorkspaceTreeDigest,
		},
		SourceArtifacts: []semanticir.ArtifactRef{artifact.Artifact},
		Graph:           *graph,
	}
	replayed := executor.ReplayDerivation(v.ctx, plan)
	if err := executor.ValidateDerivationReplay(replayed); err != nil {
		detail := err.Error()
		if len(replayed.Blockers) != 0 {
			detail = replayed.Blockers[0].Code + ": " + replayed.Blockers[0].Detail
		}
		v.add("compiler-derivation-replay-failed", detail, &graph.Provenance)
		return false
	}
	v.derivationReplays = append(v.derivationReplays, DerivationReplayBinding{
		PlanSHA256: replayed.PlanSHA256, GraphSHA256: replayed.GraphSHA256, WorkspaceSHA256: replayed.WorkspaceSHA256,
		SourceBindings: append([]executor.BindingEvidence(nil), replayed.SourceBindings...), ToolBindings: append([]executor.BindingEvidence(nil), replayed.ToolBindings...),
		IRSHA256: graph.IRDigest, DecoderOutputSHA256: graph.DecoderOutputDigest, Repetitions: len(replayed.Runs),
		Deterministic: replayed.Deterministic, OriginalWorkspaceIntact: replayed.OriginalWorkspaceIntact,
	})
	return true
}

func (v *validator) validateCentralCompilerPredicates(evidence *semanticir.CompilerEvidence, central *centralCompilerPredicates) {
	if central == nil {
		return
	}
	for _, scope := range evidence.OperationScopes {
		wanted, exists := central.scopes[scope.OperationID]
		if !exists || !compilerPredicatesEqual(scope.ScopePredicate, wanted) || scope.ScopePredicateDigest != wanted.FormulaDigest {
			v.add("noncanonical-compiler-scope", fmt.Sprintf("operation scope %q was not centrally lowered from the typed compiler semantic graph", scope.OperationID), &scope.Provenance)
		}
	}
	for _, partition := range evidence.Partitions {
		wantedScope, exists := central.scopes[partition.OperationID]
		if !exists || !compilerPredicatesEqual(partition.ScopePredicate, wantedScope) || partition.ScopePredicateDigest != wantedScope.FormulaDigest {
			v.add("noncanonical-compiler-partition", fmt.Sprintf("partition %q/%q scope was not centrally lowered from the typed compiler semantic graph", partition.OperationID, partition.DomainID), &partition.Provenance)
		}
		for _, label := range partition.Labels {
			wanted, exists := central.memberships[compilerMembershipKey(partition.OperationID, partition.DomainID, label.ValueID)]
			if !exists || !compilerPredicatesEqual(label.MembershipPredicate, wanted) || label.PredicateDigest != wanted.FormulaDigest {
				v.add("noncanonical-compiler-membership", fmt.Sprintf("partition %q/%q label %q was not centrally lowered from its frozen membership axiom", partition.OperationID, partition.DomainID, label.ValueID), &label.Provenance)
			}
		}
	}
	for _, closure := range evidence.OutcomeClosures {
		for _, declared := range closure.Declared {
			wanted, exists := central.outcomes[compilerOutcomeKey(closure.OperationID, declared.OutcomeID)]
			if !exists || declared.OutcomeID != wanted.OutcomeID || !compilerPredicatesEqual(declared.Predicate, wanted.Predicate) {
				v.add("noncanonical-compiler-outcome", fmt.Sprintf("operation %q outcome %q was not centrally lowered from terminal/effect paths", closure.OperationID, declared.OutcomeID), &closure.Provenance)
			}
		}
		for _, complement := range closure.Complements {
			wanted, exists := central.outcomes[compilerOutcomeKey(closure.OperationID, complement.ID)]
			if !exists || complement.Predicate.OutcomeID != wanted.OutcomeID || !compilerPredicatesEqual(complement.Predicate.Predicate, wanted.Predicate) {
				v.add("noncanonical-compiler-outcome", fmt.Sprintf("operation %q other outcome was not centrally lowered as the exact terminal/effect complement", closure.OperationID), &closure.Provenance)
			}
		}
	}
	for _, behavior := range evidence.BehaviorProofs {
		for _, claimed := range behavior.RealizationProof.Claim.Outcomes {
			wanted, exists := central.outcomes[compilerOutcomeKey(behavior.Behavior.OperationID, claimed.OutcomeID)]
			if !exists || !compilerPredicatesEqual(claimed.Predicate, wanted.Predicate) {
				v.add("noncanonical-compiler-behavior-proof", fmt.Sprintf("behavior case %q outcome %q was not centrally lowered from the typed compiler semantic graph", behavior.BehaviorCaseID, claimed.OutcomeID), &behavior.Provenance)
			}
		}
	}
}

func compilerPredicatesEqual(left, right semanticir.CompilerPredicate) bool {
	return left.Logic == right.Logic && string(left.Declarations) == string(right.Declarations) && left.DeclarationsDigest == right.DeclarationsDigest && string(left.Formula) == string(right.Formula) && left.FormulaDigest == right.FormulaDigest && left.Tool == right.Tool && left.IRDigest == right.IRDigest && sameStringSetProof(left.CompilerNodeIDs, right.CompilerNodeIDs)
}

func sameStringSetProof(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	wanted := map[string]int{}
	for _, value := range left {
		wanted[value]++
	}
	for _, value := range right {
		wanted[value]--
		if wanted[value] < 0 {
			return false
		}
	}
	for _, count := range wanted {
		if count != 0 {
			return false
		}
	}
	return true
}

func lowerCompilerSemanticGraph(task *semanticir.Task, artifact *semanticir.ArtifactModel, evidence *semanticir.CompilerEvidence) (*centralCompilerPredicates, error) {
	graph := evidence.SemanticGraph
	result := &centralCompilerPredicates{scopes: map[string]semanticir.CompilerPredicate{}, memberships: map[string]semanticir.CompilerPredicate{}, outcomes: map[string]semanticir.CompilerOutcomePredicate{}}
	operations := map[string]semanticir.Operation{}
	for _, operation := range artifact.Operations {
		if operation.Kind != semanticir.OperationTest {
			operations[operation.ID] = operation
		}
	}
	for _, root := range graph.Operations {
		operation, exists := operations[root.OperationID]
		if !exists {
			return nil, fmt.Errorf("semantic graph operation %q is outside the code model", root.OperationID)
		}
		lowerer, err := newGraphOperationLowerer(evidence, operation, root)
		if err != nil {
			return nil, err
		}
		scopeFormula, err := lowerer.scopeFormula()
		if err != nil {
			return nil, fmt.Errorf("operation %q scope: %w", operation.ID, err)
		}
		result.scopes[operation.ID] = lowerer.predicate(scopeFormula, lowerer.compilerIDs)
		for _, domainID := range operation.DomainIDs {
			domain, exists := taskDomain(task, domainID)
			if !exists {
				return nil, fmt.Errorf("operation %q refers to missing domain %q", operation.ID, domainID)
			}
			for _, domainValue := range domain.Values {
				grounding, ok := domainValue.GroundingFor(operation.ID)
				if !ok || grounding.Membership == nil {
					return nil, fmt.Errorf("operation %q domain %q label %q has no unique membership grounding", operation.ID, domainID, domainValue.ID)
				}
				formula, err := lowerer.groundingExpression(*grounding.Membership)
				if err != nil {
					return nil, fmt.Errorf("operation %q domain %q label %q: %w", operation.ID, domainID, domainValue.ID, err)
				}
				result.memberships[compilerMembershipKey(operation.ID, domainID, domainValue.ID)] = lowerer.predicate(formula.expression, lowerer.compilerIDs)
			}
		}
		paths, err := lowerer.terminalPaths()
		if err != nil {
			return nil, fmt.Errorf("operation %q CFG: %w", operation.ID, err)
		}
		var named []semanticir.ObservableOutcome
		otherID := semanticir.OtherOutcome(operation.ID, graph.Provenance).ID
		for _, outcomeID := range operation.OutcomeIDs {
			if outcomeID == otherID {
				continue
			}
			outcome, exists := taskOutcome(task, outcomeID)
			if !exists {
				return nil, fmt.Errorf("operation %q refers to missing outcome %q", operation.ID, outcomeID)
			}
			named = append(named, outcome)
			formula, err := lowerer.outcomeFormula(paths, outcome)
			if err != nil {
				return nil, fmt.Errorf("operation %q outcome %q: %w", operation.ID, outcomeID, err)
			}
			predicate := lowerer.predicate(formula, lowerer.compilerIDs)
			result.outcomes[compilerOutcomeKey(operation.ID, outcomeID)] = semanticir.CompilerOutcomePredicate{OutcomeID: outcomeID, Predicate: predicate}
		}
		otherFormula, err := lowerer.otherOutcomeFormula(paths, named)
		if err != nil {
			return nil, fmt.Errorf("operation %q other outcome: %w", operation.ID, err)
		}
		otherPredicate := lowerer.predicate(otherFormula, lowerer.compilerIDs)
		result.outcomes[compilerOutcomeKey(operation.ID, otherID)] = semanticir.CompilerOutcomePredicate{OutcomeID: otherID, Predicate: otherPredicate}
	}
	return result, nil
}

func newGraphOperationLowerer(evidence *semanticir.CompilerEvidence, operation semanticir.Operation, root semanticir.CompilerOperationGraph) (*graphOperationLowerer, error) {
	graph := evidence.SemanticGraph
	value := &graphOperationLowerer{
		evidence: evidence, graph: graph, operation: operation, root: root,
		nodes: map[string]semanticir.CompilerSemanticNode{}, blocks: map[string]semanticir.CompilerSemanticBlock{}, outgoing: map[string][]semanticir.CompilerControlEdge{}, numeric: map[string]semanticir.CompilerNumericSemantics{},
		inputByName: map[string]string{}, inputSymbols: map[string]string{}, nodeMemo: map[string]graphSMTValue{}, nodeErr: map[string]error{},
	}
	for _, item := range graph.Numeric {
		value.numeric[item.ID] = item
		if item.Kind == semanticir.CompilerNumericBitVector && item.Overflow != semanticir.CompilerOverflowWrap {
			return nil, fmt.Errorf("numeric semantics %q uses %q overflow; exceptional arithmetic paths are not represented by the current graph", item.ID, item.Overflow)
		}
	}
	for _, node := range graph.Nodes {
		value.nodes[node.ID] = node
		if node.Kind == semanticir.CompilerNodeDiv || node.Kind == semanticir.CompilerNodeMod {
			return nil, fmt.Errorf("node %q has divide-by-zero exceptional semantics without an explicit exceptional CFG edge", node.ID)
		}
	}
	for _, block := range graph.Blocks {
		value.blocks[block.ID] = block
	}
	for _, edge := range graph.Edges {
		value.outgoing[edge.FromBlockID] = append(value.outgoing[edge.FromBlockID], edge)
	}
	for _, input := range root.Inputs {
		value.inputByName[input.InputName] = input.NodeID
		value.inputSymbols[input.NodeID] = graphSymbol("input", operation.ID+"\x00"+input.NodeID)
	}
	for _, construct := range graph.Constructs {
		value.compilerIDs = append(value.compilerIDs, construct.ID)
	}
	sort.Strings(value.compilerIDs)
	var declarations strings.Builder
	inputs := append([]semanticir.CompilerInputNode(nil), root.Inputs...)
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].NodeID < inputs[j].NodeID })
	for _, input := range inputs {
		node := value.nodes[input.NodeID]
		sortName, err := value.smtSort(node.Type, node.NumericID)
		if err != nil {
			return nil, fmt.Errorf("input %q: %w", input.InputName, err)
		}
		fmt.Fprintf(&declarations, "(declare-const %s %s)\n", value.inputSymbols[input.NodeID], sortName)
	}
	value.declarations = []byte(strings.TrimSuffix(declarations.String(), "\n"))
	return value, nil
}

func (l *graphOperationLowerer) predicate(formula string, compilerIDs []string) semanticir.CompilerPredicate {
	ids := append([]string(nil), compilerIDs...)
	sort.Strings(ids)
	return semanticir.CompilerPredicate{
		Logic: semanticir.ProofLogicSMTLIB2, Declarations: append([]byte(nil), l.declarations...), DeclarationsDigest: semanticir.DigestBytes(l.declarations),
		Formula: []byte(formula), FormulaDigest: semanticir.DigestBytes([]byte(formula)), Tool: l.evidence.Tool, IRDigest: l.graph.IRDigest, CompilerNodeIDs: ids,
	}
}

func (l *graphOperationLowerer) scopeFormula() (string, error) {
	clauses := []string{}
	for _, input := range l.root.Inputs {
		node := l.nodes[input.NodeID]
		if node.Type != semanticir.TypeInteger {
			continue
		}
		numeric := l.numeric[node.NumericID]
		if numeric.Kind == semanticir.CompilerNumericUnbounded && numeric.Range == semanticir.CompilerRangeBounded {
			clauses = append(clauses, fmt.Sprintf("(<= %d %s)", numeric.LowerBound.Integer, l.inputSymbols[node.ID]), fmt.Sprintf("(<= %s %d)", l.inputSymbols[node.ID], numeric.UpperBound.Integer))
		}
	}
	return graphAnd(clauses), nil
}

func (l *graphOperationLowerer) smtSort(typeID semanticir.ValueType, numericID string) (string, error) {
	switch typeID {
	case semanticir.TypeBool:
		return "Bool", nil
	case semanticir.TypeString:
		return "String", nil
	case semanticir.TypeInteger:
		numeric, exists := l.numeric[numericID]
		if !exists {
			return "", fmt.Errorf("missing numeric semantics %q", numericID)
		}
		if numeric.Kind == semanticir.CompilerNumericUnbounded {
			return "Int", nil
		}
		if numeric.Kind == semanticir.CompilerNumericBitVector {
			return fmt.Sprintf("(_ BitVec %d)", numeric.Width), nil
		}
	}
	return "", fmt.Errorf("unsupported compiler graph SMT type %q", typeID)
}

func (l *graphOperationLowerer) node(id string) (graphSMTValue, error) {
	if cached, exists := l.nodeMemo[id]; exists {
		return cached, l.nodeErr[id]
	}
	node, exists := l.nodes[id]
	if !exists {
		return graphSMTValue{}, fmt.Errorf("unknown node %q", id)
	}
	var result graphSMTValue
	var err error
	operands := make([]graphSMTValue, len(node.Operands))
	for index, operandID := range node.Operands {
		operands[index], err = l.node(operandID)
		if err != nil {
			break
		}
	}
	if err == nil {
		switch node.Kind {
		case semanticir.CompilerNodeInput:
			symbol, allowed := l.inputSymbols[node.ID]
			if !allowed {
				err = fmt.Errorf("operation %q expression depends on unbound input node %q", l.operation.ID, node.ID)
			} else {
				result = graphSMTValue{symbol, node.Type, node.NumericID}
			}
		case semanticir.CompilerNodeConstant:
			var expression string
			expression, err = l.literal(*node.Literal, node.NumericID)
			result = graphSMTValue{expression, node.Type, node.NumericID}
		case semanticir.CompilerNodeAdd, semanticir.CompilerNodeSub, semanticir.CompilerNodeMul:
			operator := map[semanticir.CompilerSemanticNodeKind][2]string{
				semanticir.CompilerNodeAdd: {"+", "bvadd"}, semanticir.CompilerNodeSub: {"-", "bvsub"}, semanticir.CompilerNodeMul: {"*", "bvmul"},
			}[node.Kind]
			numeric := l.numeric[node.NumericID]
			op := operator[0]
			if numeric.Kind == semanticir.CompilerNumericBitVector {
				op = operator[1]
			}
			result = graphSMTValue{fmt.Sprintf("(%s %s %s)", op, operands[0].expression, operands[1].expression), node.Type, node.NumericID}
		case semanticir.CompilerNodeEQ, semanticir.CompilerNodeNE, semanticir.CompilerNodeLT, semanticir.CompilerNodeLE, semanticir.CompilerNodeGT, semanticir.CompilerNodeGE:
			op, opErr := l.comparisonOperator(node.Kind, operands[0])
			if opErr != nil {
				err = opErr
				break
			}
			expression := fmt.Sprintf("(%s %s %s)", op, operands[0].expression, operands[1].expression)
			if node.Kind == semanticir.CompilerNodeNE {
				expression = "(not " + fmt.Sprintf("(= %s %s)", operands[0].expression, operands[1].expression) + ")"
			}
			result = graphSMTValue{expression, semanticir.TypeBool, ""}
		case semanticir.CompilerNodeAnd, semanticir.CompilerNodeOr:
			op := "and"
			if node.Kind == semanticir.CompilerNodeOr {
				op = "or"
			}
			result = graphSMTValue{fmt.Sprintf("(%s %s %s)", op, operands[0].expression, operands[1].expression), semanticir.TypeBool, ""}
		case semanticir.CompilerNodeNot:
			result = graphSMTValue{"(not " + operands[0].expression + ")", semanticir.TypeBool, ""}
		case semanticir.CompilerNodeSelect:
			result = graphSMTValue{fmt.Sprintf("(ite %s %s %s)", operands[0].expression, operands[1].expression, operands[2].expression), node.Type, node.NumericID}
		default:
			err = fmt.Errorf("node %q kind %q is not a value expression", node.ID, node.Kind)
		}
	}
	l.nodeMemo[id], l.nodeErr[id] = result, err
	return result, err
}

func (l *graphOperationLowerer) comparisonOperator(kind semanticir.CompilerSemanticNodeKind, operand graphSMTValue) (string, error) {
	if kind == semanticir.CompilerNodeEQ || kind == semanticir.CompilerNodeNE {
		return "=", nil
	}
	if operand.typeID == semanticir.TypeString || operand.typeID == semanticir.TypeBool {
		return "", fmt.Errorf("ordered comparison has unsupported type %q", operand.typeID)
	}
	numeric := l.numeric[operand.numericID]
	if numeric.Kind == semanticir.CompilerNumericUnbounded {
		return map[semanticir.CompilerSemanticNodeKind]string{semanticir.CompilerNodeLT: "<", semanticir.CompilerNodeLE: "<=", semanticir.CompilerNodeGT: ">", semanticir.CompilerNodeGE: ">="}[kind], nil
	}
	prefix := "bvult"
	if numeric.Signed {
		prefix = "bvslt"
	}
	return map[semanticir.CompilerSemanticNodeKind]string{
		semanticir.CompilerNodeLT: prefix,
		semanticir.CompilerNodeLE: strings.Replace(prefix, "lt", "le", 1),
		semanticir.CompilerNodeGT: strings.Replace(prefix, "lt", "gt", 1),
		semanticir.CompilerNodeGE: strings.Replace(prefix, "lt", "ge", 1),
	}[kind], nil
}

func (l *graphOperationLowerer) literal(literal semanticir.Literal, numericID string) (string, error) {
	switch literal.Type {
	case semanticir.TypeBool:
		if literal.Bool {
			return "true", nil
		}
		return "false", nil
	case semanticir.TypeString:
		if !utf8.ValidString(literal.String) {
			return "", fmt.Errorf("string literal is not UTF-8")
		}
		return `"` + strings.ReplaceAll(literal.String, `"`, `""`) + `"`, nil
	case semanticir.TypeInteger:
		numeric, exists := l.numeric[numericID]
		if !exists {
			return "", fmt.Errorf("integer literal has no numeric semantics")
		}
		if numeric.Kind == semanticir.CompilerNumericUnbounded {
			return strconv.FormatInt(literal.Integer, 10), nil
		}
		modulus := uint64(1)
		if numeric.Width == 64 {
			modulus = 0
		} else {
			modulus <<= uint(numeric.Width)
		}
		bits := uint64(literal.Integer)
		if modulus != 0 {
			bits %= modulus
		}
		return fmt.Sprintf("(_ bv%d %d)", bits, numeric.Width), nil
	default:
		return "", fmt.Errorf("unsupported literal type %q", literal.Type)
	}
}

func (l *graphOperationLowerer) groundingExpression(expression semanticir.Expression) (graphSMTValue, error) {
	switch expression.Kind {
	case semanticir.ExprLiteral:
		if expression.Literal == nil {
			return graphSMTValue{}, fmt.Errorf("literal has no value")
		}
		if expression.Type == semanticir.TypeInteger {
			// Numeric ID is inferred by the arithmetic/comparison consumer.
			return graphSMTValue{"", expression.Type, ""}, nil
		}
		value, err := l.literal(*expression.Literal, "")
		return graphSMTValue{value, expression.Type, ""}, err
	case semanticir.ExprVariable:
		nodeID, exists := l.inputByName[expression.Name]
		if !exists {
			return graphSMTValue{}, fmt.Errorf("grounding refers to unknown operation input %q", expression.Name)
		}
		return l.node(nodeID)
	case semanticir.ExprBool:
		if len(expression.Operands) < 2 {
			return graphSMTValue{}, fmt.Errorf("boolean grounding has insufficient operands")
		}
		children := make([]string, len(expression.Operands))
		for index, operand := range expression.Operands {
			value, err := l.groundingExpression(operand)
			if err != nil {
				return graphSMTValue{}, err
			}
			children[index] = value.expression
		}
		op := "and"
		if expression.Operator == semanticir.OpOr {
			op = "or"
		}
		return graphSMTValue{"(" + op + " " + strings.Join(children, " ") + ")", semanticir.TypeBool, ""}, nil
	case semanticir.ExprUnary:
		if len(expression.Operands) != 1 || (expression.Operator != semanticir.OpNot && expression.Operator != semanticir.OpNeg) {
			return graphSMTValue{}, fmt.Errorf("unsupported unary grounding")
		}
		value, err := l.groundingExpression(expression.Operands[0])
		if err != nil {
			return graphSMTValue{}, err
		}
		if expression.Operator == semanticir.OpNot {
			return graphSMTValue{"(not " + value.expression + ")", semanticir.TypeBool, ""}, nil
		}
		if value.expression == "" {
			return graphSMTValue{}, fmt.Errorf("integer negation has no inferable numeric semantics")
		}
		op := "-"
		if l.numeric[value.numericID].Kind == semanticir.CompilerNumericBitVector {
			op = "bvneg"
		}
		return graphSMTValue{"(" + op + " " + value.expression + ")", semanticir.TypeInteger, value.numericID}, nil
	case semanticir.ExprBinary:
		if len(expression.Operands) != 2 || (expression.Operator != semanticir.OpAdd && expression.Operator != semanticir.OpSub && expression.Operator != semanticir.OpMul) {
			return graphSMTValue{}, fmt.Errorf("unsupported arithmetic grounding")
		}
		left, err := l.groundingExpression(expression.Operands[0])
		if err != nil {
			return graphSMTValue{}, err
		}
		right, err := l.groundingExpression(expression.Operands[1])
		if err != nil {
			return graphSMTValue{}, err
		}
		numericID := left.numericID
		if numericID == "" {
			numericID = right.numericID
		}
		if numericID == "" {
			return graphSMTValue{}, fmt.Errorf("arithmetic grounding has no inferable numeric semantics")
		}
		if left.expression == "" {
			left.expression, err = l.literal(*expression.Operands[0].Literal, numericID)
		}
		if right.expression == "" {
			right.expression, err = l.literal(*expression.Operands[1].Literal, numericID)
		}
		if err != nil || (left.numericID != "" && left.numericID != numericID) || (right.numericID != "" && right.numericID != numericID) {
			return graphSMTValue{}, fmt.Errorf("arithmetic grounding mixes numeric semantics: %v", err)
		}
		op := map[semanticir.Operator]string{semanticir.OpAdd: "+", semanticir.OpSub: "-", semanticir.OpMul: "*"}[expression.Operator]
		if l.numeric[numericID].Kind == semanticir.CompilerNumericBitVector {
			op = map[semanticir.Operator]string{semanticir.OpAdd: "bvadd", semanticir.OpSub: "bvsub", semanticir.OpMul: "bvmul"}[expression.Operator]
		}
		return graphSMTValue{fmt.Sprintf("(%s %s %s)", op, left.expression, right.expression), semanticir.TypeInteger, numericID}, nil
	case semanticir.ExprCompare:
		if len(expression.Operands) != 2 {
			return graphSMTValue{}, fmt.Errorf("grounding comparison has invalid arity")
		}
		left, err := l.groundingExpression(expression.Operands[0])
		if err != nil {
			return graphSMTValue{}, err
		}
		right, err := l.groundingExpression(expression.Operands[1])
		if err != nil {
			return graphSMTValue{}, err
		}
		if left.expression == "" && expression.Operands[0].Literal != nil {
			left.expression, err = l.literal(*expression.Operands[0].Literal, right.numericID)
		}
		if right.expression == "" && expression.Operands[1].Literal != nil {
			right.expression, err = l.literal(*expression.Operands[1].Literal, left.numericID)
		}
		if err != nil {
			return graphSMTValue{}, err
		}
		kind := map[semanticir.Operator]semanticir.CompilerSemanticNodeKind{semanticir.OpEQ: semanticir.CompilerNodeEQ, semanticir.OpNE: semanticir.CompilerNodeNE, semanticir.OpLT: semanticir.CompilerNodeLT, semanticir.OpLE: semanticir.CompilerNodeLE, semanticir.OpGT: semanticir.CompilerNodeGT, semanticir.OpGE: semanticir.CompilerNodeGE}[expression.Operator]
		op, err := l.comparisonOperator(kind, left)
		if err != nil {
			return graphSMTValue{}, err
		}
		formula := fmt.Sprintf("(%s %s %s)", op, left.expression, right.expression)
		if expression.Operator == semanticir.OpNE {
			formula = "(not (= " + left.expression + " " + right.expression + "))"
		}
		return graphSMTValue{formula, semanticir.TypeBool, ""}, nil
	default:
		return graphSMTValue{}, fmt.Errorf("unsupported grounding expression kind %q", expression.Kind)
	}
}

func (l *graphOperationLowerer) terminalPaths() ([]graphTerminalPath, error) {
	var result []graphTerminalPath
	var walk func(string, []string) error
	walk = func(blockID string, conditions []string) error {
		block, exists := l.blocks[blockID]
		if !exists {
			return fmt.Errorf("missing block %q", blockID)
		}
		edges := l.outgoing[blockID]
		if len(edges) == 0 {
			for _, nodeID := range block.NodeIDs {
				node := l.nodes[nodeID]
				if node.Kind == semanticir.CompilerNodeReturn || node.Kind == semanticir.CompilerNodeSuccess || node.Kind == semanticir.CompilerNodeRaise {
					result = append(result, graphTerminalPath{graphAnd(conditions), node})
				}
			}
			return nil
		}
		for _, edge := range edges {
			next := append([]string(nil), conditions...)
			if edge.GuardNodeID != "" {
				guard, err := l.node(edge.GuardNodeID)
				if err != nil {
					return err
				}
				formula := guard.expression
				if !edge.GuardValue {
					formula = "(not " + formula + ")"
				}
				next = append(next, formula)
			}
			if err := walk(edge.ToBlockID, next); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(l.root.EntryBlockID, nil); err != nil {
		return nil, err
	}
	return result, nil
}

func (l *graphOperationLowerer) outcomeFormula(paths []graphTerminalPath, outcome semanticir.ObservableOutcome) (string, error) {
	var cases []string
	for _, path := range paths {
		match, err := l.terminalMatches(path.node, outcome)
		if err != nil {
			return "", err
		}
		cases = append(cases, graphAnd([]string{path.condition, match}))
	}
	return graphOr(cases), nil
}

func (l *graphOperationLowerer) otherOutcomeFormula(paths []graphTerminalPath, named []semanticir.ObservableOutcome) (string, error) {
	var cases []string
	for _, path := range paths {
		var matches []string
		for _, outcome := range named {
			match, err := l.terminalMatches(path.node, outcome)
			if err != nil {
				return "", err
			}
			matches = append(matches, match)
		}
		cases = append(cases, graphAnd([]string{path.condition, "(not " + graphOr(matches) + ")"}))
	}
	return graphOr(cases), nil
}

func (l *graphOperationLowerer) terminalMatches(terminal semanticir.CompilerSemanticNode, outcome semanticir.ObservableOutcome) (string, error) {
	if outcome.OperationID != l.operation.ID {
		return "false", nil
	}
	clauses := []string{}
	switch terminal.Kind {
	case semanticir.CompilerNodeReturn:
		if outcome.Kind != semanticir.OutcomeReturn || outcome.Value == nil {
			return "false", nil
		}
		value, err := l.node(terminal.Operands[0])
		if err != nil {
			return "", err
		}
		literal, err := l.literal(*outcome.Value, value.numericID)
		if err != nil {
			return "", err
		}
		clauses = append(clauses, fmt.Sprintf("(= %s %s)", value.expression, literal))
	case semanticir.CompilerNodeSuccess:
		if outcome.Kind != semanticir.OutcomeSuccess {
			return "false", nil
		}
	case semanticir.CompilerNodeRaise:
		if outcome.Kind != semanticir.OutcomeRaise || outcome.ExceptionType != terminal.ExceptionType || outcome.Message != terminal.Message {
			return "false", nil
		}
	default:
		return "", fmt.Errorf("node %q is not a terminal", terminal.ID)
	}
	if len(terminal.EffectNodeIDs) != len(outcome.Effects) {
		return "false", nil
	}
	for index, effectID := range terminal.EffectNodeIDs {
		node := l.nodes[effectID]
		expected := outcome.Effects[index]
		if node.Kind != semanticir.CompilerNodeEffect || node.EffectKind != expected.Kind || node.EffectTarget != expected.Target || len(node.Operands) != boolInt(expected.Value != nil) {
			return "false", nil
		}
		if expected.Value != nil {
			actual, err := l.node(node.Operands[0])
			if err != nil {
				return "", err
			}
			literal, evaluationErr := evaluateExpression(*expected.Value, nil)
			if evaluationErr != nil {
				return "", fmt.Errorf("effect %q value is not a closed constant: %w", expected.ID, evaluationErr)
			}
			expectedSMT, err := l.literal(literal, actual.numericID)
			if err != nil {
				return "", err
			}
			clauses = append(clauses, fmt.Sprintf("(= %s %s)", actual.expression, expectedSMT))
		}
	}
	return graphAnd(clauses), nil
}

func taskDomain(task *semanticir.Task, id string) (semanticir.Domain, bool) {
	for _, domain := range task.Domains {
		if domain.ID == id {
			return domain, true
		}
	}
	return semanticir.Domain{}, false
}

func taskOutcome(task *semanticir.Task, id string) (semanticir.ObservableOutcome, bool) {
	for _, outcome := range task.Outcomes {
		if outcome.ID == id {
			return outcome, true
		}
	}
	return semanticir.ObservableOutcome{}, false
}

func compilerMembershipKey(operationID, domainID, valueID string) string {
	return operationID + "\x00" + domainID + "\x00" + valueID
}

func compilerOutcomeKey(operationID, outcomeID string) string {
	return operationID + "\x00" + outcomeID
}

func graphSymbol(prefix, value string) string {
	digest := sha256.Sum256([]byte(value))
	return "ray_" + prefix + "_" + hex.EncodeToString(digest[:8])
}

func graphAnd(values []string) string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && value != "true" {
			filtered = append(filtered, value)
		}
	}
	if len(filtered) == 0 {
		return "true"
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return "(and " + strings.Join(filtered, " ") + ")"
}

func graphOr(values []string) string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value == "true" {
			return "true"
		}
		if value != "" && value != "false" {
			filtered = append(filtered, value)
		}
	}
	if len(filtered) == 0 {
		return "false"
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return "(or " + strings.Join(filtered, " ") + ")"
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
