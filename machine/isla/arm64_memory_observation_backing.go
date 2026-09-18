// ARM64 observation backing checks loaded bytes and readable caller memory.
// It must not infer coverage from unmapped or execute-only storage.
package isla

import "github.com/HyperMarble/hyperray/machine"

func observationRange(address uint64, bytes uint32) (uint64, bool) {
	width := uint64(bytes)
	if width == 0 || address > ^uint64(0)-width {
		return 0, false
	}
	return address + width, true
}

func observationBacked(start uint64, end uint64, image machine.Image, memory *ARM64MemoryInput) bool {
	if observationBackedByImage(start, end, image) {
		return true
	}
	if memory == nil {
		return false
	}
	for _, backing := range memory.Backing {
		backingEnd := backing.Address + uint64(len(backing.Bytes))
		if start >= backing.Address && end <= backingEnd && readableObservationBacking(backing) {
			return true
		}
	}
	return false
}

func readableObservationBacking(backing MemoryBacking) bool {
	return backing.Permission == MemoryRead || backing.Permission == MemoryReadWrite
}

func observationBackedByImage(start uint64, end uint64, image machine.Image) bool {
	for address := start; address < end; address++ {
		if !loadedAddress(image, address) {
			return false
		}
	}
	return true
}
