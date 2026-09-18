// Generic instances preserve compiler-generated call boundaries.
// This fixture does not provide instruction semantics.
#![no_std]

#[inline(never)]
fn increase<Value: core::ops::Add<Output = Value>>(left: Value, right: Value) -> Value {
    left + right
}

#[no_mangle]
pub extern "C" fn _start(value: u64) -> u64 {
    let wide = increase(value, 7_u64);
    let narrow = increase(value as u32, 3_u32);
    wide.wrapping_add(u64::from(narrow))
}

#[test]
fn declared_result() {
    assert_eq!(_start(5), 20);
}
