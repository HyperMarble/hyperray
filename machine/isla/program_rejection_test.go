// Program rejection tests require exact errors for unsafe query construction.
// No rejected input produces a partial generated artifact.
package isla_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestProgramContentReturnsACopy(t *testing.T) {
	content := machineFixture(t)
	program, err := isla.BuildProgram(content, uint64(len(content)), executableBoundary(0x80100000, "True"))
	if err != nil {
		t.Fatalf("BuildProgram() error = %v", err)
	}
	first := program.Content()
	first[0] = 'X'
	if bytes.Equal(first, program.Content()) {
		t.Error("Content() returned shared storage")
	}
}

func TestBuildProgramRejectsMismatchedEntry(t *testing.T) {
	content := machineFixture(t)
	boundary := executableBoundary(0x80100002, "True")
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	assertProgramError(t, program, err, isla.CoverageMismatch)
}

func TestBuildProgramRejectsOutputLimit(t *testing.T) {
	content := machineFixture(t)
	boundary := executableBoundary(0x80100000, "True")
	boundary.MaximumProgramBytes = 1
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	assertProgramError(t, program, err, isla.ResourceLimit)
}

func TestBuildProgramRejectsInvalidRegister(t *testing.T) {
	content := machineFixture(t)
	boundary := executableBoundary(0x80100000, "True")
	boundary.InitialRegisters[0].Name = "x1 = 4"
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	assertProgramError(t, program, err, isla.InvalidInput)
}

func assertProgramError(t *testing.T, program isla.Program, err error, code isla.ErrorCode) {
	t.Helper()
	var failure *isla.Error
	if !errors.As(err, &failure) || failure.Code != code {
		t.Fatalf("BuildProgram() error = %v, want %s", err, code)
	}
	if len(program.Content()) != 0 || program.Digest() != "" {
		t.Errorf("rejected program = %#v", program)
	}
}
