// ARM64 register validation tests assert canonical names and relationships.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildARM64ProgramRejectsInvalidRegisterRelations(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	wrongPC := arm64ProgramBoundary(start, end)
	wrongPC.PostResetRegisters = append(wrongPC.PostResetRegisters, isla.RegisterValue{Name: "_PC", Value: "0x1000002ec"})
	wrongReturn := arm64ProgramBoundary(start, end)
	wrongReturn.PostResetRegisters[1].Value = "0x100008004"
	padded := arm64ProgramBoundary(start, end)
	padded.PostResetRegisters[0].Name = "R00"
	cases := []struct {
		name     string
		boundary isla.ARM64ProgramBoundary
		subject  string
		detail   string
	}{
		{"conflicting PC", wrongPC, "ARM64 post-reset PC", "differs"},
		{"conflicting R30", wrongReturn, "ARM64 return register", "differs"},
		{"padded register", padded, "ARM64 register", "R00"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			program, err := isla.BuildARM64Program(content, 32768, testCase.boundary)
			assertZeroARM64Program(t, program)
			assertARM64Error(t, err, isla.InvalidInput, testCase.subject, testCase.detail)
		})
	}
}
