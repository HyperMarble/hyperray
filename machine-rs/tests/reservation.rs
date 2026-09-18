// A segment that reserves address space without content must not become
// readable memory. Mach-O __PAGEZERO spans the low 4 GiB so a null access
// faults; filling it with zeros would let one succeed.
use hyperray_machine::region;

#[test]
fn a_reservation_segment_supplies_no_memory() {
    let Ok(bytes) = std::fs::read("/Volumes/Hak_SSD/hyperray-build/coverage-probe/array.bin")
    else {
        return;
    };
    let Ok(found) = region::regions(&bytes) else {
        return;
    };
    let at_zero = found.iter().find(|one| one.address == 0);
    assert!(at_zero.is_none(), "address 0 was loaded: {at_zero:?}");
    let largest = found.iter().map(|one| one.bytes.len()).max().unwrap_or(0);
    assert!(
        largest < 1 << 30,
        "a region of {largest} bytes was materialised"
    );
}
