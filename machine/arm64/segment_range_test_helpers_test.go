// These helpers inspect the public segment range rejection.
// They must preserve the exact public code and detail.
package arm64_test

import (
	"debug/macho"
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func requireSegmentRejection(t *testing.T, header macho.SegmentHeader, artifactSize uint64) *arm64.Rejection {
	t.Helper()
	err := arm64.ValidateSegmentRange(header, artifactSize)
	if err == nil {
		t.Fatal("ValidateSegmentRange() error = nil")
	}
	var rejection *arm64.Rejection
	if !errors.As(err, &rejection) {
		t.Fatalf("error type = %T, want *arm64.Rejection", err)
	}
	return rejection
}
