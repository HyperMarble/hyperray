// This fixture emits one RV64D floating-point operation.
// The test must report a reachable unsupported model primitive.
#![no_std]

#[no_mangle]
pub extern "C" fn _start(left: f64, right: f64) -> f64 {
    left + right
}
