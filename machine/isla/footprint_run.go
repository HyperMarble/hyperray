// The host enforces each instruction deadline independently of tool cooperation.
// Cancellation must finish process cleanup before it returns an error.
package isla

import (
	"context"
	"encoding/hex"
	"os/exec"
	"strings"
	"time"

	"github.com/HyperMarble/hyperray/machine"
)

func (engine FootprintEngine) runFootprint(ctx context.Context, request FootprintRequest, instruction machine.Instruction) (commandOutput, error) {
	duration, err := boundedDuration(request.timeLimit)
	if err != nil {
		return commandOutput{}, err
	}
	limitedContext, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	stdout := newLimitedBuffer(request.maximumOutputSize)
	diagnostics := newLimitedBuffer(request.maximumOutputSize)
	command := exec.CommandContext(limitedContext, engine.identity.Path, request.arguments(instruction)...)
	command.Stdout = stdout
	command.Stderr = diagnostics
	start := time.Now()
	err = command.Run()
	output := commandOutput{
		stdout: stdout.String(), diagnostics: diagnostics.String(), elapsed: time.Since(start),
		exitCode: commandExitCode(err), stdoutExceeded: stdout.exceeded, diagnosticsExceeded: diagnostics.exceeded,
	}
	if limitedContext.Err() != nil || stdout.exceeded || diagnostics.exceeded {
		detail := newResourceLimitDetail(limitedContext.Err(), output, request.maximumOutputSize)
		return output, resourceLimitError(hex.EncodeToString(instruction.Bytes), detail)
	}
	if err != nil {
		detail := strings.TrimSpace(output.diagnostics + "\n" + output.stdout)
		return commandOutput{}, engineError(ProcessFail, hex.EncodeToString(instruction.Bytes), detail)
	}
	return output, nil
}
