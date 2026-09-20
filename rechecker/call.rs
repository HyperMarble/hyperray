// The inputs a function needs before the processor can run it.
// A value the caller did not name must never be replaced by a guess.

/// The bytes a function reads or writes through a pointer.
///
/// A counterexample names the memory it read, so that memory is supplied
/// rather than left to whatever happened to be there.
#[derive(Debug, Clone, PartialEq)]
pub struct Buffer {
    /// Which argument holds the address of these bytes.
    pub argument: usize,
    pub bytes: Vec<u8>,
}

/// Everything one call needs.
///
/// The arguments are the values the witness named, in architecture order.
#[derive(Debug, Clone, PartialEq)]
pub struct Call {
    pub arguments: Vec<u64>,
    pub buffers: Vec<Buffer>,
}

/// What the processor produced.
#[derive(Debug, Clone, PartialEq)]
pub struct Outcome {
    pub returned: u64,
    /// The buffers after the call, in the order they were supplied, so a
    /// function that writes through a pointer can be checked.
    pub buffers: Vec<Vec<u8>>,
}

#[derive(Debug, PartialEq)]
pub enum CallError {
    /// A buffer names an argument the call does not have.
    BufferWithoutArgument { argument: usize },
    /// Two buffers name the same argument.
    RepeatedArgument { argument: usize },
}

/// Reports whether a call can be made as described.
pub fn validate(call: &Call) -> Result<(), CallError> {
    let mut named: Vec<usize> = Vec::new();
    for buffer in &call.buffers {
        if buffer.argument >= call.arguments.len() {
            return Err(CallError::BufferWithoutArgument { argument: buffer.argument });
        }
        if named.contains(&buffer.argument) {
            return Err(CallError::RepeatedArgument { argument: buffer.argument });
        }
        named.push(buffer.argument);
    }
    Ok(())
}
