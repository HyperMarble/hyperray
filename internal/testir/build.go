package testir

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/proof"
	"github.com/HyperMarble/ray/internal/semanticir"
)

const maxAdvisoryCrossCheckVectors = uint64(64)

// CrossCheck exhaustively materializes, semantically revalidates, and executes
// every complete behavior vector in the finite task. Any unsupported vector,
// stale evidence, timeout, or cancellation blocks the advisory result. A
// resource ceiling records NOT RUN without weakening the authoritative static
// predicate. This function never authors that predicate.
func CrossCheck(ctx context.Context, request Request) Result {
	result := Result{Status: StatusBlocked}
	if ctx == nil {
		return blocked(result, "validation", "nil-context", "Test IR construction requires a non-nil context", "")
	}
	universe, blockers := validateRequest(request)
	if len(blockers) != 0 {
		result.Blockers = blockers
		return finalize(result)
	}
	result.Harness = cloneTaskEnvironment(request.Executor)
	result.SemanticTask = *request.Task
	taskDigest, err := semanticir.Digest(result.SemanticTask)
	if err != nil {
		return blocked(result, "evidence", "semantic-task-digest-failed", err.Error(), "")
	}
	result.SemanticTaskSHA256 = taskDigest
	result.Harness.WorkspaceSHA256 = universe.workspaceSHA
	result.WorkspaceSHA256 = universe.workspaceSHA
	harnessDigest, err := semanticir.Digest(result.Harness)
	if err != nil {
		return blocked(result, "evidence", "harness-digest-failed", err.Error(), "")
	}
	result.HarnessSHA256 = harnessDigest
	result.Predicate = universe.staticPredicate
	result.TestModels = append([]semanticir.ArtifactModel(nil), universe.testModels...)
	result.TestModelDigests = append([]semanticir.ArtifactModelDigest(nil), universe.testModelDigests...)
	result.StaticPredicateSHA256, err = semanticir.Digest(result.Predicate)
	if err != nil {
		return blocked(result, "evidence", "static-predicate-digest-failed", err.Error(), "")
	}
	result.TotalVectors = universe.totalVectors
	ceiling := maxAdvisoryCrossCheckVectors
	if request.MaxVectors > 0 && request.MaxVectors < ceiling {
		ceiling = request.MaxVectors
	}
	if universe.totalVectors > ceiling {
		result.Status = StatusNotRun
		result.NotRun = &NotRunEvidence{Code: "resource-bound", TotalVectors: universe.totalVectors, VectorCeiling: ceiling,
			Detail: fmt.Sprintf("optional exhaustive execution has %d vectors, above safe advisory ceiling %d; static proof remains authoritative", universe.totalVectors, ceiling)}
		return finalize(result)
	}
	workspaceMu.Lock()
	defer workspaceMu.Unlock()

	var plans []semanticir.EditPlan
	byVector := make(map[string]int)
	err = enumerateVectors(ctx, universe, func(choices []semanticir.BehaviorChoice) error {
		vector := VectorEvidence{Choices: cloneChoices(choices)}
		vector.ID = fmt.Sprintf("test-vector-%020d", len(result.Vectors))
		var digestErr error
		vector.CandidateSHA256, digestErr = semanticir.Digest(vector.Choices)
		if digestErr != nil {
			result.Blockers = append(result.Blockers, Blocker{Stage: "evidence", Code: "candidate-digest-failed", Detail: digestErr.Error()})
			return errBlocked
		}
		vector.PredictedTestsPass, err = proof.EvaluateTestPredicate(request.Task, universe.staticPredicate, vector.Choices)
		if err != nil {
			result.Blockers = append(result.Blockers, Blocker{Stage: "static-tests", Code: "static-predicate-evaluation-failed", VectorID: vector.ID, Detail: err.Error()})
			return errBlocked
		}
		if equalChoices(vector.Choices, universe.baseline) {
			vector.Baseline = true
			models, retranslations, isolation, blocker := buildBaseline(ctx, request, universe, vector)
			if blocker != nil {
				result.Blockers = append(result.Blockers, *blocker)
				return errBlocked
			}
			if blocker := verifyCandidateSemantics(universe, vector, models); blocker != nil {
				result.Blockers = append(result.Blockers, *blocker)
				return errBlocked
			}
			for index := range models {
				freshDigest, freshErr := semanticir.Digest(models[index])
				frozenDigest, frozenErr := semanticir.Digest(request.Artifacts[index].Model)
				if freshErr != nil || frozenErr != nil || freshDigest != frozenDigest {
					result.Blockers = append(result.Blockers, Blocker{Stage: "retranslation", Code: "stale-reference-model", VectorID: vector.ID,
						Detail: fmt.Sprintf("fresh baseline translation of artifact %q differs from the supplied frozen model", request.Artifacts[index].Frontend.Artifact.ID)})
					return errBlocked
				}
			}
			vector.Retranslations = retranslations
			vector.SemanticIsolation = isolation
		} else {
			candidatePlans, retranslations, isolation, blocker := buildCandidate(ctx, request, universe, vector)
			if blocker != nil {
				result.Blockers = append(result.Blockers, *blocker)
				return errBlocked
			}
			vector.Plans = clonePlans(candidatePlans)
			vector.Retranslations = retranslations
			vector.SemanticIsolation = isolation
			plans = append(plans, candidatePlans...)
		}
		byVector[vector.ID] = len(result.Vectors)
		result.Vectors = append(result.Vectors, vector)
		return nil
	})
	if err != nil {
		if err != errBlocked {
			result.Blockers = append(result.Blockers, Blocker{Stage: "enumeration", Code: "enumeration-failed", Detail: err.Error()})
		}
		return finalize(result)
	}
	if uint64(len(result.Vectors)) != universe.totalVectors {
		return blocked(result, "enumeration", "incomplete-enumeration",
			fmt.Sprintf("enumerated %d of %d behavior vectors", len(result.Vectors), universe.totalVectors), "")
	}
	predicted := make([]semanticir.TestVectorResult, len(result.Vectors))
	for index := range result.Vectors {
		predicted[index] = semanticir.TestVectorResult{Choices: cloneChoices(result.Vectors[index].Choices), Accepted: result.Vectors[index].PredictedTestsPass}
	}
	result.PredictedVectorSHA256, _, err = semanticir.TestVectorDigests(predicted)
	if err != nil {
		return blocked(result, "evidence", "predicted-vector-digest-failed", err.Error(), "")
	}

	// ExpectedTestPasses is derived from the independently translated static
	// verifier predicate before execution. The executor compares that exact bit
	// with the real pass signal. Repeat the complete universe at least twice;
	// any mismatch or changed bit blocks rather than choosing one run.
	repetitions := request.Repetitions
	if repetitions == 0 {
		repetitions = 2
	}
	if repetitions < 2 {
		return blocked(result, "execution", "insufficient-repetitions", "exact Test IR requires at least two identical complete verifier runs", "")
	}
	result.Repetitions = repetitions
	for run := 0; run < repetitions; run++ {
		execution := cloneTaskEnvironment(request.Executor)
		execution.WorkspaceSHA256 = universe.workspaceSHA
		report := executor.ConfirmIsolated(ctx, execution, plans)
		if run == 0 {
			result.Execution = report
		}
		result.Executions = append(result.Executions, report)
		observed, commands, isolations, blocker := observationsFromReport(report, result.Vectors, byVector)
		if blocker != nil {
			result.Blockers = append(result.Blockers, *blocker)
			return finalize(result)
		}
		vectorResults := make([]semanticir.TestVectorResult, len(result.Vectors))
		for index := range result.Vectors {
			vectorResults[index] = semanticir.TestVectorResult{Choices: cloneChoices(result.Vectors[index].Choices), Accepted: observed[index]}
			result.Vectors[index].Commands = append(result.Vectors[index].Commands, cloneCommandEvidence(commands[index]))
			result.Vectors[index].ExecutionIsolations = append(result.Vectors[index].ExecutionIsolations, isolations[index])
			if run == 0 {
				result.Vectors[index].Command = cloneCommandEvidence(commands[index])
				result.Vectors[index].ExecutionIsolation = isolations[index]
				result.Vectors[index].TestsPass = observed[index]
			} else if result.Vectors[index].TestsPass != observed[index] {
				return blocked(result, "execution", "nondeterministic-verifier",
					fmt.Sprintf("vector acceptance changed between exhaustive run 1 and run %d", run+1), result.Vectors[index].ID)
			}
		}
		runDigest, _, err := semanticir.TestVectorDigests(vectorResults)
		if err != nil {
			return blocked(result, "evidence", "run-digest-failed", err.Error(), "")
		}
		result.RunDigests = append(result.RunDigests, runDigest)
		if run > 0 && result.RunDigests[run] != result.RunDigests[0] {
			return blocked(result, "execution", "nondeterministic-verifier", "complete verifier truth-table digests differ between repetitions", "")
		}
	}
	for index := range result.Vectors {
		vector := &result.Vectors[index]
		if !vector.Baseline && vector.Command.StartedAt.IsZero() {
			return blocked(result, "execution", "unexecuted-vector", "a materialized behavior vector has no verifier execution evidence", vector.ID)
		}
		if vector.TestsPass != vector.PredictedTestsPass {
			return blocked(result, "static-tests", "static-execution-mismatch",
				fmt.Sprintf("independent test predicate predicts pass=%t but isolated verifier observed pass=%t", vector.PredictedTestsPass, vector.TestsPass), vector.ID)
		}
		if vector.PredictedTestsPass {
			result.AcceptedVectors++
		}
		if err := sealVector(vector); err != nil {
			return blocked(result, "evidence", "vector-digest-failed", err.Error(), vector.ID)
		}
	}
	result.ObservedVectorSHA256, _, err = semanticir.TestVectorDigests(vectorResults(result))
	if err != nil {
		return blocked(result, "evidence", "observed-vector-digest-failed", err.Error(), "")
	}
	if result.ObservedVectorSHA256 != result.PredictedVectorSHA256 {
		return blocked(result, "static-tests", "static-execution-digest-mismatch", "static and isolated executable truth tables differ", "")
	}
	result.Status = StatusComplete
	result.Blockers = nil
	return finalize(result)
}

