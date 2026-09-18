//go:build isla_integration

// Fixture calls must enter every function that the compiler retains.
// Function names are evidence from the ELF, not production dispatch rules.
package isla_test

import (
	"bytes"
	"debug/elf"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func assertRustPatternFunctions(t *testing.T, content []byte, result isla.ExecutableResult) {
	t.Helper()
	image, err := elf.NewFile(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	symbols, err := image.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	observed := make(map[uint64]bool)
	for _, instruction := range result.Execution.Observed {
		observed[instruction.Address] = true
	}
	for _, symbol := range symbols {
		if elf.ST_TYPE(symbol.Info) != elf.STT_FUNC || symbol.Size == 0 {
			continue
		}
		if !observed[symbol.Value] {
			t.Errorf("compiler-retained function %s at %#x has no entry event", symbol.Name, symbol.Value)
		}
		t.Logf("compiler function: %s at %#x", symbol.Name, symbol.Value)
	}
}
