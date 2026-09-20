// Purpose: reads what a segment permits, from the file that declares it.
// Never:   decides a permission from a segment's name.
// In:      a segment the format reader parsed
// Out:     whether the segment is readable, writable, or executable
// Fails:   not applicable, a segment of another format permits nothing
use object::{ObjectSegment, SegmentFlags};

/// ELF records what a segment permits in its program header flags.
/// ELF records what a segment permits in its program header flags.
pub const PERMISSION_EXECUTE: u32 = 0x1;
pub const PERMISSION_WRITE: u32 = 0x2;
pub const PERMISSION_READ: u32 = 0x4;

/// Reads a permission from the file rather than assuming one.
pub fn permits(segment: &object::Segment<'_, '_>, permission: u32) -> bool {
    match segment.flags() {
        SegmentFlags::Elf { p_flags } => p_flags & permission != 0,
        _ => false,
    }
}
