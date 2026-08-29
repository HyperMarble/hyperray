// Package executor confirms proof counterexamples in a task-declared runtime.
//
// The executor is deliberately not a proof engine. It accepts only EditPlans
// already derived from semantic proof witnesses by a language frontend. It
// validates their frozen-artifact binding, materializes their exact byte-range
// replacements, observes the task's declared pass signal, and restores the
// artifact byte-for-byte. A confirmation is execution evidence attached to an
// existing witness; it can never manufacture a proof or an edit of its own.
package executor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

const (
	maxCapturedOutput = 1 << 20
	// A killed shell can leave a just-forked descendant holding stdout or
	// stderr open. Bound exec.Wait's pipe drain so cancellation cannot hang
	// restoration behind an unrelated descendant.
	processWaitDelay = 250 * time.Millisecond
)

// Status is an executable-confirmation status, not a proof verdict.
type Status string

const (
	StatusConfirmed    Status = "CONFIRMED"
	StatusNotConfirmed Status = "NOT CONFIRMED"
	StatusProofBlocked Status = "PROOF BLOCKED"
)

// ConfirmationMode identifies the executable witness forms. Edit
// confirmation materializes frontend-generated source edits; probe
// confirmation stages a frontend-generated direct harness without editing
// frozen source; baseline-witness confirmation binds a semantic witness that
// exactly equals the freshly retranslated clean solution vector.
type ConfirmationMode string

const (
	ConfirmationModeEdit            ConfirmationMode = "edit"
	ConfirmationModeProbe           ConfirmationMode = "probe"
	ConfirmationModeBaselineWitness ConfirmationMode = "baseline-witness"
	ConfirmationModeBaselineVector  ConfirmationMode = "baseline-vector"
)

// VerdictFile declares a file whose freshly written value is the verifier's
// pass signal. Path is resolved relative to TaskEnvironment.WorkDir unless it
// is absolute. Surrounding whitespace is ignored when comparing PassValue.
type VerdictFile struct {
	Path      string `json:"path"`
	PassValue string `json:"pass_value"`
}

// PassSignal declares exactly one authoritative pass signal. Use
// ExitCodeSignal or VerdictFileSignal to construct it.
type PassSignal struct {
	ExitCode    *int         `json:"exit_code,omitempty"`
	VerdictFile *VerdictFile `json:"verdict_file,omitempty"`
}

// ExitCodeSignal declares one process exit code as pass.
func ExitCodeSignal(code int) PassSignal {
	return PassSignal{ExitCode: &code}
}

// VerdictFileSignal declares a freshly created verdict file as the pass
// signal. A file left by an earlier run is removed and can never be consumed.
func VerdictFileSignal(path, passValue string) PassSignal {
	return PassSignal{VerdictFile: &VerdictFile{Path: path, PassValue: passValue}}
}

// TaskEnvironment is the complete execution configuration supplied by the
// frozen task. Command is argv, not an inferred shell command. Environment
// entries are task-declared KEY=VALUE overrides of the current process
// environment and are hashed into CommandEvidence without exposing values.
// WorkspaceRoot is the canonical frozen workspace boundary. WorkDir is the
// exact command cwd and must resolve beneath WorkspaceRoot. For compatibility,
// an empty WorkspaceRoot defaults to the resolved WorkDir.
type TaskEnvironment struct {
	Command          []string      `json:"command"`
	WorkspaceRoot    string        `json:"workspace_root,omitempty"`
	WorkspaceSHA256  string        `json:"workspace_sha256,omitempty"`
	WorkDir          string        `json:"work_dir"`
	Timeout          time.Duration `json:"timeout"`
	Environment      []string      `json:"environment,omitempty"`
	ExactEnvironment bool          `json:"exact_environment,omitempty"`
	PassSignal       PassSignal    `json:"pass_signal"`
}

// IsolationEvidence binds one execution to a fresh disposable copy of the
// frozen workspace and records both trees before and after execution. A
// differing CopyAfterSHA256 is permitted: arbitrary verifier side effects are
// isolated evidence, never state shared with another witness.
type IsolationEvidence struct {
	OriginalRoot         string `json:"original_root"`
	IsolatedRoot         string `json:"isolated_root"`
	ExpectedSHA256       string `json:"expected_sha256"`
	OriginalBeforeSHA256 string `json:"original_before_sha256"`
	CopyBeforeSHA256     string `json:"copy_before_sha256"`
	CopyAfterSHA256      string `json:"copy_after_sha256,omitempty"`
	OriginalAfterSHA256  string `json:"original_after_sha256,omitempty"`
	IsolatedRemoved      bool   `json:"isolated_removed"`
	OriginalIntact       bool   `json:"original_intact"`
	Error                string `json:"error,omitempty"`
}

// Blocker records why executable confirmation could not be completed.
type Blocker struct {
	Stage     string `json:"stage"`
	PlanID    string `json:"plan_id,omitempty"`
	WitnessID string `json:"witness_id,omitempty"`
	Code      string `json:"code"`
	Detail    string `json:"detail"`
}

// SignalEvidence records how one command result was interpreted.
type SignalEvidence struct {
	Kind                string `json:"kind"`
	ExpectedExitCode    *int   `json:"expected_exit_code,omitempty"`
	ObservedExitCode    *int   `json:"observed_exit_code,omitempty"`
	VerdictPath         string `json:"verdict_path,omitempty"`
	StaleVerdictRemoved bool   `json:"stale_verdict_removed,omitempty"`
	FreshVerdict        bool   `json:"fresh_verdict,omitempty"`
	ExpectedValueSHA256 string `json:"expected_value_sha256,omitempty"`
	ObservedValueSHA256 string `json:"observed_value_sha256,omitempty"`
}

// CommandEvidence is the auditable record of one baseline or witness run.
type CommandEvidence struct {
	Command           []string       `json:"command"`
	CommandSHA256     string         `json:"command_sha256"`
	WorkDir           string         `json:"work_dir"`
	Timeout           time.Duration  `json:"timeout"`
	EnvironmentSHA256 string         `json:"environment_sha256"`
	StartedAt         time.Time      `json:"started_at"`
	Duration          time.Duration  `json:"duration"`
	ExitCode          *int           `json:"exit_code,omitempty"`
	Stdout            string         `json:"stdout,omitempty"`
	Stderr            string         `json:"stderr,omitempty"`
	StdoutSHA256      string         `json:"stdout_sha256"`
	StderrSHA256      string         `json:"stderr_sha256"`
	SignalValueSHA256 string         `json:"signal_value_sha256,omitempty"`
	OutputTruncated   bool           `json:"output_truncated,omitempty"`
	TimedOut          bool           `json:"timed_out,omitempty"`
	Interrupted       bool           `json:"interrupted,omitempty"`
	Passed            bool           `json:"passed"`
	Signal            SignalEvidence `json:"signal"`
	Error             string         `json:"error,omitempty"`
}

