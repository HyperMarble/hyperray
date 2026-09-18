// This file validates regular section file ranges and instruction flags.
package arm64

import "fmt"

func validateRegularSection(record SectionRecord, artifactSize, alignment, metadataEnd uint64) error {
	if uint64(record.Header.Offset) < record.Segment.Offset {
		return invalidSection(record, "file range is before its segment")
	}
	offset := uint64(record.Header.Offset) - record.Segment.Offset
	if uint64(record.Header.Offset)%alignment != 0 || offset > record.Segment.Filesz || record.Header.Size > record.Segment.Filesz-offset {
		return invalidSection(record, "file range is outside its segment")
	}
	if record.Header.Addr-record.Segment.Addr != offset {
		return invalidSection(record, "address and file offset do not correspond")
	}
	if uint64(record.Header.Offset) > artifactSize || record.Header.Size > artifactSize-uint64(record.Header.Offset) {
		return invalidSection(record, "file range exceeds artifact")
	}
	if isPureInstructionSection(record.Header) && uint64(record.Header.Offset) < metadataEnd && record.Header.Size != 0 {
		return invalidSection(record, "pure instruction file range overlaps Mach-O metadata")
	}
	return validateInstructionFlags(record)
}

func invalidSection(record SectionRecord, detail string) error {
	return &Rejection{Code: InvalidSection, Detail: fmt.Sprintf("section %q: %s", record.Header.Name, detail)}
}
