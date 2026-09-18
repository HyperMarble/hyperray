// ARM64 Program copy tests protect caller and returned value independence.
// Builder state must not alias content or reset-register input slices.
package isla_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildARM64ProgramCopiesContentAndResetInputs(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	boundary := arm64ProgramBoundary(start, end)
	program, err := isla.BuildARM64Program(content, 32768, boundary)
	if err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
	content[0] ^= 0xff
	first := program.Content()
	first[0] ^= 0xff
	if bytes.Equal(first, program.Content()) || !strings.Contains(string(program.Content()), "R0\" = \"0x0000000000000003") {
		t.Fatal("builder content is not independent")
	}
	boundary.PostResetRegisters[0].Value = "99"
	boundary.Name = "changed"
	if strings.Contains(string(program.Content()), "changed") || program.PostResetRegisters()[0].Value != "0x0000000000000003" {
		t.Fatal("builder retained mutable reset input")
	}
	reset := program.PostResetRegisters()
	reset[0].Value = "99"
	if program.PostResetRegisters()[0].Value != "0x0000000000000003" {
		t.Fatal("PostResetRegisters() returned shared storage")
	}
}
