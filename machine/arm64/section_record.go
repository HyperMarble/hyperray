// This type retains one section with its owning command and segment.
// It must not classify sections or materialize section or code bytes.
package arm64

import "debug/macho"

// SectionRecord identifies a section inside its load-command owner.
type SectionRecord struct {
	CommandIndex uint32
	SectionIndex uint32
	Segment      macho.SegmentHeader
	Header       macho.SectionHeader
	Reserved1    uint32
	Reserved2    uint32
	Reserved3    uint32
}
