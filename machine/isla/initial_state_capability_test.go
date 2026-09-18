// Public execution requires typed-state support from each selected tool.
// A legacy stage must leave the caller without a proof result.
package isla_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestExecutableRejectsMissingTypedStateCapability(t *testing.T) {
	architecture := testArtifact(t, "architecture")
	configuration := testArtifact(t, "configuration")
	boundary := executableBoundary(0x80100000, "True")
	boundary.InitialState = []isla.RegisterValue{{Name: "flag", Value: "true"}}
	content := machineFixture(t)
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "typed-state.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatal(err)
	}
	request := executableRequest(t, path, program, architecture, configuration)
	pairs := [][2]string{
		{"fake-isla-no-typed-initial-state.sh", "fake-isla-dump-executable.sh"},
		{"fake-isla-executable.sh", "fake-isla-no-typed-initial-state.sh"},
	}
	for _, pair := range pairs {
		verifier := executableVerifierWithTools(t, architecture, configuration, pair[0], pair[1])
		result, err := verifier.VerifyProgram(t.Context(), request, program, executableLimits())
		if err == nil || !strings.Contains(err.Error(), "unrecognized option --typed-initial-state") {
			t.Fatalf("capability error = %v", err)
		}
		if result.Verification.Status != "" || result.Program.ProgramDigest != "" {
			t.Errorf("legacy stage produced a result: %#v", result)
		}
	}
}