// EditEvidence records exact materialization and restoration digests.
type EditEvidence struct {
	StartByte         int    `json:"start_byte"`
	EndByte           int    `json:"end_byte"`
	ExpectedSHA256    string `json:"expected_sha256"`
	ReplacementSHA256 string `json:"replacement_sha256"`
}

// MaterializationEvidence binds an executed plan to the bytes placed on disk
// and proves whether the original artifact was restored.
type MaterializationEvidence struct {
	PlanID             string         `json:"plan_id"`
	WitnessID          string         `json:"witness_id"`
	ArtifactID         string         `json:"artifact_id"`
	ArtifactPath       string         `json:"artifact_path"`
	FrozenSHA256       string         `json:"frozen_sha256"`
	MaterializedSHA256 string         `json:"materialized_sha256,omitempty"`
	ObservedSHA256     string         `json:"observed_sha256,omitempty"`
	RestoredSHA256     string         `json:"restored_sha256,omitempty"`
	OriginalSize       int            `json:"original_size"`
	MaterializedSize   int            `json:"materialized_size,omitempty"`
	Edits              []EditEvidence `json:"edits"`
	Applied            bool           `json:"applied"`
	Restored           bool           `json:"restored"`
	Error              string         `json:"error,omitempty"`
}

// Confirmation is executable evidence for exactly one pre-existing proof
// witness. ExpectedTestPasses comes from semanticir.EditPlan, never from the
// executor.
type Confirmation struct {
	PlanIDs            []string                     `json:"plan_ids"`
	Plans              []semanticir.EditPlan        `json:"plans,omitempty"`
	WitnessID          string                       `json:"witness_id"`
	Mode               ConfirmationMode             `json:"mode"`
	Status             Status                       `json:"status"`
	ExpectedTestPasses bool                         `json:"expected_test_passes"`
	ObservedTestPasses *bool                        `json:"observed_test_passes,omitempty"`
	Materializations   []MaterializationEvidence    `json:"materializations"`
	Probe              *ProbeEvidence               `json:"probe,omitempty"`
	BaselineWitness    *BaselineWitnessEvidence     `json:"baseline_witness,omitempty"`
	WitnessExecution   *WitnessConfirmationEvidence `json:"witness_execution,omitempty"`
	Isolation          *IsolationEvidence           `json:"isolation,omitempty"`
	Command            CommandEvidence              `json:"command"`
	Blockers           []Blocker                    `json:"blockers,omitempty"`
}

// Report contains a mandatory clean-baseline run followed by zero or more
// counterexample confirmations. StatusProofBlocked takes precedence over
// StatusNotConfirmed, which takes precedence over StatusConfirmed. Vacuous
// means no proof witnesses required execution; in that case CONFIRMED records
// only that the mandatory baseline passed and does not itself prove anything.
type Report struct {
	Status                Status                       `json:"status"`
	Vacuous               bool                         `json:"vacuous"`
	Baseline              CommandEvidence              `json:"baseline"`
	BaselineIsolation     *IsolationEvidence           `json:"baseline_isolation,omitempty"`
	ReferenceAcceptance   *ReferenceAcceptanceEvidence `json:"reference_acceptance,omitempty"`
	Confirmations         []Confirmation               `json:"confirmations"`
	ExpectedConfirmations int                          `json:"expected_confirmations,omitempty"`
	ConfirmationSHA256    []string                     `json:"confirmation_sha256,omitempty"`
	EvidenceSHA256        string                       `json:"evidence_sha256,omitempty"`
	Blockers              []Blocker                    `json:"blockers,omitempty"`
}

type preparedPlan struct {
	plan         semanticir.EditPlan
	path         string
	original     []byte
	materialized []byte
	mode         os.FileMode
	edits        []EditEvidence
}

type verdictSnapshot struct {
	path    string
	existed bool
	bytes   []byte
	mode    os.FileMode
}

// Confirm validates all plans against frozen source before it runs anything,
// requires the unmodified verifier baseline to pass, then materializes each
// proof witness with exact byte restoration. Plans sharing a WitnessID form
// one atomic, multi-artifact candidate. Because arbitrary verifier side
// effects can remain between groups, certificate-authoritative callers must
// use ConfirmIsolated instead. Restoration and verdict cleanup still run when
// ctx is canceled or a command times out.
func Confirm(ctx context.Context, task TaskEnvironment, plans []semanticir.EditPlan) (report Report) {
	report.Status = StatusProofBlocked
	report.Vacuous = len(plans) == 0

	resolvedWorkDir, resolvedWorkspaceRoot, blocker := validateTask(task)
	if blocker != nil {
		report.Blockers = append(report.Blockers, *blocker)
		return report
	}
	task.WorkDir = resolvedWorkDir
	task.WorkspaceRoot = resolvedWorkspaceRoot

	prepared, blockers := preparePlans(task, plans)
	if len(blockers) != 0 {
		report.Blockers = append(report.Blockers, blockers...)
		return report
	}

	snapshot, blocker := snapshotVerdictFile(task)
	if blocker != nil {
		report.Blockers = append(report.Blockers, *blocker)
		return report
	}
	if snapshot != nil {
		defer func() {
			if err := restoreVerdictFile(*snapshot); err != nil {
				report.Blockers = append(report.Blockers, Blocker{
					Stage: "cleanup", Code: "verdict-restore-failed", Detail: err.Error(),
				})
			}
			report.Status = aggregateStatus(report)
		}()
	} else {
		defer func() { report.Status = aggregateStatus(report) }()
	}

	if ctx == nil {
		report.Blockers = append(report.Blockers, Blocker{
			Stage: "baseline", Code: "nil-context", Detail: "execution context is nil",
		})
		return report
	}

	report.Baseline = runVerifier(ctx, task)
	if report.Baseline.Error != "" {
		report.Blockers = append(report.Blockers, Blocker{
			Stage: "baseline", Code: commandBlockerCode(report.Baseline), Detail: report.Baseline.Error,
		})
		verifyBaselineSources(prepared, &report)
		return report
	}
	if !report.Baseline.Passed {
		report.Blockers = append(report.Blockers, Blocker{
			Stage: "baseline", Code: "baseline-failed",
			Detail: "the unmodified reference did not produce the declared pass signal",
		})
		verifyBaselineSources(prepared, &report)
		return report
	}
	if !verifyBaselineSources(prepared, &report) {
		return report
	}

	for _, group := range groupPlans(prepared) {
		confirmation, safe := confirmGroup(ctx, task, group)
		report.Confirmations = append(report.Confirmations, confirmation)
		if len(confirmation.Blockers) != 0 {
			report.Blockers = append(report.Blockers, confirmation.Blockers...)
		}
		if !safe {
			// Continuing after a failed restoration would run later witnesses
			// against unknown bytes rather than the clean frozen artifact.
			break
		}
	}

	return report
}

