// Package certificate issues and verifies canonical, hash-bound Ray
// verification certificates. Certificates deliberately contain the complete
// frozen manifest so artifact hashes, workspace commands/results, environment
// identity, and tool versions travel with the proof result.
package certificate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/HyperMarble/ray/internal/executor"
	proofengine "github.com/HyperMarble/ray/internal/proof"
	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/taskbundle"
)

const SchemaVersion = "ray.verification-certificate/v3"

const maxCertificateBytes = 16 << 20

type Verdict string

const (
	Verified     Verdict = "VERIFIED"
	NotVerified  Verdict = "NOT VERIFIED"
	ProofBlocked Verdict = "PROOF BLOCKED"
)

type ProofObligation string

const (
	ReferenceWithinSpec ProofObligation = "reference-within-spec"
	TestsPassWithinSpec ProofObligation = "tests-pass-within-spec"
	SpecWithinTestsPass ProofObligation = "spec-within-tests-pass"
	ReferenceAccepted   ProofObligation = "reference-accepted-by-tests"
)

type ProofStatus string

const (
	ProofProved        ProofStatus = "PROVED"
	ProofRefuted       ProofStatus = "REFUTED"
	ProofBlockedStatus ProofStatus = "BLOCKED"
)

type TranslationStatus string

const (
	TranslationTranslated TranslationStatus = "TRANSLATED"
	TranslationBlocked    TranslationStatus = "BLOCKED"
)

type TranslationItem struct {
	ArtifactID string            `json:"artifact_id"`
	Location   string            `json:"location"`
	Status     TranslationStatus `json:"status"`
	Diagnostic string            `json:"diagnostic,omitempty"`
}

// TranslationCoverage is redundant by design: Total, Translated and Complete
// are checked against Items during issuance and verification. This makes the
// headline coverage human-readable without allowing it to disagree with the
// underlying evidence.
type TranslationCoverage struct {
	Total      int               `json:"total"`
	Translated int               `json:"translated"`
	Complete   bool              `json:"complete"`
	Items      []TranslationItem `json:"items"`
}

type ProofResult struct {
	Obligation        ProofObligation `json:"obligation"`
	Status            ProofStatus     `json:"status"`
	EvidenceDigests   []string        `json:"evidence_digests"`
	CounterexampleIDs []string        `json:"counterexample_ids"`
	Blockers          []string        `json:"blockers"`
}

type Counterexample struct {
	ID                 string                    `json:"id"`
	Obligation         ProofObligation           `json:"obligation"`
	Witness            semanticir.Counterexample `json:"witness"`
	WitnessDigest      string                    `json:"witness_digest"`
	Confirmation       executor.Confirmation     `json:"confirmation"`
	ConfirmationDigest string                    `json:"confirmation_digest"`
}

// Document is the unhashed certificate payload supplied by the pipeline.
// Issue validates and canonicalizes it, derives the only defensible verdict,
// and binds the result with SHA-256.
type Document struct {
	Manifest                taskbundle.Manifest                 `json:"manifest"`
	SemanticIR              semanticir.Task                     `json:"semantic_ir"`
	IRDigest                string                              `json:"ir_digest"`
	SpecIRDigest            string                              `json:"spec_ir_digest"`
	ReferenceIRDigest       string                              `json:"reference_ir_digest"`
	TestIRDigest            string                              `json:"test_ir_digest"`
	EnvironmentIRDigest     string                              `json:"environment_ir_digest"`
	TestSuiteSHA256         string                              `json:"test_suite_sha256"`
	Translation             TranslationCoverage                 `json:"translation"`
	ProofEvidence           proofengine.Result                  `json:"proof_evidence"`
	ProofEvidenceSHA256     string                              `json:"proof_evidence_sha256"`
	DerivationReplays       []executor.DerivationReplayEvidence `json:"derivation_replays,omitempty"`
	DerivationReplaysSHA256 string                              `json:"derivation_replays_sha256,omitempty"`
	ExhaustiveReplays       []executor.ExhaustiveReplayEvidence `json:"exhaustive_replays,omitempty"`
	ExhaustiveReplaysSHA256 string                              `json:"exhaustive_replays_sha256,omitempty"`
	ExecutionReport         executor.Report                     `json:"execution_report"`
	ExecutionReportSHA256   string                              `json:"execution_report_sha256"`
	Proofs                  []ProofResult                       `json:"proofs"`
	Counterexamples         []Counterexample                    `json:"counterexamples"`
}

