// Footprint request tests reject inventories that cannot map one-to-one.
// They also measure that the constructor copies caller-owned encodings.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestPublicFootprintRequestAndOperation(t *testing.T) {
	instructions := []machine.Instruction{
		{Address: 0x1000, Bytes: []byte{0x93, 0x02, 0x30, 0x00}},
		{Address: 0x1004, Bytes: []byte{0x01, 0x00}},
	}
	request := footprintRequest(t, instructions, 4096)
	instructions[0].Bytes[0] = 0xff
	report, err := footprintEngine(t).TraceInstructions(t.Context(), request)
	if err != nil {
		t.Fatalf("TraceInstructions() error = %v", err)
	}
	if len(report.Instructions) != 2 {
		t.Fatalf("instruction count = %d", len(report.Instructions))
	}
	first := report.Instructions[0]
	second := report.Instructions[1]
	if first.Address != 0x1000 || first.Encoding != "93023000" || first.TraceCount != 1 {
		t.Errorf("first instruction = %#v", first)
	}
	if second.Address != 0x1004 || second.Encoding != "0100" || second.TraceCount != 1 {
		t.Errorf("second instruction = %#v", second)
	}
	if first.OutputDigest == second.OutputDigest {
		t.Error("different instruction traces have one digest")
	}
	if first.TraceOutput == "" || second.TraceOutput == "" {
		t.Error("instruction trace output is empty")
	}
	if report.Evidence.Tool.Version != "v0.2.0/footprint-test" {
		t.Errorf("tool version = %q", report.Evidence.Tool.Version)
	}
	if report.Evidence.ReleaseID != "test-release" || report.Evidence.ManifestDigest == "" {
		t.Errorf("release evidence = %#v", report.Evidence)
	}
}

func TestFootprintRequestRejectsInvalidInventory(t *testing.T) {
	release := defaultFootprintRelease(t)
	cases := [][]machine.Instruction{
		nil,
		{{Address: 1, Bytes: []byte{0, 0}}},
		{{Address: 2, Bytes: []byte{0}}},
		{{Address: 2, Bytes: []byte{0, 0}}, {Address: 2, Bytes: []byte{1, 0}}},
	}
	for index := range cases {
		request, err := isla.NewFootprintRequest(release, cases[index], 1, 1, 1)
		if err == nil {
			t.Errorf("case %d request = %#v, error = nil", index, request)
		}
	}
}

func TestFootprintRequestRejectsZeroLimits(t *testing.T) {
	release := defaultFootprintRelease(t)
	instructions := []machine.Instruction{{Address: 2, Bytes: []byte{0, 0}}}
	request, err := isla.NewFootprintRequest(release, instructions, 0, 1, 1)
	if err == nil {
		t.Errorf("request = %#v, error = nil", request)
	}
}
