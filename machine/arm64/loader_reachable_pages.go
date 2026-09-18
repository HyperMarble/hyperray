// The pages a proof can reach. A binary holds far more than the function
// under proof, and materializing all of it costs time in every later stage.
package arm64

import "github.com/HyperMarble/hyperray/machine"

// PageSize is the granularity a proof loads at, matching the 4 KiB page the
// ARM64 memory profile maps.
const PageSize uint64 = 0x1000

// ReachablePages returns the pages holding the stated range and every
// address its calls reach.
//
// Caller memory such as the stack is supplied separately and is not part of
// the image. A page nothing reaches is absent, and reading it is refused
// rather than answered with zeros.
func ReachablePages(instructions []machine.Instruction, start uint64, end uint64) []uint64 {
	pages := map[uint64]bool{
		start / PageSize:     true,
		(end - 1) / PageSize: true,
	}
	for _, instruction := range instructions {
		if instruction.Address < start || instruction.Address >= end {
			continue
		}
		for _, address := range reachedAddresses(instruction) {
			pages[address/PageSize] = true
		}
	}
	ordered := make([]uint64, 0, len(pages))
	for page := range pages {
		ordered = append(ordered, page)
	}
	return ordered
}

// reachedAddresses returns every address one instruction names.
//
// A call names its target. ADRP names a page, and a load or store through
// that register can carry an offset past the page it named, so the page
// after it is named too.
func reachedAddresses(instruction machine.Instruction) []uint64 {
	if target, reaches := branchTarget(instruction); reaches {
		return []uint64{target}
	}
	page, names := pageAddress(instruction)
	if !names {
		return nil
	}
	return []uint64{page, page + PageSize}
}

// pageAddress returns the page an ADRP instruction names.
func pageAddress(instruction machine.Instruction) (uint64, bool) {
	if len(instruction.Bytes) != 4 {
		return 0, false
	}
	word := binaryLittleEndian(instruction.Bytes)
	// A64 ADRP is 1 immlo(2) 10000 immhi(19) Rd(5).
	if word>>31 != 1 || (word>>24)&0x1f != 0x10 {
		return 0, false
	}
	low := int64((word >> 29) & 0x3)
	high := int64(int32(word<<8) >> 13)
	offset := (high<<2 | low) * int64(PageSize)
	base := int64(instruction.Address &^ (PageSize - 1))
	return uint64(base + offset), true
}

func binaryLittleEndian(bytes []byte) uint32 {
	return uint32(bytes[0]) | uint32(bytes[1])<<8 | uint32(bytes[2])<<16 | uint32(bytes[3])<<24
}

// PageCovers reports whether any of the pages holds the address.
func PageCovers(pages []uint64, address uint64) bool {
	for _, page := range pages {
		if address/PageSize == page {
			return true
		}
	}
	return false
}
