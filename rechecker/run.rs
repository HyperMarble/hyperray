// Purpose: executes the shipped instructions on this processor.
// Never:   returns a value for code it could not run honestly.
// In:      instructions, a call with named inputs, the model's facts,
//          and the name the model gives the stack pointer
// Out:     what the processor produced, and the buffers after the call
// Fails:   wrong architecture, no return, memory the caller did not place
use crate::call::{validate, Call, Outcome};
use crate::page::executable_page;
use crate::witness::RecheckError;
use solver::Facts;

/// Runs one function and returns what the processor produced.
///
/// The bytes come from the compiled program, so the processor executes what
/// shipped. A buffer argument receives the address of its bytes, and those
/// bytes are read back afterwards so a function that writes through a
/// pointer can be checked.
pub fn run(
    instructions: &[u32],
    call: &Call,
    facts: &[Facts],
    stack_pointer: &str,
) -> Result<Outcome, RecheckError> {
    if !crate::arm64_call::HOST_RUNS_ARM64 {
        return Err(RecheckError::WrongArchitecture);
    }
    crate::runnable::check_runnable(facts, stack_pointer)?;
    if call.arguments.len() > crate::arm64_call::HOST_ARGUMENT_REGISTERS {
        return Err(RecheckError::Unavailable(
            "more arguments than this host places in registers".to_string(),
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
    let mut registers = [0u64; crate::arm64_call::HOST_ARGUMENT_REGISTERS];
    let placed = arguments.len().min(registers.len());
    registers[..placed].copy_from_slice(&arguments[..placed]);
    let returned = crate::arm64_call::call_with(page.address(), &registers);
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
