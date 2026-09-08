// These tests cover declared leftovers and command-count tail conditions.
// All byte slices in this file are synthetic command-table inputs.
package arm64_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestValidateCommandTableRejectsSyntheticTrailingBytes(t *testing.T) {
	emptyTableLeftover := syntheticHeader(0, 4)
	emptyTableLeftover = append(emptyTableLeftover, []byte{0, 0, 0, 0}...)
	assertTableRejection(t, emptyTableLeftover, arm64.TrailingCommandBytes, "after command table offset=32: cursor=0 cmdsz=4")
	emptyTableCommand := syntheticFile(0, syntheticCommand(1, 8))
	assertTableRejection(t, emptyTableCommand, arm64.TrailingCommandBytes, "after command table offset=32: cursor=0 cmdsz=8")

	table := append(syntheticCommand(1, 8), syntheticCommand(2, 8)...)
	content := syntheticFile(1, table)
	assertTableRejection(t, content, arm64.TrailingCommandBytes, "after command[0] offset=40: cursor=8 cmdsz=16")
}

func TestValidateCommandTableRejectsSyntheticIncompletePrefix(t *testing.T) {
	table := append(syntheticCommand(1, 16), make([]byte, 8)...)
	content := syntheticFile(2, table)
	assertTableRejection(t, content, arm64.TruncatedCommandPrefix, "command[1] offset=48: need 8-byte prefix, have 0 bytes")
}

func TestValidateCommandTableRejectsLargerSyntheticCommandCount(t *testing.T) {
	content := syntheticFile(2, syntheticCommand(1, 8))
	assertTableRejection(t, content, arm64.ImpossibleCommandCount, "ncmd=2 cmdsz=8 requires at least 16 bytes")
}
