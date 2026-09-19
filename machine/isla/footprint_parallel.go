// Tracing the instructions a stored trace does not already answer.
// Every instruction must succeed; one failure discards the whole report.
package isla

import (
	"context"
	"encoding/hex"
	"runtime"
	"sync"

	"github.com/HyperMarble/hyperray/machine"
)

// traceWorkerLimit is how many isla processes may run at once.
//
// Each process holds one parsed copy of the architecture, so the limit is the
// machine's cores rather than the number of instructions.
func traceWorkerLimit(instructions int) int {
	limit := runtime.NumCPU()
	if instructions < limit {
		return instructions
	}
	return limit
}

// traceMissing runs one isla process per instruction that has no stored trace.
//
// The instructions are independent: each reads the architecture and reports
// what one encoding does. Results keep the caller's order, and the first
// failure is the one reported.
//
// Each trace is stored the moment it is made. A run that is interrupted, or
// that fails on a later instruction, still leaves behind every trace it
// finished, so the next run does not repeat that work.
func (engine FootprintEngine) traceMissing(ctx context.Context, request FootprintRequest,
	store *TraceStore, missing []machine.Instruction) ([]InstructionTrace, error) {
	architecture := request.release.architecture.digest
	traced := make([]InstructionTrace, len(missing))
	failures := make([]error, len(missing))
	tokens := make(chan struct{}, traceWorkerLimit(len(missing)))
	var waiting sync.WaitGroup
	var keeping sync.Mutex
	for index := range missing {
		waiting.Add(1)
		go func(index int, instruction machine.Instruction) {
			defer waiting.Done()
			tokens <- struct{}{}
			defer func() { <-tokens }()
			trace, err := engine.traceInstruction(ctx, request, instruction)
			if err != nil {
				failures[index] = err
				return
			}
			traced[index] = trace
			keeping.Lock()
			failures[index] = store.Keep(trace.Encoding, architecture, trace)
			keeping.Unlock()
		}(index, missing[index])
	}
	waiting.Wait()
	for index := range failures {
		if failures[index] != nil {
			return nil, failures[index]
		}
	}
	return traced, nil
}

// encodingOf names one instruction the way the store keys it.
func encodingOf(instruction machine.Instruction) string {
	return hex.EncodeToString(instruction.Bytes)
}
