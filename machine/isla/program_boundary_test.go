// Boundary tests cover every explicit text, register, and generation limit.
package isla_test

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildProgramRejectsInvalidBoundaryValues(t *testing.T) {
	content := machineFixture(t)
	cases := []isla.ProgramBoundary{
		executableBoundary(0x80100000, "True"),
		executableBoundary(0x80100000, "True"),
		executableBoundary(0x80100000, "True\nFalse"),
		executableBoundary(0x80100000, "True"),
		executableBoundary(0x80100000, "True"),
	}
	cases[0].MaximumProgramBytes = 0
	cases[1].Name = ""
	cases[3].InitialRegisters[0].Value = "not-a-number"
	cases[4].InitialRegisters = append(cases[4].InitialRegisters, cases[4].InitialRegisters[0])
	for index := range cases {
		program, err := isla.BuildProgram(content, uint64(len(content)), cases[index])
		assertProgramError(t, program, err, isla.InvalidInput)
	}
}

func TestBuildProgramRejectsMalformedELF(t *testing.T) {
	boundary := executableBoundary(0x80100000, "True")
	if program, err := isla.BuildProgram([]byte("not ELF"), 64, boundary); err == nil || program.Digest() != "" {
		t.Errorf("BuildProgram() = %#v, %v", program, err)
	}
}

func TestBuildProgramSortsMultipleRegisters(t *testing.T) {
	content := machineFixture(t)
	boundary := executableBoundary(0x80100000, "True")
	boundary.InitialRegisters = append(boundary.InitialRegisters, isla.RegisterValue{Name: "x0", Value: "0"})
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatalf("BuildProgram() error = %v", err)
	}
	if !strings.Contains(string(program.Content()), "x0 = \"0\", x1 = \"0x8010000c\"") {
		t.Errorf("register table is not sorted: %s", program.Content())
	}
}
