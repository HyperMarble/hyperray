// Reads a Linux or bare-metal ELF file into the shared image.
// It must never invent bytes or accept a slice it cannot represent.
use crate::image::{Image, LoadError, Segment};
use object::elf::{FileHeader64, EM_AARCH64, EM_RISCV};
use object::read::elf::FileHeader;
use object::{Endianness, Object, ObjectSegment, SegmentFlags};

/// Both architectures this engine proves use the same ELF container.
fn supported_machine(machine: u16) -> bool {
    machine == EM_AARCH64 || machine == EM_RISCV
}

pub fn load(content: &[u8]) -> Result<Image, LoadError> {
    let header =
        FileHeader64::<Endianness>::parse(content).map_err(|e| LoadError::Parse(e.to_string()))?;
    let endian = header.endian().map_err(|e| LoadError::Parse(e.to_string()))?;
    let machine = header.e_machine(endian);
    if !supported_machine(machine) {
        return Err(LoadError::UnsupportedArchitecture { cpu: machine as u32, subtype: 0 });
    }
    let file = object::File::parse(content).map_err(|e| LoadError::Parse(e.to_string()))?;
    let mut segments = Vec::new();
    for segment in file.segments() {
        segments.push(read_segment(content, &segment)?);
    }
    Ok(Image { entry: declared_entry(&file), segments })
}

/// An executable declares where it starts. An object file declares nothing.
fn declared_entry(file: &object::File<'_>) -> Option<u64> {
    let entry = file.entry();
    if entry == 0 {
        None
    } else {
        Some(entry)
    }
}

/// Keeps the file bytes, and records the larger size the range occupies.
///
/// ELF permissions travel with the segment header rather than its name, so
/// they are read from the flags the file declares.
fn read_segment(content: &[u8], segment: &object::Segment<'_, '_>) -> Result<Segment, LoadError> {
    let name = segment.name().ok().flatten().unwrap_or("").to_string();
    let (offset, file_size) = segment.file_range();
    let mapped_size = segment.size();
    let end = offset
        .checked_add(file_size)
        .ok_or(LoadError::ExtentOverflow { name: name.clone() })?;
    if end > content.len() as u64 || file_size > mapped_size {
        return Err(LoadError::SegmentOutsideFile { name });
    }
    segment
        .address()
        .checked_add(mapped_size)
        .ok_or(LoadError::ExtentOverflow { name: name.clone() })?;
    Ok(Segment {
        name,
        address: segment.address(),
        readable: permits(segment, PERMISSION_READ),
        writable: permits(segment, PERMISSION_WRITE),
        executable: permits(segment, PERMISSION_EXECUTE),
        mapped_size,
        bytes: content[offset as usize..end as usize].to_vec(),
    })
}

/// ELF records what a segment permits in its program header flags.
const PERMISSION_EXECUTE: u32 = 0x1;
const PERMISSION_WRITE: u32 = 0x2;
const PERMISSION_READ: u32 = 0x4;

/// Reads a permission from the file rather than assuming one.
fn permits(segment: &object::Segment<'_, '_>, permission: u32) -> bool {
    match segment.flags() {
        SegmentFlags::Elf { p_flags } => p_flags & permission != 0,
        _ => false,
    }
}
