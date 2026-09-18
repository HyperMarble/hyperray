// ARM64 memory rendering tests bind the exact packet RAM to public TOML.
// They must preserve the original image and emit no implicit backing bytes.
package isla_test

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildARM64ProgramRendersTypedNormalMemory(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	memory, err := isla.NewARM64MemoryInput(
		isla.ARM64NormalS1Fixed4KSIMD,
		isla.TableArena{Base: 0x5000, CapacityPages: 41},
		[]isla.MemoryMapping{
			{VA: 0x100000000, PA: 0x100000000, Length: 0x4000, Permission: isla.MemoryReadExecute},
			{VA: 0x100004000, PA: 0x100004000, Length: 0x4000, Permission: isla.MemoryRead},
			{VA: 0x400000, PA: 0x400000, Length: 0x1000, Permission: isla.MemoryRead},
			{VA: 0x3000, PA: 0x3000, Length: 0x1000, Permission: isla.MemoryReadWrite},
		},
		[]isla.MemoryBacking{
			{Address: 0x400000, Permission: isla.MemoryRead, Bytes: make([]byte, 32)},
			{Address: 0x3bf0, Permission: isla.MemoryReadWrite, Bytes: make([]byte, 80)},
		},
	)
	if err != nil {
		t.Fatalf("NewARM64MemoryInput() error = %v", err)
	}
	boundary := arm64ProgramBoundaryWithReturn(start, end, arm64ProgramReturnAddress)
	boundary.Memory = &memory
	program, err := isla.BuildARM64Program(content, 32768, boundary)
	if err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
	source := string(program.Content())
	for _, expected := range []string{
		`memory_profile = "arm64-normal-s1-fixed-4k-simd-v1"`,
		`[arm64_memory]`,
		`table_base = "0x5000"`,
		`table_capacity_pages = 41`,
		`address = "0x400000"`,
		`bytes = "0000000000000000000000000000000000000000000000000000000000000000"`,
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated ARM64 program lacks %q", expected)
		}
	}
	copyInput, ok := program.MemoryInput()
	if !ok || program.MemoryProfile() != isla.ARM64NormalS1Fixed4KSIMD || program.Evidence().MemoryIdentity == "" {
		t.Fatal("program lacks public normal-memory identity")
	}
	copyInput.Backing[0].Bytes[0] = 0xff
	original, ok := program.MemoryInput()
	if !ok || original.Backing[0].Bytes[0] != 0 {
		t.Fatal("program memory input is not immutable")
	}
}

func TestBackingOverArm64ImageBytesIsRejected(t *testing.T) {
	// Caller memory may not restate a byte the image supplies. The loaded
	// image is the page the function occupies, which is executable, so the
	// overlap is stated directly rather than through a program.
	loaded := []machine.LoadedByte{{
		Address:     0x100000000,
		Permissions: machine.Permissions{Readable: true},
	}}
	backing := []isla.MemoryBacking{{
		Address:    0x100000000,
		Permission: isla.MemoryRead,
		Bytes:      []byte{1},
	}}
	if !isla.BackingOverlapsImage(backing, loaded) {
		t.Fatal("BackingOverlapsImage() did not report caller bytes over the image")
	}
}
