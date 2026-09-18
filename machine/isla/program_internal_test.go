// Internal program tests force integrity and layout failures that public values hide.
package isla

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestProgramCurrentRejectsInvalidState(t *testing.T) {
	valid := internalProgram()
	request := internalProgramRequest(valid.digest)
	cases := []Program{
		{},
		changedProgramContent(valid),
		changedProgramInventory(valid),
	}
	for index := range cases {
		if err := cases[index].current(request); err == nil {
			t.Errorf("current() accepted case %d", index)
		}
	}
	if err := valid.current(internalProgramRequest(strings.Repeat("b", 64))); err == nil {
		t.Error("current() accepted a different request digest")
	}
}

func TestProgramLayoutRejectsMissingEntryRegion(t *testing.T) {
	image := machine.Image{EntryAddress: 0x1000}
	if _, err := layoutProgram(image); err == nil {
		t.Error("layoutProgram() accepted a missing entry region")
	}
	boundary := ProgramBoundary{Name: "test", ThreadAddress: 0x1000, NegatedAssertion: "True", MaximumProgramBytes: 64}
	if _, err := renderProgram(image, boundary); err == nil {
		t.Error("renderProgram() accepted a missing entry region")
	}
}

func internalProgram() Program {
	content := []byte("program")
	return Program{
		content: content, digest: contentDigest(content), imageDigest: strings.Repeat("a", 64),
		entryAddress: 0x1000, instructionCount: 1,
		instructions: []machine.Instruction{{Address: 0x1000, Bytes: []byte{0x13, 0, 0, 0}}},
	}
}

func internalProgramRequest(digest string) VerificationRequest {
	return VerificationRequest{query: Request{program: Artifact{digest: digest}}}
}

func changedProgramContent(program Program) Program {
	program.content = []byte("changed")
	return program
}

func changedProgramInventory(program Program) Program {
	program.instructionCount = 2
	return program
}
