// The public result retains static coverage when the query sees a subset.
// Protocol fixtures establish API behavior, not real machine proof results.
package isla_test

import "testing"

func TestExecutableVerifierRetainsUnobservedInstructions(t *testing.T) {
	architecture := testArtifact(t, "architecture")
	configuration := testArtifact(t, "configuration")
	program, request := executableProgramRequest(t, architecture, configuration)
	verifier := executableVerifierWithTools(t, architecture, configuration,
		"fake-isla-executable.sh", "fake-isla-dump-missing-executable.sh")
	result, err := verifier.VerifyProgram(t.Context(), request, program, executableLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !result.StaticCoverage.Complete || result.StaticCoverage.TotalInstructions != program.InstructionCount() {
		t.Errorf("static coverage changed: %#v", result.StaticCoverage)
	}
	if len(result.Execution.NotObserved) != 1 {
		t.Errorf("unobserved inventory = %#v", result.Execution)
	}
	total := len(result.Execution.Observed) + len(result.Execution.NotObserved)
	if uint64(total) != program.InstructionCount() {
		t.Errorf("execution partition lost instructions: %#v", result.Execution)
	}
}
