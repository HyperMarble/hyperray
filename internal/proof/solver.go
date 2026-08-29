package proof

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

var z3ValuePattern = regexp.MustCompile(`\(b([0-9]+)\s+(-?[0-9]+)\)`)

const solverTimeoutMillis int64 = 30_000

type smtEncoder struct {
	model        *finiteModel
	caseIndexes  map[caseKey]int
	outcomeIndex map[string]int
}

func proveWithZ3(ctx context.Context, model *finiteModel, environment *semanticir.EnvironmentModel) (ObligationResult, ObligationResult, ObligationResult, ObligationResult, *SolverTranscript, error) {
	solverEnvironment, solverEnvironmentDigest, environmentErr := frozenSolverEnvironment(environment)
	if environmentErr != nil {
		return ObligationResult{}, ObligationResult{}, ObligationResult{}, ObligationResult{}, nil, environmentErr
	}
	tool, err := frozenZ3(ctx, environment, solverEnvironment)
	if err != nil {
		return ObligationResult{}, ObligationResult{}, ObligationResult{}, ObligationResult{}, nil, err
	}
	encoder := newSMTEncoder(model)
	transcript := &SolverTranscript{
		Name: tool.Name, Version: tool.Version, Digest: tool.Digest, Tool: tool,
		Argv: []string{"-in", "-smt2"}, WorkingDirectory: "/", Environment: solverEnvironment,
		EnvironmentDigest: solverEnvironmentDigest, ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: solverTimeoutMillis,
	}

	type query struct {
		obligation semanticir.ProofObligation
		formula    string
	}
	queries := []query{
		{semanticir.ObligationReferenceCorrectness, smtAnd([]string{encoder.codeFormula(), smtNot(encoder.specFormula())})},
		{semanticir.ObligationTestsSound, smtAnd([]string{encoder.testsFormula(), smtNot(encoder.specFormula())})},
		{semanticir.ObligationTestsComplete, smtAnd([]string{encoder.specFormula(), smtNot(encoder.testsFormula())})},
		{semanticir.ObligationReferenceAcceptance, smtAnd([]string{encoder.codeFormula(), smtNot(encoder.testsFormula())})},
	}
	results := make([]ObligationResult, 0, len(queries))
	for _, query := range queries {
		result, record, err := solveObligation(ctx, tool, transcript, encoder, query.obligation, query.formula)
		transcript.Queries = append(transcript.Queries, record)
		if err != nil {
			return ObligationResult{}, ObligationResult{}, ObligationResult{}, ObligationResult{}, transcript, err
		}
		results = append(results, result)
	}
	return results[0], results[1], results[2], results[3], transcript, nil
}

func frozenZ3(ctx context.Context, environment *semanticir.EnvironmentModel, frozenEnvironment []semanticir.EnvironmentVariable) (semanticir.ToolRef, error) {
	if environment == nil {
		return semanticir.ToolRef{}, fmt.Errorf("typed environment model is missing")
	}
	var candidates []semanticir.ToolRef
	candidates = append(candidates, environment.Tools...)
	for _, command := range environment.Commands {
		candidates = append(candidates, command.Tools...)
	}
	var tool semanticir.ToolRef
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.Name, "z3") {
			tool = candidate
			break
		}
	}
	if tool.Name == "" {
		return semanticir.ToolRef{}, fmt.Errorf("finite universe exceeds enumeration threshold and no frozen Z3 ToolRef is available")
	}
	if !filepath.IsAbs(tool.Path) {
		return semanticir.ToolRef{}, fmt.Errorf("frozen Z3 path %q is not absolute", tool.Path)
	}
	file, err := os.Open(tool.Path)
	if err != nil {
		return semanticir.ToolRef{}, fmt.Errorf("open frozen Z3 %q: %w", tool.Path, err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return semanticir.ToolRef{}, fmt.Errorf("hash frozen Z3 %q: %w", tool.Path, copyErr)
	}
	if closeErr != nil {
		return semanticir.ToolRef{}, fmt.Errorf("close frozen Z3 %q: %w", tool.Path, closeErr)
	}
	digest := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	if digest != tool.Digest {
		return semanticir.ToolRef{}, fmt.Errorf("frozen Z3 digest mismatch: got %s, expected %s", digest, tool.Digest)
	}
	commandEnvironment := make([]string, len(frozenEnvironment))
	for index, variable := range frozenEnvironment {
		commandEnvironment[index] = variable.Name + "=" + variable.Value
	}
	versionOutput, versionStderr, err := runHermetic(ctx, tool.Path, []string{"-version"}, nil, "/", commandEnvironment, solverTimeoutMillis)
	if err != nil {
		return semanticir.ToolRef{}, fmt.Errorf("query frozen Z3 version: %w", err)
	}
	if len(versionStderr) != 0 {
		return semanticir.ToolRef{}, fmt.Errorf("frozen Z3 version command wrote diagnostics: %s", strings.TrimSpace(string(versionStderr)))
	}
	if strings.TrimSpace(string(versionOutput)) != strings.TrimSpace(tool.Version) {
		return semanticir.ToolRef{}, fmt.Errorf("frozen Z3 version mismatch: got %q, expected %q", strings.TrimSpace(string(versionOutput)), strings.TrimSpace(tool.Version))
	}
	return tool, nil
}

