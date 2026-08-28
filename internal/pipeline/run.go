package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/HyperMarble/ray/internal/certificate"
	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/proof"
	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/taskbundle"
)

// Run executes Ray's sole production verification pipeline. It never accepts
// stage hooks, mutation results, sampled evidence, or caller-supplied verdicts.
func Run(ctx context.Context, request Request) Result {
	result := Result{Verdict: ProofBlocked}
	if ctx == nil {
		result.Stages = append(result.Stages, Stage{Name: StageFreeze, Status: StageBlocked, Diagnostic: []string{"nil execution context"}})
		result.Blockers = append(result.Blockers, "nil execution context")
		return result
	}
	root, err := filepath.Abs(request.Root)
	if err != nil {
		return blockResult(result, StageFreeze, time.Now(), fmt.Errorf("resolve task root: %w", err))
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return blockResult(result, StageFreeze, time.Now(), fmt.Errorf("resolve canonical task root: %w", err))
	}
	request.Root = root

	freezeStart := time.Now()
	cfg, configPath, configSource, err := loadConfig(request)
	if err != nil {
		return blockResult(result, StageFreeze, freezeStart, err)
	}
	manifest, err := taskbundle.FreezeContext(ctx, root, cfg.freezeRequest())
	if err != nil {
		return blockResult(result, StageFreeze, freezeStart, err)
	}
	if err := taskbundle.VerifyCurrent(root, manifest); err != nil {
		return blockResult(result, StageFreeze, freezeStart, fmt.Errorf("verify current frozen task: %w", err))
	}
	configRel, _ := filepath.Rel(root, configPath)
	configArtifact, err := manifestArtifactByPath(manifest, configRel, semanticir.ArtifactConfiguration)
	if err != nil {
		return blockResult(result, StageFreeze, freezeStart, err)
	}
	if configArtifact.Digest != semanticir.DigestBytes(configSource) {
		return blockResult(result, StageFreeze, freezeStart, fmt.Errorf("ray config changed between strict decoding and freeze"))
	}
	manifestDigest, err := taskbundle.ManifestDigest(manifest)
	if err != nil {
		return blockResult(result, StageFreeze, freezeStart, err)
	}
	result.ManifestDigest = manifestDigest
	result.Stages = append(result.Stages, Stage{
		Name: StageFreeze, Status: StageComplete, Evidence: []string{manifestDigest}, Duration: time.Since(freezeStart),
	})

	compileStart := time.Now()
	if err := taskbundle.VerifyCurrent(root, manifest); err != nil {
		return blockResult(result, StageCompileSpec, compileStart, fmt.Errorf("stale task before strict spec compilation: %w", err))
	}
	environment, err := lowerEnvironment(configArtifact, manifest)
	if err != nil {
		return blockResult(result, StageCompileSpec, compileStart, err)
	}
	task, compileBlockers := compileSkeleton(ctx, root, cfg, manifest, environment)
	if len(compileBlockers) != 0 {
		return blockResultMessages(result, StageCompileSpec, compileStart, compileBlockers)
	}
	skeletonDigest, err := semanticir.Digest(task)
	if err != nil {
		return blockResult(result, StageCompileSpec, compileStart, err)
	}
	result.Stages = append(result.Stages, Stage{
		Name: StageCompileSpec, Status: StageComplete,
		Evidence: []string{task.Spec.Digest, task.Instruction.Digest, task.SpecIRDigest, skeletonDigest},
		Duration: time.Since(compileStart),
	})

	diagnosticsStart := time.Now()
	diagnostics, diagnosticBlockers := runDiagnostics(ctx, root, cfg, manifest, task)
	if len(diagnosticBlockers) != 0 {
		return blockResultMessages(result, StageDiagnostics, diagnosticsStart, diagnosticBlockers)
	}
	diagnosticsDigest, err := semanticir.Digest(diagnostics)
	if err != nil {
		return blockResult(result, StageDiagnostics, diagnosticsStart, err)
	}
	result.Stages = append(result.Stages, Stage{
		Name: StageDiagnostics, Status: StageComplete, Evidence: []string{diagnosticsDigest}, Duration: time.Since(diagnosticsStart),
	})

	translateStart := time.Now()
	if err := taskbundle.VerifyCurrent(root, manifest); err != nil {
		return blockResult(result, StageTranslateReference, translateStart, fmt.Errorf("stale task before translation: %w", err))
	}
	records, translationBlockers := translateArtifacts(ctx, root, cfg, manifest, configArtifact, task)
	if len(translationBlockers) != 0 {
		return blockResultMessages(result, StageTranslateReference, translateStart, translationBlockers)
	}
	if blockers := validatePatchScope(root, manifest, records); len(blockers) != 0 {
		return blockResultMessages(result, StageTranslateReference, translateStart, blockers)
	}
	if blockers := attachArtifacts(task, records, semanticir.ArtifactCode); len(blockers) != 0 {
		return blockResultMessages(result, StageTranslateReference, translateStart, blockers)
	}
	translationEvidence := []string{skeletonDigest}
	if digest, err := semanticir.Digest(environment); err == nil {
		translationEvidence = append(translationEvidence, digest)
	} else {
		return blockResult(result, StageTranslateReference, translateStart, err)
	}
	for _, record := range records {
		if record.model.Kind != semanticir.ArtifactCode {
			continue
		}
		digest, err := semanticir.Digest(record.model)
		if err != nil {
			return blockResult(result, StageTranslateReference, translateStart, err)
		}
		translationEvidence = append(translationEvidence, digest)
		for _, replay := range record.exhaustiveReplay {
			replayDigest, err := semanticir.Digest(replay)
			if err != nil {
				return blockResult(result, StageTranslateReference, translateStart, err)
			}
			translationEvidence = append(translationEvidence, replayDigest)
		}
		for _, replay := range record.derivationReplay {
			replayDigest, err := semanticir.Digest(replay)
			if err != nil {
				return blockResult(result, StageTranslateReference, translateStart, err)
			}
			translationEvidence = append(translationEvidence, replayDigest)
		}
	}
	result.Stages = append(result.Stages, Stage{
		Name: StageTranslateReference, Status: StageComplete, Evidence: translationEvidence, Duration: time.Since(translateStart),
	})

	translateTestsStart := time.Now()
	if blockers := attachArtifacts(task, records, semanticir.ArtifactTests); len(blockers) != 0 {
		return blockResultMessages(result, StageTranslateTests, translateTestsStart, blockers)
	}
	testTranslationEvidence := []string{skeletonDigest}
	for _, record := range records {
		if record.model.Kind != semanticir.ArtifactTests {
			continue
		}
		digest, digestErr := semanticir.Digest(record.model)
		if digestErr != nil {
			return blockResult(result, StageTranslateTests, translateTestsStart, digestErr)
		}
		testTranslationEvidence = append(testTranslationEvidence, digest)
		for _, replay := range record.derivationReplay {
			replayDigest, replayErr := semanticir.Digest(replay)
			if replayErr != nil {
				return blockResult(result, StageTranslateTests, translateTestsStart, replayErr)
			}
			testTranslationEvidence = append(testTranslationEvidence, replayDigest)
		}
	}
	result.Stages = append(result.Stages, Stage{
		Name: StageTranslateTests, Status: StageComplete, Evidence: testTranslationEvidence, Duration: time.Since(translateTestsStart),
	})

	testIRStart := time.Now()
	if err := taskbundle.VerifyCurrent(root, manifest); err != nil {
		return blockResult(result, StageCompileTestIR, testIRStart, fmt.Errorf("stale task before static Test IR: %w", err))
	}
	testIRBlockers := compileTestSuite(ctx, task, records)
	if len(testIRBlockers) != 0 {
		return blockResultMessages(result, StageCompileTestIR, testIRStart, testIRBlockers)
	}
	if err := taskbundle.VerifyCurrent(root, manifest); err != nil {
		return blockResult(result, StageCompileTestIR, testIRStart, fmt.Errorf("task changed during static Test IR: %w", err))
	}
	if blockers := validateFinalTask(task); len(blockers) != 0 {
		return blockResultMessages(result, StageCompileTestIR, testIRStart, blockers)
	}
	irDigest, err := semanticir.Digest(task)
	if err != nil {
		return blockResult(result, StageCompileTestIR, testIRStart, err)
	}
	result.IRDigest = irDigest
	suiteDigest, err := semanticir.Digest(task.TestSuite)
	if err != nil {
		return blockResult(result, StageCompileTestIR, testIRStart, err)
	}
	result.Stages = append(result.Stages, Stage{
		Name: StageCompileTestIR, Status: StageComplete,
		Evidence: []string{suiteDigest, irDigest}, Duration: time.Since(testIRStart),
	})

	proofStart := time.Now()
	proofResult := proof.Verify(ctx, task)
	proofStages, proofStructureBlockers := stagesFromProof(proofResult, manifestDigest, irDigest, time.Since(proofStart))
	proofStructureBlockers = append(proofStructureBlockers, validateProofStructure(task, proofResult)...)
	result.Stages = append(result.Stages, proofStages...)
	for _, stage := range proofStages {
		if stage.Status == StageBlocked {
			result.Blockers = append(result.Blockers, stage.Diagnostic...)
		}
	}
	if len(proofStructureBlockers) != 0 {
		// Structure blockers are already present in the corresponding proof
		// stage diagnostics; retain this slice only to prevent execution from
		// consuming malformed proof output.
	}
	for _, counterexample := range proofResult.Counterexamples {
		result.Counterexamples = append(result.Counterexamples, counterexample.ID)
	}

	confirmStart := time.Now()
	var report executor.Report
	var confirmationBlockers []string
	if len(proofStructureBlockers) == 0 {
		if err := taskbundle.VerifyCurrent(root, manifest); err != nil {
			confirmationBlockers = append(confirmationBlockers, "stale task before confirmation: "+err.Error())
		} else {
			confirmationExecution, _, executionErr := executionEnvironment(root, manifest)
			if executionErr != nil {
				confirmationBlockers = append(confirmationBlockers, executionErr.Error())
			} else {
				frozen, freezeErr := frozenWitnessContext(root, manifest, task, records, proofResult, confirmationExecution)
				if freezeErr != nil {
					confirmationBlockers = append(confirmationBlockers, freezeErr.Error())
				} else {
					confirmationRequest, blockers := witnessConfirmationRequest(ctx, task, records, frozen, proofResult)
					confirmationBlockers = append(confirmationBlockers, blockers...)
					if len(blockers) == 0 {
						report = executor.ConfirmWitnessesIsolated(ctx, task, confirmationRequest)
						confirmationBlockers = append(confirmationBlockers, validateConfirmation(task, report, proofResult.Counterexamples)...)
					}
				}
			}
		}
	} else {
		confirmationBlockers = append(confirmationBlockers, proofStructureBlockers...)
	}
	if len(confirmationBlockers) == 0 {
		if err := taskbundle.VerifyCurrent(root, manifest); err != nil {
			confirmationBlockers = append(confirmationBlockers, "stale task after confirmation: "+err.Error())
		}
	}
	confirmStatus := StageComplete
	if len(confirmationBlockers) != 0 {
		confirmStatus = StageBlocked
		result.Blockers = append(result.Blockers, confirmationBlockers...)
	}
	confirmationEvidence := []string{}
	if len(confirmationBlockers) == 0 {
		digest, err := semanticir.Digest(report)
		if err != nil {
			confirmationBlockers = append(confirmationBlockers, err.Error())
			confirmStatus = StageBlocked
			result.Blockers = append(result.Blockers, err.Error())
		} else {
			confirmationEvidence = append(confirmationEvidence, digest)
		}
	}
	result.Stages = append(result.Stages, Stage{
		Name: StageConfirm, Status: confirmStatus, Evidence: confirmationEvidence,
		Diagnostic: append([]string(nil), confirmationBlockers...), Duration: time.Since(confirmStart),
	})

	certificateStart := time.Now()
	if err := taskbundle.VerifyCurrent(root, manifest); err != nil {
		return blockResult(result, StageCertificate, certificateStart, fmt.Errorf("stale task before certificate: %w", err))
	}
	cert, err := issueCertificate(manifest, task, records, proofResult, report, confirmationBlockers)
	if err != nil {
		return blockResult(result, StageCertificate, certificateStart, err)
	}
	if err := certificate.VerifyBindings(cert, manifest, irDigest); err != nil {
		return blockResult(result, StageCertificate, certificateStart, err)
	}
	certificatePath := cfg.certificatePath(root)
	if err := certificate.Write(certificatePath, cert); err != nil {
		return blockResult(result, StageCertificate, certificateStart, err)
	}
	written, err := certificate.Read(certificatePath)
	if err != nil {
		return blockResult(result, StageCertificate, certificateStart, err)
	}
	if err := certificate.VerifyBindings(written, manifest, irDigest); err != nil {
		return blockResult(result, StageCertificate, certificateStart, err)
	}
	if err := taskbundle.VerifyCurrent(root, manifest); err != nil {
		return blockResult(result, StageCertificate, certificateStart, fmt.Errorf("task changed during certificate issuance: %w", err))
	}
	result.CertificatePath = certificatePath
	result.Stages = append(result.Stages, Stage{
		Name: StageCertificate, Status: StageComplete, Evidence: []string{written.SHA256}, Duration: time.Since(certificateStart),
	})
	result.Verdict = Verdict(written.Verdict)
	if len(result.Blockers) != 0 && result.Verdict != ProofBlocked {
		// Defense in depth against a future certificate derivation regression.
		result.Verdict = ProofBlocked
	}
	return result
}

