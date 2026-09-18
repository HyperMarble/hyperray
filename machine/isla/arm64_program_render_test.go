// ARM64 rendering tests inspect native reset and continuation syntax.
// They must preserve the exact architecture spelling expected by Isla.
package isla_test

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildARM64ProgramRendersNativeResetAndReturnSyntax(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	program, err := isla.BuildARM64Program(content, 32768, arm64ProgramBoundary(start, end))
	if err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
	source := string(program.Content())
	for _, expected := range []string{
		"arch = \"AArch64\"",
		"memory_profile = \"arm64-fetch-only-v1\"",
		"code_ranges = [[\"0x1000002e8\", \"0x1000002ec\"], [\"0x1000002ec\", \"0x1000002f0\"]]",
		"entry = \"0x1000002e8\"",
		"reset = { \"R0\" = \"0x0000000000000003\", \"R30\" = \"0x0000000100008000\", \"SP_EL0\" = \"0x0000000000003c40\" }",
		"return_address = \"0x100008000\"",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated ARM64 program lacks %q", expected)
		}
	}
}
