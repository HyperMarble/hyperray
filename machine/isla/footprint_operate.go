// Footprint execution accepts a report only after every instruction succeeds.
// One failed instruction discards the complete partial report.
package isla

import (
	"context"
	"encoding/hex"
	"os"

	"github.com/HyperMarble/hyperray/machine"
)

// TraceInstructions obtains Sail semantics for the complete instruction inventory.
func (engine FootprintEngine) TraceInstructions(ctx context.Context, request FootprintRequest) (FootprintReport, error) {
	if ctx == nil {
		return FootprintReport{}, engineError(InvalidInput, "context", "nil")
	}
	if err := engine.current(); err != nil {
		return FootprintReport{}, err
	}
	if err := request.current(engine); err != nil {
		return FootprintReport{}, err
	}
	store := NewTraceStore(os.Getenv("HYPERRAY_TRACE_STORE"))
	architecture := request.release.architecture.digest
	traces, missing := storedTraces(store, architecture, request.instructions)
	traced, err := engine.traceMissing(ctx, request, store, missing)
	if err != nil {
		return FootprintReport{}, err
	}
	return newFootprintReport(engine, request, placeTraced(traces, traced)), nil
}

func (engine FootprintEngine) traceInstruction(ctx context.Context, request FootprintRequest, instruction machine.Instruction) (InstructionTrace, error) {
	output, err := engine.runFootprint(ctx, request, instruction)
	if err != nil {
		return InstructionTrace{}, err
	}
	traceCount, err := countTraceBlocks(output.stdout)
	if err != nil {
		return InstructionTrace{}, err
	}
	dispositions, err := classifyDiagnostics(output, "footprint diagnostic")
	if err != nil {
		return InstructionTrace{}, err
	}
	return InstructionTrace{
		Address: instruction.Address, Encoding: hex.EncodeToString(instruction.Bytes),
		TraceCount: traceCount, TraceOutput: output.stdout, OutputDigest: rawOutputDigest(output),
		ElapsedMilliseconds: output.elapsed.Milliseconds(), Diagnostics: output.diagnostics,
		Dispositions: dispositions,
	}, nil
}
