// Purpose: names the ARM64 part of a macOS file.
// Never:   assumes a universal file holds only one architecture.
// In:      the bytes of a file
// Out:     the part this engine reads, or the whole file when it is not universal
// Fails:   the file holds no ARM64 part, or the part lies outside the file
use crate::image::LoadError;
use object::macho::CPU_TYPE_ARM64;
use object::read::macho::{FatArch, MachOFatFile32, MachOFatFile64};

/// Returns the ARM64 part of the file.
///
/// A universal file holds one part per architecture, so the ARM64 part is
/// named rather than assumed to be the whole file. A file holding no ARM64
/// part is refused, because nothing in it can be read here.
///
/// Apple writes the part table in two widths, so both are read.
pub fn arm64_slice(content: &[u8]) -> Result<&[u8], LoadError> {
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
