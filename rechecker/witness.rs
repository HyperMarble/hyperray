// Purpose: compares what the processor produced with what the solver named.
// Never:   hides a disagreement between the chip and the model.
// In:      a witness, and the value the processor produced
// Out:     Confirmed, or Contradicted with both values
// Fails:   the witness does not name a register the code reads
use solver::Witness;

#[derive(Debug, PartialEq)]
pub enum Recheck {
    /// The processor produced the value the solver named. The failure is real.
    Confirmed { produced: u64 },
    /// The processor produced something else. The solver and the chip disagree.
    Contradicted { produced: u64, expected: u64 },
}

#[derive(Debug, PartialEq)]
pub enum RecheckError {
    /// The witness names no value for a register the code reads.
    MissingRegister(String),
    /// The instructions cannot run on this processor.
    WrongArchitecture,
    /// The processor could not be asked.
    Unavailable(String),
}

/// Reads one register the witness named.
///
/// A proof holds for every input, so it cannot be rechecked. Only a
/// counterexample names the one case the processor can run.
pub fn named_value(witness: &Witness, register: &str) -> Result<u64, RecheckError> {
    witness
        .registers
        .iter()
        .find(|(name, _)| name == register)
        .map(|(_, value)| *value)
        .ok_or_else(|| RecheckError::MissingRegister(register.to_string()))
}

/// Compares what the processor produced with what the solver expected.
///
/// The processor knows nothing of the model, so agreement here is evidence
/// the model described the instruction correctly.
pub fn compare(produced: u64, expected: u64) -> Recheck {
    if produced == expected {
        Recheck::Confirmed { produced }
    } else {
        Recheck::Contradicted { produced, expected }
    }
}
