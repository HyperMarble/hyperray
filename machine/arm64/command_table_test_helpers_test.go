// These helpers build named synthetic thin ARM64 Mach-O bytes.
// They must not call or mirror the production validator.
package arm64_test

import (
	"debug/macho"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func syntheticHeader(ncmd, cmdsz uint32) []byte {
	content := make([]byte, 32)
	binary.LittleEndian.PutUint32(content[0:4], macho.Magic64)
	binary.LittleEndian.PutUint32(content[4:8], uint32(macho.CpuArm64))
	binary.LittleEndian.PutUint32(content[8:12], 0)
	binary.LittleEndian.PutUint32(content[12:16], uint32(macho.TypeExec))
	binary.LittleEndian.PutUint32(content[16:20], ncmd)
	binary.LittleEndian.PutUint32(content[20:24], cmdsz)
	return content
}

func syntheticFile(ncmd uint32, table []byte) []byte {
	content := syntheticHeader(ncmd, uint32(len(table)))
	return append(content, table...)
}

func syntheticCommand(commandID, commandSize uint32) []byte {
	command := make([]byte, 8)
	binary.LittleEndian.PutUint32(command[0:4], commandID)
	binary.LittleEndian.PutUint32(command[4:8], commandSize)
	return command
}

func requireTableRejection(t *testing.T, content []byte) *arm64.Rejection {
	t.Helper()
	err := arm64.ValidateCommandTable(content)
	if err == nil {
		t.Fatal("ValidateCommandTable() error = nil")
	}
	var rejection *arm64.Rejection
	if !errors.As(err, &rejection) {
		t.Fatalf("error type = %T, want *arm64.Rejection", err)
	}
	return rejection
}

func assertTableRejection(t *testing.T, content []byte, code arm64.RejectionCode, detail string) {
	t.Helper()
	rejection := requireTableRejection(t, content)
	if rejection.Code != code {
		t.Errorf("Code = %q, want %q", rejection.Code, code)
	}
	if rejection.Detail != detail {
		t.Errorf("Detail = %q, want %q", rejection.Detail, detail)
	}
}
