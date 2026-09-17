// What a loaded region may be used for, read from the format's own flag
// bits. It must not infer a permission the file did not state.
use object::SegmentFlags;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Permission {
    Read,
    ReadWrite,
    ReadExecute,
}

/// The permission a segment declares, for either binary format. Execute wins
/// over write.
pub fn of(flags: SegmentFlags) -> Permission {
    match flags {
        SegmentFlags::MachO { initprot, .. } => from_bits(initprot & 0x2 != 0, initprot & 0x4 != 0),
        SegmentFlags::Elf { p_flags, .. } => from_bits(p_flags & 0x2 != 0, p_flags & 0x1 != 0),
        _ => Permission::Read,
    }
}

fn from_bits(writable: bool, executable: bool) -> Permission {
    match (writable, executable) {
        (_, true) => Permission::ReadExecute,
        (true, false) => Permission::ReadWrite,
        (false, false) => Permission::Read,
    }
}
