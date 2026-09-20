// Purpose: reads what a segment permits, from the file that declares it.
// Never:   decides a permission from a segment's name.
// In:      a segment the format reader parsed
// Out:     whether the segment is readable, writable, or executable
// Fails:   not applicable, a segment of another format permits nothing
use object::{ObjectSegment, SegmentFlags};

/// Mach-O records what a segment permits in its initial protection.
/// Mach-O records what a segment permits in its initial protection field.
pub const PROTECTION_READ: u32 = 0x1;
pub const PROTECTION_WRITE: u32 = 0x2;
pub const PROTECTION_EXECUTE: u32 = 0x4;

/// Reads a permission from the file rather than from the segment's name.
pub fn permits(segment: &object::Segment<'_, '_>, permission: u32) -> bool {
    match segment.flags() {
        SegmentFlags::MachO { initprot, .. } => initprot & permission != 0,
        _ => false,
    }
}