func newSMTEncoder(model *finiteModel) *smtEncoder {
	encoder := &smtEncoder{model: model, caseIndexes: make(map[caseKey]int), outcomeIndex: make(map[string]int)}
	for i, finiteCase := range model.cases {
		encoder.caseIndexes[finiteCaseKey(model, finiteCase)] = i
	}
	for i, outcomeID := range model.outcomeIDs {
		encoder.outcomeIndex[outcomeID] = i
	}
	return encoder
}

func (e *smtEncoder) script(formula string, values bool) string {
	var builder strings.Builder
	builder.WriteString("(set-logic QF_LIA)\n")
	builder.WriteString("(set-option :produce-models true)\n")
	for i := range e.model.cases {
		fmt.Fprintf(&builder, "(declare-fun b%d () Int)\n", i)
	}
	for i, finiteCase := range e.model.cases {
		var members []string
		for _, outcomeID := range e.model.operationOutcomes[finiteCase.operation] {
			members = append(members, smtEqual(i, e.outcomeIndex[outcomeID]))
		}
		fmt.Fprintf(&builder, "(assert %s)\n", smtOr(members))
	}
	fmt.Fprintf(&builder, "(assert %s)\n", formula)
	if values {
		for i := range e.model.cases {
			fmt.Fprintf(&builder, "(minimize b%d)\n", i)
		}
	}
	builder.WriteString("(check-sat)\n")
	if values {
		builder.WriteString("(get-value (")
		for i := range e.model.cases {
			if i != 0 {
				builder.WriteByte(' ')
			}
			fmt.Fprintf(&builder, "b%d", i)
		}
		builder.WriteString("))\n")
	}
	builder.WriteString("(exit)\n")
	return builder.String()
}

func (e *smtEncoder) specFormula() string {
	clauses := make([]string, 0, len(e.model.cases))
	for i, finiteCase := range e.model.cases {
		var members []string
		for _, outcomeID := range finiteCase.allowed {
			members = append(members, smtEqual(i, e.outcomeIndex[outcomeID]))
		}
		clauses = append(clauses, smtOr(members))
	}
	return smtAnd(clauses)
}

func (e *smtEncoder) codeFormula() string {
	clauses := make([]string, 0, len(e.model.cases))
	for i, finiteCase := range e.model.cases {
		var members []string
		for _, outcomeID := range sortedUnique(finiteCase.code.OutcomeIDs) {
			members = append(members, smtEqual(i, e.outcomeIndex[outcomeID]))
		}
		clauses = append(clauses, smtOr(members))
	}
	return smtAnd(clauses)
}

func (e *smtEncoder) testsFormula() string {
	clauses := make([]string, 0, len(e.model.tests))
	for _, test := range e.model.tests {
		clauses = append(clauses, e.predicate(test.Predicate))
	}
	return smtAnd(clauses)
}

