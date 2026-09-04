// These tests reject invalid load ranges, flags, and capacities.
// They must identify the first exact segment rule that fails.
package machine_test

import (
	"debug/elf"
	"math"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestRejectsDynamicAndWritableCode(t *testing.T) {
	content := fixture(t, "rv64-lp64d-dynamic.elf")
	requireRejection(t, content, uint64(len(content)), machine.DynamicExecutable)
	content = fixture(t, "rv64-lp64d-static.elf")
	setProgramUint32(content, 0, programTypeOffset, uint32(elf.PT_DYNAMIC))
	requireRejection(t, content, uint64(len(content)), machine.DynamicExecutable)
	content = fixture(t, "rv64-lp64d-writable-code.elf")
	requireRejection(t, content, uint64(len(content)), machine.WritableExecutableSegment)
}

func TestRejectsInvalidSegmentShape(t *testing.T) {
	content := fixture(t, "rv64-lp64d-static.elf")
	setProgramUint32(content, 0, programFlagsOffset, unknownLoadFlag)
	requireRejection(t, content, uint64(len(content)), machine.InvalidLoadFlags)
	content = fixture(t, "rv64-lp64d-static.elf")
	memorySize := programUint64(content, 0, programMemorySizeOffset)
	setProgramUint64(content, 0, programFileSizeOffset, memorySize+1)
	requireRejection(t, content, uint64(len(content)), machine.FileSizeExceedsMemorySize)
	content = fixture(t, "rv64-lp64d-static.elf")
	memorySize = programUint64(content, 0, programMemorySizeOffset)
	setProgramUint64(content, 0, programAddressOffset, math.MaxUint64-memorySize+1)
	requireRejection(t, content, uint64(len(content)), machine.SegmentAddressOverflow)
	content = fixture(t, "rv64-lp64d-static.elf")
	setProgramUint64(content, 0, programFileOffset, uint64(len(content)))
	requireRejection(t, content, uint64(len(content)), machine.SegmentOutsideArtifact)
}

func TestRejectsInvalidSegmentAlignment(t *testing.T) {
	content := fixture(t, "rv64-lp64d-static.elf")
	setProgramUint64(content, 0, programAlignmentOffset, nonPowerOfTwoAlignment)
	requireRejection(t, content, uint64(len(content)), machine.InvalidSegmentAlignment)
	content = fixture(t, "rv64-lp64d-static.elf")
	address := programUint64(content, 0, programAddressOffset)
	setProgramUint64(content, 0, programAddressOffset, address+instructionParcelSize)
	requireRejection(t, content, uint64(len(content)), machine.InvalidSegmentAlignment)
}

func TestRejectsCapacityOverlapAndMissingBytes(t *testing.T) {
	content := fixture(t, "rv64-lp64d-static.elf")
	capacity := programUint64(content, 0, programMemorySizeOffset)
	capacity += programUint64(content, 1, programMemorySizeOffset)
	requireRejection(t, content, capacity-1, machine.LoadedByteCapacityExceeded)
	content = fixture(t, "rv64-lp64d-static.elf")
	firstAddress := programUint64(content, 0, programAddressOffset)
	setProgramUint64(content, 1, programAddressOffset, firstAddress)
	requireRejection(t, content, uint64(len(content)), machine.OverlappingLoadSegments)
	content = fixture(t, "rv64-lp64d-static.elf")
	setProgramUint32(content, 0, programTypeOffset, uint32(elf.PT_NULL))
	setProgramUint32(content, 1, programTypeOffset, uint32(elf.PT_NULL))
	requireRejection(t, content, uint64(len(content)), machine.MissingLoadableBytes)
}
