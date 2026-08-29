package cpp

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

var (
	llvmDirectCallPattern = regexp.MustCompile(`\b(?:call|invoke)\b[^@\n]*@("[^"]+"|[-A-Za-z$._0-9]+)\(`)
	safeRunnerAtom        = regexp.MustCompile(`^[A-Za-z0-9_./:+-]+$`)
	safeGTestID           = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

type llvmTestFunction struct {
	symbol          string
	body            string
	nodeIDs         []string
	behaviorCallIDs map[string][]string
	branchCount     int
	abortCount      int
}

// buildTestProjectionEvidence accepts a deliberately narrow compiler-closed
// subset: selected functions whose bodies contain only plain C/C++ assert
// invocations over exact modeled calls. The graph is derived from the complete
// LLVM function bodies. Any other call, global read, helper, fixture, control
// construct, or assertion framework blocks.
func (l *lowerer) buildTestProjectionEvidence() (semanticir.TestObservationProjection, semanticir.RunnerSelectionEvidence, error) {
	if len(l.tests) == 0 || len(l.tests) != len(l.operations) {
		return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, fmt.Errorf("translated test operations do not have a one-to-one closed predicate model")
	}
	projectionProvenance := l.provenance(nil, semanticir.TranslationTranslated)
	testIDs := make([]string, 0, len(l.tests))
	for _, test := range l.tests {
		testIDs = append(testIDs, test.ID)
	}
	sort.Strings(testIDs)
	predicateDigest, err := semanticir.Digest(semanticir.StaticTestPredicate(l.tests, projectionProvenance))
	if err != nil {
		return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, fmt.Errorf("digest static C++ test predicate: %v", err)
	}
	projection := semanticir.TestObservationProjection{
		Source: l.request.Artifact, TestIDs: append([]string(nil), testIDs...), PredicateDigest: predicateDigest,
		Constructs: []semanticir.TestConstructEvidence{}, Dependencies: []semanticir.TestBehaviorDependency{},
		Nodes: []semanticir.TestProjectionNode{}, PassRoots: []semanticir.TestPassRoot{}, Complete: true,
		Provenance: projectionProvenance,
	}
	requestedSymbols, err := l.requestedLLVMSymbols()
	if err != nil {
		return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, err
	}
	models := make(map[string]semanticir.TestModel, len(l.tests))
	for _, test := range l.tests {
		models[test.ID] = test
	}
	for _, operation := range l.operations {
		test, exists := models[operation.operation.ID]
		if !exists {
			return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, fmt.Errorf("test operation %q has no static predicate", operation.operation.ID)
		}
		if len(operation.asserts) != len(test.Assertions) || len(operation.asserts) == 0 {
			return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, fmt.Errorf("test %q assertion inventory is incomplete", test.ID)
		}
		for _, assertion := range operation.asserts {
			if assertion.Name != "assert" {
				return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, fmt.Errorf("test %q uses assertion framework %q whose helper semantics are not compiler-closed", test.ID, assertion.Name)
			}
		}
		for _, statement := range operation.body.Inner {
			if statement != nil && l.assertionForExpansion(statement, operation.asserts) == nil && statement.Kind != "NullStmt" {
				return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, fmt.Errorf("test %q contains pass-influencing construct %s outside its exact assertions", test.ID, statement.Kind)
			}
		}
		analysis, analyzeErr := l.analyzeLLVMTestFunction(operation, requestedSymbols, test)
		if analyzeErr != nil {
			return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, analyzeErr
		}
		assertionPredicates, ok := predicateAssertions(test.Predicate, len(operation.asserts))
		if !ok {
			return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, fmt.Errorf("test %q predicate shape does not preserve its assertion boundaries", test.ID)
		}
		callOffsets := make(map[string]int)
		constructIDs := make([]string, len(operation.asserts))
		for index, assertion := range operation.asserts {
			constructID := fmt.Sprintf("%s:assert:%d", test.ID, index+1)
			constructIDs[index] = constructID
			compilerNodeIDs := append([]string(nil), analysis.nodeIDs...)
			digest, digestErr := semanticir.Digest(struct {
				Source          []byte   `json:"source"`
				CompilerNodeIDs []string `json:"compiler_node_ids"`
			}{append([]byte(nil), l.request.Source[assertion.Start:assertion.End]...), compilerNodeIDs})
			if digestErr != nil {
				return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, fmt.Errorf("digest test construct %q: %v", constructID, digestErr)
			}
			provenance := l.provenanceRange(assertion.Start, assertion.End, semanticir.TranslationTranslated)
			projection.Constructs = append(projection.Constructs, semanticir.TestConstructEvidence{
				ID: constructID, ArtifactID: l.request.Artifact.ID, Kind: semanticir.TestConstructAssertion,
				Digest: digest, IRKind: semanticir.CompilerIRLLVM, IRDigest: semanticir.DigestBytes([]byte(l.llvmIR)),
				Tool: l.request.Translator, CompilerNodeIDs: compilerNodeIDs, Provenance: provenance,
			})
			refs := predicateBehaviorRefs(assertionPredicates[index])
			if len(refs) == 0 {
				return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, fmt.Errorf("assertion %q has no modeled BehaviorRef dependency", constructID)
			}
			for _, ref := range refs {
				nodes := analysis.behaviorCallIDs[ref.OperationID]
				offset := callOffsets[ref.OperationID]
				if offset >= len(nodes) {
					return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, fmt.Errorf("assertion %q has no matching LLVM call for behavior %q", constructID, ref.OperationID)
				}
				projection.Dependencies = append(projection.Dependencies, semanticir.TestBehaviorDependency{
					ConstructID: constructID, Kind: semanticir.TestDependencyCall, Behavior: ref,
					Inputs:          cloneLiteralMap(ref.Inputs),
					CompilerNodeIDs: []string{nodes[offset]}, Provenance: provenance,
				})
				callOffsets[ref.OperationID] = offset + 1
			}
		}
		for operationID, nodes := range analysis.behaviorCallIDs {
			if callOffsets[operationID] != len(nodes) {
				return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, fmt.Errorf("test %q has unmodeled LLVM calls to %q", test.ID, operationID)
			}
		}
		rootID := l.appendProjectionPredicate(&projection, test.ID, test.Predicate, "root", analysis.nodeIDs, constructIDs)
		projection.PassRoots = append(projection.PassRoots, semanticir.TestPassRoot{TestID: test.ID, NodeID: rootID, CompilerNodeIDs: append([]string(nil), analysis.nodeIDs...)})
	}
	modelDigest, err := semanticir.Digest(l.tests)
	if err != nil {
		return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, fmt.Errorf("digest decoded C++ test model: %v", err)
	}
	environment := append([]semanticir.EnvironmentVariable(nil), l.request.Workspace.Environment...)
	environmentDigest, err := semanticir.Digest(environment)
	if err != nil {
		return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, fmt.Errorf("digest C++ test compiler environment: %v", err)
	}
	emptyDigest := semanticir.DigestBytes(nil)
	workingDirectory, err := l.evidenceWorkingDirectory()
	if err != nil {
		return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, err
	}
	arguments := []string{"-x", "c++"}
	arguments = append(arguments, l.compileFlags...)
	sourceArgument, err := filepath.Rel(l.compileDirectory, l.sourcePath)
	if err != nil || sourceArgument == "" || filepath.IsAbs(sourceArgument) || sourceArgument == ".." || strings.HasPrefix(sourceArgument, ".."+string(filepath.Separator)) {
		return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, fmt.Errorf("test compiler source argument is not workspace-contained")
	}
	arguments = append(arguments, "-S", "-emit-llvm", "-fno-color-diagnostics", "-o", "-", filepath.ToSlash(sourceArgument))
	projection.Derivation = semanticir.CompilerDerivationEvidence{
		SourceDigest: l.request.Artifact.Digest, WorkspaceTreeDigest: l.request.Workspace.TreeDigest,
		Tool: l.request.Translator, IRKind: semanticir.CompilerIRLLVM, IRDigest: semanticir.DigestBytes([]byte(l.llvmIR)),
		Steps: []semanticir.ProbeStep{{
			ID: "emit-test-llvm", Kind: semanticir.ProbeStepRun, Tool: l.request.Translator, Argv: arguments,
			Stdin: []byte{}, StdinDigest: emptyDigest, WorkingDirectory: workingDirectory,
			Environment: environment, EnvironmentDigest: environmentDigest, ClearEnvironment: true, KillProcessGroup: true,
			TimeoutMillis: 30_000, ExpectedExitCode: 0, ExpectedStdoutDigest: semanticir.DigestBytes([]byte(l.llvmIR)),
			ExpectedStderrDigest: emptyDigest, ExpectedSignalDigest: emptyDigest,
			SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone}, Provenance: projectionProvenance,
		}},
		Output: []byte(l.llvmIR), OutputDigest: semanticir.DigestBytes([]byte(l.llvmIR)), DecodedModelDigest: modelDigest, Complete: true,
	}
	runner, err := l.exactGTestRunnerSelection(testIDs, predicateDigest)
	if err != nil {
		return semanticir.TestObservationProjection{}, semanticir.RunnerSelectionEvidence{}, err
	}
	return projection, runner, nil
}

