// A loadable region of a binary: where its bytes go in memory, and what may
// be done with them. It reports what the file says. It must never reject a
// file for a shape the format permits.
use object::{Object, ObjectSegment};

use crate::permission::{self, Permission};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Region {
    pub address: u64,
    pub bytes: Vec<u8>,
    pub permission: Permission,
}

/// Every segment that occupies memory, with the bytes the file supplies.
///
/// A segment whose memory size is zero holds no runtime state: debug
/// information is the common case, and rejecting it would reject any binary
/// built with `-g`. A segment larger than its file bytes is zero filled, which
/// is how `.bss` and Mach-O zero-fill sections are expressed.
pub fn regions(binary: &[u8]) -> Result<Vec<Region>, String> {
    let file = object::File::parse(binary).map_err(|error| error.to_string())?;
    let mut found = Vec::new();
    for segment in file.segments() {
        let supplied = segment.data().map_err(|error| error.to_string())?;
        let Some(region) = occupied(
            segment.address(),
            segment.size(),
            supplied,
            permission::of(segment.flags()),
        )?
        else {
            continue;
        };
        found.push(region);
    }
    Ok(found)
}

/// One region, zero filled beyond the bytes the file supplies.
///
/// Returns `None` for a segment that occupies no memory, which is a statement
/// about the file rather than a failure.
fn occupied(
    address: u64,
    size: u64,
    supplied: &[u8],
    permission: Permission,
) -> Result<Option<Region>, String> {
    if size == 0 {
        return Ok(None);
    }
    let length = usize::try_from(size)
        .map_err(|_| format!("segment size {size} exceeds the host address range"))?;
    let mut bytes = vec![0u8; length];
    let copied = supplied.len().min(length);
    bytes[..copied].copy_from_slice(&supplied[..copied]);
    Ok(Some(Region {
        address,
        bytes,
        permission,
    }))
}
