// These tests define the public RISC-V instruction parcel length rule.
// They must not decode opcodes or import the parent machine package.
package riscv_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/riscv"
)

func TestInstructionLengthBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		parcel uint16
		length int
	}{
		{name: "lowest-two-bit-zero", parcel: 0x0000, length: 2},
		{name: "lowest-two-bit-one", parcel: 0x0001, length: 2},
		{name: "lowest-two-bit-two", parcel: 0x0002, length: 2},
		{name: "first-four-byte-encoding", parcel: 0x0003, length: 4},
		{name: "last-four-byte-encoding", parcel: 0x001b, length: 4},
		{name: "first-unsupported-encoding", parcel: 0x001f, length: 0},
		{name: "next-parcel-after-unsupported", parcel: 0x0020, length: 2},
		{name: "highest-parcel", parcel: 0xffff, length: 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := riscv.InstructionLength(test.parcel); got != test.length {
				t.Fatalf("InstructionLength(%#04x) = %d, want %d", test.parcel, got, test.length)
			}
		})
	}
}

func TestInstructionLengthMatchesEncodingRuleForEveryFirstParcel(t *testing.T) {
	for parcel := 0; parcel <= 0xffff; parcel++ {
		firstParcel := uint16(parcel)
		want := 0
		if firstParcel&0x3 != 0x3 {
			want = 2
		} else if firstParcel&0x1f != 0x1f {
			want = 4
		}
		if got := riscv.InstructionLength(firstParcel); got != want {
			t.Fatalf("InstructionLength(%#04x) = %d, want %d", firstParcel, got, want)
		}
	}
}
