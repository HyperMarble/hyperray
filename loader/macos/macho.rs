// Reads a macOS Mach-O file into the shared image.
// It must never invent bytes or accept a slice it cannot represent.
use crate::image::{Image, LoadError, Segment};
use object::macho::{MachHeader64, CPU_TYPE_ARM64};
use object::read::macho::{FatArch, MachHeader, MachOFatFile32, MachOFatFile64};
use object::{Endianness, Object, ObjectSegment};

pub fn load(content: &[u8]) -> Result<Image, LoadError> {
    let slice = arm64_slice(content)?;
    read_image(slice)
}

/// Returns the ARM64 part of the file.
///
/// A universal file holds one part per architecture, so the ARM64 part is
/// named rather than assumed to be the whole file. A file holding no ARM64
/// part is refused, because nothing in it can be read here.
///
/// Apple writes the part table in two widths, so both are read.
fn arm64_slice(content: &[u8]) -> Result<&[u8], LoadError> {
    if let Ok(fat) = MachOFatFile64::parse(content) {
        return named_arch(content, fat.arches());
    }
    if let Ok(fat) = MachOFatFile32::parse(content) {
        return named_arch(content, fat.arches());
    }
    Ok(content)
}

fn named_arch<'a, Fat: FatArch>(content: &'a [u8], arches: &[Fat]) -> Result<&'a [u8], LoadError> {
    for arch in arches {
        if arch.cputype() == CPU_TYPE_ARM64 {
            return arch.data(content).map_err(|e| LoadError::Parse(e.to_string()));
        }
    }
    Err(LoadError::UnsupportedArchitecture { cpu: 0, subtype: 0 })
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
        readable: true,
        writable: name.starts_with("__DATA"),
        executable: name == "__TEXT",
        address: segment.address(),
        mapped_size,
        bytes: content[offset as usize..end as usize].to_vec(),
        name,
    })
}
