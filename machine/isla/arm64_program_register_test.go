// ARM64 reset normalization tests protect the native 64-bit value contract.
// Equivalent source spellings must produce identical deterministic reset data.
package isla_test

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildARM64ProgramNormalizesResetValuesTo64Bits(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	boundary := arm64ProgramBoundary(start, end)
	boundary.PostResetRegisters = append(boundary.PostResetRegisters,
		isla.RegisterValue{Name: "R1", Value: "0x3"},
		isla.RegisterValue{Name: "R2", Value: "0"},
		isla.RegisterValue{Name: "R3", Value: "0xffffffffffffffff"},
	)
	program, err := isla.BuildARM64Program(content, 32768, boundary)
	if err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
	source := string(program.Content())
	for _, expected := range []string{
		"\"R0\" = \"0x0000000000000003\"",
		"\"R1\" = \"0x0000000000000003\"",
		"\"R2\" = \"0x0000000000000000\"",
		"\"R3\" = \"0xffffffffffffffff\"",
		"\"R30\" = \"0x0000000100008000\"",
		"\"SP_EL0\" = \"0x0000000000003c40\"",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated reset lacks %q", expected)
		}
	}
}

func TestBuildARM64ProgramNormalizesEquivalentRegisterSpellings(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	firstBoundary := arm64ProgramBoundary(start, end)
	secondBoundary := arm64ProgramBoundary(start, end)
	secondBoundary.PostResetRegisters[0].Value = "0x3"
	first, err := isla.BuildARM64Program(content, 32768, firstBoundary)
	if err != nil {
		t.Fatalf("first BuildARM64Program() error = %v", err)
	}
	second, err := isla.BuildARM64Program(content, 32768, secondBoundary)
	if err != nil {
		t.Fatalf("second BuildARM64Program() error = %v", err)
	}
	if string(first.Content()) != string(second.Content()) {
		t.Fatal("equivalent register spellings produced different program data")
	}
	if first.PostResetRegisters()[0].Value != second.PostResetRegisters()[0].Value {
		t.Fatal("equivalent register spellings produced different metadata")
	}
}
