// These tests define the public RISC-V ELF header predicates.
// They must not depend on the parent machine package.
package riscv_test

import (
	"debug/elf"
	"testing"

	"github.com/HyperMarble/hyperray/machine/riscv"
)

func TestAcceptsMachineRecognizesOnlyRISCV(t *testing.T) {
	cases := []struct {
		name    string
		machine elf.Machine
		accept  bool
	}{
		{name: "riscv", machine: elf.EM_RISCV, accept: true},
		{name: "before-riscv", machine: elf.Machine(242), accept: false},
		{name: "after-riscv", machine: elf.Machine(244), accept: false},
		{name: "x86_64", machine: elf.EM_X86_64, accept: false},
		{name: "aarch64", machine: elf.EM_AARCH64, accept: false},
		{name: "arm", machine: elf.EM_ARM, accept: false},
		{name: "none", machine: elf.EM_NONE, accept: false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := riscv.AcceptsMachine(test.machine); got != test.accept {
				t.Fatalf("AcceptsMachine(%v) = %t, want %t", test.machine, got, test.accept)
			}
		})
	}
}

func TestAcceptsFlagsRecognizesOnlyDoubleFloatWithOptionalCompressed(t *testing.T) {
	cases := []struct {
		name   string
		flags  uint32
		accept bool
	}{
		{name: "double-float", flags: 0x4, accept: true},
		{name: "double-float-compressed", flags: 0x5, accept: true},
		{name: "zero", flags: 0x0, accept: false},
		{name: "compressed-only", flags: 0x1, accept: false},
		{name: "single-float", flags: 0x2, accept: false},
		{name: "quad-float", flags: 0x6, accept: false},
		{name: "nearby-unknown", flags: 0x7, accept: false},
		{name: "unknown-bit", flags: 0x8, accept: false},
		{name: "double-float-unknown-bit", flags: 0xc, accept: false},
		{name: "all-bits", flags: ^uint32(0), accept: false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := riscv.AcceptsFlags(test.flags); got != test.accept {
				t.Fatalf("AcceptsFlags(%#x) = %t, want %t", test.flags, got, test.accept)
			}
		})
	}
}