func validateProofStructure(task *semanticir.Task, result proof.Result) []string {
	obligations := []proof.ObligationResult{result.Reference, result.FalsePositive, result.Fairness, result.ReferenceAcceptance}
	wantVerdict := proof.VerdictVerified
	var witnesses []semanticir.Counterexample
	for _, obligation := range obligations {
		switch obligation.Verdict {
		case proof.VerdictProofBlocked:
			wantVerdict = proof.VerdictProofBlocked
		case proof.VerdictNotVerified:
			if wantVerdict != proof.VerdictProofBlocked {
				wantVerdict = proof.VerdictNotVerified
			}
			if obligation.Witness != nil {
				witnesses = append(witnesses, *obligation.Witness)
			}
		}
	}
	var blockers []string
	if result.Verdict != wantVerdict {
		blockers = append(blockers, fmt.Sprintf("proof aggregate verdict %q disagrees with obligation verdicts; want %q", result.Verdict, wantVerdict))
	}
	if wantVerdict != proof.VerdictProofBlocked && !result.Transcript.Complete {
		blockers = append(blockers, "completed proof obligations have an incomplete global transcript")
	}
	if wantVerdict != proof.VerdictProofBlocked && len(result.Blockers) != 0 {
		blockers = append(blockers, "completed proof result carries aggregate blockers")
	}
	if len(result.Counterexamples) != len(witnesses) {
		blockers = append(blockers, "proof counterexample list differs from obligation witnesses")
		return blockers
	}
	seen := map[string]bool{}
	for i := range witnesses {
		if witnesses[i].ID == "" || seen[witnesses[i].ID] {
			blockers = append(blockers, "proof obligation witnesses have empty or duplicate IDs")
			continue
		}
		seen[witnesses[i].ID] = true
		want, wantErr := semanticir.Digest(witnesses[i])
		got, gotErr := semanticir.Digest(result.Counterexamples[i])
		if wantErr != nil || gotErr != nil || want != got {
			blockers = append(blockers, fmt.Sprintf("proof counterexample %d differs from its obligation witness", i))
		}
		if diagnostics := semanticir.ValidateCounterexample(task, witnesses[i]); semanticir.HasErrors(diagnostics) {
			blockers = append(blockers, diagnosticStrings(diagnostics)...)
		}
	}
	return blockers
}