func (e *smtEncoder) predicate(predicate semanticir.TestPredicate) string {
	switch predicate.Kind {
	case semanticir.PredicateTrue:
		return "true"
	case semanticir.PredicateFalse:
		return "false"
	case semanticir.PredicateAnd:
		children := make([]string, 0, len(predicate.Children))
		for _, child := range predicate.Children {
			children = append(children, e.predicate(child))
		}
		return smtAnd(children)
	case semanticir.PredicateOr:
		children := make([]string, 0, len(predicate.Children))
		for _, child := range predicate.Children {
			children = append(children, e.predicate(child))
		}
		return smtOr(children)
	case semanticir.PredicateNot:
		return smtNot(e.predicate(predicate.Children[0]))
	case semanticir.PredicateOutcomeIn:
		index := e.behaviorIndex(predicate.Observe.Behavior)
		members := make([]string, 0, len(predicate.Observe.OutcomeIDs))
		for _, outcomeID := range predicate.Observe.OutcomeIDs {
			members = append(members, smtEqual(index, e.outcomeIndex[outcomeID]))
		}
		return smtOr(members)
	case semanticir.PredicateOutcomeEqual:
		return fmt.Sprintf("(= b%d b%d)", e.behaviorIndex(*predicate.Left), e.behaviorIndex(*predicate.Right))
	case semanticir.PredicateRaises:
		index := e.behaviorIndex(predicate.Observe.Behavior)
		var members []string
		for _, outcomeID := range e.model.operationOutcomes[predicate.Observe.Behavior.OperationID] {
			outcome := e.model.outcomes[outcomeID]
			if outcome.Kind == semanticir.OutcomeRaise && outcome.ExceptionType == predicate.Observe.ExceptionType && (predicate.Observe.Message == "" || outcome.Message == predicate.Observe.Message) {
				members = append(members, smtEqual(index, e.outcomeIndex[outcomeID]))
			}
		}
		return smtOr(members)
	case semanticir.PredicateHasEffect:
		index := e.behaviorIndex(predicate.Observe.Behavior)
		var members []string
		var expected *semanticir.Literal
		if predicate.Observe.EffectValue != nil {
			value, err := evaluateExpression(*predicate.Observe.EffectValue, nil)
			if err != nil {
				return "false"
			}
			expected = &value
		}
		for _, outcomeID := range e.model.operationOutcomes[predicate.Observe.Behavior.OperationID] {
			for _, effect := range e.model.outcomes[outcomeID].Effects {
				if effect.Kind == predicate.Observe.EffectKind && effect.Target == predicate.Observe.EffectTarget {
					if expected != nil {
						if effect.Value == nil {
							continue
						}
						actual, err := evaluateExpression(*effect.Value, nil)
						if err != nil || !reflect.DeepEqual(*expected, actual) {
							continue
						}
					}
					members = append(members, smtEqual(index, e.outcomeIndex[outcomeID]))
					break
				}
			}
		}
		return smtOr(members)
	default:
		return "false"
	}
}

func (e *smtEncoder) behaviorIndex(ref semanticir.BehaviorRef) int {
	return e.caseIndexes[concreteCaseKey(ref.OperationID, e.model.operationDomains[ref.OperationID], ref.Conditions, ref.Inputs)]
}

func solveObligation(ctx context.Context, tool semanticir.ToolRef, invocation *SolverTranscript, encoder *smtEncoder, obligation semanticir.ProofObligation, formula string) (ObligationResult, SolverQuery, error) {
	script := encoder.script(formula, false)
	digest := sha256.Sum256([]byte(script))
	record := SolverQuery{Obligation: obligation, SMTLIB: script, SMTLIBSHA256: "sha256:" + hex.EncodeToString(digest[:])}
	output, err := runZ3(ctx, tool.Path, invocation, script)
	if err != nil {
		return ObligationResult{}, record, err
	}
	outputDigest := sha256.Sum256([]byte(output))
	record.Output = output
	record.OutputSHA256 = "sha256:" + hex.EncodeToString(outputDigest[:])
	status := firstOutputLine(output)
	record.Result = status
	result := ObligationResult{Obligation: obligation, Verdict: VerdictVerified, Method: "z3-qf-lia", Exhaustive: true, ReachableCases: uint64(len(encoder.model.cases))}
	switch status {
	case "unsat":
		return result, record, nil
	case "unknown":
		return ObligationResult{}, record, fmt.Errorf("Z3 returned unknown for %s", obligation)
	case "sat":
		modelScript := encoder.script(formula, true)
		modelDigest := sha256.Sum256([]byte(modelScript))
		record.ModelSMTLIB = modelScript
		record.ModelSMTLIBSHA256 = "sha256:" + hex.EncodeToString(modelDigest[:])
		modelOutput, err := runZ3(ctx, tool.Path, invocation, modelScript)
		if err != nil {
			return ObligationResult{}, record, err
		}
		modelOutputDigest := sha256.Sum256([]byte(modelOutput))
		record.ModelOutput = modelOutput
		record.ModelOutputSHA256 = "sha256:" + hex.EncodeToString(modelOutputDigest[:])
		vector, err := encoder.parseVector(modelOutput)
		if err != nil {
			return ObligationResult{}, record, err
		}
		witness, err := encoder.witness(obligation, vector)
		if err != nil {
			return ObligationResult{}, record, err
		}
		result.Verdict = VerdictNotVerified
		result.Witness = witness
		return result, record, nil
	default:
		return ObligationResult{}, record, fmt.Errorf("unexpected Z3 result %q for %s", status, obligation)
	}
}

