// Replay equality covers the input, output, observation count, and exit status.
// Report mutations must not become reproduced counterexamples.
package nativecheck

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/execution"
)

func TestReplayEquality(t *testing.T) {
	candidate := Counterexample{Input: 23, Output: 29}
	output := "COUNTEREXAMPLE input=23 output=29\nOBSERVATIONS count=1 overflow=0\n"
	replay := execution.Result{Status: execution.Exited, ExitCode: 1, Output: output}
	if reason, err := replayMatches(replay, candidate); err != nil || reason != "" {
		t.Fatalf("matching replay: %q, %v", reason, err)
	}
	for _, change := range [][2]string{
		{"input=23", "input=22"}, {"output=29", "output=28"},
		{"count=1", "count=2"}, {"overflow=0", "overflow=1"},
		{"input=23", "input=18446744073709551616"},
		{"output=29", "output=18446744073709551616"},
		{"COUNTEREXAMPLE", "MISSING"},
	} {
		replay.Output = strings.ReplaceAll(output, change[0], change[1])
		if _, err := replayMatches(replay, candidate); err == nil {
			t.Errorf("accepted replay mutation %q", change)
		}
	}
	replay.Output = output
	replay.ExitCode = 0
	if _, err := replayMatches(replay, candidate); err == nil {
		t.Fatal("accepted a replay without a violation")
	}
	replay.Status = execution.TimedOut
	if reason, err := replayMatches(replay, candidate); err != nil || reason == "" {
		t.Fatalf("stopped replay: %q, %v", reason, err)
	}
}
