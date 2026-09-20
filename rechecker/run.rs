// Executes the shipped instructions on this processor with named inputs.
// It must refuse what it cannot run rather than return a value anyway.
use crate::call::{validate, Call, Outcome};
use crate::page::executable_page;
use crate::witness::RecheckError;

/// This host executes ARM64 instructions, so only ARM64 bytes can be run.
#[cfg(target_arch = "aarch64")]
pub const HOST_RUNS_ARM64: bool = true;
#[cfg(not(target_arch = "aarch64"))]
pub const HOST_RUNS_ARM64: bool = false;

/// ARM64 encodes `ret` as this exact word.
const RETURN: u32 = 0xd65f_03c0;

/// Runs one function and returns what the processor produced.
///
/// The bytes come from the compiled program, so the processor executes what
/// shipped. A buffer argument receives the address of its bytes, and those
/// bytes are read back afterwards so a function that writes through a
/// pointer can be checked.
pub fn run(instructions: &[u32], call: &Call) -> Result<Outcome, RecheckError> {
    if !HOST_RUNS_ARM64 {
        return Err(RecheckError::WrongArchitecture);
    }
    if instructions.last() != Some(&RETURN) {
        return Err(RecheckError::Unavailable("the code does not return".to_string()));
    }
    if crate::abi::reads_stack_arguments(instructions) {
        return Err(RecheckError::Unavailable(
            "the function reads arguments this rechecker did not place".to_string(),
        ));
    }
    validate(call).map_err(|error| RecheckError::Unavailable(format!("{error:?}")))?;
    execute(instructions, call)
}

#[cfg(target_arch = "aarch64")]
fn execute(instructions: &[u32], call: &Call) -> Result<Outcome, RecheckError> {
    let mut storage: Vec<Vec<u8>> = call.buffers.iter().map(|b| b.bytes.clone()).collect();
    let arguments = addressed_arguments(call, &mut storage);
    let page = executable_page(instructions)?;
    let mut registers = [0u64; 8];
    registers[..arguments.len().min(8)].copy_from_slice(&arguments[..arguments.len().min(8)]);
    let returned = crate::abi::call_with(page.address(), &registers);
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
fn execute(_instructions: &[u32], _call: &Call) -> Result<Outcome, RecheckError> {
    Err(RecheckError::WrongArchitecture)
}
