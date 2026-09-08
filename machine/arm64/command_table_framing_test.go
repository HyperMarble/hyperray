// These tests cover command-table bounds, sizes, counts, and exact endings.
// All byte slices in this file are synthetic command-table inputs.
package arm64_test

import (
	"math"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestValidateCommandTableRejectsSyntheticTableBounds(t *testing.T) {
	outside := syntheticHeader(0, 8)
	assertTableRejection(t, outside, arm64.CommandTableOutsideArtifact, "offset=32 cmdsz=8 exceeds artifact size=32")

	huge := syntheticHeader(math.MaxUint32, math.MaxUint32)
	assertTableRejection(t, huge, arm64.CommandTableOutsideArtifact, "offset=32 cmdsz=4294967295 exceeds artifact size=32")
	hugeCount := syntheticFile(math.MaxUint32, syntheticCommand(1, 8))
	assertTableRejection(t, hugeCount, arm64.ImpossibleCommandCount, "ncmd=4294967295 cmdsz=8 requires at least 34359738360 bytes")

	impossible := syntheticFile(2, syntheticCommand(1, 8))
	assertTableRejection(t, impossible, arm64.ImpossibleCommandCount, "ncmd=2 cmdsz=8 requires at least 16 bytes")
}

func TestValidateCommandTableAcceptsSyntheticExactBoundaryCommands(t *testing.T) {
	one := syntheticFile(1, syntheticCommand(0xffffffff, 8))
	if err := arm64.ValidateCommandTable(one); err != nil {
		t.Fatalf("one command error = %v", err)
	}
	two := syntheticFile(2, append(syntheticCommand(1, 8), syntheticCommand(2, 8)...))
	if err := arm64.ValidateCommandTable(two); err != nil {
		t.Fatalf("two commands error = %v", err)
	}
}

func TestValidateCommandTableRejectsSyntheticCommandSizes(t *testing.T) {
	cases := []struct {
		name   string
		size   uint32
		code   arm64.RejectionCode
		detail string
	}{
		{"zero", 0, arm64.InvalidCommandSize, "command[0] offset=32: cmdsize=0 is less than 8"},
		{"four", 4, arm64.InvalidCommandSize, "command[0] offset=32: cmdsize=4 is less than 8"},
		{"seven", 7, arm64.InvalidCommandSize, "command[0] offset=32: cmdsize=7 is less than 8"},
		{"nine", 9, arm64.InvalidCommandAlignment, "command[0] offset=32: cmdsize=9 is not 8-byte aligned"},
		{"too-large", 16, arm64.CommandOutsideTable, "command[0] offset=32: cmdsize=16 exceeds remaining=8"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			content := syntheticFile(1, syntheticCommand(1, testCase.size))
			assertTableRejection(t, content, testCase.code, testCase.detail)
		})
	}
}
