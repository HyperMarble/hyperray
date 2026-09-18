// A bounded table index selects between two compiler-retained functions.
// The verifier must preserve both targets of the indirect call.
#![no_std]

#[inline(never)]
fn first() -> u64 {
    core::hint::black_box(17)
}

#[inline(never)]
fn second() -> u64 {
    core::hint::black_box(64)
}

#[no_mangle]
pub extern "C" fn _start(value: u64) -> u64 {
    let functions: [fn() -> u64; 2] = [first, second];
    let target = core::hint::black_box(functions)[(value & 1) as usize];
    target()
}

#[test]
fn declared_results() {
    assert_eq!(_start(0), 17);
    assert_eq!(_start(1), 64);
    assert_eq!(_start(u64::MAX), 64);
}
