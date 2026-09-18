// Results distinguish backend reports from reproduced counterexamples.
// These diagnostic outcomes never authorize a mathematical proof verdict.
package nativecheck

import "github.com/HyperMarble/hyperray/execution"

type Status string

const (
	SearchReportedComplete   Status = "search_reported_complete"
	CounterexampleReproduced Status = "counterexample_reproduced"
	Incomplete               Status = "incomplete"
)

type Counterexample struct {
	Input  uint64 `json:"input"`
	Output uint64 `json:"output"`
}

type Result struct {
	Status         Status            `json:"status"`
	Reason         string            `json:"reason,omitempty"`
	Counterexample *Counterexample   `json:"counterexample,omitempty"`
	Search         execution.Result  `json:"search"`
	Replay         *execution.Result `json:"replay,omitempty"`
}
