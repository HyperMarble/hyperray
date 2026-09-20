// Purpose: runs one function where the operating system placed it.
// Never:   moves the code, which would break an instruction that counts
//          from its own position to reach a value.
// In:      a loaded library, a function name, and the arguments to pass
// Out:     what the processor returned, and the buffers after the call
// Fails:   wrong processor, the name is absent, the call cannot be made
use crate::call::{validate, Call, Outcome};
use crate::placed::Placed;
use crate::witness::RecheckError;

/// Calls a named function in a library the operating system loaded.
///
/// The instructions stay where they were linked, so the distance between
/// the code and the values it reads is the distance the linker set.
pub fn run_placed(
    placed: &Placed,
    name: &str,
    call: &Call,
) -> Result<Outcome, RecheckError> {
    if !crate::arm64_call::HOST_RUNS_ARM64 {
        return Err(RecheckError::WrongArchitecture);
    }
    if call.arguments.len() > crate::arm64_call::HOST_ARGUMENT_REGISTERS {
        return Err(RecheckError::Unavailable(
            "more arguments than this host places in registers".to_string(),
        ));
    }
    validate(call).map_err(|error| RecheckError::Unavailable(format!("{error:?}")))?;
    let address = placed.address_of(name)?;
    call_at(address, call)
}

#[cfg(target_arch = "aarch64")]
fn call_at(address: *const u8, call: &Call) -> Result<Outcome, RecheckError> {
    let mut storage: Vec<Vec<u8>> = call.buffers.iter().map(|b| b.bytes.clone()).collect();
    let arguments = addressed_arguments(call, &mut storage);
    let mut registers = [0u64; crate::arm64_call::HOST_ARGUMENT_REGISTERS];
    let placed_count = arguments.len().min(registers.len());
    registers[..placed_count].copy_from_slice(&arguments[..placed_count]);
    let returned = crate::arm64_call::call_with(address, &registers);
    Ok(Outcome { returned, buffers: storage })
}

/// Replaces each buffer argument with the address of its bytes.
#[cfg(target_arch = "aarch64")]
fn addressed_arguments(call: &Call, storage: &mut [Vec<u8>]) -> Vec<u64> {
    let mut arguments = call.arguments.clone();
    for (index, buffer) in call.buffers.iter().enumerate() {
        arguments[buffer.argument] = storage[index].as_mut_ptr() as u64;
    }
    arguments
}

#[cfg(not(target_arch = "aarch64"))]
fn call_at(_address: *const u8, _call: &Call) -> Result<Outcome, RecheckError> {
    Err(RecheckError::WrongArchitecture)
}
