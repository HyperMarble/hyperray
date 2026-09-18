// This file applies Mach-O section type and instruction flag rules.
// It must not infer instruction meaning from opcode bytes.
package arm64

import (
	"debug/macho"
	"fmt"
)

const (
	// Apple SDK mach-o/loader.h defines S_REGULAR as zero.
	sectionRegular uint32 = 0x0
	// Apple SDK mach-o/loader.h defines S_ZEROFILL as one.
	sectionZeroFill uint32 = 0x1
	// Apple SDK mach-o/loader.h defines S_GB_ZEROFILL as twelve.
	sectionGreaterBSS uint32 = 0x0c
	// Apple SDK mach-o/loader.h defines these as "section with only" a
	// stated kind of content. Each occupies memory and supplies file bytes
	// exactly as a regular section does; the type states what the bytes
	// mean, not how they are loaded.
	sectionCStringLiterals   uint32 = 0x02
	sectionFourByteLiterals  uint32 = 0x03
	sectionEightByteLiterals uint32 = 0x04
	sectionLiteralPointers   uint32 = 0x05
	sectionNonLazyPointers   uint32 = 0x06
	sectionLazyPointers      uint32 = 0x07
	sectionSymbolStubs       uint32 = 0x08
	sectionModInitPointers   uint32 = 0x09
	sectionModTermPointers   uint32 = 0x0a
	sectionCoalesced         uint32 = 0x0b
	// Apple SDK mach-o/loader.h defines the thread-local types. A template
	// of initial values and the descriptors that name them supply bytes;
	// only the zero-fill template supplies none.
	sectionThreadLocalRegular      uint32 = 0x11
	sectionThreadLocalZeroFill     uint32 = 0x12
	sectionThreadLocalVariables    uint32 = 0x13
	sectionThreadLocalPointers     uint32 = 0x14
	sectionThreadLocalInitPointers uint32 = 0x15
	// Apple SDK mach-o/loader.h defines S_ATTR_PURE_INSTRUCTIONS.
	sectionPureCode uint32 = 0x80000000
	// Apple SDK mach-o/loader.h defines S_ATTR_SOME_INSTRUCTIONS.
	sectionSomeCode              uint32 = 0x00000400
	sectionUnsupportedAttributes uint32 = 0x04000300
)

func validateInstructionFlags(record SectionRecord) error {
	flags := record.Header.Flags
	pure := flags&sectionPureCode != 0
	some := flags&sectionSomeCode != 0
	if some && !pure {
		return &Rejection{Code: MixedInstructionData, Detail: fmt.Sprintf("section %q has mixed instruction-data flags", record.Header.Name)}
	}
	if !pure {
		return nil
	}
	if record.Segment.Prot&4 == 0 {
		return invalidSection(record, "pure instructions are not in an executable segment")
	}
	if record.Header.Size%4 != 0 || record.Header.Addr%4 != 0 {
		return invalidSection(record, "pure instruction section is not 4-byte aligned")
	}
	return nil
}

func isPureInstructionSection(section macho.SectionHeader) bool {
	return section.Flags&0xff == sectionRegular && section.Flags&sectionPureCode != 0
}
