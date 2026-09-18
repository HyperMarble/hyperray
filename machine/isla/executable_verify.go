// Executable verification accepts a verdict only after all three Isla checks.
// Any generation, footprint, semantic, or solver failure returns no result.
package isla

import (
	"context"
)

// VerifyProgram joins static instruction coverage and whole-program proof.
func (engine ExecutableVerifier) VerifyProgram(ctx context.Context, request VerificationRequest, program Program, limits ExecutableLimits) (ExecutableResult, error) {
	if err := engine.validateExecutableInput(ctx, request, program); err != nil {
		return ExecutableResult{}, err
	}
	if err := program.current(request); err != nil {
		return ExecutableResult{}, err
	}
	configureExecutableRequest(&request, program, limits)
	footprintRequest, err := engine.newFootprintRequest(program, limits)
	if err != nil {
		return ExecutableResult{}, err
	}
	report, err := engine.footprints.TraceInstructions(ctx, footprintRequest)
	if err != nil {
		return ExecutableResult{}, err
	}
	if err := matchFootprintQuery(report, request); err != nil {
		return ExecutableResult{}, err
	}
	result, err := engine.verifier.Verify(ctx, request)
	if err != nil {
		return ExecutableResult{}, err
	}
	if err := validateTerminalEvidence(program, result); err != nil {
		return ExecutableResult{}, err
	}
	joined, err := joinedExecutableResult(program, report, result)
	if err != nil {
		return ExecutableResult{}, err
	}
	if engine.capability != nil {
		joined.Capability = engine.capability.evidence()
	}
	return joined, nil
}
