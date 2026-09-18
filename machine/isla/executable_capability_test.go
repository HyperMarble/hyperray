// Legacy tools must reject required runtime extensions before any verdict.
// Unknown input metadata must not silently retain old semantics.
package isla_test

import (
	"strings"
	"testing"
)

func TestExecutableRejectsMissingRuntimeCapabilities(t *testing.T) {
	architecture := testArtifact(t, "architecture")
	configuration := testArtifact(t, "configuration")
	program, request := executableProgramRequest(t, architecture, configuration)
	cases := []struct{ fixture, flag string }{
		{"fake-isla-no-executable-entry.sh", "--executable-entry"},
		{"fake-isla-no-initialized-memory.sh", "--initialized-memory"},
	}
	for _, example := range cases {
		verifier := executableVerifierWithTools(t, architecture, configuration, example.fixture, "fake-isla-dump-executable.sh")
		result, err := verifier.VerifyProgram(t.Context(), request, program, executableLimits())
		if err == nil || !strings.Contains(err.Error(), "unrecognized option "+example.flag) {
			t.Fatalf("capability error = %v", err)
		}
		if result.Verification.Status != "" || result.Program.ProgramDigest != "" {
			t.Errorf("unsupported tool produced a result: %#v", result)
		}
	}
}
