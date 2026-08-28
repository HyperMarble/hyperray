package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/HyperMarble/ray/internal/semanticir"
)

// ConfirmIsolated performs edit confirmation without sharing any command side
// effects between the clean baseline or proof witnesses. It requires an exact
// WorkspaceSHA256 binding, runs the baseline in one disposable workspace copy,
// and runs every atomic WitnessID group in a different fresh copy.
func ConfirmIsolated(ctx context.Context, task TaskEnvironment, plans []semanticir.EditPlan) (report Report) {
	report.Status = StatusProofBlocked
	report.Vacuous = len(plans) == 0
	resolvedWorkDir, resolvedRoot, blocker := validateTask(task)
	if blocker != nil {
		report.Blockers = append(report.Blockers, *blocker)
		return report
	}
	task.WorkDir, task.WorkspaceRoot = resolvedWorkDir, resolvedRoot
	if !validDigest(task.WorkspaceSHA256) {
		report.Blockers = append(report.Blockers, Blocker{
			Stage: "configuration", Code: "missing-workspace-digest",
			Detail: "isolated confirmation requires a normalized frozen workspace digest",
		})
		return report
	}
	originalDigest, err := WorkspaceDigest(task.WorkspaceRoot)
	if err != nil || originalDigest != task.WorkspaceSHA256 {
		detail := fmt.Sprintf("workspace digest is %s, task requires %s", originalDigest, task.WorkspaceSHA256)
		if err != nil {
			detail = err.Error()
		}
		report.Blockers = append(report.Blockers, Blocker{Stage: "configuration", Code: "stale-workspace", Detail: detail})
		return report
	}
	prepared, blockers := preparePlans(task, plans)
	if len(blockers) != 0 {
		report.Blockers = append(report.Blockers, blockers...)
		return report
	}
	defer func() { report.Status = aggregateStatus(report) }()
	if ctx == nil {
		report.Blockers = append(report.Blockers, Blocker{Stage: "baseline", Code: "nil-context", Detail: "execution context is nil"})
		return report
	}

	baseline, isolation, baselineBlocker := runIsolatedBaseline(ctx, task)
	report.Baseline, report.BaselineIsolation = baseline, isolation
	if baselineBlocker != nil {
		report.Blockers = append(report.Blockers, *baselineBlocker)
		return report
	}
	if !baseline.Passed {
		report.Blockers = append(report.Blockers, Blocker{
			Stage: "baseline", Code: "baseline-failed",
			Detail: "the isolated unmodified reference did not produce the declared pass signal",
		})
		return report
	}

	for _, group := range groupPlans(prepared) {
		confirmation, isolationSafe := confirmIsolatedGroup(ctx, task, group)
		report.Confirmations = append(report.Confirmations, confirmation)
		report.Blockers = append(report.Blockers, confirmation.Blockers...)
		if !isolationSafe {
			break
		}
	}
	return report
}

func runIsolatedBaseline(ctx context.Context, task TaskEnvironment) (CommandEvidence, *IsolationEvidence, *Blocker) {
	tempParent, runRoot, copiedDigest, err := makeProbeWorkspaceCopy(task.WorkspaceRoot, task.WorkspaceSHA256)
	isolation := &IsolationEvidence{
		OriginalRoot: task.WorkspaceRoot, ExpectedSHA256: task.WorkspaceSHA256,
		OriginalBeforeSHA256: task.WorkspaceSHA256, CopyBeforeSHA256: copiedDigest,
	}
	if err != nil {
		isolation.Error = err.Error()
		return CommandEvidence{}, isolation, &Blocker{Stage: "baseline", Code: "baseline-copy-failed", Detail: err.Error()}
	}
	isolation.IsolatedRoot = runRoot
	copyTask, err := remapTaskEnvironment(task, runRoot)
	if err != nil {
		isolation.Error = err.Error()
		finalizeIsolation(tempParent, runRoot, task, isolation)
		return CommandEvidence{}, isolation, &Blocker{Stage: "baseline", Code: "baseline-copy-invalid", Detail: err.Error()}
	}
	evidence := runVerifier(ctx, copyTask)
	cleanupErr := finalizeIsolation(tempParent, runRoot, task, isolation)
	if cleanupErr != nil {
		return evidence, isolation, &Blocker{Stage: "baseline", Code: "baseline-copy-cleanup", Detail: cleanupErr.Error()}
	}
	if evidence.Error != "" {
		return evidence, isolation, &Blocker{Stage: "baseline", Code: commandBlockerCode(evidence), Detail: evidence.Error}
	}
	return evidence, isolation, nil
}

