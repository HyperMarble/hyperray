// A bounded recursive computation retains its stack frames.
// Compiler barriers prevent replacement with a closed-form expression.
#![no_std]

#[inline(never)]
fn sum(depth: u64) -> u64 {
    if depth == 0 {
        return 0;
    }
    let remaining = sum(core::hint::black_box(depth - 1));
    core::hint::black_box(remaining).wrapping_add(depth)
}

#[no_mangle]
pub extern "C" fn _start(value: u64) -> u64 {
    sum(value & 3)
}

#[test]
fn declared_result() {
    assert_eq!(_start(3), 6);
}
