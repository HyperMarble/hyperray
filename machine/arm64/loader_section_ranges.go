// This file validates Mach-O section address and file correspondence.
// It must reject unsupported types, relocations, and unsafe alignments.
package arm64

import (
	"fmt"
)

func validateSection(record SectionRecord, artifactSize, metadataEnd uint64) error {
	if record.Header.Seg != record.Segment.Name {
		return invalidSection(record, "owner segment does not match")
	}
	if err := validateSectionAttributes(record); err != nil {
		return err
	}
	if record.Header.Reloff != 0 || record.Header.Nreloc != 0 {
		return invalidSection(record, "relocations are unsupported")
	}
	if record.Header.Align >= 64 {
		return invalidSection(record, fmt.Sprintf("alignment exponent=%d is unsafe", record.Header.Align))
	}
	alignment := uint64(1) << record.Header.Align
	if record.Header.Addr%alignment != 0 {
		return invalidSection(record, "address is not aligned")
	}
	if !sectionVirtualRangeWithinSegment(record) {
		return invalidSection(record, "virtual range is outside its segment")
	}
	switch record.Header.Flags & 0xff {
	case sectionRegular, sectionCStringLiterals, sectionFourByteLiterals,
		sectionEightByteLiterals, sectionLiteralPointers, sectionNonLazyPointers,
		sectionLazyPointers, sectionSymbolStubs, sectionModInitPointers,
		sectionModTermPointers, sectionCoalesced, sectionThreadLocalRegular,
		sectionThreadLocalVariables, sectionThreadLocalPointers,
		sectionThreadLocalInitPointers:
		return validateRegularSection(record, artifactSize, alignment, metadataEnd)
	case sectionZeroFill:
		return validateZeroFillSection(record)
	case sectionGreaterBSS, sectionThreadLocalZeroFill:
		return validateZeroFillSection(record)
	default:
		return &Rejection{Code: UnsupportedSectionType, Detail: fmt.Sprintf("section %q type=0x%x", record.Header.Name, record.Header.Flags&0xff)}
	}
}