// Build is retained as a compatibility wrapper. New callers should use
// CompileStatic for authority and CrossCheck only as advisory confirmation.
func Build(ctx context.Context, request Request) Result { return CrossCheck(ctx, request) }

func cloneTaskEnvironment(value executor.TaskEnvironment) executor.TaskEnvironment {
	copy := value
	copy.Command = append([]string(nil), value.Command...)
	copy.Environment = append([]string(nil), value.Environment...)
	if value.PassSignal.ExitCode != nil {
		code := *value.PassSignal.ExitCode
		copy.PassSignal.ExitCode = &code
	}
	if value.PassSignal.VerdictFile != nil {
		verdict := *value.PassSignal.VerdictFile
		copy.PassSignal.VerdictFile = &verdict
	}
	return copy
}

func observationsFromReport(report executor.Report, vectors []VectorEvidence, byVector map[string]int) ([]bool, []executor.CommandEvidence, []executor.IsolationEvidence, *Blocker) {
	if len(report.Blockers) != 0 || report.Baseline.Error != "" || !report.Baseline.Passed {
		detail := "mandatory frozen-reference verifier baseline did not pass cleanly"
		if len(report.Blockers) != 0 {
			detail = report.Blockers[0].Detail
			return nil, nil, nil, &Blocker{Stage: report.Blockers[0].Stage, Code: report.Blockers[0].Code, VectorID: report.Blockers[0].WitnessID, Detail: detail}
		}
		return nil, nil, nil, &Blocker{Stage: "execution", Code: "baseline-invalid", Detail: detail}
	}
	observed := make([]bool, len(vectors))
	commands := make([]executor.CommandEvidence, len(vectors))
	isolations := make([]executor.IsolationEvidence, len(vectors))
	baselineSeen := false
	seen := map[string]bool{}
	for index := range vectors {
		if !vectors[index].Baseline {
			continue
		}
		if baselineSeen {
			return nil, nil, nil, &Blocker{Stage: "execution", Code: "duplicate-baseline", VectorID: vectors[index].ID, Detail: "more than one vector is marked as the frozen reference"}
		}
		if report.BaselineIsolation == nil {
			return nil, nil, nil, &Blocker{Stage: "execution", Code: "missing-isolation", VectorID: vectors[index].ID, Detail: "baseline vector has no fresh isolated workspace evidence"}
		}
		baselineSeen = true
		observed[index] = report.Baseline.Passed
		commands[index] = report.Baseline
		isolations[index] = *report.BaselineIsolation
	}
	if !baselineSeen {
		return nil, nil, nil, &Blocker{Stage: "execution", Code: "missing-baseline", Detail: "no vector represents the frozen reference implementation"}
	}
	for _, confirmation := range report.Confirmations {
		index, ok := byVector[confirmation.WitnessID]
		if !ok || seen[confirmation.WitnessID] {
			return nil, nil, nil, &Blocker{Stage: "execution", Code: "unknown-confirmation", VectorID: confirmation.WitnessID, Detail: "executor returned an unknown or duplicate vector confirmation"}
		}
		seen[confirmation.WitnessID] = true
		if vectors[index].Baseline || confirmation.ObservedTestPasses == nil || len(confirmation.Blockers) != 0 {
			return nil, nil, nil, &Blocker{Stage: "execution", Code: "missing-pass-observation", VectorID: confirmation.WitnessID, Detail: "candidate execution did not produce one authoritative pass observation"}
		}
		if confirmation.Isolation == nil {
			return nil, nil, nil, &Blocker{Stage: "execution", Code: "missing-isolation", VectorID: confirmation.WitnessID, Detail: "candidate vector has no fresh isolated workspace evidence"}
		}
		observed[index] = *confirmation.ObservedTestPasses
		commands[index] = confirmation.Command
		isolations[index] = *confirmation.Isolation
	}
	for index, vector := range vectors {
		if !vector.Baseline && (!seen[vector.ID] || commands[index].StartedAt.IsZero()) {
			return nil, nil, nil, &Blocker{Stage: "execution", Code: "unexecuted-vector", VectorID: vector.ID, Detail: "a materialized behavior vector has no verifier execution evidence"}
		}
	}
	return observed, commands, isolations, nil
}

