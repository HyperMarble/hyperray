// ARM64 image collision checks keep caller RAM outside the loaded image.
// It must not permit caller bytes to replace code, data, or linkedit bytes.
package isla

import "github.com/HyperMarble/hyperray/machine"

// BackingOverlapsImage reports caller bytes that restate an image byte.
func BackingOverlapsImage(backing []MemoryBacking, loaded []machine.LoadedByte) bool {
	for _, value := range backing {
		end := value.Address + uint64(len(value.Bytes))
		if backingOverlapsLoadedRange(value.Address, end, loaded) {
			return true
		}
	}
	return false
}

func backingOverlapsLoadedRange(start uint64, end uint64, loaded []machine.LoadedByte) bool {
	for _, byteValue := range loaded {
		if byteValue.Address >= start && byteValue.Address < end {
			return true
		}
	}
	return false
}
