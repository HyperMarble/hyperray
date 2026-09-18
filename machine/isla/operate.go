// Proposal execution sends one immutable bounded query to Isla.
// It must reject process and protocol errors before it returns a proposal.
package isla

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Propose asks Isla for a counterexample to the property in the program artifact.
func (engine Engine) Propose(ctx context.Context, request Request) (Proposal, error) {
	if ctx == nil {
		return Proposal{}, engineError(InvalidInput, "context", "nil")
	}
	if err := engine.current(); err != nil {
		return Proposal{}, err
	}
	if err := request.current(); err != nil {
		return Proposal{}, err
	}
	output, err := engine.operate(ctx, request)
	if err != nil {
		return Proposal{}, err
	}
	parsed, err := parseHerdResult(output.stdout, output.diagnostics)
	if err != nil {
		return Proposal{}, err
	}
	dispositions, err := classifyDiagnostics(output, "proposal diagnostic")
	if err != nil {
		return Proposal{}, err
	}
	if err := request.current(); err != nil {
		return Proposal{}, err
	}
	return proposalFromResult(engine, request, output, parsed, dispositions), nil
}

func (engine Engine) operate(ctx context.Context, request Request) (commandOutput, error) {
	duration, err := boundedDuration(request.timeLimit)
	if err != nil {
		return commandOutput{}, err
	}
	limitedContext, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	stdout := newLimitedBuffer(request.maximumOutputSize)
	diagnostics := newLimitedBuffer(request.maximumOutputSize)
	command := exec.CommandContext(limitedContext, engine.identity.Path, proposalArguments(engine.identity, request)...)
	command.Stdout = stdout
	command.Stderr = diagnostics
	start := time.Now()
	err = command.Run()
	output := commandOutput{stdout: stdout.String(), diagnostics: diagnostics.String(), elapsed: time.Since(start)}
	if limitedContext.Err() != nil || stdout.exceeded || diagnostics.exceeded {
		return commandOutput{}, engineError(ResourceLimit, request.program.path, "solver resource limit reached")
	}
	if err != nil {
		detail := strings.TrimSpace(output.diagnostics + "\n" + output.stdout)
		return commandOutput{}, engineError(ProcessFail, request.program.path, detail)
	}
	return output, nil
}
