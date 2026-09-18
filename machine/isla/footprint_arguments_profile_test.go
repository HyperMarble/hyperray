// Footprint PC syntax follows the explicitly bound execution profile.
// The default RISC-V request must retain its existing PC spelling.
package isla

import (
	"slices"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestFootprintArgumentsUseBoundPCRegister(t *testing.T) {
	instruction := machine.Instruction{Address: 0x1000, Bytes: []byte{0, 0}}
	arm := FootprintRequest{pcRegister: "_PC"}
	if !slices.Contains(arm.arguments(instruction), "_PC=0x0000000000001000") {
		t.Fatal("ARM footprint arguments did not use _PC")
	}
	rv := FootprintRequest{}
	if !slices.Contains(rv.arguments(instruction), "PC=0x0000000000001000") {
		t.Fatal("RISC-V footprint arguments did not use PC")
	}
}