type Certificate struct {
	Schema         string   `json:"schema"`
	ManifestSHA256 string   `json:"manifest_sha256"`
	Verdict        Verdict  `json:"verdict"`
	Document       Document `json:"document"`
	SHA256         string   `json:"sha256"`
}

// Issue normalizes set-like fields, verifies all cross-references, derives the
// verdict, and seals the certificate. No timestamp is included, so identical
// evidence produces byte-for-byte identical canonical JSON.
func Issue(document Document) (Certificate, error) {
	if err := taskbundle.Validate(document.Manifest); err != nil {
		return Certificate{}, fmt.Errorf("certificate manifest: %w", err)
	}
	document = normalizeDocument(document)
	if err := validateDocument(document); err != nil {
		return Certificate{}, err
	}
	manifestDigest, _ := taskbundle.ManifestDigest(document.Manifest)
	cert := Certificate{
		Schema: SchemaVersion, ManifestSHA256: manifestDigest,
		Verdict: deriveVerdict(document), Document: document,
	}
	digest, err := certificateBodyDigest(cert)
	if err != nil {
		return Certificate{}, err
	}
	cert.SHA256 = digest
	if err := Verify(cert); err != nil {
		return Certificate{}, fmt.Errorf("verify issued certificate: %w", err)
	}
	return cert, nil
}

// Verify checks a parsed certificate's schema, canonical ordering, embedded
// manifest integrity, proof/counterexample consistency, derived verdict, and
// certificate self-hash.
func Verify(cert Certificate) error {
	if cert.Schema != SchemaVersion {
		return fmt.Errorf("certificate schema %q, want %q", cert.Schema, SchemaVersion)
	}
	if err := taskbundle.Validate(cert.Document.Manifest); err != nil {
		return fmt.Errorf("certificate manifest: %w", err)
	}
	manifestDigest, _ := taskbundle.ManifestDigest(cert.Document.Manifest)
	if cert.ManifestSHA256 != manifestDigest {
		return errors.New("certificate manifest binding mismatch")
	}
	if !documentsEqual(cert.Document, normalizeDocument(cert.Document)) {
		return errors.New("certificate document is not in canonical order")
	}
	if err := validateDocument(cert.Document); err != nil {
		return err
	}
	wantVerdict := deriveVerdict(cert.Document)
	if cert.Verdict != wantVerdict {
		return fmt.Errorf("certificate verdict %q is inconsistent with evidence; want %q", cert.Verdict, wantVerdict)
	}
	if !validDigest(cert.SHA256) {
		return errors.New("certificate has invalid sha256")
	}
	wantDigest, err := certificateBodyDigest(cert)
	if err != nil {
		return err
	}
	if cert.SHA256 != wantDigest {
		return errors.New("certificate sha256 mismatch")
	}
	return nil
}

// VerifyBindings proves that the certificate was issued for the supplied
// frozen manifest and semantic IR. This is the stale-evidence boundary used by
// the pipeline before it reports the certificate's verdict.
func VerifyBindings(cert Certificate, manifest taskbundle.Manifest, irDigest string) error {
	if err := Verify(cert); err != nil {
		return err
	}
	if err := taskbundle.Validate(manifest); err != nil {
		return fmt.Errorf("binding manifest: %w", err)
	}
	if cert.ManifestSHA256 != manifest.SHA256 {
		return errors.New("certificate is bound to a different frozen manifest")
	}
	if cert.Document.IRDigest != irDigest {
		return errors.New("certificate is bound to a different semantic IR")
	}
	return nil
}

