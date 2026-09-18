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
	traces := make([]InstructionTrace, 0, len(request.instructions))
	store := NewTraceStore(os.Getenv("HYPERRAY_TRACE_STORE"))
	architecture := request.release.architecture.digest
	for index := range request.instructions {
		instruction := request.instructions[index]
		encoding := hex.EncodeToString(instruction.Bytes)
		if stored, found := store.Lookup(encoding, architecture); found {
			stored.Address = instruction.Address
			traces = append(traces, stored)
			continue
		}
		trace, err := engine.traceInstruction(ctx, request, instruction)
		if err != nil {
			return FootprintReport{}, err
		}
		if err := store.Keep(encoding, architecture, trace); err != nil {
			return FootprintReport{}, engineError(ProcessFail, encoding, err.Error())
		}
		traces = append(traces, trace)
	}
	return newFootprintReport(engine, request, traces), nil
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
