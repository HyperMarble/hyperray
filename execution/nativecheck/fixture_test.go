// Load an actual generated search report for protocol mutation tests.
// These records do not replace compiler-backed integration tests.
package nativecheck

import (
	"os"
	"testing"

	"github.com/HyperMarble/hyperray/execution"
)

func completedFixture(t *testing.T) execution.Result {
	t.Helper()
	content, err := os.ReadFile("../../fixtures/execution/native-completed.txt")
	if err != nil {
		t.Fatal(err)
	}
	return execution.Result{Status: execution.Exited, Output: string(content)}
}
