// These tests reject each unsupported ARM64 Mach-O header condition.
// They must assert the public error code and detail.
package arm64_test

import (
	"debug/macho"
	"encoding/binary"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestValidateHeaderRejectsEachUnsupportedCondition(t *testing.T) {
	cases := []struct {
		name   string
		header macho.FileHeader
		order  binary.ByteOrder
		code   arm64.RejectionCode
		detail string
	}{
		{name: "magic", header: withMagic(macho.Magic32), order: binary.LittleEndian, code: arm64.UnsupportedMagic, detail: "0xfeedface"},
		{name: "byte-order-nil", header: acceptedHeader(), code: arm64.UnsupportedByteOrder, detail: "nil"},
		{name: "byte-order-big", header: acceptedHeader(), order: binary.BigEndian, code: arm64.UnsupportedByteOrder, detail: "big-endian"},
		{name: "cpu-x86-64", header: withCPU(macho.CpuAmd64), order: binary.LittleEndian, code: arm64.UnsupportedCPU, detail: "16777223"},
		{name: "cpu-arm-32", header: withCPU(macho.CpuArm), order: binary.LittleEndian, code: arm64.UnsupportedCPU, detail: "12"},
		{name: "subcpu-arm64e", header: withSubCPU(2), order: binary.LittleEndian, code: arm64.UnsupportedSubCPU, detail: "0x00000002"},
		{name: "subcpu-capability", header: withSubCPU(0x80000000), order: binary.LittleEndian, code: arm64.UnsupportedSubCPU, detail: "0x80000000"},
		{name: "type-object", header: withType(macho.TypeObj), order: binary.LittleEndian, code: arm64.UnsupportedType, detail: "1"},
		{name: "type-dylib", header: withType(macho.TypeDylib), order: binary.LittleEndian, code: arm64.UnsupportedType, detail: "6"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			rejection := requireRejection(t, testCase.header, testCase.order)
			if rejection.Code != testCase.code {
				t.Errorf("Code = %q, want %q", rejection.Code, testCase.code)
			}
			if rejection.Detail != testCase.detail {
				t.Errorf("Detail = %q, want %q", rejection.Detail, testCase.detail)
			}
		})
	}
}
