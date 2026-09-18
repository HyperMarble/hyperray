// Model-typed state is explicit query data, not a source-language rule.
// Duplicate assignments must not choose a value by input order.
package isla

import (
	"sort"
	"strconv"
)

func validateInitialState(boundary ProgramBoundary, threads []ThreadEntry) error {
	names := make(map[string]bool)
	for _, thread := range threads {
		for _, value := range thread.InitialRegisters {
			names[value.Name] = true
		}
	}
	for _, value := range boundary.InitialState {
		if len(threads) > 1 {
			return engineError(InvalidInput, "initial state", "typed state is unsupported for multiple threads")
		}
		if !identifier(value.Name) || !plainLine(value.Value) {
			return engineError(InvalidInput, "initial state", "invalid name or value")
		}
		if names[value.Name] {
			return engineError(InvalidInput, "initial state", "duplicate "+value.Name)
		}
		names[value.Name] = true
	}
	return nil
}

func writeInitialState(output *limitedBuffer, values []RegisterValue) {
	if len(values) == 0 {
		return
	}
	values = append([]RegisterValue(nil), values...)
	sort.Slice(values, func(left, right int) bool {
		return values[left].Name < values[right].Name
	})
	appendProgramText(output, "\n[initial_state]\n")
	for _, value := range values {
		appendProgramText(output, value.Name+" = "+strconv.Quote(value.Value)+"\n")
	}
}
