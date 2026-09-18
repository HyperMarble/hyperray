// Public request construction must reject invalid limits before execution.
// A missing replay path cannot receive an implicit executable.
package nativecheck_test

import (
	"context"
	"testing"
	"time"

	"github.com/HyperMarble/hyperray/execution"
	"github.com/HyperMarble/hyperray/execution/nativecheck"
)

func TestPublicRequest(t *testing.T) {
	request := nativecheck.Request{
		Search: execution.Request{
			Executable: "/unused/search", Directory: "/unused", Environment: []string{},
			Timeout: time.Second, OutputLimitBytes: 1024, MemoryBudgetBytes: 100000000,
		},
		ReplayExecutable: "/unused/replay", Minimum: 13, Maximum: 23,
	}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*nativecheck.Request){
		func(r *nativecheck.Request) { r.Minimum = 24 },
		func(r *nativecheck.Request) { r.ReplayExecutable = "relative" },
		func(r *nativecheck.Request) { r.Search.Timeout = 0 },
	} {
		invalid := request
		change(&invalid)
		if _, err := nativecheck.Run(context.Background(), invalid); err == nil {
			t.Error("invalid public request accepted")
		}
	}
}
