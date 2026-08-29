package rust

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
	"github.com/pelletier/go-toml/v2"
)

// buildRustTestEvidence supports a deliberately narrow compiler-derived test
// shape: every selected test is a conjunction of direct boolean behavior calls
// or comparisons between two such calls. The decoder below consumes every MIR
// basic block in each selected function and admits only call, comparison,
// switch, assertion-panic, and return nodes. Source lowering supplies spans;
// the MIR CFG supplies the authoritative predicate and pass composition.
func buildRustTestEvidence(request semanticir.FrontendRequest, functions []functionDecl, compiler rustCompilerOutput, tests []semanticir.TestModel) (*semanticir.TestObservationProjection, *semanticir.RunnerSelectionEvidence, []semanticir.Diagnostic) {
	whole := wholeSpan(request.Source)
	block := func(message string) (*semanticir.TestObservationProjection, *semanticir.RunnerSelectionEvidence, []semanticir.Diagnostic) {
		return nil, nil, []semanticir.Diagnostic{diagnostic(request.Artifact, whole, semanticir.DiagnosticIncomplete, message)}
	}
	if len(tests) == 0 || compiler.MIR == "" {
		return block("Rust compiler-derived test projection requires selected tests with emitted MIR")
	}
	if request.RunnerCommand == nil || request.Configuration == nil || request.Runner.Name != "cargo" {
		return block("Rust test projection requires a frozen cargo runner, command, and configuration")
	}
	manifest, ok := rustTestWorkspaceBytes(request, *request.Configuration)
	if !ok || !strictRustTestManifest(manifest, request.Artifact.Path) {
		return block("Rust test projection supports only a strict Cargo 2021 lib manifest for the focused artifact")
	}
	command := *request.RunnerCommand
	expectedCommand := strings.Join([]string{request.Runner.Path, "test", "--manifest-path", request.Configuration.Path, "--lib", "--", "--test-threads=1"}, " ")
	if command.Command != expectedCommand || command.WorkspaceID != request.Workspace.ID || command.State != request.Workspace.State || command.TreeDigest != request.Workspace.TreeDigest || command.WorkingDirectory != request.Workspace.WorkingDirectory || command.EnvironmentDigest != request.Workspace.EnvironmentDigest || !command.ClearEnvironment || !command.KillProcessGroup || !command.ExpectedPass || command.PassSignal.Kind != semanticir.PassSignalExitCode || command.PassSignal.Expected != "0" || !rustRunnerHasTool(command.Tools, request.Runner) {
		return block("Rust cargo runner is not the exact unfiltered conjunctive libtest command")
	}
	declarations := make(map[string]functionDecl, len(tests))
	for _, function := range functions {
		if function.IsTest {
			declarations[function.Name] = function
		}
	}
	testIDs := make([]string, 0, len(tests))
	for _, test := range tests {
		testIDs = append(testIDs, test.ID)
		if declarations[test.ID].Name == "" {
			return block("translated Rust test has no selected source declaration")
		}
	}
	sort.Strings(testIDs)
	if len(declarations) != len(tests) || !rustMIRSelectsTestsExactly(compiler.MIR, testIDs) {
		return block("rustc/libtest registration does not select exactly all translated Rust tests")
	}

	irDigest := semanticir.DigestBytes([]byte(compiler.MIR))
	var constructs []semanticir.TestConstructEvidence
	var dependencies []semanticir.TestBehaviorDependency
	var nodes []semanticir.TestProjectionNode
	var roots []semanticir.TestPassRoot
	var quantification []semanticir.TestQuantificationEvidence
	for _, test := range tests {
		section, exists := mirFunction(compiler.MIR, test.ID)
		if !exists {
			return block("selected Rust test has no unique MIR body: " + test.ID)
		}
		derived, exact := deriveRustBooleanTestMIR(request, section, test.Predicate.Provenance)
		sourceLeaves := rustPredicateLeaves(test.Predicate)
		if !exact || len(sourceLeaves) != len(derived.Leaves) || !rustPredicatesSemanticallyEqual(derived.Predicate, test.Predicate) {
			return block("Rust test is outside the closed MIR-derived boolean-call subset: " + test.ID)
		}
		derived.Predicate = test.Predicate
		for index := range derived.Leaves {
			if !rustPredicatesSemanticallyEqual(derived.Leaves[index].Predicate, sourceLeaves[index]) {
				return block("Rust compiler/source predicate order differs for test " + test.ID)
			}
			derived.Leaves[index].Predicate = sourceLeaves[index]
			derived.Leaves[index].Behaviors = rustPredicateBehaviors(sourceLeaves[index])
		}
		leafIDs := make([]string, 0, len(derived.Leaves))
		rootCompilerNodes := []string{}
		for leafIndex, leaf := range derived.Leaves {
			leafProvenance := leaf.Predicate.Provenance
			constructIDs := []string{}
			for constructIndex, kind := range []semanticir.TestConstructKind{semanticir.TestConstructCall, semanticir.TestConstructControl, semanticir.TestConstructAssertion} {
				id := fmt.Sprintf("rust-mir-test::%s::%d::%d", test.ID, leafIndex+1, constructIndex+1)
				digest, _ := semanticir.Digest(struct {
					ID    string
					Kind  semanticir.TestConstructKind
					Nodes []string
				}{id, kind, leaf.CompilerNodes})
				constructIDs = append(constructIDs, id)
				constructs = append(constructs, semanticir.TestConstructEvidence{ID: id, ArtifactID: request.Artifact.ID, Kind: kind, Digest: digest, IRKind: semanticir.CompilerIRRustMIR, IRDigest: irDigest, Tool: request.Translator, CompilerNodeIDs: append([]string(nil), leaf.CompilerNodes...), Provenance: leafProvenance})
				for _, behavior := range leaf.Behaviors {
					dependencies = append(dependencies, semanticir.TestBehaviorDependency{ConstructID: id, Kind: semanticir.TestDependencyCall, Behavior: behavior, Inputs: cloneLiteralMap(behavior.Inputs), CompilerNodeIDs: append([]string(nil), leaf.CompilerNodes...), Provenance: leafProvenance})
				}
			}
			nodeID := fmt.Sprintf("rust-mir-test-predicate::%s::%d", test.ID, leafIndex+1)
			if leaf.Predicate.Kind == semanticir.PredicateNot && len(leaf.Predicate.Children) == 1 {
				child := leaf.Predicate.Children[0]
				childID := nodeID + "::child"
				nodes = append(nodes,
					semanticir.TestProjectionNode{ID: childID, Kind: child.Kind, Observe: child.Observe, Left: child.Left, Right: child.Right, CompilerNodeIDs: append([]string(nil), leaf.CompilerNodes...), ConstructIDs: constructIDs, Provenance: child.Provenance},
					semanticir.TestProjectionNode{ID: nodeID, Kind: semanticir.PredicateNot, Children: []string{childID}, CompilerNodeIDs: append([]string(nil), leaf.CompilerNodes...), Provenance: leafProvenance},
				)
			} else {
				nodes = append(nodes, semanticir.TestProjectionNode{ID: nodeID, Kind: leaf.Predicate.Kind, Observe: leaf.Predicate.Observe, Left: leaf.Predicate.Left, Right: leaf.Predicate.Right, CompilerNodeIDs: append([]string(nil), leaf.CompilerNodes...), ConstructIDs: constructIDs, Provenance: leafProvenance})
			}
			leafIDs = append(leafIDs, nodeID)
			rootCompilerNodes = append(rootCompilerNodes, leaf.CompilerNodes...)
			for _, behavior := range leaf.Behaviors {
				inputSet := []map[string]semanticir.Literal{cloneLiteralMap(behavior.Inputs)}
				inputDigest, inputErr := semanticir.TestConcreteInputsDigest(inputSet)
				if inputErr != nil {
					return block("Rust test BehaviorRef concrete input digest is invalid")
				}
				quantification = append(quantification, semanticir.TestQuantificationEvidence{Behavior: behavior, Kind: semanticir.TestQuantificationSingleton, ConcreteInputs: inputSet, ConcreteInputsDigest: inputDigest, Result: semanticir.ProofProved, Provenance: leafProvenance})
			}
		}
		rootID := leafIDs[0]
		if len(leafIDs) > 1 {
			rootID = "rust-mir-test-root::" + test.ID
			nodes = append(nodes, semanticir.TestProjectionNode{ID: rootID, Kind: semanticir.PredicateAnd, Children: leafIDs, CompilerNodeIDs: uniqueSortedStrings(rootCompilerNodes), Provenance: test.Predicate.Provenance})
		}
		roots = append(roots, semanticir.TestPassRoot{TestID: test.ID, NodeID: rootID, CompilerNodeIDs: uniqueSortedStrings(rootCompilerNodes)})
	}
	predicate := semanticir.StaticTestPredicate(tests, provenance(request.Artifact, whole, semanticir.TranslationTranslated))
	predicateDigest, _ := semanticir.Digest(predicate)
	modelDigest, _ := semanticir.Digest(tests)
	step := semanticir.ProbeStep{ID: "rustc-test-mir", Kind: semanticir.ProbeStepRun, Tool: request.Translator, Argv: append([]string(nil), compiler.Argv[1:]...), StdinDigest: semanticir.DigestBytes(nil), WorkingDirectory: compiler.WorkingDirectory, Environment: append([]semanticir.EnvironmentVariable(nil), request.Workspace.Environment...), EnvironmentDigest: request.Workspace.EnvironmentDigest, ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 30000, ExpectedExitCode: 0, ExpectedStdoutDigest: semanticir.DigestBytes([]byte(compiler.MIR)), ExpectedStderrDigest: semanticir.DigestBytes(compiler.MIRStderr), ExpectedSignalDigest: semanticir.DigestBytes(nil), SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone}, Provenance: provenance(request.Artifact, whole, semanticir.TranslationTranslated)}
	projection := &semanticir.TestObservationProjection{
		Source: request.Artifact, TestIDs: testIDs, PredicateDigest: predicateDigest,
		Constructs: constructs, Dependencies: dependencies, Nodes: nodes,
		PassRoots:      roots,
		Quantification: quantification,
		Derivation:     semanticir.CompilerDerivationEvidence{SourceDigest: request.Artifact.Digest, WorkspaceTreeDigest: request.Workspace.TreeDigest, Tool: request.Translator, IRKind: semanticir.CompilerIRRustMIR, IRDigest: irDigest, Steps: []semanticir.ProbeStep{step}, Output: []byte(compiler.MIR), OutputDigest: irDigest, DecodedModelDigest: modelDigest, Complete: true},
		Complete:       true, Provenance: provenance(request.Artifact, whole, semanticir.TranslationTranslated),
	}
	configProv := semanticir.NewProvenance(*request.Configuration, semanticir.SourceLocation{Path: request.Configuration.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	runner := &semanticir.RunnerSelectionEvidence{TestIDs: testIDs, PredicateDigest: predicateDigest, Configuration: *request.Configuration, Verifier: request.Runner, Command: command, ConjunctivePass: true, Complete: true, Provenance: configProv}
	return projection, runner, nil
}

func cloneLiteralMap(source map[string]semanticir.Literal) map[string]semanticir.Literal {
	result := make(map[string]semanticir.Literal, len(source))
	for name, literal := range source {
		result[name] = literal
	}
	return result
}

type rustMIRTestLeaf struct {
	Predicate     semanticir.TestPredicate
	Behaviors     []semanticir.BehaviorRef
	CompilerNodes []string
}

type rustMIRTestDerivation struct {
	Predicate semanticir.TestPredicate
	Leaves    []rustMIRTestLeaf
}

type rustMIRDirectCall struct {
	Result      string
	OperationID string
	Arguments   []semanticir.Literal
	ReturnBlock string
	Node        string
}

func deriveRustBooleanTestMIR(request semanticir.FrontendRequest, section string, prov semanticir.Provenance) (rustMIRTestDerivation, bool) {
	blocks, exact := rustMIRBlocks(section)
	if !exact {
		return rustMIRTestDerivation{}, false
	}
	visited := map[string]bool{}
	current := "bb0"
	var leaves []rustMIRTestLeaf
	for {
		lines, exists := blocks[current]
		if !exists || visited[current] {
			return rustMIRTestDerivation{}, false
		}
		visited[current] = true
		if len(lines) == 1 && strings.TrimSpace(lines[0]) == "return;" {
			break
		}
		if len(lines) != 1 {
			return rustMIRTestDerivation{}, false
		}
		first, ok := parseRustMIRDirectCall(lines[0])
		if !ok {
			return rustMIRTestDerivation{}, false
		}
		visited[current] = true
		behavior, behaviorOK := rustBehaviorFromMIRCall(request, first, prov)
		if !behaviorOK {
			return rustMIRTestDerivation{}, false
		}
		nextLines, exists := blocks[first.ReturnBlock]
		if !exists || visited[first.ReturnBlock] {
			return rustMIRTestDerivation{}, false
		}
		visited[first.ReturnBlock] = true

		if len(nextLines) == 1 {
			if second, secondOK := parseRustMIRDirectCall(nextLines[0]); secondOK {
				secondBehavior, secondBehaviorOK := rustBehaviorFromMIRCall(request, second, prov)
				comparisonLines, comparisonExists := blocks[second.ReturnBlock]
				if !secondBehaviorOK || !comparisonExists || visited[second.ReturnBlock] || len(comparisonLines) != 2 {
					return rustMIRTestDerivation{}, false
				}
				visited[second.ReturnBlock] = true
				operator, result, left, right, comparisonOK := parseRustMIRBooleanComparison(comparisonLines[0])
				zero, otherwise, switched, switchOK := parseRustMIRSwitch(comparisonLines[1])
				if !comparisonOK || !switchOK || switched != result || left != first.Result || right != second.Result {
					return rustMIRTestDerivation{}, false
				}
				success, panic, expectedTrue, branchOK := rustMIRAssertionTargets(blocks, zero, otherwise)
				if !branchOK || visited[panic] {
					return rustMIRTestDerivation{}, false
				}
				visited[panic] = true
				predicate := semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeEqual, Left: &behavior, Right: &secondBehavior, Provenance: prov}
				comparisonTrueMeansEqual := operator == "Eq"
				if expectedTrue != comparisonTrueMeansEqual {
					predicate = semanticir.TestPredicate{Kind: semanticir.PredicateNot, Children: []semanticir.TestPredicate{predicate}, Provenance: prov}
				}
				nodes := uniqueSortedStrings([]string{first.Node, second.Node, rustMIRTestNode(comparisonLines[0]), rustMIRTestNode(comparisonLines[1]), rustMIRTestNode(blocks[panic][0])})
				leaves = append(leaves, rustMIRTestLeaf{Predicate: predicate, Behaviors: []semanticir.BehaviorRef{behavior, secondBehavior}, CompilerNodes: nodes})
				current = success
				continue
			}
			zero, otherwise, switched, switchOK := parseRustMIRSwitch(nextLines[0])
			if !switchOK || switched != first.Result {
				return rustMIRTestDerivation{}, false
			}
			success, panic, expected, branchOK := rustMIRAssertionTargets(blocks, zero, otherwise)
			if !branchOK || visited[panic] {
				return rustMIRTestDerivation{}, false
			}
			visited[panic] = true
			literal := semanticir.Literal{Type: semanticir.TypeBool, Bool: expected}
			outcomeID, classifyErr := semanticir.ClassifyRawOutcome(mustRustOperation(request, behavior.OperationID), semanticir.RawOutcomeTrace{Kind: semanticir.OutcomeReturn, Value: &literal}, prov)
			if classifyErr != nil {
				return rustMIRTestDerivation{}, false
			}
			observation := semanticir.Observation{Kind: semanticir.ObserveOutcome, Behavior: behavior, OutcomeIDs: []string{outcomeID}, Provenance: prov}
			predicate := semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeIn, Observe: &observation, Provenance: prov}
			nodes := uniqueSortedStrings([]string{first.Node, rustMIRTestNode(nextLines[0]), rustMIRTestNode(blocks[panic][0])})
			leaves = append(leaves, rustMIRTestLeaf{Predicate: predicate, Behaviors: []semanticir.BehaviorRef{behavior}, CompilerNodes: nodes})
			current = success
			continue
		}
		return rustMIRTestDerivation{}, false
	}
	if len(leaves) == 0 || len(visited) != len(blocks) {
		return rustMIRTestDerivation{}, false
	}
	predicate := leaves[0].Predicate
	if len(leaves) > 1 {
		children := make([]semanticir.TestPredicate, len(leaves))
		for index := range leaves {
			children[index] = leaves[index].Predicate
		}
		predicate = semanticir.TestPredicate{Kind: semanticir.PredicateAnd, Children: children, Provenance: prov}
	}
	return rustMIRTestDerivation{Predicate: predicate, Leaves: leaves}, true
}