func runZ3(ctx context.Context, path string, invocation *SolverTranscript, script string) (string, error) {
	environment := make([]string, len(invocation.Environment))
	for index, variable := range invocation.Environment {
		environment[index] = variable.Name + "=" + variable.Value
	}
	stdout, stderr, err := runHermetic(ctx, path, invocation.Argv, []byte(script), invocation.WorkingDirectory, environment, invocation.TimeoutMillis)
	if err != nil {
		return string(stdout), fmt.Errorf("run frozen Z3: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	if len(stderr) != 0 {
		return string(stdout), fmt.Errorf("frozen Z3 wrote diagnostics: %s", strings.TrimSpace(string(stderr)))
	}
	return string(stdout), nil
}

func runHermetic(ctx context.Context, path string, argv []string, stdin []byte, workingDirectory string, environment []string, timeoutMillis int64) ([]byte, []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	runContext, cancel := context.WithTimeout(ctx, time.Duration(timeoutMillis)*time.Millisecond)
	defer cancel()
	command := exec.CommandContext(runContext, path, argv...)
	command.Stdin = bytes.NewReader(stdin)
	command.Env = append([]string(nil), environment...)
	command.Dir = workingDirectory
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	command.WaitDelay = 2 * time.Second
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if runContext.Err() != nil {
		return stdout.Bytes(), stderr.Bytes(), runContext.Err()
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

func environmentHasExactEnvironment(environment *semanticir.EnvironmentModel, entries []semanticir.EnvironmentVariable, digest string) bool {
	if environment == nil {
		return false
	}
	for _, command := range environment.Commands {
		if command.ClearEnvironment && command.KillProcessGroup && command.EnvironmentDigest == digest && reflect.DeepEqual(command.Environment, entries) {
			return true
		}
	}
	return false
}

func frozenSolverEnvironment(environment *semanticir.EnvironmentModel) ([]semanticir.EnvironmentVariable, string, error) {
	if environment == nil {
		return nil, "", fmt.Errorf("typed environment model is missing")
	}
	for _, command := range environment.Commands {
		if command.State != semanticir.WorkspaceSolutionNewTests || !command.ClearEnvironment || !command.KillProcessGroup {
			continue
		}
		entries := make([]semanticir.EnvironmentVariable, len(command.Environment))
		copy(entries, command.Environment)
		digest, err := semanticir.Digest(entries)
		if err != nil || digest != command.EnvironmentDigest {
			return nil, "", fmt.Errorf("solution workspace command has invalid exact environment evidence: got %q, want %q: %v", digest, command.EnvironmentDigest, err)
		}
		return entries, digest, nil
	}
	return nil, "", fmt.Errorf("no hermetic solution workspace environment is frozen for the solver")
}

func firstOutputLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

func (e *smtEncoder) parseVector(output string) (behaviorVector, error) {
	matches := z3ValuePattern.FindAllStringSubmatch(output, -1)
	if len(matches) != len(e.model.cases) {
		return nil, fmt.Errorf("Z3 model returned %d behavior choices; want %d", len(matches), len(e.model.cases))
	}
	byIndex := make(map[int]int, len(matches))
	for _, match := range matches {
		caseIndex, _ := strconv.Atoi(match[1])
		outcomeIndex, err := strconv.Atoi(match[2])
		if err != nil || caseIndex < 0 || caseIndex >= len(e.model.cases) || outcomeIndex < 0 || outcomeIndex >= len(e.model.outcomeIDs) {
			return nil, fmt.Errorf("Z3 returned invalid behavior choice %q", match[0])
		}
		byIndex[caseIndex] = outcomeIndex
	}
	if len(byIndex) != len(e.model.cases) {
		return nil, fmt.Errorf("Z3 model omitted or duplicated a behavior variable")
	}
	vector := make(behaviorVector, len(e.model.cases))
	for i, finiteCase := range e.model.cases {
		outcomeID := e.model.outcomeIDs[byIndex[i]]
		if !containsString(e.model.operationOutcomes[finiteCase.operation], outcomeID) {
			return nil, fmt.Errorf("Z3 selected outcome %q outside operation %q's universe", outcomeID, finiteCase.operation)
		}
		vector[finiteCaseKey(e.model, finiteCase)] = outcomeID
	}
	return vector, nil
}

func (e *smtEncoder) witness(obligation semanticir.ProofObligation, vector behaviorVector) (*semanticir.Counterexample, error) {
	switch obligation {
	case semanticir.ObligationReferenceCorrectness:
		_, violation := specSatisfied(e.model, vector)
		if violation == nil {
			return nil, fmt.Errorf("Z3 reference model does not refute Spec")
		}
		testsPass, _, err := evaluateSuite(e.model, vector)
		if err != nil {
			return nil, err
		}
		return makeCounterexample(obligation, e.model, vector, violation.finiteCase, violation.requirement, testsPass, violation.finiteCase.code.Provenance), nil
	case semanticir.ObligationTestsSound:
		testsPass, _, err := evaluateSuite(e.model, vector)
		if err != nil || !testsPass {
			return nil, fmt.Errorf("Z3 soundness model does not satisfy TestsPass: %w", err)
		}
		_, violation := specSatisfied(e.model, vector)
		if violation == nil {
			return nil, fmt.Errorf("Z3 soundness model does not refute Spec")
		}
		return makeCounterexample(obligation, e.model, vector, violation.finiteCase, violation.requirement, true, violation.requirement.Provenance), nil
	case semanticir.ObligationTestsComplete:
		specPass, _ := specSatisfied(e.model, vector)
		testsPass, failedTest, err := evaluateSuite(e.model, vector)
		if err != nil || !specPass || testsPass {
			return nil, fmt.Errorf("Z3 fairness model does not satisfy Spec and refute TestsPass: %w", err)
		}
		highlight := fairnessHighlight(e.model, failedTest)
		requirement := &highlight.requirements[0]
		provenance := requirement.Provenance
		if failedTest != nil {
			provenance = failedTest.Provenance
		}
		return makeCounterexample(obligation, e.model, vector, highlight, requirement, false, provenance), nil
	case semanticir.ObligationReferenceAcceptance:
		testsPass, failedTest, err := evaluateSuite(e.model, vector)
		if err != nil || testsPass {
			return nil, fmt.Errorf("Z3 reference-acceptance model does not fix C and refute TestsPass: %w", err)
		}
		highlight := fairnessHighlight(e.model, failedTest)
		requirement := &highlight.requirements[0]
		provenance := requirement.Provenance
		if failedTest != nil {
			provenance = failedTest.Provenance
		}
		return makeCounterexample(obligation, e.model, vector, highlight, requirement, false, provenance), nil
	default:
		return nil, fmt.Errorf("unknown proof obligation %q", obligation)
	}
}

func smtEqual(variable, value int) string {
	return fmt.Sprintf("(= b%d %d)", variable, value)
}

func smtAnd(expressions []string) string {
	switch len(expressions) {
	case 0:
		return "true"
	case 1:
		return expressions[0]
	default:
		return "(and " + strings.Join(expressions, " ") + ")"
	}
}

func smtOr(expressions []string) string {
	switch len(expressions) {
	case 0:
		return "false"
	case 1:
		return expressions[0]
	default:
		return "(or " + strings.Join(expressions, " ") + ")"
	}
}

func smtNot(expression string) string {
	return "(not " + expression + ")"
}
