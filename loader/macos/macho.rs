// Purpose: reads a macOS Mach-O file into the shared image.
// Never:   invents bytes, or reads a slice it cannot represent.
// In:      the bytes of a file
// Out:     an Image with its segments and its entry, when one is declared
// Fails:   unreadable header, no ARM64 part, a segment outside the file
use crate::image::{Image, LoadError, Segment};
use object::macho::{MachHeader64, CPU_TYPE_ARM64};
use object::read::macho::MachHeader;
use object::{Endianness, Object, ObjectSegment};

pub fn load(content: &[u8]) -> Result<Image, LoadError> {
    let slice = crate::slice::arm64_slice(content)?;
    read_image(slice)
}

fn read_image(content: &[u8]) -> Result<Image, LoadError> {
    let header =
        MachHeader64::<Endianness>::parse(content, 0).map_err(|e| LoadError::Parse(e.to_string()))?;
    let endian = header.endian().map_err(|e| LoadError::Parse(e.to_string()))?;
    let cpu = header.cputype(endian);
    if cpu != CPU_TYPE_ARM64 {
        return Err(LoadError::UnsupportedArchitecture { cpu, subtype: header.cpusubtype(endian) });
    }
    let file = object::File::parse(content).map_err(|e| LoadError::Parse(e.to_string()))?;
    let mut segments = Vec::new();
    for segment in file.segments() {
        segments.push(read_segment(content, &segment)?);
    }
    Ok(Image { entry: declared_entry(&file), segments })
}

/// An executable declares where it starts. A library declares nothing.
fn declared_entry(file: &object::File<'_>) -> Option<u64> {
    let entry = file.entry();
    if entry == 0 {
        None
    } else {
        Some(entry)
    }
}

/// Keeps the file bytes, and records the larger size the range occupies.
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
        readable: crate::macos_permission::permits(segment, crate::macos_permission::PROTECTION_READ),
        writable: crate::macos_permission::permits(segment, crate::macos_permission::PROTECTION_WRITE),
        executable: crate::macos_permission::permits(segment, crate::macos_permission::PROTECTION_EXECUTE),
        address: segment.address(),
        mapped_size,
        bytes: content[offset as usize..end as usize].to_vec(),
        name,
    })
}
