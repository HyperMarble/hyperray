// These tests preserve the existing reader's first typed rejection.
// They must return nil records for every malformed command or segment.
package arm64_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestReadSectionHeadersPreservesExistingErrors(t *testing.T) {
	valid := sectionCommand("VALID", sectionFixture{name: "text", segment: "VALID"})
	later := syntheticSegment(0x19, 80, "invalid", 0)
	laterContent := syntheticFile(2, append(valid, later...))
	truncated := syntheticSegment(0x19, 152, "truncated", 1)[:112]
	cases := []struct {
		name    string
		content []byte
	}{
		{name: "outer command", content: syntheticFile(1, syntheticCommand(1, 4))},
		{name: "segment range", content: invalidRangeContent()},
		{name: "later segment", content: laterContent},
		{name: "huge section count", content: syntheticFile(1, syntheticSegment(0x19, 72, "huge", ^uint32(0)))},
		{name: "truncated record", content: syntheticFile(1, truncated)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			want, wantErr := arm64.ReadSegmentHeaders(test.content)
			if want != nil || wantErr == nil {
				t.Fatalf("ReadSegmentHeaders() = %#v, %v; want error", want, wantErr)
			}
			got, gotErr := arm64.ReadSectionHeaders(test.content)
			if got != nil || gotErr == nil {
				t.Fatalf("ReadSectionHeaders() = %#v, %v; want nil and error", got, gotErr)
			}
			assertSameRejection(t, gotErr, wantErr)
		})
	}
}

func TestReadSectionHeadersReturnsEmptyForValidEmptyInput(t *testing.T) {
	got, err := arm64.ReadSectionHeaders(syntheticFile(0, nil))
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("ReadSectionHeaders() = %#v, %v; want non-nil empty records", got, err)
	}
}

func invalidRangeContent() []byte {
	command := syntheticSegment(0x19, 72, "outside", 0)
	setSegmentRange(command, 0, 1, 999, 1)
	return syntheticFile(1, command)
}

func assertSameRejection(t *testing.T, got, want error) {
	t.Helper()
	var gotRejection, wantRejection *arm64.Rejection
	if !errors.As(got, &gotRejection) || !errors.As(want, &wantRejection) {
		t.Fatalf("error types = %T and %T; want *arm64.Rejection", got, want)
	}
	if !reflect.DeepEqual(gotRejection, wantRejection) {
		t.Errorf("rejection = %#v, want %#v", gotRejection, wantRejection)
	}
}
