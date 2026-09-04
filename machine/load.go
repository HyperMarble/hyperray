// This file coordinates parsing and recording of one ELF image.
// It must return a profile rejection instead of a proof result.
package machine

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
)

// Load accepts one ELF and records its exact bounded load image.
func Load(content []byte, maximumLoadedBytes uint64) (Image, error) {
	if maximumLoadedBytes == 0 {
		return Image{}, reject(InvalidLoadCapacity, "maximum loaded bytes is zero", nil)
	}
	maximumSliceLength := uint64(^uint(0) >> 1)
	if maximumLoadedBytes > maximumSliceLength {
		return Image{}, reject(InvalidLoadCapacity, "maximum loaded bytes exceeds slice capacity", nil)
	}
	file, err := elf.NewFile(bytes.NewReader(content))
	if err != nil {
		return Image{}, reject(MalformedELF, "debug/elf rejected the artifact", err)
	}
	flags, err := validateHeader(file, content)
	if err != nil {
		return Image{}, err
	}
	segments, err := collectSegments(file, content, maximumLoadedBytes)
	if err != nil {
		return Image{}, err
	}
	regions, instructions, err := recordInstructions(segments)
	if err != nil {
		return Image{}, err
	}
	if err := validateInstructionInventory(instructions, file.Entry, flags); err != nil {
		return Image{}, err
	}
	return acceptedImage(content, maximumLoadedBytes, file.Entry, flags, segments, regions, instructions), nil
}

func acceptedImage(content []byte, maximum uint64, entry uint64, flags uint32, segments []loadSegment, regions []ExecutableRegion, instructions []Instruction) Image {
	digest := sha256.Sum256(content)
	return Image{
		Profile:            ProfileName,
		ArtifactSHA256:     hex.EncodeToString(digest[:]),
		EntryAddress:       entry,
		ELFFlags:           flags,
		MaximumLoadedBytes: maximum,
		LoadedBytes:        recordLoadedBytes(segments),
		ExecutableRegions:  regions,
		Instructions:       instructions,
	}
}
