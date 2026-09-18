// ARM64 backing edge tests require one complete readable caller backing.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestARM64MemoryObservationRejectsBackingEndOverrun(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	boundary := arm64ProgramBoundary(start, end)
	boundary.Memory = callerMemoryForObservation(t, []isla.MemoryBacking{{
		Address: 0x400000, Permission: isla.MemoryRead, Bytes: make([]byte, 8),
	}})
	boundary.MemoryObservations = []isla.MemoryObservation{{Name: "lane0", Address: 0x400007, Bytes: 2}}
	if _, err := isla.BuildARM64Program(content, 32768, boundary); err == nil {
		t.Fatal("BuildARM64Program() accepted backing end overrun")
	}
}

func TestARM64MemoryObservationRejectsAdjacentBackingsAsOneRange(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	boundary := arm64ProgramBoundary(start, end)
	boundary.Memory = callerMemoryForObservation(t, []isla.MemoryBacking{
		{Address: 0x400000, Permission: isla.MemoryRead, Bytes: make([]byte, 4)},
		{Address: 0x400004, Permission: isla.MemoryRead, Bytes: make([]byte, 4)},
	})
	boundary.MemoryObservations = []isla.MemoryObservation{{Name: "lane0", Address: 0x400002, Bytes: 4}}
	if _, err := isla.BuildARM64Program(content, 32768, boundary); err == nil {
		t.Fatal("BuildARM64Program() joined adjacent backings")
	}
}

func callerMemoryForObservation(t *testing.T, backings []isla.MemoryBacking) *isla.ARM64MemoryInput {
	t.Helper()
	memory, err := isla.NewARM64MemoryInput(
		isla.ARM64NormalS1Fixed4KSIMD,
		isla.TableArena{Base: 0x5000, CapacityPages: 41},
		[]isla.MemoryMapping{
			{VA: 0x100000000, PA: 0x100000000, Length: 0x8000, Permission: isla.MemoryReadExecute},
			{VA: 0x400000, PA: 0x400000, Length: 0x1000, Permission: isla.MemoryRead},
			{VA: 0x3000, PA: 0x3000, Length: 0x1000, Permission: isla.MemoryReadWrite},
		}, backings)
	if err != nil {
		t.Fatalf("NewARM64MemoryInput() error = %v", err)
	}
	return &memory
}
