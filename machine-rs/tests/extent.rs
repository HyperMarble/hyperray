// Function extents must come from real binaries, including the two C shapes
// whose extent the objdump-based finder could not determine.
use hyperray_machine::extent::{self, Extent};

const PROBES: &str = "/Volumes/Hak_SSD/hyperray-build/coverage-probe";

fn extent_of(path: &str, name: &str) -> Result<Extent, String> {
    let binary = std::fs::read(path).map_err(|error| format!("{path}: {error}"))?;
    extent::of(&binary, name)
}

/// The Python finder scanned objdump text for the first `ret` after the entry
/// symbol. c_call-never and c_call-nested reported "no return instruction
/// found" because the helper the linker placed first has no reachable ret in
/// that window. The symbol table states the size outright.
#[test]
fn a_function_with_a_call_has_a_stated_extent() {
    let found = extent_of(&format!("{PROBES}/c_call-never.bin"), "_start");
    let reason = match &found {
        Err(error) => error.clone(),
        Ok(extent) if extent.end <= extent.start => format!("empty extent {extent:?}"),
        Ok(_) => String::new(),
    };
    assert!(reason.is_empty(), "{reason}");
}

/// A name the binary does not carry must be a reported failure, never a
/// guessed address.
#[test]
fn an_absent_symbol_is_a_reported_failure() {
    let found = extent_of(&format!("{PROBES}/c_call-never.bin"), "no_such_function");
    assert!(found.is_err(), "absent symbol returned {found:?}");
}

/// c_call-never ends in a tail call: `b 0x1000002e8 <_h>` with no `ret`. The
/// objdump scan looked for the first `ret` after the entry and reported "no
/// return instruction found". The stated extent must still cover the two
/// instructions the linker placed there.
#[test]
fn a_tail_call_has_the_extent_the_linker_laid_out() {
    let found = extent_of(&format!("{PROBES}/c_call-never.bin"), "_start");
    let reason = match &found {
        Err(error) => error.clone(),
        Ok(extent) if extent.start != 0x1_0000_0300 => format!("start {extent:?}"),
        Ok(extent) if extent.end < 0x1_0000_0308 => format!("end before the branch {extent:?}"),
        Ok(_) => String::new(),
    };
    assert!(reason.is_empty(), "{reason}");
}
