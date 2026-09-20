// Purpose: decides whether instructions can be run with what the caller placed.
// Never:   decodes an instruction, which the model already describes.
// In:      the model's facts, and the name it gives the stack pointer
// Out:     nothing when the code can be run
// Fails:   no instructions, no return, memory the caller did not place
use crate::witness::RecheckError;
use solver::{reads_caller_memory, returns, Facts};

/// Reports whether these instructions can be run with what the caller placed.
///
/// The model says what each instruction does, so neither the return nor the
/// stack read is decoded here.
pub fn check_runnable(facts: &[Facts], stack_pointer: &str) -> Result<(), RecheckError> {
    let Some(last) = facts.last() else {
        return Err(RecheckError::Unavailable("no instructions to run".to_string()));
    };
    if !returns(last) {
        return Err(RecheckError::Unavailable("the code does not return".to_string()));
    }
    if facts.iter().any(|fact| reads_caller_memory(fact, stack_pointer)) {
        return Err(RecheckError::Unavailable(
            "the function reads memory this rechecker did not place".to_string(),
        ));
    }
    Ok(())
}