func confirmIsolatedGroup(ctx context.Context, task TaskEnvironment, group []preparedPlan) (Confirmation, bool) {
	tempParent, runRoot, copiedDigest, err := makeProbeWorkspaceCopy(task.WorkspaceRoot, task.WorkspaceSHA256)
	isolation := &IsolationEvidence{
		OriginalRoot: task.WorkspaceRoot, ExpectedSHA256: task.WorkspaceSHA256,
		OriginalBeforeSHA256: task.WorkspaceSHA256, CopyBeforeSHA256: copiedDigest,
	}
	if err != nil {
		confirmation := blockedIsolatedConfirmation(group, "isolation-copy-failed", err.Error())
		isolation.Error = err.Error()
		confirmation.Isolation = isolation
		return confirmation, false
	}
	isolation.IsolatedRoot = runRoot
	copyTask, err := remapTaskEnvironment(task, runRoot)
	if err != nil {
		confirmation := blockedIsolatedConfirmation(group, "isolation-copy-invalid", err.Error())
		isolation.Error = err.Error()
		_ = finalizeIsolation(tempParent, runRoot, task, isolation)
		confirmation.Isolation = isolation
		return confirmation, isolation.IsolatedRemoved && isolation.OriginalIntact
	}
	copyGroup, err := remapPreparedGroup(task.WorkspaceRoot, runRoot, group)
	if err != nil {
		confirmation := blockedIsolatedConfirmation(group, "isolation-plan-remap", err.Error())
		isolation.Error = err.Error()
		_ = finalizeIsolation(tempParent, runRoot, task, isolation)
		confirmation.Isolation = isolation
		return confirmation, isolation.IsolatedRemoved && isolation.OriginalIntact
	}
	confirmation, _ := confirmGroup(ctx, copyTask, copyGroup)
	confirmation.Isolation = isolation
	if cleanupErr := finalizeIsolation(tempParent, runRoot, task, isolation); cleanupErr != nil {
		confirmation.Blockers = append(confirmation.Blockers, Blocker{
			Stage: "cleanup", PlanID: group[0].plan.ID, WitnessID: group[0].plan.WitnessID,
			Code: "isolation-cleanup-failed", Detail: cleanupErr.Error(),
		})
		confirmation.Status = StatusProofBlocked
	}
	return confirmation, isolation.IsolatedRemoved && isolation.OriginalIntact
}

func remapTaskEnvironment(task TaskEnvironment, runRoot string) (TaskEnvironment, error) {
	relativeWork, err := filepath.Rel(task.WorkspaceRoot, task.WorkDir)
	if err != nil || filepath.IsAbs(relativeWork) || relativeWork == ".." || len(relativeWork) >= 3 && relativeWork[:3] == ".."+string(os.PathSeparator) {
		return TaskEnvironment{}, fmt.Errorf("command workdir cannot be remapped into isolated workspace")
	}
	copyTask := task
	copyTask.WorkspaceRoot = runRoot
	copyTask.WorkDir = filepath.Join(runRoot, relativeWork)
	if task.PassSignal.VerdictFile != nil {
		declared := task.PassSignal.VerdictFile.Path
		if filepath.IsAbs(declared) {
			relativeVerdict, relErr := filepath.Rel(task.WorkspaceRoot, declared)
			if relErr != nil || filepath.IsAbs(relativeVerdict) || relativeVerdict == ".." || len(relativeVerdict) >= 3 && relativeVerdict[:3] == ".."+string(os.PathSeparator) {
				return TaskEnvironment{}, fmt.Errorf("absolute verdict path cannot be remapped into isolated workspace")
			}
			declared = filepath.Join(runRoot, relativeVerdict)
		}
		copyTask.PassSignal = VerdictFileSignal(declared, task.PassSignal.VerdictFile.PassValue)
	}
	return copyTask, nil
}

func remapPreparedGroup(originalRoot, runRoot string, group []preparedPlan) ([]preparedPlan, error) {
	result := make([]preparedPlan, len(group))
	for index, plan := range group {
		relative, err := filepath.Rel(originalRoot, plan.path)
		if err != nil || filepath.IsAbs(relative) || relative == ".." || len(relative) >= 3 && relative[:3] == ".."+string(os.PathSeparator) {
			return nil, fmt.Errorf("plan %q artifact cannot be remapped into isolated workspace", plan.plan.ID)
		}
		result[index] = plan
		result[index].path = filepath.Join(runRoot, relative)
	}
	return result, nil
}

func finalizeIsolation(tempParent, runRoot string, task TaskEnvironment, isolation *IsolationEvidence) error {
	var cleanupErr error
	record := func(err error) { cleanupErr = errors.Join(cleanupErr, err) }
	copyAfter, err := WorkspaceDigest(runRoot)
	if err != nil {
		record(fmt.Errorf("snapshot isolated workspace after execution: %w", err))
	} else {
		isolation.CopyAfterSHA256 = copyAfter
	}
	if err := os.RemoveAll(tempParent); err != nil {
		record(fmt.Errorf("remove isolated workspace: %w", err))
	} else if _, err := os.Stat(tempParent); os.IsNotExist(err) {
		isolation.IsolatedRemoved = true
	} else {
		record(fmt.Errorf("isolated workspace still exists after cleanup"))
	}
	originalAfter, err := WorkspaceDigest(task.WorkspaceRoot)
	if err != nil {
		record(fmt.Errorf("snapshot original workspace after execution: %w", err))
	} else {
		isolation.OriginalAfterSHA256 = originalAfter
		isolation.OriginalIntact = originalAfter == task.WorkspaceSHA256
		if !isolation.OriginalIntact {
			record(fmt.Errorf("original workspace changed during isolated execution"))
		}
	}
	if cleanupErr != nil {
		isolation.Error = appendError(isolation.Error, cleanupErr.Error())
	}
	return cleanupErr
}

func blockedIsolatedConfirmation(group []preparedPlan, code, detail string) Confirmation {
	confirmation := Confirmation{
		Mode: ConfirmationModeEdit, Status: StatusProofBlocked,
		WitnessID: group[0].plan.WitnessID, ExpectedTestPasses: group[0].plan.Expected.TestPasses,
	}
	for _, plan := range group {
		confirmation.PlanIDs = append(confirmation.PlanIDs, plan.plan.ID)
		confirmation.Plans = append(confirmation.Plans, plan.plan)
	}
	confirmation.Blockers = append(confirmation.Blockers, Blocker{
		Stage: "confirmation", PlanID: group[0].plan.ID, WitnessID: group[0].plan.WitnessID,
		Code: code, Detail: detail,
	})
	return confirmation
}
