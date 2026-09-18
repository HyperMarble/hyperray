//go:build isla_integration

// The compiler-built ELF supplies the entry and the fixture return symbol.
// The return address is a declared Isla harness boundary, not an OS exit.
package isla_test

import (
	"bytes"
	"debug/elf"
	"fmt"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func rustProgramBoundary(t *testing.T, content []byte, input uint64, expected uint64) isla.ProgramBoundary {
	t.Helper()
	image, err := elf.NewFile(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	symbols, err := image.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range symbols {
		if symbol.Name == "__hyperray_return" {
			return isla.ProgramBoundary{
				Name: "rust-machine", ThreadAddress: image.Entry,
				InitialRegisters: []isla.RegisterValue{
					{Name: "x1", Value: fmt.Sprintf("%#x", symbol.Value)},
					{Name: "x10", Value: fmt.Sprintf("%d", input)},
				},
				NegatedAssertion: fmt.Sprintf("~(0:x10 = %d)", expected), MaximumProgramBytes: 1 << 20,
			}
		}
	}
	t.Fatal("compiler-built ELF has no declared return symbol")
	return isla.ProgramBoundary{}
}