func parseRustMIRDirectCall(line string) (rustMIRDirectCall, bool) {
	trimmed := strings.TrimSpace(line)
	left, right, found := strings.Cut(trimmed, " = ")
	open := strings.IndexByte(right, '(')
	close := strings.LastIndex(right, ") -> [return: ")
	if !found || open <= 0 || close <= open || !strings.HasSuffix(right, ", unwind continue];") {
		return rustMIRDirectCall{}, false
	}
	tail := strings.TrimSuffix(right[close+len(") -> [return: "):], ", unwind continue];")
	arguments, ok := rustMIRScalarArguments(right[open+1 : close])
	operationID := strings.TrimSpace(right[:open])
	if !ok || operationID == "" || strings.Contains(operationID, "::") || !strings.HasPrefix(strings.TrimSpace(left), "_") {
		return rustMIRDirectCall{}, false
	}
	return rustMIRDirectCall{Result: strings.TrimSpace(left), OperationID: operationID, Arguments: arguments, ReturnBlock: strings.TrimSpace(tail), Node: rustMIRTestNode(line)}, true
}

func rustMIRScalarArguments(value string) ([]semanticir.Literal, bool) {
	if strings.TrimSpace(value) == "" {
		return nil, true
	}
	parts := splitTopLevelString(value, ',')
	result := make([]semanticir.Literal, 0, len(parts))
	for _, part := range parts {
		text := strings.TrimSpace(part)
		if !strings.HasPrefix(text, "const ") {
			return nil, false
		}
		text = strings.TrimPrefix(text, "const ")
		if text == "true" || text == "false" {
			result = append(result, semanticir.Literal{Type: semanticir.TypeBool, Bool: text == "true"})
			continue
		}
		if strings.HasPrefix(text, "\"") {
			literal, ok := parseRustLiteral(text)
			if !ok {
				return nil, false
			}
			result = append(result, literal)
			continue
		}
		underscore := strings.LastIndexByte(text, '_')
		if underscore <= 0 {
			return nil, false
		}
		integer, err := strconv.ParseInt(text[:underscore], 10, 64)
		if err != nil {
			return nil, false
		}
		result = append(result, semanticir.Literal{Type: semanticir.TypeInteger, Integer: integer})
	}
	return result, true
}

