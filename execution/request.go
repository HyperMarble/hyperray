// Requests describe one trusted, single-process checker and its resource limits.
// This API must not infer a proof boundary or inherit an unspecified environment.
package execution

import (
	"errors"
	"path/filepath"
	"time"
)

const MaximumMemoryBudgetBytes int64 = 100_000_000

type Request struct {
	Executable        string        `json:"executable"`
	Arguments         []string      `json:"arguments"`
	Directory         string        `json:"directory"`
	Environment       []string      `json:"environment"`
	Timeout           time.Duration `json:"timeout_nanoseconds"`
	OutputLimitBytes  int           `json:"output_limit_bytes"`
	MemoryBudgetBytes int64         `json:"memory_budget_bytes"`
}

func (request Request) Validate() error {
	if !filepath.IsAbs(request.Executable) || !filepath.IsAbs(request.Directory) {
		return errors.New("executable and directory must be absolute paths")
	}
	if request.Environment == nil {
		return errors.New("environment must be explicit; use an empty array for none")
	}
	if request.Timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if request.MemoryBudgetBytes <= 0 || request.MemoryBudgetBytes > MaximumMemoryBudgetBytes {
		return errors.New("memory budget must be positive and at most 100000000 bytes")
	}
	if request.OutputLimitBytes <= 0 || int64(request.OutputLimitBytes) > request.MemoryBudgetBytes {
		return errors.New("output limit must be positive and no greater than the memory budget")
	}
	return nil
}