func (l *lowerer) requestedLLVMSymbols() (map[string]string, error) {
	requested := make(map[string]string)
	var visit func(*astNode)
	visit = func(node *astNode) {
		if node == nil {
			return
		}
		if (node.Kind == "FunctionDecl" || node.Kind == "CXXMethodDecl") && node.MangledName != "" {
			for _, operation := range l.request.Operations {
				if node.Name == shortName(operation.ID) {
					requested[normalizeLLVMSymbol(node.MangledName)] = operation.ID
				}
			}
		}
		for _, child := range node.Inner {
			visit(child)
		}
	}
	visit(l.root)
	for _, operation := range l.request.Operations {
		found := false
		for _, id := range requested {
			found = found || id == operation.ID
		}
		if !found {
			return nil, fmt.Errorf("compiled test AST has no exact mangled declaration for modeled operation %q", operation.ID)
		}
	}
	return requested, nil
}

func (l *lowerer) analyzeLLVMTestFunction(operation loweredOperation, requestedSymbols map[string]string, test semanticir.TestModel) (llvmTestFunction, error) {
	symbol := operation.node.MangledName
	if symbol == "" {
		symbol = operation.node.Name
	}
	body, ok := llvmFunctionBody(l.llvmIR, symbol)
	if !ok && strings.HasPrefix(symbol, "__Z") {
		body, ok = llvmFunctionBody(l.llvmIR, symbol[1:])
	}
	if !ok {
		return llvmTestFunction{}, fmt.Errorf("test %q has no exact LLVM definition for %q", test.ID, symbol)
	}
	result := llvmTestFunction{symbol: normalizeLLVMSymbol(symbol), body: body, behaviorCallIDs: map[string][]string{}}
	for index, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "{" || trimmed == "}" || strings.HasPrefix(trimmed, ";") || strings.HasSuffix(trimmed, ":") || strings.HasPrefix(trimmed, "define ") {
			continue
		}
		nodeID := fmt.Sprintf("llvm:%s:%d:%s", result.symbol, index+1, strings.TrimPrefix(semanticir.DigestBytes([]byte(trimmed)), "sha256:")[:16])
		result.nodeIDs = append(result.nodeIDs, nodeID)
		if strings.Contains(trimmed, " asm ") || strings.Contains(trimmed, " indirectbr ") || strings.Contains(trimmed, " switch ") || strings.Contains(trimmed, " invoke ") || strings.Contains(trimmed, " landingpad ") || strings.Contains(trimmed, " resume ") || strings.Contains(trimmed, " atomicrmw ") || strings.Contains(trimmed, " cmpxchg ") || strings.Contains(trimmed, " volatile ") {
			return llvmTestFunction{}, fmt.Errorf("test %q LLVM contains unsupported pass dependency %q", test.ID, trimmed)
		}
		if (strings.Contains(trimmed, " load ") || strings.Contains(trimmed, " store ")) && strings.Contains(trimmed, "@") {
			return llvmTestFunction{}, fmt.Errorf("test %q LLVM reads or writes unmodeled global state: %q", test.ID, trimmed)
		}
		if strings.Contains(trimmed, " br i1 ") || strings.HasPrefix(trimmed, "br i1 ") {
			result.branchCount++
		}
		match := llvmDirectCallPattern.FindStringSubmatch(trimmed)
		if len(match) == 0 {
			continue
		}
		target := normalizeLLVMSymbol(strings.Trim(match[1], `"`))
		if operationID, exists := requestedSymbols[target]; exists {
			result.behaviorCallIDs[operationID] = append(result.behaviorCallIDs[operationID], nodeID)
			continue
		}
		if target == "__assert_rtn" || target == "__assert_fail" {
			result.abortCount++
			continue
		}
		if strings.HasPrefix(target, "llvm.") {
			continue
		}
		return llvmTestFunction{}, fmt.Errorf("test %q LLVM call %q is an unmodeled helper/framework dependency", test.ID, target)
	}
	if len(result.nodeIDs) == 0 || !strings.Contains(body, "ret void") {
		return llvmTestFunction{}, fmt.Errorf("test %q LLVM has no complete void pass terminal", test.ID)
	}
	assertionPredicates, ok := predicateAssertions(test.Predicate, len(test.Assertions))
	if !ok {
		return llvmTestFunction{}, fmt.Errorf("test %q predicate shape does not retain assertion control boundaries", test.ID)
	}
	wantBranches := len(test.Assertions)
	for _, predicate := range assertionPredicates {
		wantBranches += predicateShortCircuitCount(predicate)
	}
	if result.abortCount != len(test.Assertions) || result.branchCount != wantBranches {
		return llvmTestFunction{}, fmt.Errorf("test %q LLVM assert/control inventory is abort=%d branches=%d, want %d/%d", test.ID, result.abortCount, result.branchCount, len(test.Assertions), wantBranches)
	}
	return result, nil
}