func validateTask(task TaskEnvironment) (string, string, *Blocker) {
	block := func(code, detail string) (string, string, *Blocker) {
		return "", "", &Blocker{Stage: "configuration", Code: code, Detail: detail}
	}
	if len(task.Command) == 0 || strings.TrimSpace(task.Command[0]) == "" {
		return block("missing-command", "task declares no verifier command")
	}
	for _, argument := range task.Command {
		if strings.ContainsRune(argument, '\x00') || !utf8.ValidString(argument) {
			return block("invalid-command", "task command contains non-canonical text")
		}
	}
	if task.WorkDir == "" {
		return block("missing-workdir", "task declares no verifier working directory")
	}
	if !utf8.ValidString(task.WorkDir) || !utf8.ValidString(task.WorkspaceRoot) {
		return block("invalid-workdir", "task workspace paths contain non-canonical text")
	}
	workDir, err := filepath.Abs(task.WorkDir)
	if err != nil {
		return block("invalid-workdir", err.Error())
	}
	workDir, err = filepath.EvalSymlinks(workDir)
	if err != nil {
		return block("invalid-workdir", fmt.Sprintf("resolving declared working directory: %v", err))
	}
	workDir = filepath.Clean(workDir)
	info, err := os.Stat(workDir)
	if err != nil || !info.IsDir() {
		return block("invalid-workdir", fmt.Sprintf("declared working directory %q is not a readable directory", workDir))
	}
	workspaceRoot := workDir
	if task.WorkspaceRoot != "" {
		if !filepath.IsAbs(task.WorkspaceRoot) || filepath.Clean(task.WorkspaceRoot) != task.WorkspaceRoot {
			return block("invalid-workspace-root", "declared workspace root must be an absolute canonical path")
		}
		workspaceRoot, err = filepath.EvalSymlinks(task.WorkspaceRoot)
		if err != nil {
			return block("invalid-workspace-root", fmt.Sprintf("resolving declared workspace root: %v", err))
		}
		workspaceRoot = filepath.Clean(workspaceRoot)
		rootInfo, statErr := os.Stat(workspaceRoot)
		if statErr != nil || !rootInfo.IsDir() {
			return block("invalid-workspace-root", fmt.Sprintf("declared workspace root %q is not a readable directory", workspaceRoot))
		}
	}
	if !pathWithin(workspaceRoot, workDir) {
		return block("workdir-outside-workspace", fmt.Sprintf("declared working directory %q resolves outside workspace root %q", workDir, workspaceRoot))
	}
	if task.Timeout <= 0 {
		return block("invalid-timeout", "task timeout must be greater than zero")
	}
	for _, entry := range task.Environment {
		name, _, ok := strings.Cut(entry, "=")
		if !ok || name == "" || strings.ContainsRune(entry, '\x00') || !utf8.ValidString(entry) {
			return block("invalid-environment", fmt.Sprintf("invalid task environment entry %q", entry))
		}
	}
	exit, file := task.PassSignal.ExitCode, task.PassSignal.VerdictFile
	if (exit == nil) == (file == nil) {
		return block("invalid-pass-signal", "task must declare exactly one of exit code or verdict file")
	}
	if file != nil {
		if strings.TrimSpace(file.Path) == "" || !utf8.ValidString(file.Path) || !utf8.ValidString(file.PassValue) {
			return block("invalid-pass-signal", "verdict file path is empty")
		}
		if strings.TrimSpace(file.PassValue) == "" {
			return block("invalid-pass-signal", "verdict file pass value is empty")
		}
		if _, err := resolveVerdictPath(workspaceRoot, file.Path); err != nil {
			return block("verdict-outside-workspace", err.Error())
		}
	}
	return workDir, workspaceRoot, nil
}

func preparePlans(task TaskEnvironment, plans []semanticir.EditPlan) ([]preparedPlan, []Blocker) {
	ids := make(map[string]bool, len(plans))
	prepared := make([]preparedPlan, 0, len(plans))
	for _, plan := range plans {
		p, blocker := preparePlan(task, plan)
		if blocker != nil {
			return nil, []Blocker{*blocker}
		}
		if ids[plan.ID] {
			return nil, []Blocker{{
				Stage: "materialization", PlanID: plan.ID, WitnessID: plan.WitnessID,
				Code: "duplicate-plan", Detail: fmt.Sprintf("duplicate edit plan ID %q", plan.ID),
			}}
		}
		ids[plan.ID] = true
		prepared = append(prepared, p)
	}
	for _, group := range groupPlans(prepared) {
		paths := make(map[string]bool, len(group))
		artifacts := make(map[string]bool, len(group))
		expected := group[0].plan.Expected
		for _, plan := range group {
			if paths[plan.path] || artifacts[plan.plan.Artifact.ID] {
				return nil, []Blocker{{
					Stage: "materialization", PlanID: plan.plan.ID, WitnessID: plan.plan.WitnessID,
					Code:   "duplicate-witness-artifact",
					Detail: "one witness must provide at most one edit plan per frozen artifact",
				}}
			}
			paths[plan.path], artifacts[plan.plan.Artifact.ID] = true, true
			if !reflect.DeepEqual(plan.plan.Expected, expected) {
				return nil, []Blocker{{
					Stage: "materialization", PlanID: plan.plan.ID, WitnessID: plan.plan.WitnessID,
					Code:   "inconsistent-witness-semantics",
					Detail: "all edit plans for one proof witness must declare identical expected semantics",
				}}
			}
		}
	}
	return prepared, nil
}

