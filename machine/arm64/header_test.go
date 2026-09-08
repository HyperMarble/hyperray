// These tests define the public ARM64 Mach-O header policy.
// They must not parse content or imply loader support.
package arm64_test

import (
	"debug/macho"
	"encoding/binary"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestValidateHeaderAcceptsThinARM64Executable(t *testing.T) {
	header := acceptedHeader()
	if err := arm64.ValidateHeader(header, binary.LittleEndian); err != nil {
		t.Fatalf("ValidateHeader() error = %v", err)
	}
}

func TestValidateHeaderUsesPolicyOrder(t *testing.T) {
	header := acceptedHeader()
	header.Magic = macho.Magic32
	header.Cpu = macho.CpuAmd64
	header.SubCpu = 2
	header.Type = macho.TypeDylib

	rejection := requireRejection(t, header, binary.BigEndian)
	if rejection.Code != arm64.UnsupportedMagic {
		t.Fatalf("Code = %q, want %q", rejection.Code, arm64.UnsupportedMagic)
	}
	if rejection.Detail != "0xfeedface" {
		t.Fatalf("Detail = %q, want %q", rejection.Detail, "0xfeedface")
	}
}

func TestValidateHeaderReportsNextInvalidCondition(t *testing.T) {
	cases := []struct {
		name   string
		header macho.FileHeader
		order  binary.ByteOrder
		code   arm64.RejectionCode
		detail string
	}{
		{name: "byte-order", header: macho.FileHeader{Magic: macho.Magic64, Cpu: macho.CpuAmd64, SubCpu: 2, Type: macho.TypeDylib}, order: binary.BigEndian, code: arm64.UnsupportedByteOrder, detail: "big-endian"},
		{name: "cpu", header: macho.FileHeader{Magic: macho.Magic64, Cpu: macho.CpuAmd64, SubCpu: 2, Type: macho.TypeDylib}, order: binary.LittleEndian, code: arm64.UnsupportedCPU, detail: "16777223"},
		{name: "subcpu", header: macho.FileHeader{Magic: macho.Magic64, Cpu: macho.CpuArm64, SubCpu: 2, Type: macho.TypeDylib}, order: binary.LittleEndian, code: arm64.UnsupportedSubCPU, detail: "0x00000002"},
		{name: "type", header: macho.FileHeader{Magic: macho.Magic64, Cpu: macho.CpuArm64, SubCpu: 0, Type: macho.TypeDylib}, order: binary.LittleEndian, code: arm64.UnsupportedType, detail: "6"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			rejection := requireRejection(t, testCase.header, testCase.order)
			if rejection.Code != testCase.code || rejection.Detail != testCase.detail {
				t.Fatalf("rejection = %q: %q, want %q: %q", rejection.Code, rejection.Detail, testCase.code, testCase.detail)
			}
		})
	}
}
