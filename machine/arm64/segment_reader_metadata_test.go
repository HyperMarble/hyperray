// These tests cover ordered metadata, names, unknown commands, and empty results.
// They must not infer runtime behavior from metadata-only success.
package arm64_test

import (
	"debug/macho"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestReadSegmentHeadersPreservesOrderedMetadata(t *testing.T) {
	first := syntheticSegment(macho.LoadCmdSegment64, 72, "first", 0)
	second := syntheticSegment(macho.LoadCmdSegment64, 72, "second", 0)
	setSegmentRange(first, 0x1000, 0x20, 0, 0)
	setSegmentRange(second, 0x2000, 0x20, 72, 8)
	binary.LittleEndian.PutUint32(first[56:60], 7)
	binary.LittleEndian.PutUint32(first[60:64], 5)
	binary.LittleEndian.PutUint32(first[68:72], 9)
	content := syntheticFile(3, append(syntheticCommand(0x12345678, 8), append(first, second...)...))

	headers, err := arm64.ReadSegmentHeaders(content)
	if err != nil {
		t.Fatalf("ReadSegmentHeaders() error = %v", err)
	}
	want := []macho.SegmentHeader{{Cmd: macho.LoadCmdSegment64, Len: 72, Name: "first", Addr: 0x1000, Memsz: 0x20, Offset: 0, Filesz: 0, Maxprot: 7, Prot: 5, Nsect: 0, Flag: 9}, {Cmd: macho.LoadCmdSegment64, Len: 72, Name: "second", Addr: 0x2000, Memsz: 0x20, Offset: 72, Filesz: 8}}
	if !reflect.DeepEqual(headers, want) {
		t.Errorf("headers = %#v, want %#v", headers, want)
	}
}

func TestReadSegmentHeadersUsesMachNames(t *testing.T) {
	noNUL := syntheticSegment(macho.LoadCmdSegment64, 72, "0123456789abcdef", 0)
	embedded := syntheticSegment(macho.LoadCmdSegment64, 72, "prefix\x00suffix", 0)
	content := syntheticFile(2, append(noNUL, embedded...))

	headers, err := arm64.ReadSegmentHeaders(content)
	if err != nil {
		t.Fatalf("ReadSegmentHeaders() error = %v", err)
	}
	want := []string{"0123456789abcdef", "prefix"}
	if len(headers) != len(want) {
		t.Fatalf("len(headers) = %d, want %d", len(headers), len(want))
	}
	for index, header := range headers {
		if header.Name != want[index] || strings.Contains(header.Name, "suffix") {
			t.Errorf("headers[%d].Name = %q, want %q", index, header.Name, want[index])
		}
	}
}

func TestReadSegmentHeadersAcceptsUnknownCommandsAndEmptyTables(t *testing.T) {
	unknownBefore := syntheticCommand(0x7fffffff, 8)
	segment := syntheticSegment(macho.LoadCmdSegment64, 72, "only", 0)
	unknownAfter := syntheticCommand(0x7ffffffe, 8)
	table := append(unknownBefore, segment...)
	table = append(table, unknownAfter...)
	content := syntheticFile(3, table)
	headers, err := arm64.ReadSegmentHeaders(content)
	if err != nil || len(headers) != 1 || headers[0].Name != "only" {
		t.Errorf("unknown command result = %#v, %v", headers, err)
	}
	empty, err := arm64.ReadSegmentHeaders(syntheticFile(0, nil))
	if err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("empty result = %#v, %v", empty, err)
	}
}
