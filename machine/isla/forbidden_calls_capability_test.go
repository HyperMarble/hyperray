// Each selected tool must support the declared model-call condition.
// Rejection leaves the public caller without a partial proof result.
package isla_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestExecutableRejectsMissingCallCapability(t *testing.T) {
	architecture := testArtifact(t, "architecture")
	configuration := testArtifact(t, "configuration")
	boundary := executableBoundary(0x80100000, "True")
	boundary.MemoryProfile = isla.SequentialMemory
	boundary.ForbiddenModelCalls = []string{"trap_handler"}
	content := machineFixture(t)
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "calls.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatal(err)
	}
	request := executableRequest(t, path, program, architecture, configuration)
	pairs := [][2]string{
		{"fake-isla-no-forbidden-calls.sh", "fake-isla-dump-executable.sh"},
		{"fake-isla-executable.sh", "fake-isla-no-forbidden-calls.sh"},
	}
	for _, pair := range pairs {
		verifier := executableVerifierWithTools(t, architecture, configuration, pair[0], pair[1])
		result, err := verifier.VerifyProgram(t.Context(), request, program, executableLimits())
		if err == nil || !strings.Contains(err.Error(), "unrecognized option --forbidden-model-calls") {
			t.Fatalf("capability error = %v", err)
		}
		if result.Verification.Status != "" || result.Program.ProgramDigest != "" {
			t.Errorf("legacy stage produced a result: %#v", result)
		}
	}
}