func parseRustMIRSwitch(line string) (string, string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "switchInt(") || !strings.HasSuffix(trimmed, "];") {
		return "", "", "", false
	}
	close := strings.Index(trimmed, ") -> [0: ")
	if close < 0 {
		return "", "", "", false
	}
	variable := strings.TrimSpace(trimmed[len("switchInt("):close])
	variable = strings.TrimPrefix(variable, "move ")
	variable = strings.TrimPrefix(variable, "copy ")
	targets := strings.TrimSuffix(trimmed[close+len(") -> [0: "):], "];")
	zero, otherwise, found := strings.Cut(targets, ", otherwise: ")
	return strings.TrimSpace(zero), strings.TrimSpace(otherwise), variable, found
}

func parseRustMIRBooleanComparison(line string) (string, string, string, string, bool) {
	trimmed := strings.TrimSpace(line)
	result, right, found := strings.Cut(trimmed, " = ")
	if !found || !strings.HasSuffix(right, ");") {
		return "", "", "", "", false
	}
	open := strings.IndexByte(right, '(')
	if open <= 0 {
		return "", "", "", "", false
	}
	operator := right[:open]
	if operator != "Eq" && operator != "Ne" {
		return "", "", "", "", false
	}
	operands := splitTopLevelString(strings.TrimSuffix(right[open+1:], ");"), ',')
	if len(operands) != 2 {
		return "", "", "", "", false
	}
	normalize := func(value string) string {
		value = strings.TrimSpace(value)
		value = strings.TrimPrefix(value, "move ")
		return strings.TrimPrefix(value, "copy ")
	}
	return operator, strings.TrimSpace(result), normalize(operands[0]), normalize(operands[1]), true
}