// CanonicalJSON emits the sole accepted representation: compact JSON with
// fixed struct-field order, lexicographically ordered maps (encoding/json's
// contract), and canonically sorted set-like slices.
func CanonicalJSON(cert Certificate) ([]byte, error) {
	if err := Verify(cert); err != nil {
		return nil, err
	}
	return json.Marshal(cert)
}

// VerifyBytes rejects both semantic tampering and non-canonical encodings such
// as extra whitespace, reordered object keys, duplicate fields, or trailing
// data.
func VerifyBytes(data []byte) (Certificate, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var cert Certificate
	if err := dec.Decode(&cert); err != nil {
		return Certificate{}, fmt.Errorf("decode certificate: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Certificate{}, errors.New("decode certificate: trailing JSON value")
		}
		return Certificate{}, fmt.Errorf("decode certificate trailing data: %w", err)
	}
	if err := Verify(cert); err != nil {
		return Certificate{}, err
	}
	canonical, err := json.Marshal(cert)
	if err != nil {
		return Certificate{}, err
	}
	if !bytes.Equal(data, canonical) {
		return Certificate{}, errors.New("certificate JSON is not canonical")
	}
	return cert, nil
}

// Write atomically replaces path with a verified canonical certificate.
func Write(path string, cert Certificate) error {
	data, err := CanonicalJSON(cert)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".ray-certificate-*")
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o644); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write certificate: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync certificate: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close certificate: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("install certificate: %w", err)
	}
	return nil
}