func stagesFromProof(result proof.Result, manifestDigest, irDigest string, duration time.Duration) ([]Stage, []string) {
	results := []proof.ObligationResult{result.Reference, result.FalsePositive, result.Fairness, result.ReferenceAcceptance}
	want := []struct {
		obligation semanticir.ProofObligation
		stage      StageName
	}{
		{semanticir.ObligationReferenceCorrectness, StageProofReference},
		{semanticir.ObligationTestsSound, StageProofTestsSound},
		{semanticir.ObligationTestsComplete, StageProofTestsComplete},
		{semanticir.ObligationReferenceAcceptance, StageReferenceAcceptance},
	}
	stages := make([]Stage, 0, 4)
	var blockers []string
	for index, obligation := range results {
		status := StageBlocked
		var diagnostics []string
		structurallyInvalid := false
		if obligation.Obligation != want[index].obligation {
			diagnostics = append(diagnostics, fmt.Sprintf("proof returned obligation %q at position %d, want %q", obligation.Obligation, index, want[index].obligation))
			structurallyInvalid = true
		} else {
			switch obligation.Verdict {
			case proof.VerdictVerified:
				if !obligation.Exhaustive || obligation.Method == "" || !result.Transcript.Complete {
					diagnostics = append(diagnostics, "proof claimed VERIFIED without complete exhaustive evidence")
					structurallyInvalid = true
				} else {
					status = StageComplete
				}
			case proof.VerdictNotVerified:
				if obligation.Witness == nil {
					diagnostics = append(diagnostics, "refuted proof has no semantic counterexample")
					structurallyInvalid = true
				} else {
					status = StageRefuted
				}
			case proof.VerdictProofBlocked:
				for _, blocker := range obligation.Blockers {
					diagnostics = append(diagnostics, blocker.Error())
				}
				if len(diagnostics) == 0 {
					diagnostics = append(diagnostics, "proof blocked without a reason")
				}
			default:
				diagnostics = append(diagnostics, "proof returned an unknown verdict")
				structurallyInvalid = true
			}
		}
		if structurallyInvalid {
			blockers = append(blockers, diagnostics...)
		}
		evidenceDigest, _ := semanticir.Digest(obligation)
		stages = append(stages, Stage{
			Name: want[index].stage, Status: status,
			Evidence: []string{manifestDigest, irDigest, evidenceDigest}, Diagnostic: diagnostics,
			Duration: duration,
		})
	}
	return stages, blockers
}

