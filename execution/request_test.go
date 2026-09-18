// Request tests exercise explicit public bounds without starting a worker.
// Invalid requests must not silently receive defaults.
package execution_test

import (
	"testing"
	"time"

	"github.com/HyperMarble/hyperray/execution"
)

func TestRequestValidation(t *testing.T) {
	valid := execution.Request{Executable: "/bin/sh", Directory: "/tmp",
		Environment: []string{}, Timeout: time.Second, OutputLimitBytes: 64,
		MemoryBudgetBytes: execution.MaximumMemoryBudgetBytes}
	mutations := map[string]func(*execution.Request){
		"relative executable":  func(r *execution.Request) { r.Executable = "sh" },
		"relative directory":   func(r *execution.Request) { r.Directory = "." },
		"missing environment":  func(r *execution.Request) { r.Environment = nil },
		"zero timeout":         func(r *execution.Request) { r.Timeout = 0 },
		"negative timeout":     func(r *execution.Request) { r.Timeout = -1 },
		"zero output":          func(r *execution.Request) { r.OutputLimitBytes = 0 },
		"negative output":      func(r *execution.Request) { r.OutputLimitBytes = -1 },
		"zero memory":          func(r *execution.Request) { r.MemoryBudgetBytes = 0 },
		"excess memory":        func(r *execution.Request) { r.MemoryBudgetBytes++ },
		"output beyond budget": func(r *execution.Request) { r.OutputLimitBytes = 100_000_001 },
	}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			request := valid
			mutate(&request)
			if err := request.Validate(); err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
}
