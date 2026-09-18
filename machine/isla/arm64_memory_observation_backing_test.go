// ARM64 observation backing tests use explicit readable caller bytes.
// They must not treat executable-only or missing storage as observations.
package isla_test

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildARM64ProgramAcceptsReadableCallerBackingObservation(t *testing.T) {
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
		[]isla.MemoryBacking{{Address: 0x400000, Permission: isla.MemoryRead, Bytes: make([]byte, 8)}},
	)
	if err != nil {
		t.Fatalf("NewARM64MemoryInput() error = %v", err)
	}
	boundary := arm64ProgramBoundary(start, end)
	boundary.Memory = &memory
	boundary.MemoryObservations = []isla.MemoryObservation{{Name: "lane0", Address: 0x400000, Bytes: 4}}
	program, err := isla.BuildARM64Program(content, 32768, boundary)
	if err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
	if !strings.Contains(string(program.Content()), "address = \"0x400000\"\nbytes = 4") {
		t.Fatal("generated ARM64 program lacks caller-backed observation")
	}
}
