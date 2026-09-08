// These helpers build public Mach-O header inputs for policy tests.
// They must keep each negative case based on one valid baseline.
package arm64_test

import (
	"debug/macho"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func requireRejection(t *testing.T, header macho.FileHeader, order binary.ByteOrder) *arm64.Rejection {
	t.Helper()
	err := arm64.ValidateHeader(header, order)
	if err == nil {
		t.Fatal("ValidateHeader() error = nil")
	}
	var rejection *arm64.Rejection
	if !errors.As(err, &rejection) {
		t.Fatalf("error type = %T, want *arm64.Rejection", err)
	}
	return rejection
}

func acceptedHeader() macho.FileHeader {
	return macho.FileHeader{Magic: macho.Magic64, Cpu: macho.CpuArm64, Type: macho.TypeExec, Flags: ^uint32(0)}
}

func withMagic(magic uint32) macho.FileHeader {
	header := acceptedHeader()
	header.Magic = magic
	return header
}

func withCPU(cpu macho.Cpu) macho.FileHeader {
	header := acceptedHeader()
	header.Cpu = cpu
	return header
}

func withSubCPU(subCPU uint32) macho.FileHeader {
	header := acceptedHeader()
	header.SubCpu = subCPU
	return header
}

func withType(fileType macho.Type) macho.FileHeader {
	header := acceptedHeader()
	header.Type = fileType
	return header
}
