// ARM64 image validation joins caller RAM with the complete loaded image.
// It must not permit image bytes or the entry outside declared mappings.
package isla

import (
	"fmt"

	"github.com/HyperMarble/hyperray/machine"
)

func copyValidatedARM64Memory(input *ARM64MemoryInput, image machine.Image) (*ARM64MemoryInput, error) {
	if input == nil {
		return nil, nil
	}
	copyInput := copyARM64MemoryInput(*input)
	if err := validateARM64MemoryInput(copyInput); err != nil {
		return nil, err
	}
	if err := validateARM64ImageMappings(copyInput, image); err != nil {
		return nil, err
	}
	return &copyInput, nil
}

func copyARM64MemoryInput(input ARM64MemoryInput) ARM64MemoryInput {
	return ARM64MemoryInput{
		Profile: input.Profile, Table: input.Table,
		Mappings: append([]MemoryMapping(nil), input.Mappings...),
		Backing:  copyMemoryBackings(input.Backing),
	}
}

func validateARM64ImageMappings(input ARM64MemoryInput, image machine.Image) error {
	if BackingOverlapsImage(input.Backing, image.LoadedBytes) {
		return engineError(CoverageMismatch, "ARM64 memory backing", "caller bytes overlap the loaded image")
	}
	for _, loaded := range image.LoadedBytes {
		if !mappedImageByte(input.Mappings, loaded) {
			return engineError(CoverageMismatch, "ARM64 memory mapping",
				fmt.Sprintf("loaded image byte 0x%x is not covered", loaded.Address))
		}
	}
	if !mappedExecutableAddress(input.Mappings, image.EntryAddress) {
		return engineError(CoverageMismatch, "ARM64 memory mapping", "image entry is not executable")
	}
	return nil
}

func mappedImageByte(mappings []MemoryMapping, loaded machine.LoadedByte) bool {
	for _, mapping := range mappings {
		end := mapping.VA + mapping.Length
		if loaded.Address >= mapping.VA && loaded.Address < end && imagePermissionAllows(mapping.Permission, loaded.Permissions) {
			return true
		}
	}
	return false
}

func mappedExecutableAddress(mappings []MemoryMapping, address uint64) bool {
	for _, mapping := range mappings {
		if mapping.Permission == MemoryReadExecute && address >= mapping.VA && address < mapping.VA+mapping.Length {
			return true
		}
	}
	return false
}

func imagePermissionAllows(mapping MemoryPermission, loaded machine.Permissions) bool {
	if loaded.Writable {
		return mapping == MemoryReadWrite
	}
	if loaded.Executable {
		return mapping == MemoryReadExecute
	}
	return mapping == MemoryRead || mapping == MemoryReadWrite || mapping == MemoryReadExecute
}
