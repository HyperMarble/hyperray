// This negative fixture adds eight where the stack-update requirement demands nine.
// Its repaired counterpart is stack_array.rs; the SDK receives the same requirement.
#![no_std]

#[no_mangle]
pub extern "C" fn _start(value: u64) -> u64 {
    let mut values = [value, value.wrapping_add(1), value.wrapping_add(2), 0];
    let index = (value & 3) as usize;
    let selected = &mut core::hint::black_box(&mut values)[index];
    *selected = selected.wrapping_add(8);
    core::hint::black_box(values)[index]
}

#[test]
fn faulty_result() {
    assert_eq!(_start(2), 12);
}