func validateConfirmation(task *semanticir.Task, report executor.Report, counterexamples []semanticir.Counterexample) []string {
	if err := executor.ValidateWitnessReport(task, report); err != nil {
		return []string{"invalid typed witness report: " + err.Error()}
	}
	wantReferencePass := true
	wantConfirmations := map[string]bool{}
	for _, counterexample := range counterexamples {
		if counterexample.Obligation == semanticir.ObligationReferenceAcceptance {
			wantReferencePass = false
			continue
		}
		wantConfirmations[counterexample.ID] = true
	}
	if report.ReferenceAcceptance == nil || report.ReferenceAcceptance.ObservedPass != wantReferencePass {
		return []string{"fresh T(C) result differs from the formal reference-acceptance obligation"}
	}
	if wantReferencePass && report.Status != executor.StatusConfirmed {
		return []string{"fresh T(C) execution did not confirm the accepted reference"}
	}
	if !wantReferencePass && report.Status != executor.StatusNotConfirmed {
		return []string{"fresh T(C) execution did not confirm the rejected reference witness"}
	}
	if len(report.Confirmations) != len(wantConfirmations) {
		return []string{"executor confirmation count does not match non-T(C) proof counterexamples"}
	}
	seen := map[string]bool{}
	for _, confirmation := range report.Confirmations {
		if confirmation.Status != executor.StatusConfirmed || confirmation.WitnessID == "" || seen[confirmation.WitnessID] || !wantConfirmations[confirmation.WitnessID] {
			return []string{"executor returned a missing, duplicate, or unconfirmed witness"}
		}
		switch confirmation.Mode {
		case executor.ConfirmationModeEdit:
			if err := executor.ValidateEditConfirmation(confirmation); err != nil {
				return []string{"invalid edit confirmation: " + err.Error()}
			}
		case executor.ConfirmationModeProbe:
			if err := executor.ValidateProbeConfirmation(confirmation); err != nil {
				return []string{"invalid probe confirmation: " + err.Error()}
			}
		case executor.ConfirmationModeBaselineWitness:
			if err := executor.ValidateBaselineWitnessConfirmation(confirmation); err != nil {
				return []string{"invalid baseline-witness confirmation: " + err.Error()}
			}
		default:
			return []string{fmt.Sprintf("executor returned unknown confirmation mode %q", confirmation.Mode)}
		}
		seen[confirmation.WitnessID] = true
	}
	for id := range wantConfirmations {
		if !seen[id] {
			return []string{fmt.Sprintf("proof counterexample %q was not executed", id)}
		}
	}
	return nil
}

