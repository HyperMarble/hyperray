package pipeline

import (
	"fmt"
	"sort"

	"github.com/HyperMarble/ray/internal/certificate"
	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/proof"
	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/taskbundle"
)

func issueCertificate(manifest taskbundle.Manifest, task *semanticir.Task, records []translationRecord, proofResult proof.Result, report executor.Report, confirmationBlockers []string) (certificate.Certificate, error) {
	if task == nil {
		return certificate.Certificate{}, fmt.Errorf("cannot issue certificate for nil Semantic IR")
	}
	irDigest, err := semanticir.Digest(task)
	if err != nil {
		return certificate.Certificate{}, err
	}
	if task.TestSuite == nil {
		return certificate.Certificate{}, fmt.Errorf("cannot issue certificate without static TestSuite evidence")
	}
	specIRDigest, err := semanticir.CanonicalSpecIRDigest(task)
	if err != nil {
		return certificate.Certificate{}, fmt.Errorf("digest canonical Spec IR: %w", err)
	}
	referenceIRDigest, err := semanticir.CanonicalReferenceIRDigest(task)
	if err != nil {
		return certificate.Certificate{}, fmt.Errorf("digest canonical reference IR: %w", err)
	}
	testIRDigest, err := semanticir.CanonicalTestIRDigest(task)
	if err != nil {
		return certificate.Certificate{}, fmt.Errorf("digest canonical Test IR: %w", err)
	}
	environmentIRDigest, err := semanticir.CanonicalEnvironmentIRDigest(task)
	if err != nil {
		return certificate.Certificate{}, fmt.Errorf("digest canonical environment IR: %w", err)
	}
	testSuiteDigest, err := semanticir.Digest(*task.TestSuite)
	if err != nil {
		return certificate.Certificate{}, fmt.Errorf("digest static TestSuite evidence: %w", err)
	}
	proofDigest, err := semanticir.Digest(proofResult)
	if err != nil {
		return certificate.Certificate{}, fmt.Errorf("digest full proof evidence: %w", err)
	}
	report = certificateExecutionReport(report)
	executionReportDigest, err := semanticir.Digest(report)
	if err != nil {
		return certificate.Certificate{}, fmt.Errorf("digest isolated execution report: %w", err)
	}
	exhaustiveReplays := certificateExhaustiveReplays(records)
	exhaustiveReplaysDigest := ""
	if len(exhaustiveReplays) != 0 {
		exhaustiveReplaysDigest, err = semanticir.Digest(exhaustiveReplays)
		if err != nil {
			return certificate.Certificate{}, fmt.Errorf("digest exhaustive replay evidence: %w", err)
		}
	}
	derivationReplays := certificateDerivationReplays(records)
	derivationReplaysDigest := ""
	if len(derivationReplays) != 0 {
		derivationReplaysDigest, err = semanticir.Digest(derivationReplays)
		if err != nil {
			return certificate.Certificate{}, fmt.Errorf("digest compiler derivation replay evidence: %w", err)
		}
	}
	document := certificate.Document{
		Manifest: manifest, SemanticIR: *task, IRDigest: irDigest,
		SpecIRDigest: specIRDigest, ReferenceIRDigest: referenceIRDigest,
		TestIRDigest: testIRDigest, EnvironmentIRDigest: environmentIRDigest,
		TestSuiteSHA256:         testSuiteDigest,
		Translation:             certificateCoverage(manifest, task, records),
		ProofEvidence:           proofResult,
		ProofEvidenceSHA256:     proofDigest,
		ExhaustiveReplays:       exhaustiveReplays,
		ExhaustiveReplaysSHA256: exhaustiveReplaysDigest,
		DerivationReplays:       derivationReplays,
		DerivationReplaysSHA256: derivationReplaysDigest,
		ExecutionReport:         report,
		ExecutionReportSHA256:   executionReportDigest,
	}
	obligations := []struct {
		certificate certificate.ProofObligation
		proof       proof.ObligationResult
	}{
		{certificate.ReferenceWithinSpec, proofResult.Reference},
		{certificate.TestsPassWithinSpec, proofResult.FalsePositive},
		{certificate.SpecWithinTestsPass, proofResult.Fairness},
		{certificate.ReferenceAccepted, proofResult.ReferenceAcceptance},
	}
	globalBlockers := append([]string(nil), confirmationBlockers...)
	for _, item := range obligations {
		if item.proof.Verdict == proof.VerdictProofBlocked {
			for _, blocker := range item.proof.Blockers {
				globalBlockers = append(globalBlockers, blocker.Error())
			}
			if len(item.proof.Blockers) == 0 {
				globalBlockers = append(globalBlockers, "proof blocked without a reason")
			}
		}
	}
	globalBlockers = uniqueStrings(globalBlockers)
	confirmations := make(map[string]executor.Confirmation, len(report.Confirmations))
	for _, confirmation := range report.Confirmations {
		confirmations[confirmation.WitnessID] = confirmation
	}
	for _, item := range obligations {
		obligationDigest, err := semanticir.Digest(item.proof)
		if err != nil {
			return certificate.Certificate{}, err
		}
		converted := certificate.ProofResult{
			Obligation:      item.certificate,
			EvidenceDigests: []string{manifest.SHA256, irDigest, specIRDigest, referenceIRDigest, testIRDigest, environmentIRDigest, testSuiteDigest, proofDigest, executionReportDigest, obligationDigest},
		}
		if exhaustiveReplaysDigest != "" {
			converted.EvidenceDigests = append(converted.EvidenceDigests, exhaustiveReplaysDigest)
		}
		if derivationReplaysDigest != "" {
			converted.EvidenceDigests = append(converted.EvidenceDigests, derivationReplaysDigest)
		}
		if len(globalBlockers) != 0 {
			converted.Status = certificate.ProofBlockedStatus
			converted.Blockers = append([]string(nil), globalBlockers...)
		} else {
			switch item.proof.Verdict {
			case proof.VerdictVerified:
				converted.Status = certificate.ProofProved
			case proof.VerdictNotVerified:
				if item.proof.Witness == nil {
					converted.Status = certificate.ProofBlockedStatus
					converted.Blockers = []string{"refuted proof omitted its semantic witness"}
					break
				}
				converted.Status = certificate.ProofRefuted
				converted.CounterexampleIDs = []string{item.proof.Witness.ID}
				var counterexample certificate.Counterexample
				if item.certificate == certificate.ReferenceAccepted {
					if report.ReferenceAcceptance == nil {
						converted.Status = certificate.ProofBlockedStatus
						converted.CounterexampleIDs = nil
						converted.Blockers = []string{"reference-acceptance witness lacks fresh T(C) evidence"}
						break
					}
					counterexample, err = certificateReferenceAcceptanceCounterexample(*item.proof.Witness, *report.ReferenceAcceptance)
				} else {
					confirmation, ok := confirmations[item.proof.Witness.ID]
					if !ok {
						converted.Status = certificate.ProofBlockedStatus
						converted.CounterexampleIDs = nil
						converted.Blockers = []string{"semantic witness lacks executable confirmation"}
						break
					}
					counterexample, err = certificateCounterexample(item.certificate, *item.proof.Witness, confirmation)
				}
				if err != nil {
					return certificate.Certificate{}, err
				}
				document.Counterexamples = append(document.Counterexamples, counterexample)
			case proof.VerdictProofBlocked:
				converted.Status = certificate.ProofBlockedStatus
				for _, blocker := range item.proof.Blockers {
					converted.Blockers = append(converted.Blockers, blocker.Error())
				}
				if len(converted.Blockers) == 0 {
					converted.Blockers = []string{"proof blocked without a reason"}
				}
			default:
				converted.Status = certificate.ProofBlockedStatus
				converted.Blockers = []string{"proof returned an unknown verdict"}
			}
		}
		document.Proofs = append(document.Proofs, converted)
	}
	return certificate.Issue(document)
}

