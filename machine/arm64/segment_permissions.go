// This file maps ARM64 Mach-O initial segment protections to machine permissions.
// It must reject unknown bits, excessive protections, and writable-executable segments.
package arm64

import (
	"debug/macho"
	"fmt"

	"github.com/HyperMarble/hyperray/machine"
)

// Apple SDK mach/vm_prot.h:85-101 defines these protection bit values.
const (
	segmentProtectionNone    uint32 = 0
	segmentProtectionRead    uint32 = 1
	segmentProtectionWrite   uint32 = 2
	segmentProtectionExecute uint32 = 4
	segmentProtectionAll     uint32 = 7
)

// SegmentPermissions converts one segment's initial protection bits.
func SegmentPermissions(header macho.SegmentHeader) (machine.Permissions, error) {
	if header.Prot&^segmentProtectionAll != segmentProtectionNone {
		return machine.Permissions{}, &Rejection{Code: InvalidSegmentProtection, Detail: fmt.Sprintf("segment %q: initprot=%d has bits outside %d", header.Name, header.Prot, segmentProtectionAll)}
	}
	if header.Maxprot&^segmentProtectionAll != segmentProtectionNone {
		return machine.Permissions{}, &Rejection{Code: InvalidSegmentProtection, Detail: fmt.Sprintf("segment %q: maxprot=%d has bits outside %d", header.Name, header.Maxprot, segmentProtectionAll)}
	}
	if header.Prot&^header.Maxprot != segmentProtectionNone {
		return machine.Permissions{}, &Rejection{Code: SegmentProtectionExceedsMaximum, Detail: fmt.Sprintf("segment %q: initprot=%d exceeds maxprot=%d", header.Name, header.Prot, header.Maxprot)}
	}
	if header.Prot&(segmentProtectionWrite|segmentProtectionExecute) == segmentProtectionWrite|segmentProtectionExecute {
		return machine.Permissions{}, &Rejection{Code: WritableExecutableSegment, Detail: fmt.Sprintf("segment %q: initprot=%d is writable and executable", header.Name, header.Prot)}
	}
	return machine.Permissions{
		Readable:   header.Prot&segmentProtectionRead != segmentProtectionNone,
		Writable:   header.Prot&segmentProtectionWrite != segmentProtectionNone,
		Executable: header.Prot&segmentProtectionExecute != segmentProtectionNone,
	}, nil
}
