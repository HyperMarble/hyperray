// External callers construct fixture requests through public fields only.
// Fixtures must not require edits to the execution package.
package execution_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/HyperMarble/hyperray/execution"
)

func fixtureRequest(t *testing.T, mode string) execution.Request {
	t.Helper()
	fixture, err := filepath.Abs("../fixtures/execution/worker.sh")
	if err != nil {
		t.Fatal(err)
	}
	return execution.Request{Executable: "/bin/sh", Arguments: []string{fixture, mode},
		Directory: t.TempDir(), Environment: []string{}, Timeout: time.Second,
		OutputLimitBytes: 4096, MemoryBudgetBytes: execution.MaximumMemoryBudgetBytes}
}