// groupPlans preserves first-witness order while making every witness an
// atomic execution candidate. A frontend may need to edit several artifacts
// to realize one semantic witness; running those plans independently would
// execute states the proof never produced.
func groupPlans(prepared []preparedPlan) [][]preparedPlan {
	indices := make(map[string]int, len(prepared))
	groups := make([][]preparedPlan, 0, len(prepared))
	for _, plan := range prepared {
		index, ok := indices[plan.plan.WitnessID]
		if !ok {
			index = len(groups)
			indices[plan.plan.WitnessID] = index
			groups = append(groups, nil)
		}
		groups[index] = append(groups[index], plan)
	}
	return groups
}

func preparePlan(task TaskEnvironment, plan semanticir.EditPlan) (preparedPlan, *Blocker) {
	block := func(code, detail string) (preparedPlan, *Blocker) {
		return preparedPlan{}, &Blocker{
			Stage: "materialization", PlanID: plan.ID, WitnessID: plan.WitnessID,
			Code: code, Detail: detail,
		}
	}
	if plan.ID == "" || plan.WitnessID == "" {
		return block("unbound-edit-plan", "edit plan and proof witness IDs must both be non-empty")
	}
	if plan.Artifact.ID == "" || plan.Artifact.Path == "" || plan.Artifact.Kind != semanticir.ArtifactCode {
		return block("invalid-artifact", "edit plan must identify one frozen code artifact")
	}
	if !validDigest(plan.Artifact.Digest) {
		return block("invalid-artifact-digest", fmt.Sprintf("artifact %q has no canonical sha256 digest", plan.Artifact.ID))
	}
	if plan.Provenance.ArtifactID != plan.Artifact.ID ||
		plan.Provenance.ArtifactDigest != plan.Artifact.Digest ||
		(plan.Provenance.Translation != semanticir.TranslationTranslated &&
			plan.Provenance.Translation != semanticir.TranslationComplete) {
		return block("invalid-plan-provenance", "edit plan provenance is not completely bound to its frozen code artifact")
	}
	if plan.Expected.OperationID == "" || len(plan.Expected.OutcomeIDs) == 0 {
		return block("unbound-semantics", "edit plan must retain its expected operation and outcome IDs")
	}
	for _, outcomeID := range plan.Expected.OutcomeIDs {
		if outcomeID == "" {
			return block("unbound-semantics", "edit plan contains an empty expected outcome ID")
		}
	}
	if len(plan.Edits) == 0 {
		return block("empty-edit-plan", "edit plan contains no materialization edits")
	}

	candidate := plan.Artifact.Path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(task.WorkspaceRoot, candidate)
	}
	candidate = filepath.Clean(candidate)
	// Reject a symlink at the leaf itself. Parent symlinks are evaluated
	// below: they are allowed only when they still resolve inside WorkspaceRoot.
	candidateInfo, err := os.Lstat(candidate)
	if err != nil {
		return block("artifact-unreadable", fmt.Sprintf("reading artifact metadata: %v", err))
	}
	if candidateInfo.Mode()&os.ModeSymlink != 0 {
		return block("artifact-not-regular", fmt.Sprintf("artifact %q is a symlink", candidate))
	}
	path, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return block("artifact-unreadable", fmt.Sprintf("resolving artifact path: %v", err))
	}
	path = filepath.Clean(path)
	if !pathWithin(task.WorkspaceRoot, path) {
		return block("artifact-outside-workdir", fmt.Sprintf("artifact %q resolves outside frozen workspace %q", plan.Artifact.Path, task.WorkspaceRoot))
	}
	if task.PassSignal.VerdictFile != nil && samePath(path, verdictPath(task)) {
		return block("artifact-is-verdict", "source artifact and verdict file resolve to the same path")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return block("artifact-unreadable", fmt.Sprintf("reading artifact metadata: %v", err))
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return block("artifact-not-regular", fmt.Sprintf("artifact %q is not a regular non-symlink file", path))
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return block("artifact-unreadable", err.Error())
	}
	if got := digestBytes(original); got != plan.Artifact.Digest {
		return block("stale-artifact", fmt.Sprintf("artifact digest is %s, edit plan requires %s", got, plan.Artifact.Digest))
	}

	edits := append([]semanticir.ByteRangeReplacement(nil), plan.Edits...)
	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].StartByte == edits[j].StartByte {
			return edits[i].EndByte < edits[j].EndByte
		}
		return edits[i].StartByte < edits[j].StartByte
	})
	for i, edit := range edits {
		if edit.StartByte < 0 || edit.EndByte < edit.StartByte || edit.EndByte > len(original) {
			return block("invalid-edit-range", fmt.Sprintf("edit %d range [%d,%d) is outside %d artifact bytes", i, edit.StartByte, edit.EndByte, len(original)))
		}
		if i > 0 && (edit.StartByte < edits[i-1].EndByte || edit.StartByte == edits[i-1].StartByte) {
			return block("overlapping-edits", fmt.Sprintf("edit %d overlaps or is ambiguous with edit %d", i, i-1))
		}
		if !bytes.Equal(original[edit.StartByte:edit.EndByte], edit.ExpectedBytes) {
			return block("stale-edit-range", fmt.Sprintf("edit %d expected bytes do not match frozen artifact range", i))
		}
	}

	materialized := applyEdits(original, edits)
	if bytes.Equal(materialized, original) {
		return block("no-op-edit-plan", "edit plan does not materially change the artifact")
	}
	evidence := make([]EditEvidence, 0, len(edits))
	for _, edit := range edits {
		evidence = append(evidence, EditEvidence{
			StartByte: edit.StartByte, EndByte: edit.EndByte,
			ExpectedSHA256: digestBytes(edit.ExpectedBytes), ReplacementSHA256: digestBytes(edit.Replacement),
		})
	}
	plan.Edits = edits
	return preparedPlan{
		plan: plan, path: path, original: original, materialized: materialized,
		mode: info.Mode().Perm(), edits: evidence,
	}, nil
}

