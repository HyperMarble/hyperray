// Footprint execution accepts a report only after every instruction succeeds.
// One failed instruction discards the complete partial report.
package isla

import (
	"context"
	"encoding/hex"
	"os/exec"
	"strings"
	"time"

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
	for index := range request.instructions {
		trace, err := engine.traceInstruction(ctx, request, request.instructions[index])
		if err != nil {
			return FootprintReport{}, err
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
	dispositions, err := classifyDiagnostics(output)
	if err != nil {
		return InstructionTrace{}, err
	}
	return InstructionTrace{
		Address: instruction.Address, Encoding: hex.EncodeToString(instruction.Bytes),
		TraceCount: traceCount, OutputDigest: rawOutputDigest(output),
		ElapsedMilliseconds: output.elapsed.Milliseconds(), Diagnostics: output.diagnostics,
		Dispositions: dispositions,
	}, nil
}

func (engine FootprintEngine) runFootprint(ctx context.Context, request FootprintRequest, instruction machine.Instruction) (commandOutput, error) {
	stdout := newLimitedBuffer(request.maximumOutputSize)
	diagnostics := newLimitedBuffer(request.maximumOutputSize)
	command := exec.CommandContext(ctx, engine.identity.Path, request.arguments(instruction)...)
	command.Stdout = stdout
	command.Stderr = diagnostics
	start := time.Now()
	err := command.Run()
	output := commandOutput{stdout: stdout.String(), diagnostics: diagnostics.String(), elapsed: time.Since(start)}
	if ctx.Err() != nil || stdout.exceeded || diagnostics.exceeded {
		return commandOutput{}, engineError(ResourceLimit, hex.EncodeToString(instruction.Bytes), "footprint resource limit reached")
	}
	if err != nil {
		detail := strings.TrimSpace(output.diagnostics + "\n" + output.stdout)
		return commandOutput{}, engineError(ProcessFail, hex.EncodeToString(instruction.Bytes), detail)
	}
	return output, nil
}