func rustMIRAssertionTargets(blocks map[string][]string, zero, otherwise string) (string, string, bool, bool) {
	isPanic := func(blockID string) bool {
		lines, exists := blocks[blockID]
		return exists && len(lines) == 1 && strings.Contains(strings.TrimSpace(lines[0]), "core::panicking::panic(const \"assertion failed:") && strings.HasSuffix(strings.TrimSpace(lines[0]), ") -> unwind continue;")
	}
	switch {
	case isPanic(zero) && !isPanic(otherwise):
		return otherwise, zero, true, true
	case !isPanic(zero) && isPanic(otherwise):
		return zero, otherwise, false, true
	default:
		return "", "", false, false
	}
}

func rustBehaviorFromMIRCall(request semanticir.FrontendRequest, call rustMIRDirectCall, prov semanticir.Provenance) (semanticir.BehaviorRef, bool) {
	operation, exists := requestOperation(request.Operations, call.OperationID)
	if !exists || len(operation.Inputs) != len(call.Arguments) || len(operation.DomainIDs) != len(call.Arguments) {
		return semanticir.BehaviorRef{}, false
	}
	conditions := make(semanticir.Assignment, len(call.Arguments))
	inputs := make(map[string]semanticir.Literal, len(call.Arguments))
	for index, literal := range call.Arguments {
		domain, domainExists := findDomain(request.FiniteDomains, operation.DomainIDs[index])
		valueID, member := findDomainValueID(domain, literal, operation.ID, operation.Inputs[index].Name)
		if !domainExists || !member || literal.Type != operation.Inputs[index].Type {
			return semanticir.BehaviorRef{}, false
		}
		conditions[domain.ID] = valueID
		inputs[operation.Inputs[index].Name] = literal
	}
	exactInputs, singleton := semanticir.ExactGroundingInputs(operation, request.FiniteDomains, conditions)
	if !singleton || !reflect.DeepEqual(exactInputs, inputs) {
		return semanticir.BehaviorRef{}, false
	}
	return semanticir.BehaviorRef{OperationID: operation.ID, Conditions: conditions, Inputs: inputs, Provenance: prov}, true
}