func applyEdits(original []byte, edits []semanticir.ByteRangeReplacement) []byte {
	capacity := len(original)
	for _, edit := range edits {
		capacity += len(edit.Replacement) - (edit.EndByte - edit.StartByte)
	}
	out := make([]byte, 0, capacity)
	position := 0
	for _, edit := range edits {
		out = append(out, original[position:edit.StartByte]...)
		out = append(out, edit.Replacement...)
		position = edit.EndByte
	}
	return append(out, original[position:]...)
}

func confirmGroup(ctx context.Context, task TaskEnvironment, group []preparedPlan) (confirmation Confirmation, safe bool) {
	confirmation.Status = StatusProofBlocked
	confirmation.Mode = ConfirmationModeEdit
	confirmation.WitnessID = group[0].plan.WitnessID
	confirmation.ExpectedTestPasses = group[0].plan.Expected.TestPasses
	confirmation.PlanIDs = make([]string, 0, len(group))
	confirmation.Plans = make([]semanticir.EditPlan, 0, len(group))
	confirmation.Materializations = make([]MaterializationEvidence, 0, len(group))
	for _, prepared := range group {
		confirmation.PlanIDs = append(confirmation.PlanIDs, prepared.plan.ID)
		confirmation.Plans = append(confirmation.Plans, prepared.plan)
		confirmation.Materializations = append(confirmation.Materializations, MaterializationEvidence{
			PlanID: prepared.plan.ID, WitnessID: prepared.plan.WitnessID,
			ArtifactID: prepared.plan.Artifact.ID, ArtifactPath: prepared.path,
			FrozenSHA256:       prepared.plan.Artifact.Digest,
			MaterializedSHA256: digestBytes(prepared.materialized),
			OriginalSize:       len(prepared.original), MaterializedSize: len(prepared.materialized),
			Edits: append([]EditEvidence(nil), prepared.edits...),
		})
	}
	block := func(plan preparedPlan, code, detail string) {
		confirmation.Blockers = append(confirmation.Blockers, Blocker{
			Stage: "confirmation", PlanID: plan.plan.ID, WitnessID: plan.plan.WitnessID,
			Code: code, Detail: detail,
		})
	}

	// Validate every artifact before writing the first one. This makes the
	// transition from the clean baseline to a multi-artifact witness atomic
	// from the executor's perspective.
	for _, prepared := range group {
		current, err := os.ReadFile(prepared.path)
		if err != nil {
			block(prepared, "artifact-unreadable", err.Error())
			return confirmation, false
		}
		if !bytes.Equal(current, prepared.original) {
			block(prepared, "stale-artifact", fmt.Sprintf("artifact changed after baseline: got %s, want %s", digestBytes(current), digestBytes(prepared.original)))
			return confirmation, false
		}
	}
	if err := ctx.Err(); err != nil {
		block(group[0], "interrupted", err.Error())
		return confirmation, true
	}

	applied := 0
	defer func() {
		safe = true
		// Restore in reverse materialization order. Restoration does not use
		// the canceled execution context and therefore also runs after timeout
		// or interruption.
		for index := applied - 1; index >= 0; index-- {
			prepared := group[index]
			materialization := &confirmation.Materializations[index]
			if err := writeExact(prepared.path, prepared.original, prepared.mode); err != nil {
				materialization.Error = appendError(materialization.Error, "restoring artifact: "+err.Error())
				block(prepared, "restore-failed", err.Error())
				confirmation.Status = StatusProofBlocked
				safe = false
				continue
			}
			restored, err := os.ReadFile(prepared.path)
			if err != nil {
				materialization.Error = appendError(materialization.Error, "verifying restoration: "+err.Error())
				block(prepared, "restore-verification-failed", err.Error())
				confirmation.Status = StatusProofBlocked
				safe = false
				continue
			}
			materialization.RestoredSHA256 = digestBytes(restored)
			materialization.Restored = bytes.Equal(restored, prepared.original)
			if !materialization.Restored {
				detail := fmt.Sprintf("restored digest is %s, want %s", digestBytes(restored), digestBytes(prepared.original))
				materialization.Error = appendError(materialization.Error, detail)
				block(prepared, "restore-mismatch", detail)
				confirmation.Status = StatusProofBlocked
				safe = false
			}
		}
	}()

	for index, prepared := range group {
		// Increment before the write attempt so even an unusual partial-write
		// failure is followed by restoration of this artifact.
		applied = index + 1
		materialization := &confirmation.Materializations[index]
		if err := writeExact(prepared.path, prepared.materialized, prepared.mode); err != nil {
			materialization.Error = "materializing artifact: " + err.Error()
			block(prepared, "materialization-failed", err.Error())
			return confirmation, false
		}
		observed, err := os.ReadFile(prepared.path)
		if err != nil {
			materialization.Error = "reading materialized artifact: " + err.Error()
			block(prepared, "materialization-unreadable", err.Error())
			return confirmation, false
		}
		materialization.ObservedSHA256 = digestBytes(observed)
		if !bytes.Equal(observed, prepared.materialized) {
			detail := fmt.Sprintf("materialized digest is %s, want %s", digestBytes(observed), digestBytes(prepared.materialized))
			materialization.Error = detail
			block(prepared, "materialization-mismatch", detail)
			return confirmation, false
		}
		materialization.Applied = true
	}

	confirmation.Command = runVerifier(ctx, task)
	if confirmation.Command.Error != "" {
		block(group[0], commandBlockerCode(confirmation.Command), confirmation.Command.Error)
		return confirmation, false
	}
	observedPass := confirmation.Command.Passed
	confirmation.ObservedTestPasses = &observedPass
	if observedPass == confirmation.ExpectedTestPasses {
		confirmation.Status = StatusConfirmed
	} else {
		block(group[0], "model-execution-mismatch", fmt.Sprintf(
			"frozen verifier observed tests_pass=%t, but the semantic witness requires tests_pass=%t",
			observedPass, confirmation.ExpectedTestPasses,
		))
		confirmation.Status = StatusProofBlocked
	}
	return confirmation, false
}

