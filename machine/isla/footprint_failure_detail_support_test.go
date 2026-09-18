// Resource-limit test helpers inspect typed command details.
// They reject partial reports and errors without structured failure data.
package isla_test

import (
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func assertResourceLimitDetail(t *testing.T, report isla.FootprintReport, err error) *isla.ResourceLimitDetail {
	t.Helper()
	if err == nil || len(report.Instructions) != 0 {
		t.Fatalf("report = %#v, error = %v", report, err)
	}
	var failure *isla.Error
	if !errors.As(err, &failure) || failure.ResourceLimit == nil {
		t.Fatalf("error = %v", err)
	}
	if failure.ResourceLimit.ElapsedMilliseconds < 0 {
		t.Fatalf("elapsed milliseconds = %d", failure.ResourceLimit.ElapsedMilliseconds)
	}
	return failure.ResourceLimit
}
