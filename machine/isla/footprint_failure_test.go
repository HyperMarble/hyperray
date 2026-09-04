// Footprint failure tests require all-or-nothing inventory results.
// They exercise process, protocol, output, context, and identity failures.
package isla_test

import (
	"context"
	"os"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestFootprintOperationRejectsToolResults(t *testing.T) {
	cases := []struct {
		bytes []byte
		limit uint64
		code  isla.ErrorCode
	}{
		{bytes: []byte{0xde, 0xad, 0xde, 0xad}, limit: 4096, code: isla.ProcessFail},
		{bytes: []byte{0, 0, 0, 0}, limit: 4096, code: isla.ProtocolError},
		{bytes: []byte{0xff, 0xff, 0xff, 0xff}, limit: 32, code: isla.ResourceLimit},
	}
	for index := range cases {
		testCase := cases[index]
		t.Run(string(testCase.code), func(t *testing.T) {
			instructions := []machine.Instruction{{Address: 2, Bytes: testCase.bytes}}
			request := footprintRequest(t, instructions, testCase.limit)
			report, err := footprintEngine(t).TraceInstructions(t.Context(), request)
			assertFootprintError(t, report, err, testCase.code)
		})
	}
}

func TestFootprintOperationRejectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	instructions := []machine.Instruction{{Address: 2, Bytes: []byte{1, 0}}}
	report, err := footprintEngine(t).TraceInstructions(ctx, footprintRequest(t, instructions, 4096))
	assertFootprintError(t, report, err, isla.ResourceLimit)
}

func TestFootprintOperationRejectsChangedTool(t *testing.T) {
	path := temporaryTool(t, "printf '%s\\n' v1")
	engine, err := isla.NewFootprintEngine(t.Context(), path)
	if err != nil {
		t.Fatalf("NewFootprintEngine() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0o700); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	instructions := []machine.Instruction{{Address: 2, Bytes: []byte{1, 0}}}
	report, err := engine.TraceInstructions(t.Context(), footprintRequest(t, instructions, 4096))
	assertFootprintError(t, report, err, isla.ToolChanged)
}

func TestZeroFootprintEngineCannotOperate(t *testing.T) {
	instructions := []machine.Instruction{{Address: 2, Bytes: []byte{1, 0}}}
	report, err := (isla.FootprintEngine{}).TraceInstructions(t.Context(), footprintRequest(t, instructions, 4096))
	assertFootprintError(t, report, err, isla.ToolIdentityFail)
}

func assertFootprintError(t *testing.T, report isla.FootprintReport, err error, code isla.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("report = %#v, error = nil", report)
	}
	assertErrorCode(t, err, code)
}
