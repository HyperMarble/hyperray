package speccompiler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"

	"github.com/HyperMarble/ray/internal/semanticir"
)

// AcceptanceRecordV1 is the canonical JSON author/reviewer attestation. The
// Markdown authoring template is UI only and is never parsed as proof input.
type AcceptanceRecordV1 struct {
	Schema                  string                                   `json:"schema"`
	TaskID                  string                                   `json:"task_id"`
	PhaseASpecIRDigest      string                                   `json:"phase_a_spec_ir_digest"`
	FrozenSemanticsDigest   string                                   `json:"frozen_semantics_digest"`
	EnvironmentConfigDigest string                                   `json:"environment_config_digest"`
	Manifest                []semanticir.AcceptanceSourceBinding     `json:"manifest"`
	Operations              []semanticir.AcceptanceOperationBinding  `json:"operations"`
	Domains                 []semanticir.AcceptanceDomainBinding     `json:"domains"`
	Constraints             []semanticir.AcceptanceConstraintBinding `json:"constraints"`
	Reviews                 []semanticir.AcceptanceReviewBinding     `json:"reviews"`
	Resolutions             []semanticir.AcceptanceResolution        `json:"resolutions"`
	NoDisagreements         bool                                     `json:"no_disagreements"`
	LintCommand             string                                   `json:"lint_command"`
	TestAccess              string                                   `json:"test_access"`
	Decision                semanticir.SpecAcceptanceDecision        `json:"decision"`
	ExpandedTableReview     semanticir.SpecAcceptanceDecision        `json:"expanded_table_review"`
	ExpectedGroundingReview semanticir.SpecAcceptanceDecision        `json:"expected_grounding_review"`
	AuthorIdentity          string                                   `json:"author_identity"`
	IndependentReviewer     string                                   `json:"independent_reviewer"`
	CompletedAtUTC          string                                   `json:"completed_at_utc"`
	SnapshotPath            string                                   `json:"snapshot_path"`
	FinalPath               string                                   `json:"final_path"`
	LedgerPath              string                                   `json:"ledger_path"`
	Complete                bool                                     `json:"complete"`
}

type AcceptanceLedgerEntry struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type AcceptanceLedgerV1 struct {
	Schema  string                  `json:"schema"`
	Entries []AcceptanceLedgerEntry `json:"entries"`
}

const AcceptanceLedgerSchemaV1 = "ray.spec-acceptance-ledger/v1"

type AcceptanceRequest struct {
	AuthoringRecord         semanticir.ArtifactRef `json:"authoring_record"`
	AuthoringRecordSource   []byte                 `json:"authoring_record_source"`
	DetachedLedger          semanticir.ArtifactRef `json:"detached_ledger"`
	DetachedLedgerSource    []byte                 `json:"detached_ledger_source"`
	PhaseASpec              semanticir.ArtifactRef `json:"phase_a_spec"`
	PhaseASpecSource        []byte                 `json:"phase_a_spec_source"`
	PhaseAEnvironment       semanticir.ArtifactRef `json:"phase_a_environment"`
	PhaseAEnvironmentSource []byte                 `json:"phase_a_environment_source"`
	PhaseATask              *semanticir.Task       `json:"-"`
	FinalTask               *semanticir.Task       `json:"-"`
}