func validateCleanBaseline(report executor.Report) error {
	isolation := report.BaselineIsolation
	if !report.Baseline.Passed || report.Baseline.Error != "" || isolation == nil {
		return fmt.Errorf("confirmation did not retain a clean isolated passing baseline")
	}
	if isolation.Error != "" || !isolation.IsolatedRemoved || !isolation.OriginalIntact ||
		!semanticir.ValidDigest(isolation.ExpectedSHA256) || isolation.OriginalBeforeSHA256 != isolation.ExpectedSHA256 ||
		isolation.CopyBeforeSHA256 != isolation.ExpectedSHA256 || isolation.OriginalAfterSHA256 != isolation.ExpectedSHA256 ||
		!semanticir.ValidDigest(isolation.CopyAfterSHA256) || isolation.OriginalRoot == "" || isolation.IsolatedRoot == "" ||
		isolation.OriginalRoot == isolation.IsolatedRoot {
		return fmt.Errorf("confirmation baseline isolation is incomplete or detached from the frozen workspace")
	}
	return nil
}

func blockResult(result Result, stage StageName, started time.Time, err error) Result {
	return blockResultMessages(result, stage, started, []string{err.Error()})
}

func blockResultMessages(result Result, stage StageName, started time.Time, messages []string) Result {
	result.Verdict = ProofBlocked
	result.Blockers = append(result.Blockers, messages...)
	result.Stages = append(result.Stages, Stage{
		Name: stage, Status: StageBlocked, Diagnostic: append([]string(nil), messages...), Duration: time.Since(started),
	})
	return result
}
