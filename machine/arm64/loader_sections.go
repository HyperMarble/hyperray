// This file validates Mach-O section ownership and collection order.
// It must reject malformed section records before code inventory.
package arm64

import (
	"debug/macho"
	"sort"
)

func validateSections(content []byte) ([]SectionRecord, error) {
	records, err := ReadSectionHeaders(content)
	if err != nil {
		return nil, err
	}
	sections := make([]SectionRecord, 0, len(records))
	order := commandTableByteOrder(content)
	header := decodeMachHeader(content, order)
	metadataEnd := uint64(machHeader64Size) + uint64(header.Cmdsz)
	for _, record := range records {
		if err := validateSection(record, uint64(len(content)), metadataEnd); err != nil {
			return nil, err
		}
		sections = append(sections, record)
	}
	if err := validateSectionOverlap(sectionHeaders(sections)); err != nil {
		return nil, err
	}
	return sections, nil
}

func sectionHeaders(records []SectionRecord) []macho.SectionHeader {
	headers := make([]macho.SectionHeader, 0, len(records))
	for _, record := range records {
		headers = append(headers, record.Header)
	}
	return headers
}

func validateSectionOverlap(sections []macho.SectionHeader) error {
	ordered := append([]macho.SectionHeader(nil), sections...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].Addr < ordered[right].Addr })
	for index := 1; index < len(ordered); index++ {
		previous := ordered[index-1]
		current := ordered[index]
		if previous.Size > current.Addr-previous.Addr {
			return &Rejection{Code: SectionOverlap, Detail: "Mach-O sections overlap"}
		}
	}
	return nil
}
