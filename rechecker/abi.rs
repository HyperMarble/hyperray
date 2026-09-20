// Places the registers a witness names, and nothing the witness did not name.
// A register the solver did not report must not be given an invented value.

/// The registers ARM64 passes arguments in, in the order the architecture
/// assigns them.
pub const ARGUMENT_REGISTERS: [&str; 8] = ["X0", "X1", "X2", "X3", "X4", "X5", "X6", "X7"];

/// Reads the argument registers out of a witness, in architecture order.
///
/// The solver names the registers the instructions read, so the count comes
/// from the witness rather than from a limit chosen here. A register the
/// witness does not name is zero, which is what an unread register holds.
pub fn argument_values(named: &[(String, u64)]) -> Vec<u64> {
    let mut values = vec![0u64; ARGUMENT_REGISTERS.len()];
    for (name, value) in named {
        if let Some(index) = ARGUMENT_REGISTERS.iter().position(|register| register == name) {
            values[index] = *value;
        }
    }
    values
}

/// Calls a function with the eight argument registers set.
///
/// A callee reading fewer registers ignores the rest. A callee reading its
/// later arguments from the stack is refused by `stack_reader`, because the
/// values it would read were never placed there.
#[cfg(target_arch = "aarch64")]
pub fn call_with(address: *const u8, values: &[u64; 8]) -> u64 {
    // Safety: the address holds instructions ending in `ret`, and every
    // register the architecture passes an argument in is given a value.
    let function: extern "C" fn(u64, u64, u64, u64, u64, u64, u64, u64) -> u64 =
        unsafe { std::mem::transmute(address) };
    function(values[0], values[1], values[2], values[3], values[4], values[5], values[6], values[7])
}

/// ARM64 reads a stack argument through the stack pointer, so an instruction
/// that loads from `[sp]` before writing it is reading an argument this
/// rechecker did not place.
pub fn reads_stack_arguments(instructions: &[u32]) -> bool {
    instructions.iter().any(|word| loads_from_stack_pointer(*word))
}

/// Matches `ldr` and `ldp` with the stack pointer as the base register.
fn loads_from_stack_pointer(word: u32) -> bool {
    let base = (word >> 5) & 0x1f;
    if base != 31 {
        return false;
    }
    let load_pair = word & 0xffc0_0000 == 0xa940_0000;
    let load_single = word & 0xffc0_0000 == 0xf940_0000;
    load_pair || load_single
}