func mustRustOperation(request semanticir.FrontendRequest, operationID string) semanticir.Operation {
	operation, _ := requestOperation(request.Operations, operationID)
	return operation
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func rustPredicateLeaves(predicate semanticir.TestPredicate) []semanticir.TestPredicate {
	if predicate.Kind == semanticir.PredicateAnd {
		var result []semanticir.TestPredicate
		for _, child := range predicate.Children {
			result = append(result, rustPredicateLeaves(child)...)
		}
		return result
	}
	return []semanticir.TestPredicate{predicate}
}

func rustPredicateBehaviors(predicate semanticir.TestPredicate) []semanticir.BehaviorRef {
	switch predicate.Kind {
	case semanticir.PredicateNot:
		if len(predicate.Children) == 1 {
			return rustPredicateBehaviors(predicate.Children[0])
		}
	case semanticir.PredicateOutcomeIn, semanticir.PredicateRaises, semanticir.PredicateHasEffect:
		if predicate.Observe != nil {
			return []semanticir.BehaviorRef{predicate.Observe.Behavior}
		}
	case semanticir.PredicateOutcomeEqual:
		if predicate.Left != nil && predicate.Right != nil {
			return []semanticir.BehaviorRef{*predicate.Left, *predicate.Right}
		}
	}
	return nil
}

func rustPredicatesSemanticallyEqual(left, right semanticir.TestPredicate) bool {
	var normalize func(semanticir.TestPredicate) semanticir.TestPredicate
	normalize = func(predicate semanticir.TestPredicate) semanticir.TestPredicate {
		normalized := semanticir.TestPredicate{Kind: predicate.Kind}
		if len(predicate.Children) != 0 {
			normalized.Children = make([]semanticir.TestPredicate, len(predicate.Children))
			for index := range predicate.Children {
				normalized.Children[index] = normalize(predicate.Children[index])
			}
		}
		if predicate.Observe != nil {
			copy := *predicate.Observe
			copy.Provenance = semanticir.Provenance{}
			copy.Behavior.Provenance = semanticir.Provenance{}
			normalized.Observe = &copy
		}
		if predicate.Left != nil {
			copy := *predicate.Left
			copy.Provenance = semanticir.Provenance{}
			normalized.Left = &copy
		}
		if predicate.Right != nil {
			copy := *predicate.Right
			copy.Provenance = semanticir.Provenance{}
			normalized.Right = &copy
		}
		return normalized
	}
	return reflect.DeepEqual(normalize(left), normalize(right))
}

func deriveRustZeroEqualityPredicate(request semanticir.FrontendRequest, section string, prov semanticir.Provenance) (semanticir.TestPredicate, []string, bool) {
	blocks, exact := rustMIRBlocks(section)
	if !exact || len(blocks) != 4 {
		return semanticir.TestPredicate{}, nil, false
	}
	var callResult, operationID, returnBlock, panicBlock, zeroBlock, otherwiseBlock string
	var arguments []semanticir.Literal
	var nodes []string
	blockIDs := make([]string, 0, len(blocks))
	for blockID := range blocks {
		blockIDs = append(blockIDs, blockID)
	}
	sort.Strings(blockIDs)
	for _, blockID := range blockIDs {
		lines := blocks[blockID]
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "return;" {
				if returnBlock != "" {
					return semanticir.TestPredicate{}, nil, false
				}
				returnBlock = blockID
				nodes = append(nodes, rustMIRTestNode(line))
				continue
			}
			if strings.Contains(trimmed, " = ") && strings.Contains(trimmed, ") -> [return: ") {
				left, right, found := strings.Cut(trimmed, " = ")
				open := strings.IndexByte(right, '(')
				close := strings.Index(right, ") -> [return: ")
				if !found || open <= 0 || close <= open || operationID != "" {
					return semanticir.TestPredicate{}, nil, false
				}
				operationID, callResult = strings.TrimSpace(right[:open]), strings.TrimSpace(left)
				parsed, ok := rustMIRIntegerArguments(right[open+1 : close])
				if !ok {
					return semanticir.TestPredicate{}, nil, false
				}
				arguments = parsed
				nodes = append(nodes, rustMIRTestNode(line))
				continue
			}
			if strings.HasPrefix(trimmed, "switchInt(move ") {
				wantPrefix := "switchInt(move " + callResult + ") -> [0: "
				if callResult == "" || !strings.HasPrefix(trimmed, wantPrefix) || !strings.HasSuffix(trimmed, "];") {
					return semanticir.TestPredicate{}, nil, false
				}
				targets := strings.TrimSuffix(strings.TrimPrefix(trimmed, wantPrefix), "];")
				zero, otherwise, found := strings.Cut(targets, ", otherwise: ")
				if !found {
					return semanticir.TestPredicate{}, nil, false
				}
				zeroBlock, otherwiseBlock = strings.TrimSpace(zero), strings.TrimSpace(otherwise)
				nodes = append(nodes, rustMIRTestNode(line))
				continue
			}
			if strings.Contains(trimmed, " = core::panicking::panic(const ") && strings.HasSuffix(trimmed, ") -> unwind continue;") {
				if panicBlock != "" {
					return semanticir.TestPredicate{}, nil, false
				}
				panicBlock = blockID
				nodes = append(nodes, rustMIRTestNode(line))
				continue
			}
			return semanticir.TestPredicate{}, nil, false
		}
	}
	if operationID == "" || returnBlock == "" || panicBlock == "" || zeroBlock == "" || otherwiseBlock == "" || returnBlock == panicBlock || zeroBlock == otherwiseBlock {
		return semanticir.TestPredicate{}, nil, false
	}
	expected := false
	switch {
	case zeroBlock == panicBlock && otherwiseBlock == returnBlock:
		expected = true
	case zeroBlock == returnBlock && otherwiseBlock == panicBlock:
		expected = false
	default:
		return semanticir.TestPredicate{}, nil, false
	}
	operation, exists := requestOperation(request.Operations, operationID)
	if !exists || len(arguments) != len(operation.Inputs) || len(operation.DomainIDs) != len(operation.Inputs) {
		return semanticir.TestPredicate{}, nil, false
	}
	conditions := make(semanticir.Assignment, len(arguments))
	for index, input := range operation.Inputs {
		domainID := operation.DomainIDs[index]
		if domainID == "" || input.DomainID != "" && input.DomainID != domainID {
			return semanticir.TestPredicate{}, nil, false
		}
		domain, domainExists := findDomain(request.FiniteDomains, domainID)
		matches := ""
		if !domainExists {
			return semanticir.TestPredicate{}, nil, false
		}
		for _, member := range domain.Values {
			literal, grounded := rustLiteralForDomainValue(domain, member, operation.ID, input.Name, input.Type)
			if grounded && reflect.DeepEqual(literal, arguments[index]) {
				if matches != "" {
					return semanticir.TestPredicate{}, nil, false
				}
				matches = member.ID
			}
		}
		if matches == "" {
			return semanticir.TestPredicate{}, nil, false
		}
		conditions[domainID] = matches
	}
	expectedLiteral := semanticir.Literal{Type: semanticir.TypeBool, Bool: expected}
	trace := semanticir.RawOutcomeTrace{Kind: semanticir.OutcomeReturn, Value: &expectedLiteral}
	outcomeID, err := semanticir.ClassifyRawOutcome(operation, trace, prov)
	if err != nil || outcomeID == semanticir.OtherOutcome(operation.ID, prov).ID {
		return semanticir.TestPredicate{}, nil, false
	}
	inputs, singleton := semanticir.ExactGroundingInputs(operation, request.FiniteDomains, conditions)
	if !singleton {
		return semanticir.TestPredicate{}, nil, false
	}
	behavior := semanticir.BehaviorRef{OperationID: operation.ID, Conditions: conditions, Inputs: inputs, Provenance: prov}
	observe := semanticir.Observation{Kind: semanticir.ObserveOutcome, Behavior: behavior, OutcomeIDs: []string{outcomeID}, Provenance: prov}
	sort.Strings(nodes)
	return semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeIn, Observe: &observe, Provenance: prov}, nodes, len(nodes) == 4
}

