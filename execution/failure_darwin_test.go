// Invalid starts and canceled callers must produce explicit errors.
// The observer must not fabricate a worker result when nothing started.
package execution_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/execution"
)

func TestCanceledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := execution.Run(ctx, fixtureRequest(t, "silent"))
	if !errors.Is(err, context.Canceled) || result.Status != "" {
		t.Fatalf("cancellation result = %+v, error = %v", result, err)
	}
}

func TestInvalidStart(t *testing.T) {
	request := fixtureRequest(t, "silent")
	request.Executable = filepath.Join(t.TempDir(), "missing-checker")
	result, err := execution.Run(context.Background(), request)
	if err == nil || result.Status != "" {
		t.Fatalf("missing executable result = %+v, error = %v", result, err)
	}
	request = fixtureRequest(t, "silent")
	request.Timeout = 0
	if _, err := execution.Run(context.Background(), request); err == nil {
		t.Fatal("invalid request started")
	}
	if _, err := execution.Run(nil, request); err == nil {
		t.Fatal("nil context accepted")
	}
}