// certificateExecutionReport matches certificate normalization before its
// digest is computed. An empty confirmation set remains nil, while witness
// records have one canonical order.
func certificateExecutionReport(report executor.Report) executor.Report {
	if len(report.Confirmations) == 0 {
		report.Confirmations = nil
		return report
	}
	report.Confirmations = append([]executor.Confirmation(nil), report.Confirmations...)
	sort.Slice(report.Confirmations, func(i, j int) bool {
		return report.Confirmations[i].WitnessID < report.Confirmations[j].WitnessID
	})
	return report
}

// certificateExhaustiveReplays returns the full executor records in the same
// canonical order used by certificate normalization. Computing the aggregate
// digest over that order avoids a translation-declaration ordering dependency.
func certificateExhaustiveReplays(records []translationRecord) []executor.ExhaustiveReplayEvidence {
	var replays []executor.ExhaustiveReplayEvidence
	for _, record := range records {
		replays = append(replays, record.exhaustiveReplay...)
	}
	sort.Slice(replays, func(i, j int) bool {
		left, right := replays[i].Plan.Evidence, replays[j].Plan.Evidence
		if left.Provenance.ArtifactID != right.Provenance.ArtifactID {
			return left.Provenance.ArtifactID < right.Provenance.ArtifactID
		}
		return left.ID < right.ID
	})
	return replays
}

