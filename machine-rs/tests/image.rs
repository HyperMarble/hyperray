// The image must carry every byte the binary supplies and must name a bad
// unknown range instead of altering it.
use hyperray_machine::image::{self, Image, Range};
use hyperray_machine::region;

const PROBES: &str = "/Volumes/Hak_SSD/hyperray-build/coverage-probe";

fn image_of(path: &str, unknown: &[Range]) -> Result<Image, String> {
    let binary = std::fs::read(path).map_err(|error| format!("{path}: {error}"))?;
    let regions = region::regions(&binary)?;
    image::of(&regions, unknown)
}

/// The constant the hand-written loader skipped must be present as a byte,
/// not merely inside a mapped range. option-some.bin reads 0x100000370.
#[test]
fn the_constant_a_read_reaches_is_present() {
    let found = image_of(&format!("{PROBES}/option-some.bin"), &[]);
    let reason = match &found {
        Err(error) => error.clone(),
        Ok(built) if !holds(built, 0x1_0000_0370) => "0x100000370 has no byte".to_string(),
        Ok(_) => String::new(),
    };
    assert!(reason.is_empty(), "{reason}");
}

/// Two unknown ranges that overlap would have to be merged. The caller is
/// told which pair instead.
#[test]
fn overlapping_unknown_ranges_are_named() {
    let ranges = [
        Range {
            low: 0x1000,
            high: 0x2000,
        },
        Range {
            low: 0x1800,
            high: 0x2800,
        },
    ];
    let found = image_of(&format!("{PROBES}/option-some.bin"), &ranges);
    assert!(found.is_err(), "overlap accepted: {found:?}");
}

fn holds(built: &Image, address: u64) -> bool {
    built.concrete.iter().any(|placed| {
        let end = placed.address + placed.bytes.len() as u64;
        placed.address <= address && address < end
    })
}
