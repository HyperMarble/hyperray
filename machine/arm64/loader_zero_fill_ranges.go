// Ranges a binary declares as zero-fill. The linker may place such a section
// inside the file-backed part of its segment, where stale file bytes sit.
package arm64

// ZeroFillRange is one address range that must read as zero.
type ZeroFillRange struct {
	Address uint64
	Length  uint64
}

// ZeroFillRanges returns every range the sections declare as zero-fill.
//
// A section with file bytes is not zero-fill and is skipped.
func ZeroFillRanges(content []byte) ([]ZeroFillRange, error) {
	records, err := ReadSectionHeaders(content)
	if err != nil {
		return nil, err
	}
	ranges := make([]ZeroFillRange, 0)
	for _, record := range records {
		if !declaresZeroFill(record) {
			continue
		}
		ranges = append(ranges, ZeroFillRange{Address: record.Header.Addr, Length: record.Header.Size})
	}
	return ranges, nil
}

func declaresZeroFill(record SectionRecord) bool {
	kind := record.Header.Flags & 0xff
	return kind == sectionZeroFill || kind == sectionGreaterBSS
}

// Covers reports whether the range contains the address.
func (span ZeroFillRange) Covers(address uint64) bool {
	return address >= span.Address && address-span.Address < span.Length
}
