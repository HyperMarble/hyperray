//go:build isla_integration && arm64_acceptance

// Packet witness checks require exact ARM register assignments.
// They must not accept substring-only or alias-ambiguous evidence.
package isla_test

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func assertPacketWitness(t *testing.T, assignments map[string]string) {
	t.Helper()
	r0, err := arm64WitnessValue(assignments, []string{"0:R0", "0:X0"})
	if err != nil || r0 != packetCorrectValue {
		t.Fatalf("R0 witness = %#x, %v", r0, err)
	}
	sp, err := arm64WitnessValue(assignments, []string{"0:SP_EL0"})
	if err != nil || sp != 0x3c40 {
		t.Fatalf("SP_EL0 witness = %#x, %v", sp, err)
	}
	pc, err := arm64WitnessValue(assignments, []string{"0:_PC"})
	if err != nil || pc != packetReturn {
		t.Fatalf("PC witness = %#x, %v", pc, err)
	}
}

func assertPacketInitialBinding(t *testing.T, program isla.Program, testCase packetCase) {
	t.Helper()
	registers := make(map[string]string)
	for _, register := range program.PostResetRegisters() {
		registers[register.Name] = register.Value
	}
	for name, want := range map[string]uint64{"R0": 0x400000, "R1": uint64(testCase.InputLength), "R30": packetReturn, "SP_EL0": 0x3c40} {
		value, err := strconv.ParseUint(registers[name], 0, 64)
		if err != nil || value != want {
			t.Fatalf("case %q initial %s = %q, want %#x", testCase.Name, name, registers[name], want)
		}
	}
	input, ok := program.MemoryInput()
	if !ok || len(input.Backing) < 1 {
		t.Fatalf("case %q lacks explicit input backing", testCase.Name)
	}
	want, err := hex.DecodeString(testCase.Bytes)
	if err != nil {
		t.Fatalf("case %q decode input: %v", testCase.Name, err)
	}
	if input.Backing[0].Address != 0x400000 || !bytes.Equal(input.Backing[0].Bytes, want) {
		t.Fatalf("case %q input backing is not the exact 32-byte packet", testCase.Name)
	}
}

func assertPacketResultBinding(t *testing.T, result isla.ExecutableResult, program isla.Program) {
	t.Helper()
	if result.Program.ProgramDigest != program.Digest() || result.Program.MemoryIdentity != program.Evidence().MemoryIdentity {
		t.Fatalf("result does not bind the rendered query to the initial input: %#v", result.Program)
	}
	if result.Capability == nil || result.Capability.ExecutionProfile != string(isla.ARM64NormalS1Fixed4KSIMD) || result.Capability.MemoryConfigurationDigest == "" || result.Capability.EffectiveStateDigest == "" {
		t.Fatalf("result lacks static measured capability evidence: %#v", result.Capability)
	}
}