func normalizeLLVMSymbol(value string) string {
	value = strings.TrimPrefix(value, `\01`)
	if strings.HasPrefix(value, "__Z") {
		return value[1:]
	}
	return value
}

func predicateAssertions(predicate semanticir.TestPredicate, count int) ([]semanticir.TestPredicate, bool) {
	if count == 1 {
		return []semanticir.TestPredicate{predicate}, true
	}
	if predicate.Kind != semanticir.PredicateAnd || len(predicate.Children) != count {
		return nil, false
	}
	return append([]semanticir.TestPredicate(nil), predicate.Children...), true
}

func predicateBehaviorRefs(predicate semanticir.TestPredicate) []semanticir.BehaviorRef {
	var result []semanticir.BehaviorRef
	if predicate.Observe != nil {
		result = append(result, predicate.Observe.Behavior)
	}
	if predicate.Left != nil {
		result = append(result, *predicate.Left)
	}
	if predicate.Right != nil {
		result = append(result, *predicate.Right)
	}
	for _, child := range predicate.Children {
		result = append(result, predicateBehaviorRefs(child)...)
	}
	return result
}

func predicateShortCircuitCount(predicate semanticir.TestPredicate) int {
	count := 0
	if predicate.Kind == semanticir.PredicateAnd || predicate.Kind == semanticir.PredicateOr {
		count += len(predicate.Children) - 1
	}
	for _, child := range predicate.Children {
		count += predicateShortCircuitCount(child)
	}
	return count
}

