// Footprint deadlines remain host-enforced when a tool ignores its timeout.
// A timed-out instruction must discard earlier traces in the same request.
package isla_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestFootprintHostDeadline(t *testing.T) {
	engine, processPath := delayedFootprintEngine(t)
	request := deadlineFootprintRequest(t, engine, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	start := time.Now()
	report, err := engine.TraceInstructions(ctx, request)
	elapsed := time.Since(start)
	assertFootprintError(t, report, err, isla.ResourceLimit)
	if !reflect.DeepEqual(report, isla.FootprintReport{}) {
		t.Errorf("deadline returned partial report: %#v", report)
	}
	if elapsed >= 3*time.Second {
		t.Errorf("one-second host deadline took %s", elapsed)
	}
	assertFootprintProcessReaped(t, processPath)
	t.Logf("host deadline elapsed: %s", elapsed)
}

func TestFootprintCallerDeadlinePrecedesToolLimit(t *testing.T) {
	engine, processPath := delayedFootprintEngine(t)
	request := deadlineFootprintRequest(t, engine, 60)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	report, err := engine.TraceInstructions(ctx, request)
	assertFootprintError(t, report, err, isla.ResourceLimit)
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Errorf("caller deadline took %s", elapsed)
	}
	assertFootprintProcessReaped(t, processPath)
}

func TestFootprintOperationRejectsUnrepresentableDeadline(t *testing.T) {
	instructions := []machine.Instruction{{Address: 2, Bytes: []byte{1, 0}}}
	request, err := isla.NewFootprintRequest(defaultFootprintRelease(t), instructions, 1, ^uint64(0), 4096)
	if err != nil {
		t.Fatal(err)
	}
	report, err := footprintEngine(t).TraceInstructions(t.Context(), request)
	assertFootprintError(t, report, err, isla.InvalidInput)
}