func verifyBaselineSources(prepared []preparedPlan, report *Report) bool {
	seen := map[string]bool{}
	ok := true
	for _, plan := range prepared {
		if seen[plan.path] {
			continue
		}
		seen[plan.path] = true
		current, err := os.ReadFile(plan.path)
		if err == nil && bytes.Equal(current, plan.original) {
			continue
		}
		ok = false
		detail := "baseline command changed or removed a frozen source artifact"
		if err == nil {
			detail = fmt.Sprintf("baseline command changed source from %s to %s", digestBytes(plan.original), digestBytes(current))
		} else {
			detail += ": " + err.Error()
		}
		report.Blockers = append(report.Blockers, Blocker{
			Stage: "baseline", PlanID: plan.plan.ID, WitnessID: plan.plan.WitnessID,
			Code: "baseline-mutated-source", Detail: detail,
		})
		if restoreErr := writeExact(plan.path, plan.original, plan.mode); restoreErr != nil {
			report.Blockers = append(report.Blockers, Blocker{
				Stage: "cleanup", PlanID: plan.plan.ID, WitnessID: plan.plan.WitnessID,
				Code: "baseline-restore-failed", Detail: restoreErr.Error(),
			})
			continue
		}
		restored, restoreErr := os.ReadFile(plan.path)
		if restoreErr != nil || !bytes.Equal(restored, plan.original) {
			detail := "baseline source restoration did not reproduce the frozen bytes"
			if restoreErr != nil {
				detail += ": " + restoreErr.Error()
			} else {
				detail += fmt.Sprintf(": got %s, want %s", digestBytes(restored), digestBytes(plan.original))
			}
			report.Blockers = append(report.Blockers, Blocker{
				Stage: "cleanup", PlanID: plan.plan.ID, WitnessID: plan.plan.WitnessID,
				Code: "baseline-restore-mismatch", Detail: detail,
			})
		}
	}
	return ok
}

func runVerifier(ctx context.Context, task TaskEnvironment) CommandEvidence {
	evidence := CommandEvidence{
		Command: append([]string(nil), task.Command...), WorkDir: task.WorkDir,
		Timeout: task.Timeout, EnvironmentSHA256: digestEnvironment(task.Environment),
		StdoutSHA256: digestBytes(nil), StderrSHA256: digestBytes(nil),
	}
	evidence.CommandSHA256, _ = semanticir.Digest(task)
	if ctx == nil {
		evidence.Error = "execution context is nil"
		return evidence
	}
	if task.PassSignal.VerdictFile != nil {
		path := verdictPath(task)
		evidence.Signal = SignalEvidence{
			Kind: "verdict-file", VerdictPath: path,
			ExpectedValueSHA256: digestBytes([]byte(strings.TrimSpace(task.PassSignal.VerdictFile.PassValue))),
		}
		removed, err := clearVerdictFile(path)
		evidence.Signal.StaleVerdictRemoved = removed
		if err != nil {
			evidence.Error = "preparing fresh verdict file: " + err.Error()
			return evidence
		}
	} else {
		code := *task.PassSignal.ExitCode
		evidence.Signal = SignalEvidence{Kind: "exit-code", ExpectedExitCode: &code}
	}

	runCtx, cancel := context.WithTimeout(ctx, task.Timeout)
	defer cancel()
	if err := runCtx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			evidence.TimedOut = true
			evidence.Error = fmt.Sprintf("verifier exceeded task timeout %s", task.Timeout)
		} else {
			evidence.Interrupted = true
			evidence.Error = "verifier interrupted: " + err.Error()
		}
		return evidence
	}
	cmd := exec.Command(task.Command[0], task.Command[1:]...)
	cmd.Dir = task.WorkDir
	if task.ExactEnvironment {
		cmd.Env = append([]string{}, task.Environment...)
	} else {
		cmd.Env = mergedEnvironment(task.Environment)
	}
	cmd.WaitDelay = processWaitDelay
	configureProcess(cmd)
	stdout := &limitedBuffer{limit: maxCapturedOutput}
	stderr := &limitedBuffer{limit: maxCapturedOutput}
	cmd.Stdout, cmd.Stderr = stdout, stderr

	evidence.StartedAt = time.Now().UTC()
	start := time.Now()
	if err := cmd.Start(); err != nil {
		evidence.Duration = time.Since(start)
		evidence.Error = "starting verifier: " + err.Error()
		return evidence
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var waitErr error
	select {
	case waitErr = <-done:
		// A command which exits without waiting for its descendants must not
		// leave them mutating evidence after we begin restoration/cleanup.
		terminateProcess(cmd)
	case <-runCtx.Done():
		select {
		case waitErr = <-done:
			terminateProcess(cmd)
		default:
			terminateProcess(cmd)
			waitErr = <-done
			if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
				evidence.TimedOut = true
				evidence.Error = fmt.Sprintf("verifier exceeded task timeout %s", task.Timeout)
			} else {
				evidence.Interrupted = true
				evidence.Error = "verifier interrupted: " + runCtx.Err().Error()
			}
		}
	}
	evidence.Duration = time.Since(start)
	evidence.Stdout, evidence.Stderr = stdout.String(), stderr.String()
	evidence.StdoutSHA256, evidence.StderrSHA256 = digestBytes([]byte(evidence.Stdout)), digestBytes([]byte(evidence.Stderr))
	evidence.OutputTruncated = stdout.truncated || stderr.truncated

	if cmd.ProcessState != nil {
		code := cmd.ProcessState.ExitCode()
		evidence.ExitCode = &code
		evidence.Signal.ObservedExitCode = &code
	}
	if evidence.Error != "" {
		return evidence
	}
	if waitErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) {
			evidence.Error = "waiting for verifier: " + waitErr.Error()
			return evidence
		}
	}

	if task.PassSignal.ExitCode != nil {
		evidence.Passed = evidence.ExitCode != nil && *evidence.ExitCode == *task.PassSignal.ExitCode
		if evidence.ExitCode != nil {
			evidence.SignalValueSHA256 = digestBytes([]byte(strconv.Itoa(*evidence.ExitCode)))
		}
		return evidence
	}
	path := verdictPath(task)
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			evidence.Error = fmt.Sprintf("verifier did not create fresh verdict file %q", path)
		} else {
			evidence.Error = "reading fresh verdict file metadata: " + err.Error()
		}
		return evidence
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		evidence.Error = fmt.Sprintf("fresh verdict path %q is not a regular non-symlink file", path)
		return evidence
	}
	if info.Size() > maxCapturedOutput {
		evidence.Error = fmt.Sprintf("fresh verdict file %q exceeds %d bytes", path, maxCapturedOutput)
		return evidence
	}
	body, err := os.ReadFile(path)
	if err != nil {
		evidence.Error = "reading fresh verdict file: " + err.Error()
		return evidence
	}
	evidence.Signal.FreshVerdict = true
	normalized := []byte(strings.TrimSpace(string(body)))
	evidence.Signal.ObservedValueSHA256 = digestBytes(normalized)
	evidence.SignalValueSHA256 = evidence.Signal.ObservedValueSHA256
	evidence.Passed = string(normalized) == strings.TrimSpace(task.PassSignal.VerdictFile.PassValue)
	return evidence
}

