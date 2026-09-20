// Purpose: calls a function the way ARM64 passes its arguments.
// Never:   places a register this architecture does not pass arguments in.
// In:      the address of the instructions, and the register values
// Out:     what the function returned
// Fails:   not applicable, a count beyond the registers is refused earlier

/// This host executes ARM64 instructions, so only ARM64 bytes can be run.
#[cfg(target_arch = "aarch64")]
pub const HOST_RUNS_ARM64: bool = true;
#[cfg(not(target_arch = "aarch64"))]
pub const HOST_RUNS_ARM64: bool = false;

/// The registers this host places before it calls.
///
/// The host runs ARM64, which passes its integer arguments in eight
/// registers. A caller naming more than this is refused, because the rest
/// would be read from a stack the rechecker did not write.
pub const HOST_ARGUMENT_REGISTERS: usize = 8;

/// Calls a function with the host's argument registers set.
///
/// A callee reading fewer registers ignores the rest. A callee reading more
/// is refused before it reaches here.
#[cfg(target_arch = "aarch64")]
pub fn call_with(address: *const u8, values: &[u64; HOST_ARGUMENT_REGISTERS]) -> u64 {
    // Safety: the address holds instructions ending in `ret`, and every
    // register the architecture passes an argument in is given a value.
    let function: extern "C" fn(u64, u64, u64, u64, u64, u64, u64, u64) -> u64 =
        unsafe { std::mem::transmute(address) };
    function(values[0], values[1], values[2], values[3], values[4], values[5], values[6], values[7])
}
