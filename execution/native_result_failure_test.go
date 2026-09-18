//go:build preparation_integration && darwin

// Real search results cannot pass with another replay or a false input boundary.
// Failed validation must preserve search output without publishing a reproduced bug.
package execution_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/execution/nativecheck"
)

func TestNativeResultRejectedReplay(t *testing.T) {
	root, preparer, tools := preparationTools(t)
	request := nativePreparation(root, filepath.Join(t.TempDir(), "broken"), tools)
	request.Subject.Function = "workflow::broken"
	broken := prepareNative(t, preparer, request)
	request.Directory = filepath.Join(t.TempDir(), "correct")
	request.Subject.Function = "workflow::solve"
	correct := prepareNative(t, preparer, request)
	for _, change := range []func(*nativecheck.Request){
		func(r *nativecheck.Request) { r.ReplayExecutable = correct.Replay },
		func(r *nativecheck.Request) { r.ReplayExecutable = filepath.Join(t.TempDir(), "missing") },
		func(r *nativecheck.Request) {
			r.Minimum = 0
			r.Maximum = 0
		},
	} {
		input := nativeRequest(broken)
		change(&input)
		result, err := nativecheck.Run(context.Background(), input)
		if err == nil || result.Status != nativecheck.Incomplete || result.Counterexample != nil {
			t.Fatalf("accepted invalid replay or boundary: %+v, %v", result, err)
		}
		if !strings.Contains(result.Search.Output, "COUNTEREXAMPLE input=23") {
			t.Fatal("validation error lost the original search evidence")
		}
	}
}
