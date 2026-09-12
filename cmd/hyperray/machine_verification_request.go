// The verification request writes program text where the engine reads it.
// It must pin the program by the digest the builder measured.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/HyperMarble/hyperray/machine/isla"
)

// buildVerificationRequest writes the program to a file, because the engine
// reads its input by path.
func buildVerificationRequest(request machineRequest, program isla.Program) (isla.VerificationRequest, error) {
	directory, err := os.MkdirTemp("", "hyperray-machine")
	if err != nil {
		return isla.VerificationRequest{}, fmt.Errorf("create program directory: %w", err)
	}
	path := filepath.Join(directory, "program.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		return isla.VerificationRequest{}, fmt.Errorf("write program: %w", err)
	}
	artifact, err := isla.NewArtifact(path, program.Digest())
	if err != nil {
		return isla.VerificationRequest{}, fmt.Errorf("program artifact: %w", err)
	}
	artifacts, err := toolArtifacts(request.Tools)
	if err != nil {
		return isla.VerificationRequest{}, err
	}
	query, err := isla.NewRequest(artifacts.architecture, artifacts.configuration, artifacts.memoryModel,
		artifact, request.Limits.PCVisitLimit, request.Limits.TimeLimitSeconds, request.Limits.MaximumOutputBytes)
	if err != nil {
		return isla.VerificationRequest{}, fmt.Errorf("query: %w", err)
	}
	return isla.NewVerificationRequest(query, 1, 2048)
}