func rustMIRBlocks(section string) (map[string][]string, bool) {
	blocks := map[string][]string{}
	current := ""
	inBody := false
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "bb") && strings.HasSuffix(trimmed, ": {") {
			current = strings.TrimSuffix(trimmed, ": {")
			if _, duplicate := blocks[current]; duplicate {
				return nil, false
			}
			blocks[current] = nil
			inBody = true
			continue
		}
		if !inBody {
			continue // function header, locals, and debug declarations
		}
		if trimmed == "}" || trimmed == "" {
			if trimmed == "}" {
				current = ""
			}
			continue
		}
		if current == "" {
			return nil, false
		}
		blocks[current] = append(blocks[current], line)
	}
	return blocks, len(blocks) != 0
}

func rustMIRIntegerArguments(value string) ([]semanticir.Literal, bool) {
	if strings.TrimSpace(value) == "" {
		return nil, true
	}
	parts := splitTopLevelString(value, ',')
	result := make([]semanticir.Literal, 0, len(parts))
	for _, part := range parts {
		text := strings.TrimSpace(part)
		if !strings.HasPrefix(text, "const ") {
			return nil, false
		}
		text = strings.TrimPrefix(text, "const ")
		underscore := strings.LastIndexByte(text, '_')
		if underscore <= 0 {
			return nil, false
		}
		integer, err := strconv.ParseInt(text[:underscore], 10, 64)
		typeName := text[underscore+1:]
		if err != nil || !strings.Contains("i8 i16 i32 i64 isize u8 u16 u32 u64 usize", typeName) {
			return nil, false
		}
		result = append(result, semanticir.Literal{Type: semanticir.TypeInteger, Integer: integer})
	}
	return result, true
}

