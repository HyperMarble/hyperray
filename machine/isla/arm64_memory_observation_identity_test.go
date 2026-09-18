// ARM64 observation identity tests preserve nil and explicit-empty contracts.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestARM64MemoryObservationEmptyBackingKeepsIdentity(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	nilMemory := arm64MemoryWithoutBacking(t)
	emptyMemory := nilMemory
	emptyMemory.Backing = []isla.MemoryBacking{}
	nilProgram := buildWithMemory(t, content, start, end, nilMemory)
	emptyProgram := buildWithMemory(t, content, start, end, emptyMemory)
	if nilProgram.Evidence().MemoryIdentity != emptyProgram.Evidence().MemoryIdentity {
		t.Fatal("nil and explicit-empty backing changed byte identity")
	}
}

func arm64MemoryWithoutBacking(t *testing.T) isla.ARM64MemoryInput {
	t.Helper()
	memory, err := isla.NewARM64MemoryInput(
		isla.ARM64NormalS1Fixed4KSIMD,
		isla.TableArena{Base: 0x5000, CapacityPages: 41},
		[]isla.MemoryMapping{
			{VA: 0x100000000, PA: 0x100000000, Length: 0x8000, Permission: isla.MemoryReadExecute},
		}, nil,
	)
	if err != nil {
		t.Fatalf("NewARM64MemoryInput() error = %v", err)
	}
	return memory
}

func buildWithMemory(t *testing.T, content []byte, start uint64, end uint64, memory isla.ARM64MemoryInput) isla.Program {
	t.Helper()
	boundary := arm64ProgramBoundary(start, end)
	boundary.Memory = &memory
	boundary.MemoryObservations = []isla.MemoryObservation{{Name: "lane0", Address: start, Bytes: 1}}
	return buildObservedARM64Program(t, content, boundary)
}
