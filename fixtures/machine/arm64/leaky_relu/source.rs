// This source supplies a bounded no_std ARM64 Leaky ReLU fixture.
// It must not allocate, call an operating system, or use inline assembly.
// Values are Q16.16 fixed point stored in i32. The negative slope is 1/8.
#![no_std]

const LANES: usize = 8;
const SLOPE_SHIFT: u32 = 3;

#[inline(never)]
fn leaky_relu_lane(value: i32) -> i32 {
    if value >= 0 {
        return value;
    }
    // Arithmetic shift keeps the sign. Rounding is toward negative infinity.
    value >> SLOPE_SHIFT
}

/// Applies Leaky ReLU to `LANES` values in place and returns the lane count
/// that were negative before the operation.
#[no_mangle]
pub extern "C" fn arm64_leaky_relu(values: &mut [i32; LANES]) -> u64 {
    let mut negative_count = 0u64;
    let mut index = 0;
    while index < LANES {
        let value = values[index];
        if value < 0 {
            negative_count += 1;
        }
        values[index] = leaky_relu_lane(value);
        index += 1;
    }
    negative_count
}
