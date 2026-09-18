// This file validates section attributes and zero-fill placement.
package arm64

import "fmt"

func sectionVirtualRangeWithinSegment(record SectionRecord) bool {
	if record.Header.Addr < record.Segment.Addr {
		return false
	}
	offset := record.Header.Addr - record.Segment.Addr
	if offset > record.Segment.Memsz {
		return false
	}
	return record.Header.Size <= record.Segment.Memsz-offset
}

func validateSectionAttributes(record SectionRecord) error {
	if record.Header.Flags&sectionUnsupportedAttributes != 0 {
		return &Rejection{Code: UnsupportedSectionAttributes, Detail: fmt.Sprintf("section %q flags=0x%08x", record.Header.Name, record.Header.Flags)}
	}
	return nil
}

func validateZeroFillSection(record SectionRecord) error {
	if record.Header.Offset != 0 {
		return invalidSection(record, "zero-fill section has file bytes")
	}
	if !sectionVirtualRangeWithinSegment(record) {
		return invalidSection(record, "zero-fill section lies outside its segment")
	}
	return nil
}
