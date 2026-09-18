// Command tests require JSON diagnostics for both normal and nonzero worker exits.
// Command success must never add a proof verdict to the result.
package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HyperMarble/hyperray/execution"
)

func TestObserveCommand(t *testing.T) {
	for _, mode := range []string{"streams", "failure"} {
		t.Run(mode, func(t *testing.T) {
			fixture, err := filepath.Abs("../../fixtures/execution/worker.sh")
			if err != nil {
				t.Fatal(err)
			}
			request := execution.Request{Executable: "/bin/sh", Arguments: []string{fixture, mode},
				Directory: t.TempDir(), Environment: []string{}, Timeout: time.Second,
				OutputLimitBytes: 4096, MemoryBudgetBytes: execution.MaximumMemoryBudgetBytes}
			output, commandError := executeObserve(writeJSONFixture(t, "request.json", request))
			var result execution.Result
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatalf("invalid result %q: %v", output, err)
			}
			if (commandError != nil) != (mode == "failure") {
				t.Fatalf("command error = %v", commandError)
			}
			if result.Status != execution.Exited || strings.Contains(output, "PROVED") {
				t.Fatalf("unexpected result %q", output)
			}
		})
	}
}

func executeObserve(path string) (string, error) {
	output := new(bytes.Buffer)
	command := newRootCmd()
	command.SetOut(output)
	command.SetArgs([]string{"observe", path})
	err := command.Execute()
	return output.String(), err
}
