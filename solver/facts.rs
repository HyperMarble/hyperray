// Purpose: what an instruction does, as the architecture model reports it.
// Never:   decodes an instruction by hand.
// In:      a Facts value the model produced
// Out:     whether the instruction returns, or reads the caller's memory
// Fails:   not applicable, the model is asked elsewhere

/// What one instruction reads, writes, and whether it leaves the function.
///
/// These come from the architecture model, which is translated from the
/// vendor's own specification. A mask written here instead would be a second
/// opinion about the same instruction, and the two can disagree.
#[derive(Debug, Clone, PartialEq)]
pub struct Facts {
    /// The registers the instruction reads, by the model's own names.
    pub register_reads: Vec<String>,
    /// The registers the instruction writes.
    pub register_writes: Vec<String>,
    pub is_load: bool,
    pub is_store: bool,
    pub is_branch: bool,
}

#[derive(Debug, PartialEq)]
pub enum FactsError {
    /// The model does not describe this instruction.
    Unknown { opcode: u32 },
    /// The model could not be asked.
    Unavailable(String),
}

/// Reports whether the instruction returns from the function.
///
/// A return is a branch that writes no register, because it moves control
/// without producing a value. The model says both, so neither is decoded
/// here.
pub fn returns(facts: &Facts) -> bool {
    facts.is_branch && facts.register_writes.is_empty()
}

/// Reports whether the instruction reads memory the caller did not supply.
///
/// A load whose address comes from the stack pointer reads a value that was
/// placed by whoever called the function. A rechecker that did not place it
/// cannot run this instruction honestly.
pub fn reads_caller_memory(facts: &Facts, stack_pointer: &str) -> bool {
    facts.is_load && facts.register_reads.iter().any(|name| name == stack_pointer)
}
