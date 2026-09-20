// Installs loaded segments into Isla memory as concrete regions.
// A zero-filled segment must not be expanded byte by byte.
use crate::image::Image;
use isla_lib::bitvector::b64::B64;
use isla_lib::memory::Memory;
use std::collections::HashMap;

/// Returns the number of bytes installed, so the caller can assert coverage.
pub fn install_regions(memory: &mut Memory<B64>, image: &Image) -> u64 {
    let mut installed = 0u64;
    for segment in &image.segments {
        installed += segment.mapped_size;
        let file_end = segment.address + segment.bytes.len() as u64;
        if !segment.bytes.is_empty() {
            memory.add_concrete_region(
                segment.address..file_end,
                byte_map(segment.address, &segment.bytes),
            );
        }
        let mapped_end = segment.address + segment.mapped_size;
        if mapped_end > file_end {
            memory.add_zero_region(file_end..mapped_end);
        }
    }
    installed
}

fn byte_map(address: u64, bytes: &[u8]) -> HashMap<u64, u8> {
    let mut contents = HashMap::with_capacity(bytes.len());
    for (index, byte) in bytes.iter().enumerate() {
        contents.insert(address + index as u64, *byte);
    }
    contents
}
