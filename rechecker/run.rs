// Executes instruction bytes on this processor with the witness values.
// It must refuse to run bytes it was not built to execute.
use crate::witness::RecheckError;

/// This host executes ARM64 instructions, so only ARM64 bytes can be run.
#[cfg(target_arch = "aarch64")]
pub const HOST_RUNS_ARM64: bool = true;
#[cfg(not(target_arch = "aarch64"))]
pub const HOST_RUNS_ARM64: bool = false;

/// Runs one two-argument ARM64 function and returns what it produced.
///
/// The bytes come from the compiled program, so the processor executes what
/// shipped rather than a copy of it. The instructions must end in `ret`,
/// because control has to come back.
pub fn run_arm64(instructions: &[u32], x0: u64, x1: u64) -> Result<u64, RecheckError> {
    if !HOST_RUNS_ARM64 {
        return Err(RecheckError::WrongArchitecture);
    }
    if !ends_in_return(instructions) {
        return Err(RecheckError::Unavailable("the code does not return".to_string()));
    }
    execute(instructions, x0, x1)
}

/// ARM64 encodes `ret` as this exact word.
const RET: u32 = 0xd65f_03c0;

fn ends_in_return(instructions: &[u32]) -> bool {
    instructions.last() == Some(&RET)
}

#[cfg(target_arch = "aarch64")]
fn execute(instructions: &[u32], x0: u64, x1: u64) -> Result<u64, RecheckError> {
    let page = crate::page::executable_page(instructions)?;
    // The bytes are the compiled function, so calling them runs what shipped.
    let function: extern "C" fn(u64, u64) -> u64 = unsafe { std::mem::transmute(page.address()) };
    Ok(function(x0, x1))
}

#[cfg(not(target_arch = "aarch64"))]
fn execute(_instructions: &[u32], _x0: u64, _x1: u64) -> Result<u64, RecheckError> {
    Err(RecheckError::WrongArchitecture)
}
