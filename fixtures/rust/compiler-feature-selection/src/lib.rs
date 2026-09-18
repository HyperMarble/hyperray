// Each permitted feature selection exposes a different compiler inventory.
// Enabling both alternatives must remain a compilation error.
#[cfg(all(feature = "left", feature = "right"))]
compile_error!("left and right cannot be compiled together");

#[cfg(feature = "left")]
pub fn left_only(input: u64) -> u64 {
    input.wrapping_add(1)
}

#[cfg(feature = "right")]
pub fn right_only(input: u64) -> u64 {
    input.wrapping_sub(1)
}

#[cfg(not(any(feature = "left", feature = "right")))]
pub fn neither(input: u64) -> u64 {
    input
}
