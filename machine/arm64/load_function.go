// This file coordinates complete ARM64 Mach-O preflight and mapping.
// It must return a zero image for every rejected artifact.
package arm64

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"

	"github.com/HyperMarble/hyperray/machine"
)

// LoadFunction validates and maps the complete static ARM64 Mach-O image.
// LoadFunction places the pages the stated range can reach.
func LoadFunction(content []byte, maximumLoadedBytes uint64, boundary FunctionBoundary) (machine.Image, error) {
	return LoadFunctionPages(content, maximumLoadedBytes, boundary, nil)
}

// LoadFunctionPages places the given pages as well as the ones the range
// reaches. An empty list places only what the range reaches.
func LoadFunctionPages(content []byte, maximumLoadedBytes uint64, boundary FunctionBoundary, also []uint64) (machine.Image, error) {
	if err := validateLoadCapacity(maximumLoadedBytes); err != nil {
		return machine.Image{}, err
	}
	if err := ValidateCommandTable(content); err != nil {
		return machine.Image{}, err
	}
	segments, err := collectSegments(content, maximumLoadedBytes)
	if err != nil {
		return machine.Image{}, err
	}
	sections, err := validateSections(content)
	if err != nil {
		return machine.Image{}, err
	}
	if err := validateLoadCommands(content, len(sections)); err != nil {
		return machine.Image{}, err
	}
	codeRegions, instructions, err := recordCode(content, sections)
	if err != nil {
		return machine.Image{}, err
	}
	if err := validateBoundary(boundary, codeRegions); err != nil {
		return machine.Image{}, err
	}
	pages := append(ReachablePages(instructions, boundary.StartAddress, boundary.EndAddress), also...)
	return acceptedImage(content, maximumLoadedBytes, boundary.StartAddress, segments, instructions, pages), nil
}

func validateLoadCapacity(maximum uint64) error {
	if maximum == 0 {
		return &Rejection{Code: InvalidLoadCapacity, Detail: "maximum loaded bytes is zero"}
	}
	if maximum > uint64(math.MaxInt) {
		return &Rejection{Code: InvalidLoadCapacity, Detail: fmt.Sprintf("maximum loaded bytes=%d exceeds slice capacity", maximum)}
	}
	return nil
}

func acceptedImage(content []byte, maximum, entry uint64, segments []mappedSegment, instructions []machine.Instruction, pages []uint64) machine.Image {
	blanks, _ := ZeroFillRanges(content)
	digest := sha256.Sum256(content)
	return machine.Image{Profile: ProfileName, ArtifactSHA256: hex.EncodeToString(digest[:]), EntryAddress: entry, MaximumLoadedBytes: maximum, LoadedBytes: materializeSegments(content, segments, blanks, pages), ExecutableRegions: executableSegmentRegions(segments), Instructions: instructions}
}
