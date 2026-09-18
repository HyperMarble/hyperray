// Run observes an existing checker through one shared process boundary.
// It must not interpret checker output as mathematical evidence.
package execution

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

func Run(parent context.Context, request Request) (Result, error) {
	if parent == nil {
		return Result{}, errors.New("execution context is nil")
	}
	if err := request.Validate(); err != nil {
		return Result{}, fmt.Errorf("execution request: %w", err)
	}
	ctx, cancel := context.WithTimeout(parent, request.Timeout)
	defer cancel()
	output := &boundedOutput{limit: request.OutputLimitBytes, cancel: cancel}
	command := exec.CommandContext(ctx, request.Executable, request.Arguments...)
	command.Dir = request.Directory
	command.Env = append([]string{}, request.Environment...)
	command.Stdout = output
	command.Stderr = output
	command.WaitDelay = request.Timeout
	if err := configureProcess(command); err != nil {
		return Result{}, err
	}
	started := time.Now()
	if err := command.Start(); err != nil {
		return Result{}, fmt.Errorf("start checker: %w", err)
	}
	waitError := command.Wait()
	return measuredResult(command, request, output, ctx.Err(), waitError, started)
}
