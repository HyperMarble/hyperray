package testir

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/proof"
	"github.com/HyperMarble/ray/internal/semanticir"
)

func sealVector(vector *VectorEvidence) error {
	copy := *vector
	copy.EvidenceSHA256 = ""
	digest, err := semanticir.Digest(copy)
	if err != nil {
		return err
	}
	vector.EvidenceSHA256 = digest
	return nil
}

func sealResult(result *Result) error {
	copy := *result
	copy.EvidenceSHA256 = ""
	digest, err := semanticir.Digest(copy)
	if err != nil {
		return err
	}
	result.EvidenceSHA256 = digest
	return nil
}

// ValidateEvidence detects mutation or truncation of any persisted Test IR
// truth-table record before it is consumed by proof or a certificate.
func ValidateEvidence(result Result) error {
	if result.HarnessSHA256 != "" || result.WorkspaceSHA256 != "" {
		harnessDigest, err := semanticir.Digest(result.Harness)
		if err != nil || harnessDigest != result.HarnessSHA256 || !semanticir.ValidDigest(result.WorkspaceSHA256) {
			return fmt.Errorf("Test IR frozen harness/workspace digest mismatch")
		}
	}
	for index, vector := range result.Vectors {
		copy := vector
		copy.EvidenceSHA256 = ""
		digest, err := semanticir.Digest(copy)
		if err != nil {
			return fmt.Errorf("vector %d evidence: %w", index, err)
		}
		if !semanticir.ValidDigest(vector.EvidenceSHA256) || digest != vector.EvidenceSHA256 {
			return fmt.Errorf("vector %q evidence digest mismatch", vector.ID)
		}
	}
	copy := result
	copy.EvidenceSHA256 = ""
	digest, err := semanticir.Digest(copy)
	if err != nil {
		return fmt.Errorf("result evidence: %w", err)
	}
	if !semanticir.ValidDigest(result.EvidenceSHA256) || digest != result.EvidenceSHA256 {
		return fmt.Errorf("Test IR result evidence digest mismatch")
	}
	if result.Status == StatusNotRun {
		if result.NotRun == nil || result.NotRun.Code != "resource-bound" || result.NotRun.TotalVectors != result.TotalVectors ||
			result.NotRun.VectorCeiling == 0 || result.TotalVectors <= result.NotRun.VectorCeiling || len(result.Blockers) != 0 ||
			len(result.Vectors) != 0 || len(result.Executions) != 0 || !result.Execution.Baseline.StartedAt.IsZero() {
			return fmt.Errorf("resource-bound advisory cross-check has invalid not-run evidence")
		}
		staticPredicate, staticDigests, err := compileModelPredicates(&result.SemanticTask, result.TestModels)
		staticDigest, digestErr := semanticir.Digest(result.Predicate)
		taskDigest, taskErr := semanticir.Digest(result.SemanticTask)
		if err != nil || digestErr != nil || taskErr != nil || !reflect.DeepEqual(staticPredicate, result.Predicate) ||
			!reflect.DeepEqual(staticDigests, result.TestModelDigests) || staticDigest != result.StaticPredicateSHA256 || taskDigest != result.SemanticTaskSHA256 {
			return fmt.Errorf("resource-bound advisory cross-check lost its authoritative static predicate evidence")
		}
		return nil
	}
	if result.Status == StatusComplete {
		if result.NotRun != nil || len(result.Blockers) != 0 || uint64(len(result.Vectors)) != result.TotalVectors {
			return fmt.Errorf("complete Test IR has blockers or an incomplete vector table")
		}
		if result.TotalVectors == 0 || result.Repetitions < 2 || len(result.Executions) != result.Repetitions || len(result.RunDigests) != result.Repetitions ||
			!reflect.DeepEqual(result.Execution, result.Executions[0]) || !semanticir.ValidDigest(result.HarnessSHA256) || !semanticir.ValidDigest(result.WorkspaceSHA256) ||
			!semanticir.ValidDigest(result.SemanticTaskSHA256) || !semanticir.ValidDigest(result.StaticPredicateSHA256) ||
			!semanticir.ValidDigest(result.PredictedVectorSHA256) || !semanticir.ValidDigest(result.ObservedVectorSHA256) {
			return fmt.Errorf("complete Test IR lacks repeated exhaustive execution evidence")
		}
		taskDigest, err := semanticir.Digest(result.SemanticTask)
		if err != nil || taskDigest != result.SemanticTaskSHA256 {
			return fmt.Errorf("complete Test IR semantic task digest mismatch")
		}
		staticPredicate, staticDigests, err := compileModelPredicates(&result.SemanticTask, result.TestModels)
		if err != nil || !reflect.DeepEqual(staticPredicate, result.Predicate) || !reflect.DeepEqual(staticDigests, result.TestModelDigests) {
			return fmt.Errorf("global TestPredicate is not the exact conjunction of complete frozen test models")
		}
		staticDigest, err := semanticir.Digest(result.Predicate)
		if err != nil || staticDigest != result.StaticPredicateSHA256 {
			return fmt.Errorf("static TestPredicate digest mismatch")
		}
		var accepted uint64
		baselineCount := 0
		ids := make(map[string]bool, len(result.Vectors))
		baselineIndex := -1
		planIDsByVector := make(map[string]map[string]bool, len(result.Vectors))
		for index, vector := range result.Vectors {
			if vector.ID == "" || ids[vector.ID] {
				return fmt.Errorf("truth table contains an empty or duplicate vector ID")
			}
			ids[vector.ID] = true
			choiceDigest, err := semanticir.Digest(vector.Choices)
			if err != nil || len(vector.Choices) == 0 || vector.CandidateSHA256 != choiceDigest {
				return fmt.Errorf("vector %q has missing choices or a semantic digest mismatch", vector.ID)
			}
			if len(vector.Retranslations) == 0 {
				return fmt.Errorf("vector %q has no semantic retranslation evidence", vector.ID)
			}
			isolation := vector.SemanticIsolation
			if isolation.SourceRoot != result.Harness.WorkspaceRoot || isolation.SourceSHA256 != result.WorkspaceSHA256 ||
				isolation.InitialIsolatedSHA256 != result.WorkspaceSHA256 || isolation.RestoredIsolatedSHA256 != result.WorkspaceSHA256 ||
				isolation.OriginalAfterSHA256 != result.WorkspaceSHA256 || isolation.IsolatedRoot == "" || !isolation.IsolatedWorkspaceRemoved {
				return fmt.Errorf("vector %q lacks exact restored semantic-isolation evidence", vector.ID)
			}
			for _, translation := range vector.Retranslations {
				if translation.ArtifactID == "" || translation.Coverage != semanticir.TranslationComplete ||
					!semanticir.ValidDigest(translation.CandidateSHA256) || !semanticir.ValidDigest(translation.ModelSHA256) ||
					len(translation.CategoryProofSHA256) == 0 {
					return fmt.Errorf("vector %q has incomplete retranslation evidence", vector.ID)
				}
				if translation.Model.Artifact.ID != translation.ArtifactID || translation.Model.Artifact.Digest != translation.CandidateSHA256 ||
					translation.Model.Coverage.Status != translation.Coverage {
					return fmt.Errorf("vector %q retranslation model is not bound to its candidate artifact", vector.ID)
				}
				modelDigest, err := semanticir.Digest(translation.Model)
				diagnostics := semanticir.ValidateArtifactModel(translation.Model)
				if err != nil || modelDigest != translation.ModelSHA256 || semanticir.HasErrors(diagnostics) {
					return fmt.Errorf("vector %q retranslation model is invalid or has a digest mismatch (digest error=%v, digest matches=%t): %s", vector.ID, err, modelDigest == translation.ModelSHA256, formatSemanticErrors(diagnostics))
				}
				items := categoryProofItems(translation.Model)
				if len(items) != len(translation.CategoryProofSHA256) {
					return fmt.Errorf("vector %q category-proof evidence is truncated", vector.ID)
				}
				for proofIndex, item := range items {
					proofDigest, err := semanticir.Digest(item.proof)
					if err != nil || proofDigest != translation.CategoryProofSHA256[proofIndex] {
						return fmt.Errorf("vector %q has an invalid category-proof digest", vector.ID)
					}
				}
			}
			for _, choice := range vector.Choices {
				matches := 0
				for _, translation := range vector.Retranslations {
					for _, behaviorCase := range translation.Model.Cases {
						if semanticir.BehaviorCaseKey(behaviorCase) == semanticir.BehaviorRefKey(choice.Behavior) &&
							len(behaviorCase.OutcomeIDs) == 1 && behaviorCase.OutcomeIDs[0] == choice.OutcomeID {
							matches++
						}
					}
				}
				if matches != 1 {
					return fmt.Errorf("vector %q choice has %d exact retranslation cases, want one", vector.ID, matches)
				}
			}
			if len(vector.Commands) != result.Repetitions || !reflect.DeepEqual(vector.Command, vector.Commands[0]) ||
				len(vector.ExecutionIsolations) != result.Repetitions || !reflect.DeepEqual(vector.ExecutionIsolation, vector.ExecutionIsolations[0]) {
				return fmt.Errorf("vector %q lacks repeated command evidence", vector.ID)
			}
			predicted, err := proof.EvaluateTestPredicate(&result.SemanticTask, result.Predicate, vector.Choices)
			if err != nil || predicted != vector.PredictedTestsPass || vector.PredictedTestsPass != vector.TestsPass {
				return fmt.Errorf("vector %q static predicate/executable pass bit mismatch", vector.ID)
			}
			for run, command := range vector.Commands {
				if len(command.Command) == 0 || command.WorkDir == "" || command.StartedAt.IsZero() || command.ExitCode == nil || command.Error != "" || command.Passed != vector.TestsPass {
					return fmt.Errorf("vector %q has incomplete or nondeterministic command/result evidence", vector.ID)
				}
				isolation := vector.ExecutionIsolations[run]
				remapped, err := isolatedHarness(result.Harness, isolation)
				if err != nil {
					return fmt.Errorf("vector %q has invalid execution isolation: %w", vector.ID, err)
				}
				remappedDigest, err := semanticir.Digest(remapped)
				if err != nil || !reflect.DeepEqual(command.Command, remapped.Command) || command.CommandSHA256 != remappedDigest ||
					command.WorkDir != remapped.WorkDir || command.Timeout != remapped.Timeout ||
					command.EnvironmentSHA256 != semanticir.DigestBytes([]byte(strings.Join(result.Harness.Environment, "\x00"))) {
					return fmt.Errorf("vector %q command evidence differs from the frozen harness", vector.ID)
				}
			}
			if vector.Baseline {
				baselineCount++
				baselineIndex = index
				if len(vector.Plans) != 0 {
					return fmt.Errorf("baseline vector %q contains edits", vector.ID)
				}
				if !vector.TestsPass {
					return fmt.Errorf("frozen reference baseline vector does not pass")
				}
			} else {
				if len(vector.Plans) == 0 {
					return fmt.Errorf("non-baseline vector %q has no materialization plans", vector.ID)
				}
				planIDs := make(map[string]bool, len(vector.Plans))
				for _, plan := range vector.Plans {
					if plan.ID == "" || plan.WitnessID != vector.ID || len(plan.Edits) == 0 || !reflect.DeepEqual(plan.Expected.Choices, vector.Choices) {
						return fmt.Errorf("vector %q contains an invalid materialization plan", vector.ID)
					}
					planIDs[plan.ID] = true
				}
				if len(planIDs) != len(vector.Plans) {
					return fmt.Errorf("vector %q contains duplicate materialization plan IDs", vector.ID)
				}
				planIDsByVector[vector.ID] = planIDs
			}
			if vector.TestsPass {
				accepted++
			}
		}
		if baselineCount != 1 || baselineIndex < 0 {
			return fmt.Errorf("truth table has %d frozen reference baselines, want one", baselineCount)
		}
		if accepted != result.AcceptedVectors {
			return fmt.Errorf("accepted-vector count mismatch")
		}
		vectorDigest, _, err := semanticir.TestVectorDigests(vectorResults(result))
		if err != nil {
			return fmt.Errorf("canonical vector digest: %w", err)
		}
		predictedResults := make([]semanticir.TestVectorResult, len(result.Vectors))
		for index, vector := range result.Vectors {
			predictedResults[index] = semanticir.TestVectorResult{Choices: cloneChoices(vector.Choices), Accepted: vector.PredictedTestsPass}
		}
		predictedDigest, _, err := semanticir.TestVectorDigests(predictedResults)
		if err != nil || predictedDigest != result.PredictedVectorSHA256 || vectorDigest != result.ObservedVectorSHA256 || predictedDigest != vectorDigest {
			return fmt.Errorf("static/executable vector evidence digest mismatch")
		}
		for run, report := range result.Executions {
			if len(report.Blockers) != 0 || report.Baseline.Error != "" || !report.Baseline.Passed ||
				!reflect.DeepEqual(report.Baseline, result.Vectors[baselineIndex].Commands[run]) {
				return fmt.Errorf("exhaustive run %d has invalid baseline evidence", run+1)
			}
			confirmations := make(map[string]executor.Confirmation, len(report.Confirmations))
			isolatedRoots := map[string]bool{}
			if report.BaselineIsolation == nil || !reflect.DeepEqual(*report.BaselineIsolation, result.Vectors[baselineIndex].ExecutionIsolations[run]) {
				return fmt.Errorf("exhaustive run %d baseline isolation evidence differs from its vector", run+1)
			}
			isolatedRoots[report.BaselineIsolation.IsolatedRoot] = true
			for _, confirmation := range report.Confirmations {
				if confirmation.WitnessID == "" || confirmations[confirmation.WitnessID].WitnessID != "" {
					return fmt.Errorf("exhaustive run %d contains an empty or duplicate confirmation", run+1)
				}
				confirmations[confirmation.WitnessID] = confirmation
			}
			if len(confirmations) != len(result.Vectors)-1 {
				return fmt.Errorf("exhaustive run %d confirms %d/%d non-baseline vectors", run+1, len(confirmations), len(result.Vectors)-1)
			}
			for vectorIndex, vector := range result.Vectors {
				if vector.Baseline {
					continue
				}
				confirmation, ok := confirmations[vector.ID]
				if !ok || confirmation.ObservedTestPasses == nil || len(confirmation.Blockers) != 0 ||
					*confirmation.ObservedTestPasses != vector.TestsPass || !reflect.DeepEqual(confirmation.Command, vector.Commands[run]) ||
					confirmation.Isolation == nil || !reflect.DeepEqual(*confirmation.Isolation, vector.ExecutionIsolations[run]) {
					return fmt.Errorf("run %d vector %q has no exact executor confirmation", run+1, vector.ID)
				}
				if isolatedRoots[confirmation.Isolation.IsolatedRoot] {
					return fmt.Errorf("exhaustive run %d reused an isolated workspace between vectors", run+1)
				}
				isolatedRoots[confirmation.Isolation.IsolatedRoot] = true
				planIDs := planIDsByVector[vector.ID]
				if len(confirmation.PlanIDs) != len(planIDs) {
					return fmt.Errorf("run %d vector %q plan/confirmation cardinality mismatch", run+1, vector.ID)
				}
				for _, planID := range confirmation.PlanIDs {
					if !planIDs[planID] {
						return fmt.Errorf("run %d vector %q references unknown plan %q", run+1, vector.ID, planID)
					}
				}
				_ = vectorIndex
			}
			if result.RunDigests[run] != vectorDigest {
				return fmt.Errorf("exhaustive run %d vector digest is inconsistent or nondeterministic", run+1)
			}
		}
	}
	return nil
}

