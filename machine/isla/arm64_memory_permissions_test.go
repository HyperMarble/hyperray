// ARM64 caller RAM tests enforce the exact shared permission matrix.
// They must reject executable backing and permission widening.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestNewARM64MemoryInputUsesExactCallerPermissionPairs(t *testing.T) {
	cases := []struct {
		name      string
		mapping   isla.MemoryPermission
		backing   isla.MemoryPermission
		wantError bool
	}{
		{"read-read", isla.MemoryRead, isla.MemoryRead, false},
		{"readwrite-readwrite", isla.MemoryReadWrite, isla.MemoryReadWrite, false},
		{"readwrite-read", isla.MemoryReadWrite, isla.MemoryRead, true},
		{"readexecute-read", isla.MemoryReadExecute, isla.MemoryRead, true},
		{"read-readwrite", isla.MemoryRead, isla.MemoryReadWrite, true},
		{"readwrite-readexecute", isla.MemoryReadWrite, isla.MemoryReadExecute, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := isla.NewARM64MemoryInput(
				isla.ARM64NormalS1Fixed4KSIMD,
				isla.TableArena{Base: 0x5000, CapacityPages: 5},
				[]isla.MemoryMapping{{VA: 0x400000, PA: 0x400000, Length: 0x1000, Permission: testCase.mapping}},
				[]isla.MemoryBacking{{Address: 0x400000, Permission: testCase.backing, Bytes: []byte{1}}},
			)
			if (err != nil) != testCase.wantError {
				t.Fatalf("permission pair error = %v, want error %t", err, testCase.wantError)
			}
		})
	}
}
