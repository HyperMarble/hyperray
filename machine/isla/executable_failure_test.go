// Executable failure tests require every incomplete path to return zero evidence.
package isla_test

import (
	"context"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestExecutableVerifierRejectsInvalidRequests(t *testing.T) {
	architecture := testArtifact(t, "architecture")
	configuration := testArtifact(t, "configuration")
	program, request := executableProgramRequest(t, architecture, configuration)
	verifier := executableVerifier(t, architecture, configuration)
	limits := executableLimits()
	assertExecutableError(t, verifier, nil, request, program, limits)
	assertExecutableError(t, verifier, t.Context(), request, isla.Program{}, limits)
	limits.ThreadLimit = 0
	assertExecutableError(t, verifier, t.Context(), request, program, limits)
	limits = executableLimits()
	limits.MaximumOutputBytes = 1
	assertExecutableError(t, verifier, t.Context(), request, program, limits)
}

func TestExecutableVerifierRejectsFootprintQueryMismatch(t *testing.T) {
	releaseArchitecture := testArtifact(t, "architecture")
	requestArchitecture := testArtifact(t, "other-architecture")
	configuration := testArtifact(t, "configuration")
	program, request := executableProgramRequest(t, requestArchitecture, configuration)
	verifier := executableVerifier(t, releaseArchitecture, configuration)
	assertExecutableError(t, verifier, t.Context(), request, program, executableLimits())
}

func TestExecutableVerifierRejectsSolverAndSemanticFailures(t *testing.T) {
	architecture := testArtifact(t, "architecture")
	configuration := testArtifact(t, "configuration")
	program, request := executableProgramRequest(t, architecture, configuration)
	solverFailure := executableVerifierWithTools(t, architecture, configuration, "fake-isla-executable-error.sh", "fake-isla-dump-executable.sh")
	assertExecutableError(t, solverFailure, t.Context(), request, program, executableLimits())
	missingSemantic := executableVerifierWithTools(t, architecture, configuration, "fake-isla-executable.sh", "fake-isla-dump-no-entry.sh")
	assertExecutableError(t, missingSemantic, t.Context(), request, program, executableLimits())
}

func assertExecutableError(t *testing.T, verifier isla.ExecutableVerifier, ctx context.Context, request isla.VerificationRequest, program isla.Program, limits isla.ExecutableLimits) {
	t.Helper()
	result, err := verifier.VerifyProgram(ctx, request, program, limits)
	if err == nil || result.Verification.Status != "" || result.Program.ProgramDigest != "" {
		t.Errorf("VerifyProgram() = %#v, %v", result, err)
	}
}
