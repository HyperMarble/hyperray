// A local array preserves mutable stack storage and a dynamic index.
// The compiler supplies its loads, stores, and address calculations.
#![no_std]

#[no_mangle]
pub extern "C" fn _start(value: u64) -> u64 {
    let mut values = [value, value.wrapping_add(1), value.wrapping_add(2), 0];
    let index = (value & 3) as usize;
    let selected = &mut core::hint::black_box(&mut values)[index];
    *selected = selected.wrapping_add(9);
    core::hint::black_box(values)[index]
}

#[test]
fn declared_result() {
    assert_eq!(_start(2), 13);
}