func isolatedHarness(harness executor.TaskEnvironment, isolation executor.IsolationEvidence) (executor.TaskEnvironment, error) {
	if isolation.OriginalRoot != harness.WorkspaceRoot || isolation.ExpectedSHA256 != harness.WorkspaceSHA256 ||
		isolation.OriginalBeforeSHA256 != harness.WorkspaceSHA256 || isolation.CopyBeforeSHA256 != harness.WorkspaceSHA256 ||
		isolation.OriginalAfterSHA256 != harness.WorkspaceSHA256 || isolation.IsolatedRoot == "" ||
		!semanticir.ValidDigest(isolation.CopyAfterSHA256) || !isolation.IsolatedRemoved || !isolation.OriginalIntact || isolation.Error != "" {
		return executor.TaskEnvironment{}, fmt.Errorf("fresh copy/original/removal evidence is incomplete")
	}
	relativeWorkDir, err := filepath.Rel(harness.WorkspaceRoot, harness.WorkDir)
	if err != nil || filepath.IsAbs(relativeWorkDir) || relativeWorkDir == ".." || strings.HasPrefix(relativeWorkDir, ".."+string(filepath.Separator)) {
		return executor.TaskEnvironment{}, fmt.Errorf("frozen workdir cannot be remapped")
	}
	remapped := cloneTaskEnvironment(harness)
	remapped.WorkspaceRoot = isolation.IsolatedRoot
	remapped.WorkDir = filepath.Join(isolation.IsolatedRoot, relativeWorkDir)
	if harness.PassSignal.VerdictFile != nil && filepath.IsAbs(harness.PassSignal.VerdictFile.Path) {
		relativeVerdict, err := filepath.Rel(harness.WorkspaceRoot, harness.PassSignal.VerdictFile.Path)
		if err != nil || filepath.IsAbs(relativeVerdict) || relativeVerdict == ".." || strings.HasPrefix(relativeVerdict, ".."+string(filepath.Separator)) {
			return executor.TaskEnvironment{}, fmt.Errorf("frozen verdict path cannot be remapped")
		}
		remapped.PassSignal.VerdictFile.Path = filepath.Join(isolation.IsolatedRoot, relativeVerdict)
	}
	return remapped, nil
}