func snapshotVerdictFile(task TaskEnvironment) (*verdictSnapshot, *Blocker) {
	if task.PassSignal.VerdictFile == nil {
		return nil, nil
	}
	path := verdictPath(task)
	snapshot := &verdictSnapshot{path: path}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return snapshot, nil
	}
	if err != nil {
		return nil, &Blocker{Stage: "configuration", Code: "verdict-unreadable", Detail: err.Error()}
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, &Blocker{
			Stage: "configuration", Code: "invalid-verdict-file",
			Detail: fmt.Sprintf("declared verdict path %q is not a regular non-symlink file", path),
		}
	}
	if info.Size() > maxCapturedOutput {
		return nil, &Blocker{Stage: "configuration", Code: "verdict-too-large", Detail: fmt.Sprintf("declared verdict path %q exceeds %d bytes", path, maxCapturedOutput)}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, &Blocker{Stage: "configuration", Code: "verdict-unreadable", Detail: err.Error()}
	}
	snapshot.existed, snapshot.bytes, snapshot.mode = true, body, info.Mode().Perm()
	return snapshot, nil
}

func restoreVerdictFile(snapshot verdictSnapshot) error {
	if snapshot.existed {
		return writeExact(snapshot.path, snapshot.bytes, snapshot.mode)
	}
	err := os.Remove(snapshot.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing generated verdict file: %w", err)
	}
	return nil
}

func clearVerdictFile(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("verdict path %q is not a regular non-symlink file", path)
	}
	if err := os.Remove(path); err != nil {
		return false, err
	}
	return true, nil
}

func verdictPath(task TaskEnvironment) string {
	path := task.PassSignal.VerdictFile.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(task.WorkspaceRoot, path)
	}
	return filepath.Clean(path)
}

func resolveVerdictPath(workspaceRoot, declared string) (string, error) {
	candidate := declared
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(workspaceRoot, candidate)
	}
	candidate = filepath.Clean(candidate)
	parent, err := filepath.EvalSymlinks(filepath.Dir(candidate))
	if err != nil {
		return "", fmt.Errorf("resolving verdict parent: %w", err)
	}
	if !pathWithin(workspaceRoot, parent) {
		return "", fmt.Errorf("verdict path %q resolves outside frozen workspace %q", declared, workspaceRoot)
	}
	target := filepath.Join(parent, filepath.Base(candidate))
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", fmt.Errorf("verdict path %q is not a regular non-symlink file", declared)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return target, nil
}

func writeExact(path string, body []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".hyperray-executor-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode.Perm()); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func aggregateStatus(report Report) Status {
	if len(report.Blockers) != 0 {
		return StatusProofBlocked
	}
	if !report.Baseline.Passed {
		return StatusProofBlocked
	}
	for _, confirmation := range report.Confirmations {
		if confirmation.Status == StatusProofBlocked {
			return StatusProofBlocked
		}
		if confirmation.Status != StatusConfirmed {
			return StatusNotConfirmed
		}
	}
	return StatusConfirmed
}

// ValidateEditConfirmation checks the certificate-facing internal
// consistency of a confirmed edit witness. Authoritative confirmation must
// come from ConfirmIsolated: restoration alone cannot isolate arbitrary
// compiler/verifier side effects between semantic vectors.
func ValidateEditConfirmation(confirmation Confirmation) error {
	if confirmation.Mode != ConfirmationModeEdit || confirmation.Status != StatusConfirmed || confirmation.WitnessID == "" || confirmation.Probe != nil || confirmation.BaselineWitness != nil || len(confirmation.Blockers) != 0 {
		return fmt.Errorf("edit confirmation mixes evidence modes or is not confirmed")
	}
	if len(confirmation.Plans) == 0 || len(confirmation.PlanIDs) != len(confirmation.Plans) || len(confirmation.Materializations) != len(confirmation.Plans) {
		return fmt.Errorf("edit confirmation does not retain every full plan and materialization")
	}
	if confirmation.Isolation == nil || !validGenericIsolation(*confirmation.Isolation) {
		return fmt.Errorf("edit confirmation lacks a complete isolated workspace record")
	}
	if err := validateCommandEvidenceIntegrity(confirmation.Command); err != nil {
		return err
	}
	if confirmation.ObservedTestPasses == nil || *confirmation.ObservedTestPasses != confirmation.Command.Passed || confirmation.ExpectedTestPasses != *confirmation.ObservedTestPasses {
		return fmt.Errorf("edit confirmation pass observation differs from its semantic expectation")
	}
	seenPlans := make(map[string]bool, len(confirmation.Plans))
	expected := confirmation.Plans[0].Expected
	for index, plan := range confirmation.Plans {
		materialization := confirmation.Materializations[index]
		if plan.ID == "" || plan.WitnessID != confirmation.WitnessID || confirmation.PlanIDs[index] != plan.ID || seenPlans[plan.ID] || !reflect.DeepEqual(plan.Expected, expected) || plan.Expected.TestPasses != confirmation.ExpectedTestPasses {
			return fmt.Errorf("edit plan %d has inconsistent identity or witness semantics", index)
		}
		seenPlans[plan.ID] = true
		if plan.Artifact.ID == "" || plan.Artifact.Kind != semanticir.ArtifactCode || !validDigest(plan.Artifact.Digest) || plan.Provenance.ArtifactID != plan.Artifact.ID || plan.Provenance.ArtifactDigest != plan.Artifact.Digest || len(plan.Edits) == 0 {
			return fmt.Errorf("edit plan %q has incomplete frozen artifact/provenance binding", plan.ID)
		}
		if materialization.PlanID != plan.ID || materialization.WitnessID != plan.WitnessID || materialization.ArtifactID != plan.Artifact.ID || materialization.FrozenSHA256 != plan.Artifact.Digest || !materialization.Applied || !materialization.Restored || materialization.Error != "" || materialization.ObservedSHA256 != materialization.MaterializedSHA256 || materialization.RestoredSHA256 != plan.Artifact.Digest || !validDigest(materialization.MaterializedSHA256) {
			return fmt.Errorf("edit plan %q materialization/restoration record is inconsistent", plan.ID)
		}
		if !pathWithin(confirmation.Isolation.IsolatedRoot, materialization.ArtifactPath) {
			return fmt.Errorf("edit plan %q materialized outside its isolated workspace", plan.ID)
		}
		if len(materialization.Edits) != len(plan.Edits) {
			return fmt.Errorf("edit plan %q replacement evidence cardinality differs", plan.ID)
		}
		edits := append([]semanticir.ByteRangeReplacement(nil), plan.Edits...)
		sort.SliceStable(edits, func(i, j int) bool {
			if edits[i].StartByte == edits[j].StartByte {
				return edits[i].EndByte < edits[j].EndByte
			}
			return edits[i].StartByte < edits[j].StartByte
		})
		for editIndex, edit := range edits {
			record := materialization.Edits[editIndex]
			if edit.StartByte < 0 || edit.EndByte < edit.StartByte || record.StartByte != edit.StartByte || record.EndByte != edit.EndByte || record.ExpectedSHA256 != digestBytes(edit.ExpectedBytes) || record.ReplacementSHA256 != digestBytes(edit.Replacement) {
				return fmt.Errorf("edit plan %q replacement %d digest/range differs", plan.ID, editIndex)
			}
			if editIndex > 0 && (edit.StartByte < edits[editIndex-1].EndByte || edit.StartByte == edits[editIndex-1].StartByte) {
				return fmt.Errorf("edit plan %q replacements overlap or are ambiguous", plan.ID)
			}
		}
	}
	return nil
}