var errBlocked = fmt.Errorf("proof blocked")

func blocked(result Result, stage, code, detail, vectorID string) Result {
	result.Status = StatusBlocked
	result.Blockers = append(result.Blockers, Blocker{Stage: stage, Code: code, Detail: detail, VectorID: vectorID})
	return finalize(result)
}

func finalize(result Result) Result {
	for index := range result.Vectors {
		if err := sealVector(&result.Vectors[index]); err != nil {
			result.Status = StatusBlocked
			result.EvidenceSHA256 = ""
			result.Blockers = append(result.Blockers, Blocker{Stage: "evidence", Code: "vector-digest-failed", VectorID: result.Vectors[index].ID, Detail: err.Error()})
		}
	}
	if err := sealResult(&result); err != nil {
		result.Status = StatusBlocked
		result.EvidenceSHA256 = ""
		result.Blockers = append(result.Blockers, Blocker{Stage: "evidence", Code: "result-digest-failed", Detail: err.Error()})
	}
	return result
}

func enumerateVectors(ctx context.Context, universe *finiteUniverse, visit func([]semanticir.BehaviorChoice) error) error {
	choices := make([]semanticir.BehaviorChoice, len(universe.cases))
	var walk func(int) error
	walk = func(index int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if index == len(universe.cases) {
			return visit(cloneChoices(choices))
		}
		item := universe.cases[index]
		for _, outcomeID := range item.outcomeIDs {
			choices[index] = semanticir.BehaviorChoice{Behavior: cloneBehavior(item.behavior), OutcomeID: outcomeID}
			if err := walk(index + 1); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(0)
}

func totalVectorCount(cases []finiteCase) (uint64, error) {
	total := uint64(1)
	for _, item := range cases {
		if len(item.outcomeIDs) == 0 {
			return 0, fmt.Errorf("case %s has no operation-local outcomes", item.key)
		}
		if total > math.MaxUint64/uint64(len(item.outcomeIDs)) {
			return 0, fmt.Errorf("finite behavior-vector count exceeds uint64 accounting capacity")
		}
		total *= uint64(len(item.outcomeIDs))
	}
	return total, nil
}

func canonicalExpected(choices []semanticir.BehaviorChoice, testsPass bool) semanticir.ExpectedSemantics {
	first := choices[0]
	outcomes := make([]string, 0, len(choices))
	seen := map[string]bool{}
	for _, choice := range choices {
		if !seen[choice.OutcomeID] {
			seen[choice.OutcomeID] = true
			outcomes = append(outcomes, choice.OutcomeID)
		}
	}
	sort.Strings(outcomes)
	return semanticir.ExpectedSemantics{
		Conditions: cloneAssignment(first.Behavior.Conditions), OperationID: first.Behavior.OperationID,
		OutcomeIDs: outcomes, Choices: cloneChoices(choices), TestPasses: testsPass,
	}
}
