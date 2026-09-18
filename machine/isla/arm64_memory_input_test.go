// ARM64 memory input tests define the public typed caller contract.
// They must reject aliases, invalid ranges, and unsupported profiles.
package isla_test

import (
	"bytes"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestNewARM64MemoryInputCopiesBackingBytes(t *testing.T) {
	bytesIn := []byte{0x01, 0x02, 0x03, 0x04}
	input, err := isla.NewARM64MemoryInput(
		isla.ARM64NormalS1Fixed4KSIMD,
		isla.TableArena{Base: 0x5000, CapacityPages: 5},
		[]isla.MemoryMapping{{VA: 0x400000, PA: 0x400000, Length: 0x1000, Permission: isla.MemoryRead}},
		[]isla.MemoryBacking{{Address: 0x400000, Permission: isla.MemoryRead, Bytes: bytesIn}},
	)
	if err != nil {
		t.Fatalf("NewARM64MemoryInput() error = %v", err)
	}
	bytesIn[0] = 0xff
	if input.Backing[0].Bytes[0] != 0x01 {
		t.Fatalf("memory input retained caller byte mutation: %#v", input.Backing[0].Bytes)
	}
	if !bytes.Equal(input.Backing[0].Bytes, []byte{1, 2, 3, 4}) {
		t.Fatalf("memory input bytes = %#v", input.Backing[0].Bytes)
	}
}

func TestNewARM64MemoryInputRejectsInvalidContracts(t *testing.T) {
	cases := []struct {
		name  string
		input isla.MemoryProfile
		table isla.TableArena
		maps  []isla.MemoryMapping
	}{
		{"unknown-profile", "unknown", isla.TableArena{Base: 0x5000, CapacityPages: 1}, nil},
		{"zero-capacity", isla.ARM64NormalS1Fixed4KSIMD, isla.TableArena{Base: 0x5000}, nil},
		{"insufficient-capacity", isla.ARM64NormalS1Fixed4KSIMD, isla.TableArena{Base: 0x5000, CapacityPages: 1}, []isla.MemoryMapping{{VA: 0x400000, PA: 0x400000, Length: 0x1000, Permission: isla.MemoryRead}}},
		{"overflow-range", isla.ARM64NormalS1Fixed4KSIMD, isla.TableArena{Base: 0x5000, CapacityPages: 1}, []isla.MemoryMapping{{VA: ^uint64(0), PA: ^uint64(0), Length: 2, Permission: isla.MemoryRead}}},
		{"overlap", isla.ARM64NormalS1Fixed4KSIMD, isla.TableArena{Base: 0x5000, CapacityPages: 1}, []isla.MemoryMapping{{VA: 0x400000, PA: 0x400000, Length: 0x1000, Permission: isla.MemoryRead}, {VA: 0x400800, PA: 0x400800, Length: 0x1000, Permission: isla.MemoryRead}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := isla.NewARM64MemoryInput(testCase.input, testCase.table, testCase.maps, nil)
			if err == nil {
				t.Fatalf("NewARM64MemoryInput() accepted %s", testCase.name)
			}
		})
	}
}
