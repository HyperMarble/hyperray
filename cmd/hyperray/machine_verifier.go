// This file builds the real ARM64 verifier from named tool paths.
// It must never substitute a fake engine or an unpinned artifact.
package main

import (
	"context"
	"fmt"

	"github.com/HyperMarble/hyperray/machine/isla"
)

// buildVerifier constructs the measured native trio and its capability.
func buildVerifier(ctx context.Context, tools machineTools) (isla.ExecutableVerifier, error) {
	solver, err := isla.NewEngine(ctx, tools.Solver)
	if err != nil {
		return isla.ExecutableVerifier{}, fmt.Errorf("solver engine: %w", err)
	}
	semantics, err := isla.NewSemanticEngine(ctx, tools.Semantics)
	if err != nil {
		return isla.ExecutableVerifier{}, fmt.Errorf("semantic engine: %w", err)
	}
	verifier, err := isla.NewVerifier(solver, semantics)
	if err != nil {
		return isla.ExecutableVerifier{}, fmt.Errorf("verifier: %w", err)
	}
	footprints, err := isla.NewFootprintEngine(ctx, tools.Footprints)
	if err != nil {
		return isla.ExecutableVerifier{}, fmt.Errorf("footprint engine: %w", err)
	}
	return buildCapabilityVerifier(tools, verifier, footprints)
}

func buildCapabilityVerifier(tools machineTools, verifier isla.Verifier,
	footprints isla.FootprintEngine) (isla.ExecutableVerifier, error) {
	artifacts, err := toolArtifacts(tools)
	if err != nil {
		return isla.ExecutableVerifier{}, err
	}
	release, err := measuredFootprintRelease(footprints, artifacts.architecture, artifacts.configuration)
	if err != nil {
		return isla.ExecutableVerifier{}, err
	}
	capability, err := isla.NewARM64ExecutionCapability(artifacts.manifest, verifier, footprints,
		artifacts.architecture, artifacts.configuration, artifacts.memoryModel)
	if err != nil {
		return isla.ExecutableVerifier{}, fmt.Errorf("ARM64 capability: %w", err)
	}
	executable, err := isla.NewExecutableVerifierWithCapability(verifier, footprints, release, capability)
	if err != nil {
		return isla.ExecutableVerifier{}, fmt.Errorf("executable verifier: %w", err)
	}
	return executable, nil
}
