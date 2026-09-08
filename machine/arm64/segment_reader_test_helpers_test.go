// These helpers build bounded segment-reader inputs and inspect typed errors.
// They must not mirror validation policy or allocate from an encoded section count.
package arm64_test

import (
	"debug/macho"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func syntheticSegment(commandID macho.LoadCmd, commandSize uint32, name string, nsect uint32) []byte {
	command := make([]byte, int(commandSize))
	binary.LittleEndian.PutUint32(command[0:4], uint32(commandID))
	binary.LittleEndian.PutUint32(command[4:8], commandSize)
	copy(command[8:24], []byte(name))
	if commandSize >= 72 {
		binary.LittleEndian.PutUint32(command[64:68], nsect)
	}
	return command
}

func setSegmentRange(command []byte, addr, memsz, offset, filesz uint64) {
	binary.LittleEndian.PutUint64(command[24:32], addr)
	binary.LittleEndian.PutUint64(command[32:40], memsz)
	binary.LittleEndian.PutUint64(command[40:48], offset)
	binary.LittleEndian.PutUint64(command[48:56], filesz)
}

func requireReaderRejection(t *testing.T, content []byte) *arm64.Rejection {
	t.Helper()
	headers, err := arm64.ReadSegmentHeaders(content)
	if err == nil {
		t.Fatalf("ReadSegmentHeaders() headers = %#v, error = nil", headers)
	}
	if headers != nil {
		t.Errorf("ReadSegmentHeaders() headers = %#v, want nil", headers)
	}
	var rejection *arm64.Rejection
	if !errors.As(err, &rejection) {
		t.Fatalf("error type = %T, want *arm64.Rejection", err)
	}
	return rejection
}

func assertReaderRejection(t *testing.T, content []byte, want arm64.RejectionCode, detail string) {
	t.Helper()
	rejection := requireReaderRejection(t, content)
	if rejection.Code != want || rejection.Detail != detail {
		t.Errorf("rejection = %#v, want code %q and detail %q", rejection, want, detail)
	}
}
