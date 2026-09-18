// Public program tests compare generated input with an independently loaded ELF.
// They prove that callers can build and inspect the complete program artifact.
package isla_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildProgramPreservesAcceptedELF(t *testing.T) {
	content := machineFixture(t)
	image, err := machine.Load(content, uint64(len(content)))
	if err != nil {
		t.Fatalf("machine.Load() error = %v", err)
	}
	boundary := executableBoundary(image.EntryAddress, "~(0:x11 = 7)")
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatalf("BuildProgram() error = %v", err)
	}
	if program.ImageDigest() != image.ArtifactSHA256 || program.EntryAddress() != image.EntryAddress {
		t.Errorf("program source identity does not match image")
	}
	if program.InstructionCount() != uint64(len(image.Instructions)) {
		t.Errorf("InstructionCount() = %d", program.InstructionCount())
	}
	if program.LoadedByteCount() != uint64(len(image.LoadedBytes)) {
		t.Errorf("LoadedByteCount() = %d", program.LoadedByteCount())
	}
	assertProgramText(t, string(program.Content()))
}

func machineFixture(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("../../fixtures/machine", "rv64-lp64d-static.elf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	return content
}

func executableBoundary(address uint64, assertion string) isla.ProgramBoundary {
	return isla.ProgramBoundary{
		Name: "GENERATED-ELF", ThreadAddress: address,
		InitialRegisters: []isla.RegisterValue{{Name: "x1", Value: "0x8010000c"}},
		NegatedAssertion: assertion, MaximumProgramBytes: 1 << 20,
	}
}

func assertProgramText(t *testing.T, source string) {
	t.Helper()
	values := []string{"entry = \"0x80100000\"", "code = \"\"\n", "address = \"0x80100000\"", "address = \"0x80200000\"", "assertion = \"~(0:x11 = 7)\""}
	for index := range values {
		if !strings.Contains(source, values[index]) {
			t.Errorf("generated program lacks %q", values[index])
		}
	}
}
