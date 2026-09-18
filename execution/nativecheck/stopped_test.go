// Resource and depth limits retain incomplete status despite success markers.
// Unknown statuses remain errors rather than plausible defaults.
package nativecheck

import (
	"testing"

	"github.com/HyperMarble/hyperray/execution"
)

func TestStoppedSearch(t *testing.T) {
	for _, status := range []execution.Status{
		execution.TimedOut, execution.Canceled, execution.OutputLimit,
		execution.MemoryLimit, execution.WorkerError,
	} {
		search := completedFixture(t)
		search.Status = status
		result, err := assess(Request{Minimum: 13, Maximum: 23}, search)
		if err != nil || result.Status != Incomplete || result.Reason == "" {
			t.Errorf("status %s: %+v, error %v", status, result, err)
		}
	}
}

func TestPartialSearch(t *testing.T) {
	for _, diagnostic := range []string{
		"error: max search depth too small",
		"Warning: Search not completed", "Warning: Search incomplete",
	} {
		search := completedFixture(t)
		search.Output = diagnostic + "\n" + search.Output
		result, err := assess(Request{Minimum: 13, Maximum: 23}, search)
		if err != nil || result.Status != Incomplete || result.Reason != diagnostic {
			t.Errorf("diagnostic %s: %+v, error %v", diagnostic, result, err)
		}
	}
}

func TestUnknownStatus(t *testing.T) {
	if _, err := stopped(execution.Result{Status: "invented"}); err == nil {
		t.Fatal("unknown status accepted")
	}
	reason, err := stopped(execution.Result{Status: execution.Exited, ExitCode: 2})
	if err != nil || reason == "" {
		t.Fatalf("nonzero exit: %q, %v", reason, err)
	}
}