func certificateDerivationReplays(records []translationRecord) []executor.DerivationReplayEvidence {
	var replays []executor.DerivationReplayEvidence
	for _, record := range records {
		replays = append(replays, record.derivationReplay...)
	}
	sort.Slice(replays, func(i, j int) bool {
		if replays[i].GraphSHA256 != replays[j].GraphSHA256 {
			return replays[i].GraphSHA256 < replays[j].GraphSHA256
		}
		return replays[i].Plan.ID < replays[j].Plan.ID
	})
	return replays
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func certificateCoverage(manifest taskbundle.Manifest, task *semanticir.Task, records []translationRecord) certificate.TranslationCoverage {
	items := []certificate.TranslationItem{
		{ArtifactID: task.Spec.ID, Location: task.Spec.Path, Status: certificate.TranslationTranslated},
		{ArtifactID: task.Instruction.ID, Location: task.Instruction.Path, Status: certificate.TranslationTranslated},
	}
	if task.Environment != nil {
		items = append(items, certificate.TranslationItem{
			ArtifactID: task.Environment.Configuration.ID, Location: task.Environment.Configuration.Path,
			Status: certificate.TranslationTranslated,
		})
		for _, source := range task.Environment.SourceArtifacts {
			items = append(items, certificate.TranslationItem{
				ArtifactID: source.ID, Location: source.Path, Status: certificate.TranslationTranslated,
			})
		}
	}
	for _, record := range records {
		location := record.request.Artifact.Path
		for _, artifact := range manifest.Artifacts {
			if artifact.ID == record.request.Artifact.ID {
				location = artifact.Path
				break
			}
		}
		items = append(items, certificate.TranslationItem{
			ArtifactID: record.request.Artifact.ID, Location: location,
			Status: certificate.TranslationTranslated,
		})
	}
	return certificate.TranslationCoverage{Total: len(items), Translated: len(items), Complete: true, Items: items}
}

func certificateCounterexample(obligation certificate.ProofObligation, witness semanticir.Counterexample, confirmation executor.Confirmation) (certificate.Counterexample, error) {
	witnessDigest, err := semanticir.Digest(witness)
	if err != nil {
		return certificate.Counterexample{}, fmt.Errorf("digest counterexample %q: %w", witness.ID, err)
	}
	confirmationDigest, err := semanticir.Digest(confirmation)
	if err != nil {
		return certificate.Counterexample{}, fmt.Errorf("digest confirmation %q: %w", witness.ID, err)
	}
	return certificate.Counterexample{
		ID: witness.ID, Obligation: obligation, Witness: witness, WitnessDigest: witnessDigest,
		Confirmation: confirmation, ConfirmationDigest: confirmationDigest,
	}, nil
}

func certificateReferenceAcceptanceCounterexample(witness semanticir.Counterexample, acceptance executor.ReferenceAcceptanceEvidence) (certificate.Counterexample, error) {
	witnessDigest, err := semanticir.Digest(witness)
	if err != nil {
		return certificate.Counterexample{}, fmt.Errorf("digest counterexample %q: %w", witness.ID, err)
	}
	acceptanceDigest, err := semanticir.Digest(acceptance)
	if err != nil {
		return certificate.Counterexample{}, fmt.Errorf("digest reference acceptance %q: %w", witness.ID, err)
	}
	return certificate.Counterexample{
		ID: witness.ID, Obligation: certificate.ReferenceAccepted, Witness: witness,
		WitnessDigest: witnessDigest, ConfirmationDigest: acceptanceDigest,
	}, nil
}
