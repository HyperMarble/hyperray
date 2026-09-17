// The memory image a proof starts from: every loaded byte at its address,
// plus the ranges left unknown. It must not invent a byte the binary does
// not supply, and must not leave a loaded byte out.
use crate::permission::Permission;
use crate::region::Region;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Image {
    pub concrete: Vec<Placed>,
    pub unknown: Vec<Range>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Placed {
    pub address: u64,
    pub bytes: Vec<u8>,
    pub writable: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Range {
    pub low: u64,
    pub high: u64,
}

/// The image the engine starts from: the binary's own bytes, and the caller's
/// declared unknown ranges.
///
/// A writable region is placed like any other, because its initial contents
/// are part of the program even where the program later overwrites them.
pub fn of(regions: &[Region], unknown: &[Range]) -> Result<Image, String> {
    if let Some(reason) = unusable(unknown) {
        return Err(reason);
    }
    let concrete = regions.iter().map(placed).collect();
    Ok(Image {
        concrete,
        unknown: unknown.to_vec(),
    })
}

fn placed(region: &Region) -> Placed {
    Placed {
        address: region.address,
        bytes: region.bytes.clone(),
        writable: region.permission == Permission::ReadWrite,
    }
}

/// Why the declared unknown ranges cannot be used, or `None` when they can.
///
/// An empty range declares nothing, and overlapping ranges would have to be
/// merged. Naming either lets the caller fix the request instead of receiving
/// a silently altered one.
fn unusable(unknown: &[Range]) -> Option<String> {
    for (index, range) in unknown.iter().enumerate() {
        if range.high <= range.low {
            return Some(format!("unknown range {index} is empty: {range:?}"));
        }
        let clash = unknown
            .iter()
            .take(index)
            .find(|earlier| overlaps(earlier, range));
        if let Some(earlier) = clash {
            return Some(format!("unknown ranges overlap: {earlier:?} and {range:?}"));
        }
    }
    None
}

fn overlaps(left: &Range, right: &Range) -> bool {
    right.low < left.high && left.low < right.high
}
