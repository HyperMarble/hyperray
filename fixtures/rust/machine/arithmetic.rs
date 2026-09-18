// Wrapping arithmetic has a result for every machine-width input.
// The test must not replace compiler output with instruction rules.
#![no_std]

#[no_mangle]
pub extern "C" fn _start(value: u64) -> u64 {
    value.wrapping_mul(5).wrapping_add(13)
}