func Read(path string) (Certificate, error) {
	f, err := os.Open(path)
	if err != nil {
		return Certificate{}, fmt.Errorf("read certificate: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxCertificateBytes+1))
	if err != nil {
		return Certificate{}, fmt.Errorf("read certificate: %w", err)
	}
	if len(data) > maxCertificateBytes {
		return Certificate{}, fmt.Errorf("read certificate: exceeds %d bytes", maxCertificateBytes)
	}
	return VerifyBytes(data)
}

func validateDocument(document Document) error {
	if document.Manifest.Repository == nil {
		return errors.New("certificate requires a replayed repository base commit and ordered patch provenance")
	}
	if !validDigest(document.IRDigest) {
		return errors.New("certificate has invalid IR digest")
	}
	if err := validateSemanticEvidence(document); err != nil {
		return err
	}
	if err := validateDerivationReplays(document); err != nil {
		return err
	}
	if err := validateExhaustiveReplays(document); err != nil {
		return err
	}
	if err := validateTestSuiteEvidence(document); err != nil {
		return err
	}
	if err := validateExecutionReport(document); err != nil {
		return err
	}
	if err := validateProofEvidence(document); err != nil {
		return err
	}
	artifacts := make(map[string]taskbundle.Artifact, len(document.Manifest.Artifacts))
	for _, artifact := range document.Manifest.Artifacts {
		artifacts[artifact.ID] = artifact
	}
	if err := validateTranslation(document.Translation, artifacts); err != nil {
		return err
	}
	if len(document.Proofs) != 4 {
		return fmt.Errorf("certificate requires exactly 4 proof results, got %d", len(document.Proofs))
	}
	wantObligations := []ProofObligation{ReferenceWithinSpec, TestsPassWithinSpec, SpecWithinTestsPass, ReferenceAccepted}
	counterexamples := make(map[string]Counterexample, len(document.Counterexamples))
	for _, counterexample := range document.Counterexamples {
		if counterexample.ID == "" || counterexamples[counterexample.ID].ID != "" {
			return fmt.Errorf("certificate has empty or duplicate counterexample id %q", counterexample.ID)
		}
		if !validObligation(counterexample.Obligation) {
			return fmt.Errorf("counterexample %q has invalid obligation %q", counterexample.ID, counterexample.Obligation)
		}
		if err := validateCounterexample(document, counterexample); err != nil {
			return err
		}
		counterexamples[counterexample.ID] = counterexample
	}
	referenced := make(map[string]bool, len(counterexamples))
	for i, proof := range document.Proofs {
		if proof.Obligation != wantObligations[i] {
			return fmt.Errorf("certificate proof %d is %q, want %q", i, proof.Obligation, wantObligations[i])
		}
		if len(proof.EvidenceDigests) == 0 {
			return fmt.Errorf("proof %q has no evidence digests", proof.Obligation)
		}
		for _, digest := range proof.EvidenceDigests {
			if !validDigest(digest) {
				return fmt.Errorf("proof %q has invalid evidence digest %q", proof.Obligation, digest)
			}
		}
		requiredDigests := []string{
			document.Manifest.SHA256,
			document.IRDigest,
			document.SpecIRDigest,
			document.ReferenceIRDigest,
			document.TestIRDigest,
			document.EnvironmentIRDigest,
			document.TestSuiteSHA256,
			document.ProofEvidenceSHA256,
			document.ExecutionReportSHA256,
		}
		if document.ExhaustiveReplaysSHA256 != "" {
			requiredDigests = append(requiredDigests, document.ExhaustiveReplaysSHA256)
		}
		if document.DerivationReplaysSHA256 != "" {
			requiredDigests = append(requiredDigests, document.DerivationReplaysSHA256)
		}
		for _, required := range requiredDigests {
			if !containsString(proof.EvidenceDigests, required) {
				return fmt.Errorf("proof %q omits required typed evidence digest %q", proof.Obligation, required)
			}
		}
		if hasDuplicate(proof.EvidenceDigests) || hasDuplicate(proof.CounterexampleIDs) || hasDuplicate(proof.Blockers) {
			return fmt.Errorf("proof %q has duplicate evidence, counterexample, or blocker entries", proof.Obligation)
		}
		switch proof.Status {
		case ProofProved:
			if typedProofResult(document.ProofEvidence, proof.Obligation).Verdict != proofengine.VerdictVerified {
				return fmt.Errorf("proved obligation %q is not proved by the typed proof record", proof.Obligation)
			}
			if len(proof.CounterexampleIDs) != 0 || len(proof.Blockers) != 0 {
				return fmt.Errorf("proved obligation %q cannot contain counterexamples or blockers", proof.Obligation)
			}
		case ProofRefuted:
			typed := typedProofResult(document.ProofEvidence, proof.Obligation)
			if typed.Verdict != proofengine.VerdictNotVerified || typed.Witness == nil {
				return fmt.Errorf("refuted obligation %q is not refuted by the typed proof record", proof.Obligation)
			}
			if len(proof.CounterexampleIDs) == 0 || len(proof.Blockers) != 0 {
				return fmt.Errorf("refuted obligation %q requires counterexamples and no blockers", proof.Obligation)
			}
			for _, id := range proof.CounterexampleIDs {
				counterexample, ok := counterexamples[id]
				if !ok || counterexample.Obligation != proof.Obligation {
					return fmt.Errorf("proof %q references missing or mismatched counterexample %q", proof.Obligation, id)
				}
				witnessDigest, _ := semanticir.Digest(*typed.Witness)
				if counterexample.WitnessDigest != witnessDigest {
					return fmt.Errorf("proof %q counterexample %q differs from its typed proof witness", proof.Obligation, id)
				}
				referenced[id] = true
			}
		case ProofBlockedStatus:
			// Execution or freshness failures may conservatively downgrade a
			// completed typed proof, but a headline result may never be stronger
			// than its full typed record.
			if len(proof.Blockers) == 0 || len(proof.CounterexampleIDs) != 0 {
				return fmt.Errorf("blocked obligation %q requires blockers and no counterexamples", proof.Obligation)
			}
			for _, blocker := range proof.Blockers {
				if strings.TrimSpace(blocker) == "" {
					return fmt.Errorf("blocked obligation %q has empty blocker", proof.Obligation)
				}
			}
		default:
			return fmt.Errorf("proof %q has invalid status %q", proof.Obligation, proof.Status)
		}
	}
	for id := range counterexamples {
		if !referenced[id] {
			return fmt.Errorf("counterexample %q is not referenced by a refuted proof", id)
		}
	}
	return nil
}

func validateCounterexample(document Document, counterexample Counterexample) error {
	witness := counterexample.Witness
	if witness.ID != counterexample.ID {
		return fmt.Errorf("counterexample %q witness id is %q", counterexample.ID, witness.ID)
	}
	if string(witness.Obligation) != string(counterexample.Obligation) {
		return fmt.Errorf("counterexample %q witness obligation %q does not match %q", counterexample.ID, witness.Obligation, counterexample.Obligation)
	}
	if strings.TrimSpace(witness.OperationID) == "" || strings.TrimSpace(witness.RequirementID) == "" || len(witness.Choices) == 0 {
		return fmt.Errorf("counterexample %q has a truncated semantic witness", counterexample.ID)
	}
	if len(witness.ObservedOutcomes) != len(witness.Choices) || len(witness.ExpectedOutcomes) == 0 {
		return fmt.Errorf("counterexample %q has a truncated outcome vector", counterexample.ID)
	}
	seenChoices := map[string]bool{}
	for index, choice := range witness.Choices {
		conditions, err := semanticir.CanonicalJSON(choice.Behavior.Conditions)
		if err != nil {
			return fmt.Errorf("counterexample %q choice conditions: %w", counterexample.ID, err)
		}
		key := choice.Behavior.OperationID + "\x00" + string(conditions)
		if strings.TrimSpace(choice.Behavior.OperationID) == "" || strings.TrimSpace(choice.OutcomeID) == "" || seenChoices[key] {
			return fmt.Errorf("counterexample %q has an empty or duplicate behavior choice", counterexample.ID)
		}
		seenChoices[key] = true
		if witness.ObservedOutcomes[index] != choice.OutcomeID {
			return fmt.Errorf("counterexample %q observed outcome vector does not match its behavior choices", counterexample.ID)
		}
	}
	witnessDigest, err := semanticir.Digest(witness)
	if err != nil {
		return fmt.Errorf("counterexample %q witness digest: %w", counterexample.ID, err)
	}
	if counterexample.WitnessDigest != witnessDigest {
		return fmt.Errorf("counterexample %q witness digest mismatch", counterexample.ID)
	}
	if diagnostics := semanticir.ValidateCounterexample(&document.SemanticIR, witness); semanticir.HasErrors(diagnostics) {
		return fmt.Errorf("counterexample %q is not a complete witness for the bound semantic IR", counterexample.ID)
	}
	if counterexample.Obligation == ReferenceAccepted {
		acceptance := document.ExecutionReport.ReferenceAcceptance
		if acceptance == nil || acceptance.ObservedPass || acceptance.Status != executor.StatusNotConfirmed || !reflect.DeepEqual(acceptance.Plan.ReferenceChoices, witness.Choices) {
			return fmt.Errorf("counterexample %q is detached from the fresh failing T(C) execution", counterexample.ID)
		}
		if !reflect.DeepEqual(counterexample.Confirmation, executor.Confirmation{}) {
			return fmt.Errorf("counterexample %q T(C) witness must use the typed reference-acceptance record", counterexample.ID)
		}
		acceptanceDigest, digestErr := semanticir.Digest(*acceptance)
		if digestErr != nil || counterexample.ConfirmationDigest != acceptanceDigest {
			return fmt.Errorf("counterexample %q T(C) evidence digest mismatch", counterexample.ID)
		}
		return nil
	}

	confirmation := counterexample.Confirmation
	if confirmation.WitnessID != counterexample.ID {
		return fmt.Errorf("counterexample %q confirmation witness id is %q", counterexample.ID, confirmation.WitnessID)
	}
	if confirmation.Status != executor.StatusConfirmed || len(confirmation.Blockers) != 0 {
		return fmt.Errorf("counterexample %q lacks successful executable confirmation", counterexample.ID)
	}
	if confirmation.WitnessExecution == nil || !reflect.DeepEqual(confirmation.WitnessExecution.Plan.Witness, witness) {
		return fmt.Errorf("counterexample %q lacks its full typed witness execution plan", counterexample.ID)
	}
	if err := validateWitnessContextBindings(document, confirmation.WitnessExecution.Plan.Context); err != nil {
		return fmt.Errorf("counterexample %q model bindings: %w", counterexample.ID, err)
	}
	var validationErr error
	switch counterexample.Obligation {
	case ReferenceWithinSpec:
		validationErr = executor.ValidateReferenceWitnessConfirmation(confirmation)
	case TestsPassWithinSpec:
		validationErr = executor.ValidateFalsePositiveWitnessConfirmation(confirmation)
	case SpecWithinTestsPass:
		validationErr = executor.ValidateFalseNegativeWitnessConfirmation(confirmation)
	default:
		return fmt.Errorf("counterexample %q cannot use a witness confirmation for obligation %q", counterexample.ID, counterexample.Obligation)
	}
	if validationErr != nil {
		return fmt.Errorf("counterexample %q typed execution: %w", counterexample.ID, validationErr)
	}
	confirmationDigest, err := semanticir.Digest(confirmation)
	if err != nil {
		return fmt.Errorf("counterexample %q confirmation digest: %w", counterexample.ID, err)
	}
	if counterexample.ConfirmationDigest != confirmationDigest {
		return fmt.Errorf("counterexample %q confirmation digest mismatch", counterexample.ID)
	}
	return nil
}

func validateEditConfirmation(document Document, counterexample Counterexample, confirmation executor.Confirmation) error {
	wantExpected := semanticir.ExpectedSemantics{
		Conditions: counterexample.Witness.Conditions, OperationID: counterexample.Witness.OperationID,
		OutcomeIDs: counterexample.Witness.ObservedOutcomes, Choices: counterexample.Witness.Choices, TestPasses: counterexample.Witness.TestPasses,
	}
	for _, plan := range confirmation.Plans {
		bound := false
		for _, model := range document.SemanticIR.Artifacts {
			if model.Kind == semanticir.ArtifactCode && model.Artifact == plan.Artifact {
				bound = true
				break
			}
		}
		if !bound || !reflect.DeepEqual(plan.Expected, wantExpected) || len(plan.Steps) != 0 {
			return fmt.Errorf("counterexample %q full edit plan %q is source-detached, witness-truncated, or mixes probe steps", counterexample.ID, plan.ID)
		}
	}
	if err := validateConfirmationExecution(document, confirmation); err != nil {
		return fmt.Errorf("counterexample %q edit execution: %w", counterexample.ID, err)
	}
	plans := map[string]bool{}
	for _, planID := range confirmation.PlanIDs {
		if strings.TrimSpace(planID) == "" || plans[planID] {
			return fmt.Errorf("counterexample %q confirmation has empty or duplicate plan ids", counterexample.ID)
		}
		plans[planID] = true
	}
	materializedPlans := map[string]bool{}
	for _, materialization := range confirmation.Materializations {
		if !plans[materialization.PlanID] || materialization.WitnessID != counterexample.ID || materializedPlans[materialization.PlanID] {
			return fmt.Errorf("counterexample %q has mismatched materialization evidence", counterexample.ID)
		}
		if !materialization.Applied || !materialization.Restored || materialization.Error != "" {
			return fmt.Errorf("counterexample %q materialization %q was not applied and restored", counterexample.ID, materialization.PlanID)
		}
		for _, digest := range []string{materialization.FrozenSHA256, materialization.MaterializedSHA256, materialization.ObservedSHA256, materialization.RestoredSHA256} {
			if !validDigest(digest) {
				return fmt.Errorf("counterexample %q materialization %q has invalid digest", counterexample.ID, materialization.PlanID)
			}
		}
		if materialization.MaterializedSHA256 != materialization.ObservedSHA256 || materialization.RestoredSHA256 != materialization.FrozenSHA256 || len(materialization.Edits) == 0 {
			return fmt.Errorf("counterexample %q materialization %q has stale before/applied/observed/restored evidence", counterexample.ID, materialization.PlanID)
		}
		bound := false
		for _, model := range document.SemanticIR.Artifacts {
			if model.Kind == semanticir.ArtifactCode && model.Artifact.ID == materialization.ArtifactID && model.Artifact.Digest == materialization.FrozenSHA256 {
				bound = true
				break
			}
		}
		if !bound {
			return fmt.Errorf("counterexample %q materialization %q is not bound to a frozen semantic code artifact", counterexample.ID, materialization.PlanID)
		}
		for _, edit := range materialization.Edits {
			if edit.StartByte < 0 || edit.EndByte < edit.StartByte || !validDigest(edit.ExpectedSHA256) || !validDigest(edit.ReplacementSHA256) || edit.ExpectedSHA256 == edit.ReplacementSHA256 {
				return fmt.Errorf("counterexample %q materialization %q has truncated edit evidence", counterexample.ID, materialization.PlanID)
			}
		}
		materializedPlans[materialization.PlanID] = true
	}
	if len(materializedPlans) != len(plans) {
		return fmt.Errorf("counterexample %q confirmation omits plan materialization evidence", counterexample.ID)
	}
	command := confirmation.Command
	if err := validateCommandEvidence(command, counterexample.Witness.TestPasses); err != nil {
		return fmt.Errorf("counterexample %q has invalid command confirmation evidence", counterexample.ID)
	}
	return nil
}

func validateTranslation(coverage TranslationCoverage, artifacts map[string]taskbundle.Artifact) error {
	if coverage.Total <= 0 || coverage.Total != len(coverage.Items) {
		return fmt.Errorf("translation total %d does not match %d items", coverage.Total, len(coverage.Items))
	}
	translated := 0
	seen := map[string]bool{}
	for _, item := range coverage.Items {
		key := item.ArtifactID + "\x00" + item.Location
		if strings.TrimSpace(item.ArtifactID) == "" || strings.TrimSpace(item.Location) == "" || seen[key] {
			return errors.New("translation items require unique artifact/location pairs")
		}
		seen[key] = true
		artifact, ok := artifacts[item.ArtifactID]
		if !ok {
			return fmt.Errorf("translation item references unknown artifact %q", item.ArtifactID)
		}
		if item.Location != artifact.Path {
			return fmt.Errorf("translation item %q location %q differs from frozen artifact path %q", item.ArtifactID, item.Location, artifact.Path)
		}
		switch item.Status {
		case TranslationTranslated:
			translated++
			if item.Diagnostic != "" {
				return fmt.Errorf("translated item %s:%s must not carry a blocker diagnostic", item.ArtifactID, item.Location)
			}
		case TranslationBlocked:
			if strings.TrimSpace(item.Diagnostic) == "" {
				return fmt.Errorf("blocked translation %s:%s requires a diagnostic", item.ArtifactID, item.Location)
			}
		default:
			return fmt.Errorf("translation item %s:%s has invalid status %q", item.ArtifactID, item.Location, item.Status)
		}
	}
	if coverage.Translated != translated {
		return fmt.Errorf("translation translated count %d, want %d", coverage.Translated, translated)
	}
	if coverage.Complete != (translated == coverage.Total) {
		return errors.New("translation complete flag is inconsistent with item statuses")
	}
	return nil
}

func deriveVerdict(document Document) Verdict {
	if !document.Translation.Complete {
		return ProofBlocked
	}
	for _, proof := range document.Proofs {
		if proof.Status == ProofBlockedStatus {
			return ProofBlocked
		}
	}
	for _, proof := range document.Proofs {
		if proof.Status == ProofRefuted {
			return NotVerified
		}
	}
	return Verified
}

func normalizeDocument(document Document) Document {
	document.Translation.Items = append([]TranslationItem{}, document.Translation.Items...)
	sort.Slice(document.Translation.Items, func(i, j int) bool {
		a, b := document.Translation.Items[i], document.Translation.Items[j]
		if a.ArtifactID != b.ArtifactID {
			return a.ArtifactID < b.ArtifactID
		}
		return a.Location < b.Location
	})
	document.Proofs = append([]ProofResult{}, document.Proofs...)
	order := map[ProofObligation]int{ReferenceWithinSpec: 0, TestsPassWithinSpec: 1, SpecWithinTestsPass: 2, ReferenceAccepted: 3}
	for i := range document.Proofs {
		document.Proofs[i].EvidenceDigests = sortedStrings(document.Proofs[i].EvidenceDigests)
		document.Proofs[i].CounterexampleIDs = sortedStrings(document.Proofs[i].CounterexampleIDs)
		document.Proofs[i].Blockers = sortedStrings(document.Proofs[i].Blockers)
	}
	sort.Slice(document.Proofs, func(i, j int) bool {
		return order[document.Proofs[i].Obligation] < order[document.Proofs[j].Obligation]
	})
	document.Counterexamples = append([]Counterexample{}, document.Counterexamples...)
	sort.Slice(document.Counterexamples, func(i, j int) bool { return document.Counterexamples[i].ID < document.Counterexamples[j].ID })
	if document.ExecutionReport.Confirmations != nil {
		document.ExecutionReport.Confirmations = append([]executor.Confirmation(nil), document.ExecutionReport.Confirmations...)
		sort.Slice(document.ExecutionReport.Confirmations, func(i, j int) bool {
			return document.ExecutionReport.Confirmations[i].WitnessID < document.ExecutionReport.Confirmations[j].WitnessID
		})
	}
	if len(document.ExhaustiveReplays) != 0 {
		document.ExhaustiveReplays = append([]executor.ExhaustiveReplayEvidence{}, document.ExhaustiveReplays...)
		sort.Slice(document.ExhaustiveReplays, func(i, j int) bool {
			left, right := document.ExhaustiveReplays[i].Plan.Evidence, document.ExhaustiveReplays[j].Plan.Evidence
			if left.Provenance.ArtifactID != right.Provenance.ArtifactID {
				return left.Provenance.ArtifactID < right.Provenance.ArtifactID
			}
			return left.ID < right.ID
		})
	}
	if len(document.DerivationReplays) != 0 {
		document.DerivationReplays = append([]executor.DerivationReplayEvidence{}, document.DerivationReplays...)
		sort.Slice(document.DerivationReplays, func(i, j int) bool {
			if document.DerivationReplays[i].GraphSHA256 != document.DerivationReplays[j].GraphSHA256 {
				return document.DerivationReplays[i].GraphSHA256 < document.DerivationReplays[j].GraphSHA256
			}
			return document.DerivationReplays[i].Plan.ID < document.DerivationReplays[j].Plan.ID
		})
	}
	return document
}

func certificateBodyDigest(cert Certificate) (string, error) {
	cert.SHA256 = ""
	b, err := json.Marshal(cert)
	if err != nil {
		return "", fmt.Errorf("encode certificate: %w", err)
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func validObligation(obligation ProofObligation) bool {
	return obligation == ReferenceWithinSpec || obligation == TestsPassWithinSpec || obligation == SpecWithinTestsPass || obligation == ReferenceAccepted
}

func validDigest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil && value == strings.ToLower(value)
}

func sortedStrings(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

func hasDuplicate(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i-1] == values[i] {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	at := sort.SearchStrings(values, target)
	return at < len(values) && values[at] == target
}

func documentsEqual(a, b Document) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ab, bb)
}
