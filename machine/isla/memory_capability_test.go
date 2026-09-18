// The public executable API requires sequential support from both tools.
// A legacy tool must not silently ignore the selected memory contract.
package isla_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestExecutableRejectsMissingSequentialCapability(t *testing.T) {
	architecture := testArtifact(t, "architecture")
	configuration := testArtifact(t, "configuration")
	boundary := executableBoundary(0x80100000, "True")
	boundary.MemoryProfile = isla.SequentialMemory
	content := machineFixture(t)
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "sequential.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatal(err)
	}
	request := executableRequest(t, path, program, architecture, configuration)
	verifier := executableVerifierWithTools(t, architecture, configuration,
		"fake-isla-no-sequential-memory.sh", "fake-isla-dump-executable.sh")
	result, err := verifier.VerifyProgram(t.Context(), request, program, executableLimits())
	if err == nil || !strings.Contains(err.Error(), "unrecognized option --sequential-memory") {
		t.Fatalf("capability error = %v", err)
	}
	if result.Verification.Status != "" || result.Program.ProgramDigest != "" {
		t.Errorf("unsupported profile produced a result: %#v", result)
	}
}
