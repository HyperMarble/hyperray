// Proposal execution sends one immutable bounded query to Isla.
// It must reject process and protocol errors before it returns a proposal.
package isla

import (
	"bytes"
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
	return proposalFromResult(engine, request, output, parsed), nil
}

func (engine Engine) operate(ctx context.Context, request Request) (commandOutput, error) {
	stdout := &bytes.Buffer{}
	diagnostics := &bytes.Buffer{}
	command := exec.CommandContext(ctx, engine.identity.Path, request.arguments()...)
	command.Stdout = stdout
	command.Stderr = diagnostics
	start := time.Now()
	err := command.Run()
	output := commandOutput{stdout: stdout.String(), diagnostics: diagnostics.String(), elapsed: time.Since(start)}
	if ctx.Err() != nil {
		return commandOutput{}, engineError(ResourceLimit, request.program.path, ctx.Err().Error())
	}
	if err != nil {
		detail := strings.TrimSpace(output.diagnostics + "\n" + output.stdout)
		return commandOutput{}, engineError(ProcessFail, request.program.path, detail)
	}
	return output, nil
}