func rustMIRTestNode(line string) string {
	return "mir:test-node:" + strings.TrimPrefix(semanticir.DigestBytes([]byte(strings.TrimSpace(line))), "sha256:")[:16]
}

func rustMIRSelectsExactly(mir, testID string) bool {
	if strings.Count(mir, "const "+testID+": TestDescAndFn = {") != 1 || strings.Count(mir, "StaticTestName(const \""+testID+"\")") != 1 || !strings.Contains(mir, "ignore: const false") || !strings.Contains(mir, "no_run: const false") || !strings.Contains(mir, "test::ShouldPanic::No") || !strings.Contains(mir, "test_main_static(") {
		return false
	}
	marker := " = const " + testID + ";"
	return strings.Count(mir, marker) == 1
}

func rustMIRSelectsTestsExactly(mir string, testIDs []string) bool {
	if len(testIDs) == 0 || strings.Count(mir, "const main::promoted[0]: &[&TestDescAndFn;") != 1 || !strings.Contains(mir, "test_main_static(") {
		return false
	}
	for _, testID := range testIDs {
		if strings.Count(mir, "const "+testID+": TestDescAndFn = {") != 1 || strings.Count(mir, "StaticTestName(const \""+testID+"\")") != 1 || strings.Count(mir, " = const "+testID+";") != 1 {
			return false
		}
	}
	return strings.Count(mir, "StaticTestName(const \"") == len(testIDs) && strings.Count(mir, "ignore: const false") == len(testIDs) && strings.Count(mir, "no_run: const false") == len(testIDs) && strings.Count(mir, "test::ShouldPanic::No") == len(testIDs)
}

func strictRustTestManifest(body []byte, artifactPath string) bool {
	var manifest struct {
		Package struct {
			Name    string `toml:"name"`
			Version string `toml:"version"`
			Edition string `toml:"edition"`
		} `toml:"package"`
		Lib struct {
			Path string `toml:"path"`
		} `toml:"lib"`
	}
	decoder := toml.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(&manifest) == nil && manifest.Package.Name != "" && manifest.Package.Version != "" && manifest.Package.Edition == "2021" && filepath.Clean(manifest.Lib.Path) == filepath.Clean(artifactPath)
}

func rustTestWorkspaceBytes(request semanticir.FrontendRequest, artifact semanticir.ArtifactRef) ([]byte, bool) {
	for _, entry := range request.Workspace.Entries {
		if entry.Artifact == artifact {
			path, safe := withinRustWorkspace(request.Workspace.Root, entry.Path)
			body, err := os.ReadFile(path)
			return body, safe && err == nil && semanticir.VerifyArtifact(artifact, body) == nil
		}
	}
	return nil, false
}

func rustRunnerHasTool(tools []semanticir.ToolRef, target semanticir.ToolRef) bool {
	for _, tool := range tools {
		if tool == target {
			return true
		}
	}
	return false
}
