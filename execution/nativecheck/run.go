// Operate the search and replay through the measured execution observer.
// Retain unsuccessful execution evidence and never infer a proof verdict.
package nativecheck

import (
	"context"
	"fmt"
	"strconv"

	"github.com/HyperMarble/hyperray/execution"
)

func Run(ctx context.Context, request Request) (Result, error) {
	if err := request.Validate(); err != nil {
		return Result{}, err
	}
	search, err := execution.Run(ctx, request.Search)
	if err != nil {
		return Result{Status: Incomplete, Search: search}, err
	}
	result, err := assess(request, search)
	reported := result.Counterexample
	result.Counterexample = nil
	if err != nil || reported == nil {
		return result, err
	}
	candidate := *reported
	if candidate.Input < request.Minimum || candidate.Input > request.Maximum {
		return result, fmt.Errorf("counterexample input %d is outside the interval", candidate.Input)
	}
	replayRequest := request.Search
	replayRequest.Executable = request.ReplayExecutable
	replayRequest.Arguments = []string{strconv.FormatUint(candidate.Input, 10)}
	replay, err := execution.Run(ctx, replayRequest)
	result.Replay = &replay
	if err != nil {
		return result, err
	}
	reason, err := replayMatches(replay, candidate)
	result.Reason = reason
	if err != nil || reason != "" {
		return result, err
	}
	result.Status = CounterexampleReproduced
	result.Counterexample = &candidate
	return result, nil
}