func validGenericIsolation(isolation IsolationEvidence) bool {
	return isolation.OriginalRoot != "" && isolation.IsolatedRoot != "" && isolation.OriginalRoot != isolation.IsolatedRoot &&
		validDigest(isolation.ExpectedSHA256) && isolation.OriginalBeforeSHA256 == isolation.ExpectedSHA256 && isolation.CopyBeforeSHA256 == isolation.ExpectedSHA256 &&
		validDigest(isolation.CopyAfterSHA256) && isolation.OriginalAfterSHA256 == isolation.ExpectedSHA256 && isolation.IsolatedRemoved && isolation.OriginalIntact && isolation.Error == ""
}

func validateCommandEvidenceIntegrity(command CommandEvidence) error {
	if command.Error != "" || command.TimedOut || command.Interrupted || command.OutputTruncated || command.StartedAt.IsZero() || len(command.Command) == 0 || command.Timeout <= 0 {
		return fmt.Errorf("command lacks a complete execution record")
	}
	if !utf8.ValidString(command.Stdout) || !utf8.ValidString(command.Stderr) || command.StdoutSHA256 != digestBytes([]byte(command.Stdout)) || command.StderrSHA256 != digestBytes([]byte(command.Stderr)) || !validDigest(command.CommandSHA256) || !validDigest(command.EnvironmentSHA256) {
		return fmt.Errorf("command output or invocation digests are inconsistent")
	}
	switch command.Signal.Kind {
	case "exit-code":
		if command.Signal.ExpectedExitCode == nil || command.Signal.ObservedExitCode == nil || command.ExitCode == nil || *command.Signal.ObservedExitCode != *command.ExitCode || command.Passed != (*command.Signal.ExpectedExitCode == *command.Signal.ObservedExitCode) || command.SignalValueSHA256 != digestBytes([]byte(fmt.Sprint(*command.Signal.ObservedExitCode))) {
			return fmt.Errorf("command exit-code signal is inconsistent")
		}
	case "verdict-file":
		if !command.Signal.FreshVerdict || !validDigest(command.Signal.ExpectedValueSHA256) || !validDigest(command.Signal.ObservedValueSHA256) || command.SignalValueSHA256 != command.Signal.ObservedValueSHA256 || command.Passed != (command.Signal.ExpectedValueSHA256 == command.Signal.ObservedValueSHA256) {
			return fmt.Errorf("command verdict-file signal is inconsistent")
		}
	default:
		return fmt.Errorf("command has unsupported pass-signal evidence")
	}
	return nil
}

func commandBlockerCode(evidence CommandEvidence) string {
	switch {
	case evidence.TimedOut:
		return "timeout"
	case evidence.Interrupted:
		return "interrupted"
	case strings.Contains(evidence.Error, "fresh verdict"):
		return "stale-or-missing-verdict"
	default:
		return "execution-failed"
	}
}

func validDigest(digest string) bool {
	if !strings.HasPrefix(digest, "sha256:") || len(digest) != len("sha256:")+sha256.Size*2 {
		return false
	}
	encoded := strings.TrimPrefix(digest, "sha256:")
	if strings.ToLower(encoded) != encoded {
		return false
	}
	_, err := hex.DecodeString(encoded)
	return err == nil
}

func digestBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func digestEnvironment(environment []string) string {
	return digestBytes([]byte(strings.Join(environment, "\x00")))
}

func mergedEnvironment(overrides []string) []string {
	environment := append([]string(nil), os.Environ()...)
	positions := make(map[string]int, len(environment))
	for i, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		positions[name] = i
	}
	for _, entry := range overrides {
		name, _, _ := strings.Cut(entry, "=")
		if i, ok := positions[name]; ok {
			environment[i] = entry
		} else {
			positions[name] = len(environment)
			environment = append(environment, entry)
		}
	}
	return environment
}

func samePath(left, right string) bool {
	if resolved, err := filepath.EvalSymlinks(left); err == nil {
		left = resolved
	}
	if resolved, err := filepath.EvalSymlinks(right); err == nil {
		right = resolved
	}
	a, errA := filepath.Abs(left)
	b, errB := filepath.Abs(right)
	return errA == nil && errB == nil && filepath.Clean(a) == filepath.Clean(b)
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func appendError(existing, added string) string {
	if existing == "" {
		return added
	}
	return existing + "; " + added
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		b.truncated = b.truncated || original > 0
		return original, nil
	}
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.buffer.Write(p)
	return original, nil
}

func (b *limitedBuffer) String() string { return b.buffer.String() }

var _ io.Writer = (*limitedBuffer)(nil)