func (l *lowerer) appendProjectionPredicate(projection *semanticir.TestObservationProjection, testID string, predicate semanticir.TestPredicate, path string, compilerNodeIDs, assertionConstructIDs []string) string {
	id := testID + ":predicate:" + path
	node := semanticir.TestProjectionNode{
		ID: id, Kind: predicate.Kind, Observe: predicate.Observe, Left: predicate.Left, Right: predicate.Right,
		CompilerNodeIDs: append([]string(nil), compilerNodeIDs...), Provenance: predicate.Provenance,
	}
	if len(assertionConstructIDs) == 1 {
		node.ConstructIDs = append([]string(nil), assertionConstructIDs...)
	}
	for index, child := range predicate.Children {
		constructs := []string(nil)
		if len(assertionConstructIDs) == len(predicate.Children) {
			constructs = []string{assertionConstructIDs[index]}
		}
		childID := l.appendProjectionPredicate(projection, testID, child, fmt.Sprintf("%s.%d", path, index), compilerNodeIDs, constructs)
		node.Children = append(node.Children, childID)
	}
	projection.Nodes = append(projection.Nodes, node)
	return id
}

func (l *lowerer) exactGTestRunnerSelection(testIDs []string, predicateDigest string) (semanticir.RunnerSelectionEvidence, error) {
	if l.request.RunnerCommand == nil || l.request.Configuration == nil {
		return semanticir.RunnerSelectionEvidence{}, fmt.Errorf("C++ tests require a frozen exact runner command and configuration")
	}
	runner := l.request.Runner
	if runner.Name == "" || !filepath.IsAbs(runner.Path) || !semanticir.ValidDigest(runner.Digest) || runner.Version == "" || !safeRunnerAtom.MatchString(runner.Path) {
		return semanticir.RunnerSelectionEvidence{}, fmt.Errorf("C++ test runner identity/path is incomplete or not canonically tokenizable")
	}
	bytes, err := os.ReadFile(runner.Path)
	if err != nil || semanticir.DigestBytes(bytes) != runner.Digest {
		return semanticir.RunnerSelectionEvidence{}, fmt.Errorf("C++ test runner bytes differ from frozen tool identity")
	}
	for _, id := range testIDs {
		if !safeGTestID.MatchString(id) {
			return semanticir.RunnerSelectionEvidence{}, fmt.Errorf("translated test ID %q is not exactly representable by the strict GTest filter", id)
		}
	}
	command := *l.request.RunnerCommand
	prefix := runner.Path + " --gtest_filter="
	if !strings.HasPrefix(command.Command, prefix) || strings.Count(command.Command, " ") != 1 {
		return semanticir.RunnerSelectionEvidence{}, fmt.Errorf("C++ runner must use one exact no-shell --gtest_filter argument")
	}
	selected := strings.Split(strings.TrimPrefix(command.Command, prefix), ":")
	selectedSet := make(map[string]bool, len(selected))
	for _, id := range selected {
		if !safeGTestID.MatchString(id) || selectedSet[id] {
			return semanticir.RunnerSelectionEvidence{}, fmt.Errorf("C++ runner filter contains an invalid or duplicate test ID %q", id)
		}
		selectedSet[id] = true
	}
	for _, id := range testIDs {
		if !selectedSet[id] {
			return semanticir.RunnerSelectionEvidence{}, fmt.Errorf("C++ runner filter omits translated test ID %q", id)
		}
	}
	if command.WorkspaceID != l.request.Workspace.ID || command.State != l.request.Workspace.State || command.TreeDigest != l.request.Workspace.TreeDigest || command.WorkingDirectory == "" || command.TimeoutMillis <= 0 || !command.ClearEnvironment || !command.KillProcessGroup || command.PassSignal.Kind != semanticir.PassSignalExitCode || command.PassSignal.Expected != "0" || !command.ExpectedPass || !command.ObservedPass || command.ExitCode != 0 || len(command.Tools) != 1 || command.Tools[0] != runner || !reflect.DeepEqual(command.Environment, l.request.Workspace.Environment) || command.EnvironmentDigest != l.request.Workspace.EnvironmentDigest {
		return semanticir.RunnerSelectionEvidence{}, fmt.Errorf("C++ runner must be the exact hermetic conjunctive exit-code-zero command for the translated tests")
	}
	configuration := *l.request.Configuration
	found := false
	for _, entry := range l.request.Workspace.Entries {
		found = found || entry.Artifact == configuration
	}
	if configuration.Kind != semanticir.ArtifactConfiguration || !found {
		return semanticir.RunnerSelectionEvidence{}, fmt.Errorf("C++ runner configuration is not an exact frozen workspace artifact")
	}
	provenance := semanticir.NewProvenance(configuration, semanticir.SourceLocation{Path: configuration.Path, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1}, semanticir.TranslationTranslated)
	return semanticir.RunnerSelectionEvidence{
		TestIDs: append([]string(nil), testIDs...), PredicateDigest: predicateDigest, Configuration: configuration,
		Verifier: runner, Command: command, ConjunctivePass: true, Complete: true, Provenance: provenance,
	}, nil
}
