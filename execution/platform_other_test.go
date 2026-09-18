//go:build !darwin

// Other platforms must report unavailable accounting instead of running unchecked.
package execution_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/HyperMarble/hyperray/execution"
)

func TestUnsupportedPlatform(t *testing.T) {
	request := execution.Request{Executable: "/not-executed", Directory: "/tmp",
		Environment: []string{}, Timeout: time.Second, OutputLimitBytes: 1,
		MemoryBudgetBytes: execution.MaximumMemoryBudgetBytes}
	result, err := execution.Run(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "requires macOS") || result.Status != "" {
		t.Fatalf("unsupported platform result = %+v, error = %v", result, err)
	}
}
