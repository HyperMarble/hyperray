// Addresses a dynamic loader rewrites before a program runs. A proof reads
// the bytes in the file, so a function that reads a rewritten address would
// be proved against bytes the running program never sees.
package arm64

import (
	"debug/macho"
	"encoding/binary"
	"fmt"

	"github.com/HyperMarble/hyperray/machine"
)

// RewrittenRange is one region the dynamic loader changes at load time.
type RewrittenRange struct {
	Address uint64
	Length  uint64
}

// Covers reports whether the range contains the address.
func (span RewrittenRange) Covers(address uint64) bool {
	return address >= span.Address && address-span.Address < span.Length
}

// RewrittenRanges returns the sections a dynamic loader fills in.
//
// Apple SDK mach-o/loader.h names these section types as holding pointers
// resolved at load time: non-lazy and lazy symbol pointers, symbol stubs,
// and pointers to thread-local descriptors.
func RewrittenRanges(content []byte) ([]RewrittenRange, error) {
	records, err := ReadSectionHeaders(content)
	if err != nil {
		return nil, err
	}
	ranges := make([]RewrittenRange, 0)
	for _, record := range records {
		if !filledByTheLoader(record.Header) {
			continue
		}
		ranges = append(ranges, RewrittenRange{
			Address: record.Header.Addr,
			Length:  record.Header.Size,
		})
	}
	return ranges, nil
}

func filledByTheLoader(header macho.SectionHeader) bool {
	switch header.Flags & 0xff {
	case sectionNonLazyPointers, sectionLazyPointers, sectionSymbolStubs,
		sectionThreadLocalPointers:
		return true
	default:
		return false
	}
}

// ReadsRewrittenAddress reports the first instruction in the range that
// reads or calls an address the loader rewrites, or nil when none does.
func ReadsRewrittenAddress(instructions []machine.Instruction, ranges []RewrittenRange) error {
	for _, instruction := range instructions {
		target, reaches := branchTarget(instruction)
		if !reaches {
			continue
		}
		for _, span := range ranges {
			if span.Covers(target) {
				return fmt.Errorf("instruction at 0x%x reaches 0x%x, which the loader rewrites",
					instruction.Address, target)
			}
		}
	}
	return nil
}

func branchTarget(instruction machine.Instruction) (uint64, bool) {
	if len(instruction.Bytes) != 4 {
		return 0, false
	}
	word := binary.LittleEndian.Uint32(instruction.Bytes)
	// A64 BL is 100101 followed by a signed 26-bit word offset.
	if word>>26 != 0x25 {
		return 0, false
	}
	offset := int64(int32(word<<6) >> 6 * 4)
	return uint64(int64(instruction.Address) + offset), true
}
