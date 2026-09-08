// These tests cover byte-order and ARM64 profile rejection through raw bytes.
// Each input is synthetic and uses one changed header field.
package arm64_test

import (
	"debug/macho"
	"encoding/binary"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestValidateCommandTableRejectsSyntheticProfileConditions(t *testing.T) {
	cases := []struct {
		name   string
		change func([]byte)
		code   arm64.RejectionCode
		detail string
	}{
		{"swapped64", putSwappedMagic, arm64.UnsupportedByteOrder, "big-endian"},
		{"cpu", putUnsupportedCPU, arm64.UnsupportedCPU, "16777223"},
		{"subcpu", putUnsupportedSubCPU, arm64.UnsupportedSubCPU, "0x00000002"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			content := syntheticHeader(0, 0)
			testCase.change(content)
			assertTableRejection(t, content, testCase.code, testCase.detail)
		})
	}
}

func putSwappedMagic(content []byte) {
	binary.BigEndian.PutUint32(content[0:4], macho.Magic64)
	binary.BigEndian.PutUint32(content[4:8], uint32(macho.CpuArm64))
	binary.BigEndian.PutUint32(content[12:16], uint32(macho.TypeExec))
}

func putUnsupportedCPU(content []byte) {
	binary.LittleEndian.PutUint32(content[4:8], uint32(macho.CpuAmd64))
}

func putUnsupportedSubCPU(content []byte) {
	binary.LittleEndian.PutUint32(content[8:12], 2)
}
