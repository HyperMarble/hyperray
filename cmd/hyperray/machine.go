// The machine command proves one property of a compiled ARM64 binary.
// It reports the engine's own status and never converts an error into a verdict.
package main

import (
	"fmt"

	"github.com/HyperMarble/hyperray/machine/isla"
	"github.com/spf13/cobra"
)

func newMachineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "machine REQUEST.json",
		Short: "Prove one property of a compiled ARM64 binary",
		Args:  cobra.ExactArgs(1),
		RunE:  executeMachineProof,
	}
}

func executeMachineProof(command *cobra.Command, arguments []string) error {
	request, err := readMachineRequest(arguments[0])
	if err != nil {
		return err
	}
	program, err := buildMachineProgram(request)
	if err != nil {
		return fmt.Errorf("build program: %w", err)
	}
	result, err := runMachineProof(command, request, program)
	if err != nil {
		return err
	}
	return writeMachineResult(command, result)
}

func runMachineProof(command *cobra.Command, request machineRequest,
	program isla.Program) (isla.ExecutableResult, error) {
	verifier, err := buildVerifier(command.Context(), request.Tools)
	if err != nil {
		return isla.ExecutableResult{}, fmt.Errorf("build verifier: %w", err)
	}
	verification, err := buildVerificationRequest(request, program)
	if err != nil {
		return isla.ExecutableResult{}, fmt.Errorf("build verification request: %w", err)
	}
	limits := isla.ExecutableLimits{
		ThreadLimit:        1,
		TimeLimitSeconds:   request.Limits.TimeLimitSeconds,
		MaximumOutputBytes: request.Limits.MaximumOutputBytes,
	}
	result, err := verifier.VerifyProgram(command.Context(), verification, program, limits)
	if err != nil {
		return isla.ExecutableResult{}, fmt.Errorf("verify program: %w", err)
	}
	return result, nil
}
