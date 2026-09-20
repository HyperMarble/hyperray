// What a proof run answers, and whether the answer covers the code.
// A run that did not cover the code must never be read as a verdict.

/// The answer to one claim.
#[derive(Debug, PartialEq)]
pub enum Verdict {
    /// No input violates the claim.
    Proved,
    /// At least one input violates the claim, and the witness names it.
    Disproved { witness: Witness },
}

/// The inputs that break a claim.
///
/// The values are carried out so they can be run on the real instructions,
/// which checks the answer without trusting the solver that produced it.
#[derive(Debug, PartialEq)]
pub struct Witness {
    pub registers: Vec<(String, u64)>,
}

/// Whether a run reached every instruction it was given.
#[derive(Debug, PartialEq)]
pub enum Coverage {
    Complete,
    /// The run stopped early, so its answer describes part of the code.
    Incomplete { reason: String },
}

/// Why a run produced no usable answer.
#[derive(Debug, PartialEq)]
pub enum SolverError {
    /// The run stopped before it covered the code.
    NotCovered { reason: String },
    /// The engine failed rather than answering.
    Failed(String),
    /// The output holds no answer this reader understands.
    Unreadable(String),
}