// CompileAcceptance strict-decodes canonical JSON, verifies every frozen
// digest and ledger binding, checks Phase A/final semantic equality, and
// returns the sole typed acceptance evidence accepted by Semantic IR.
func CompileAcceptance(ctx context.Context, request AcceptanceRequest) (*semanticir.SpecAcceptanceEvidence, []semanticir.Diagnostic) {
	provenance := semanticir.NewProvenance(request.AuthoringRecord, semanticir.SourceLocation{Path: request.AuthoringRecord.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	fail := func(message string) (*semanticir.SpecAcceptanceEvidence, []semanticir.Diagnostic) {
		return nil, []semanticir.Diagnostic{{Severity: semanticir.SeverityError, Code: semanticir.DiagnosticInvalidInput, Message: message, Provenance: provenance}}
	}
	if err := ctx.Err(); err != nil {
		return fail(err.Error())
	}
	if request.PhaseATask == nil || request.FinalTask == nil {
		return fail("acceptance compile requires Phase-A and final compiled tasks")
	}
	for label, item := range []struct {
		ref    semanticir.ArtifactRef
		source []byte
	}{{request.AuthoringRecord, request.AuthoringRecordSource}, {request.DetachedLedger, request.DetachedLedgerSource}, {request.PhaseASpec, request.PhaseASpecSource}, {request.PhaseAEnvironment, request.PhaseAEnvironmentSource}} {
		if err := semanticir.VerifyArtifact(item.ref, item.source); err != nil {
			return fail(fmt.Sprintf("acceptance artifact %d: %v", label, err))
		}
	}
	if request.AuthoringRecord.Kind != semanticir.ArtifactSpecAuthoringRecord || request.DetachedLedger.Kind != semanticir.ArtifactSpecLedger || request.PhaseASpec.Kind != semanticir.ArtifactSpec || request.PhaseAEnvironment.Kind != semanticir.ArtifactEnvironment {
		return fail("acceptance artifact kinds are invalid")
	}
	var record AcceptanceRecordV1
	if err := decodeCanonicalJSON(request.AuthoringRecordSource, &record); err != nil {
		return fail("authoring record: " + err.Error())
	}
	var ledger AcceptanceLedgerV1
	if err := decodeCanonicalJSON(request.DetachedLedgerSource, &ledger); err != nil {
		return fail("detached ledger: " + err.Error())
	}
	var phaseEnvironment semanticir.PhaseAEnvironmentModel
	if err := decodeCanonicalJSON(request.PhaseAEnvironmentSource, &phaseEnvironment); err != nil {
		return fail("Phase-A environment: " + err.Error())
	}
	if ledger.Schema != AcceptanceLedgerSchemaV1 || len(ledger.Entries) != 3 {
		return fail("detached ledger must be schema v1 with exactly three entries")
	}
	wantLedger := []AcceptanceLedgerEntry{{Path: request.AuthoringRecord.Path, Digest: request.AuthoringRecord.Digest}, {Path: request.PhaseASpec.Path, Digest: request.PhaseASpec.Digest}, {Path: request.PhaseAEnvironment.Path, Digest: request.PhaseAEnvironment.Digest}}
	sort.Slice(wantLedger, func(i, j int) bool { return wantLedger[i].Path < wantLedger[j].Path })
	gotLedger := append([]AcceptanceLedgerEntry(nil), ledger.Entries...)
	sort.Slice(gotLedger, func(i, j int) bool { return gotLedger[i].Path < gotLedger[j].Path })
	if !reflect.DeepEqual(gotLedger, wantLedger) {
		return fail("detached ledger does not exactly bind the Phase-A spec, Phase-A environment, and authoring record")
	}
	phaseIRDigest, err := semanticir.Digest(request.PhaseATask)
	if err != nil {
		return fail("digest Phase-A task: " + err.Error())
	}
	phaseFrozen, err := semanticir.FrozenSpecSemanticsDigest(request.PhaseATask)
	if err != nil {
		return fail("digest Phase-A semantics: " + err.Error())
	}
	finalFrozen, err := semanticir.FrozenSpecSemanticsDigest(request.FinalTask)
	if err != nil {
		return fail("digest final semantics: " + err.Error())
	}
	if record.Schema != semanticir.SpecAuthoringRecordSchemaV1 || record.PhaseASpecIRDigest != phaseIRDigest || record.FrozenSemanticsDigest != phaseFrozen || phaseFrozen != finalFrozen {
		return fail("authoring record schema or frozen semantic digests are stale")
	}
	evidence := &semanticir.SpecAcceptanceEvidence{
		Schema: record.Schema, AuthoringRecord: request.AuthoringRecord, DetachedLedger: request.DetachedLedger, PhaseASpec: request.PhaseASpec,
		PhaseAEnvironment: request.PhaseAEnvironment, PhaseAEnvironmentModel: phaseEnvironment, FinalSpec: request.FinalTask.Spec, Instruction: request.FinalTask.Instruction, Environment: request.PhaseAEnvironment, TaskID: record.TaskID,
		PhaseASpecIRDigest: record.PhaseASpecIRDigest, FrozenSemanticsDigest: record.FrozenSemanticsDigest, EnvironmentConfigDigest: record.EnvironmentConfigDigest,
		Manifest: record.Manifest, Operations: record.Operations, Domains: record.Domains, Constraints: record.Constraints, Reviews: record.Reviews, Resolutions: record.Resolutions, NoDisagreements: record.NoDisagreements, LintCommand: record.LintCommand,
		TestAccess: record.TestAccess, Decision: record.Decision, ExpandedTableReview: record.ExpandedTableReview, ExpectedGroundingReview: record.ExpectedGroundingReview,
		AuthorIdentity: record.AuthorIdentity, IndependentReviewer: record.IndependentReviewer, CompletedAtUTC: record.CompletedAtUTC,
		SnapshotPath: record.SnapshotPath, FinalPath: record.FinalPath, LedgerPath: record.LedgerPath, Complete: record.Complete,
		Evidence: []semanticir.Provenance{provenance, semanticir.NewProvenance(request.DetachedLedger, semanticir.SourceLocation{Path: request.DetachedLedger.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated), semanticir.NewProvenance(request.PhaseASpec, semanticir.SourceLocation{Path: request.PhaseASpec.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated), semanticir.NewProvenance(request.PhaseAEnvironment, semanticir.SourceLocation{Path: request.PhaseAEnvironment.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)},
	}
	task := *request.FinalTask
	task.SpecAcceptance = evidence
	if diagnostics := semanticir.ValidateSpecAcceptance(&task); len(diagnostics) != 0 {
		return nil, diagnostics
	}
	return evidence, nil
}

// CompileSpecAcceptance is a descriptive alias used by orchestration code.
func CompileSpecAcceptance(ctx context.Context, request AcceptanceRequest) (*semanticir.SpecAcceptanceEvidence, []semanticir.Diagnostic) {
	return CompileAcceptance(ctx, request)
}

func DecodeAcceptanceRecord(source []byte) (AcceptanceRecordV1, error) {
	var value AcceptanceRecordV1
	err := decodeCanonicalJSON(source, &value)
	return value, err
}
func DecodeAcceptanceLedger(source []byte) (AcceptanceLedgerV1, error) {
	var value AcceptanceLedgerV1
	err := decodeCanonicalJSON(source, &value)
	return value, err
}
func DecodePhaseAEnvironment(source []byte) (semanticir.PhaseAEnvironmentModel, error) {
	var value semanticir.PhaseAEnvironmentModel
	err := decodeCanonicalJSON(source, &value)
	return value, err
}

func decodeCanonicalJSON(source []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing JSON content")
	}
	canonical, err := semanticir.CanonicalJSON(target)
	if err != nil {
		return err
	}
	if !bytes.Equal(source, canonical) {
		return fmt.Errorf("input is not exact canonical JSON")
	}
	return nil
}
