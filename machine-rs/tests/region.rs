// Regions must come from real binaries that the hand-written loader rejected.
// A fixture that only exercises the happy shape would not have caught the
// rules that blocked 23 of 83 measured shapes.
use hyperray_machine::region::{self, Region};

const PROBES: &str = "/Volumes/Hak_SSD/hyperray-build/coverage-probe";

fn regions_of(path: &str) -> Result<Vec<Region>, String> {
    let binary = std::fs::read(path).map_err(|error| format!("{path}: {error}"))?;
    region::regions(&binary)
}

fn covers(found: &[Region], address: u64) -> bool {
    found.iter().any(|region| {
        let end = region.address + region.bytes.len() as u64;
        region.address <= address && address < end
    })
}

/// Go emits `__DWARF` with memory size 0 and file size 704,517. The
/// hand-written loader called that `segment_file_size_exceeds_memory_size`
/// and refused the binary, which blocked all seven measured Go shapes.
#[test]
fn a_debug_only_segment_does_not_reject_the_binary() {
    let found = regions_of(&format!("{PROBES}/goshapes/goshapes.bin"));
    assert!(found.is_ok(), "{:?}", found.err());
}

/// rustc puts `Option`, arrays, and structs in `__const`, which carries no
/// pure-instruction flag. The hand-written loader loaded only flagged
/// sections, so those reads hit unmapped memory and 16 shapes blocked.
#[test]
fn constant_data_is_loaded_with_the_code() {
    let found = regions_of(&format!("{PROBES}/option-some.bin"));
    let reason = match &found {
        Err(error) => error.clone(),
        Ok(regions) if !covers(regions, 0x1_0000_0370) => {
            "0x100000370 is covered by no region".to_string()
        }
        Ok(_) => String::new(),
    };
    assert!(reason.is_empty(), "{reason}");
}
