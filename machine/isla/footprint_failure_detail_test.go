// Resource-limit tests preserve the command cause and bounded output.
// They must distinguish context failures from stdout and stderr limits.
package isla_test

import (
	"context"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestFootprintTimeoutPreservesFailureCause(t *testing.T) {
	engine, _ := delayedFootprintEngine(t)
	report, err := engine.TraceInstructions(t.Context(), deadlineFootprintRequest(t, engine, 1))
	detail := assertResourceLimitDetail(t, report, err)
	if detail.Cause != isla.ResourceLimitTimeout || detail.ElapsedMilliseconds == 0 {
		t.Errorf("timeout detail = %#v", detail)
	}
}

func TestFootprintCancellationPreservesFailureCause(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	instructions := []machine.Instruction{{Address: 2, Bytes: []byte{1, 0}}}
	report, err := footprintEngine(t).TraceInstructions(ctx, footprintRequest(t, instructions, 4096))
	detail := assertResourceLimitDetail(t, report, err)
	if detail.Cause != isla.ResourceLimitCanceled {
		t.Errorf("cancellation detail = %#v", detail)
	}
}

func TestFootprintStdoutLimitPreservesBoundedOutput(t *testing.T) {
	instructions := []machine.Instruction{{Address: 2, Bytes: []byte{0xff, 0xff, 0xff, 0xff}}}
	report, err := footprintEngine(t).TraceInstructions(t.Context(), footprintRequest(t, instructions, 32))
	detail := assertResourceLimitDetail(t, report, err)
	if detail.Cause != isla.ResourceLimitStdoutExceeded || !detail.StdoutLimitExceeded {
		t.Errorf("stdout detail = %#v", detail)
	}
	if len(detail.Stdout) != 32 || detail.ExitCode != 0 {
		t.Errorf("stdout capture = %d bytes, exit = %d", len(detail.Stdout), detail.ExitCode)
	}
}

func TestFootprintStderrLimitPreservesBoundedOutput(t *testing.T) {
	engine, err := isla.NewFootprintEngine(t.Context(), temporaryTool(t, "printf '%s' 0123456789 >&2"))
	if err != nil {
		t.Fatal(err)
	}
	instructions := []machine.Instruction{{Address: 2, Bytes: []byte{1, 0}}}
	architecture := testArtifact(t, "stderr-limit-architecture")
	configuration := testArtifact(t, "stderr-limit-configuration")
	release := footprintRelease(t, engine, architecture, configuration)
	request, err := isla.NewFootprintRequest(release, instructions, 1, 1, 8)
	if err != nil {
		t.Fatal(err)
	}
	report, err := engine.TraceInstructions(t.Context(), request)
	detail := assertResourceLimitDetail(t, report, err)
	if detail.Cause != isla.ResourceLimitStderrExceeded || !detail.StderrLimitExceeded {
		t.Errorf("stderr detail = %#v", detail)
	}
	if len(detail.Stderr) != 8 {
		t.Errorf("stderr capture = %d bytes", len(detail.Stderr))
	}
}
