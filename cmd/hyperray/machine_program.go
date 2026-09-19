// This file turns a machine request into the program and verification inputs.
// It must pass the declared boundary through unchanged.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func bytesReader(content []byte) io.Reader { return bytes.NewReader(content) }

// buildMachineProgram loads the binary and renders the Isla program text.
func buildMachineProgram(request machineRequest) (isla.Program, error) {
	content, err := os.ReadFile(request.Binary)
	if err != nil {
		return isla.Program{}, fmt.Errorf("read binary: %w", err)
	}
	memory, err := buildMemoryInput(request.Memory)
	if err != nil {
		return isla.Program{}, fmt.Errorf("memory input: %w", err)
	}
	nativeRegisters, err := isla.ModelRegisterNames(request.Tools.Architecture)
	if err != nil {
		return isla.Program{}, fmt.Errorf("model registers: %w", err)
	}
	boundary := isla.ARM64ProgramBoundary{
		Name:                request.Boundary.Name,
		FunctionStart:       request.Boundary.FunctionStart,
		FunctionEnd:         request.Boundary.FunctionEnd,
		ReturnAddress:       request.Boundary.ReturnAddress,
		NegatedAssertion:    request.Assertion,
		MaximumProgramBytes: request.Limits.MaximumOutputBytes,
		Memory:              &memory,
		PostResetRegisters:  registerValues(request.Boundary.Registers),
		MemoryObservations:  memoryObservations(request.Boundary.Observations),
		NativeRegisterNames: nativeRegisters,
	}
	return isla.BuildARM64Program(content, request.Limits.MaximumLoadedBytes, boundary)
}
