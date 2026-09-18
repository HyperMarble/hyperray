// Terminal PC reads cannot invent an instruction or invalidate an earlier one.
// A symbolic PC must clear stale addresses without affecting sibling traces.
package isla

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSemanticTreeTerminalProgramCounter(t *testing.T) {
	cases := []struct {
		name  string
		valid bool
	}{
		{"terminal", true}, {"overwritten", false}, {"siblings", true},
		{"used", false}, {"malformed", false}, {"replaced", true},
	}
	for _, example := range cases {
		t.Run(example.name, func(t *testing.T) {
			path := filepath.Join("../../fixtures/isla/terminal-pc", example.name+".trace")
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			instructions, _, count, err := semanticInstructionEvents(string(content))
			if !example.valid && err == nil {
				t.Fatal("invalid address evidence was accepted")
			}
			if !example.valid {
				return
			}
			if err != nil || count != 1 || len(instructions) != 1 {
				t.Fatalf("instructions=%v count=%d error=%v", instructions, count, err)
			}
			if _, found := instructions[SemanticInstruction{0x1000, "00000013"}]; !found {
				t.Errorf("instruction mapping differs: %v", instructions)
			}
		})
	}
}
