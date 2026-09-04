// Event inventory ordering depends on instruction addresses and trace order.
// Caller slice order and later caller mutations must not change the result.
package isla

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/HyperMarble/hyperray/machine"
)

// InventoryTraceEvents records every direct event from validated traces.
func InventoryTraceEvents(instructions []machine.Instruction, report FootprintReport) (TraceEventCatalog, error) {
	if _, err := ValidateFootprintInventory(instructions, report); err != nil {
		return TraceEventCatalog{}, err
	}
	traces := append([]InstructionTrace(nil), report.Instructions...)
	slices.SortFunc(traces, func(left InstructionTrace, right InstructionTrace) int {
		return cmp.Compare(left.Address, right.Address)
	})
	events := make([]TraceEvent, 0)
	var traceCount uint64
	for traceIndex := range traces {
		trace := traces[traceIndex]
		scan, err := scanTraceOutput(trace.TraceOutput)
		if err != nil {
			return TraceEventCatalog{}, err
		}
		traceCount += scan.TraceCount
		for eventIndex := range scan.Events {
			event := scan.Events[eventIndex]
			events = append(events, TraceEvent{
				Identifier: fmt.Sprintf("%016x:%d:%d", trace.Address, event.TraceIndex, event.EventIndex),
				Address:    trace.Address, TraceIndex: event.TraceIndex, EventIndex: event.EventIndex,
				Kind: event.Kind, TraceDigest: trace.OutputDigest,
			})
		}
	}
	return TraceEventCatalog{
		Complete: true, InstructionCount: uint64(len(traces)),
		TraceCount: traceCount, EventCount: uint64(len(events)), Events: events,
	}, nil
}
