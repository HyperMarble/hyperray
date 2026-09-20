// Purpose: reads the engine's answer and the inputs that break a claim.
// Never:   returns a verdict for a run that did not cover the code.
// In:      the engine's output
// Out:     Proved, or Disproved with the registers the engine named
// Fails:   the run stopped early, or the output holds no answer
use crate::coverage::read_coverage;
use crate::verdict::{Coverage, SolverError, Verdict, Witness};

/// Reads one answer from the engine's output.
pub fn read_answer(output: &str) -> Result<Verdict, SolverError> {
    match read_coverage(output) {
        Coverage::Incomplete { reason } => Err(SolverError::NotCovered { reason }),
        Coverage::Complete => read_covered_answer(output),
    }
}

/// The engine states the answer on its observation line.
///
/// `Always` means no execution violated the claim. `Sometimes` means some did
/// and some did not. `Never` means every execution violated it.
fn read_covered_answer(output: &str) -> Result<Verdict, SolverError> {
    let Some(line) = output.lines().find(|line| line.starts_with("Observation")) else {
        return Err(SolverError::Unreadable("no observation line".to_string()));
    };
    if line.contains("Always") {
        return Ok(Verdict::Proved);
    }
    if line.contains("Sometimes") || line.contains("Never") {
        return Ok(Verdict::Disproved { witness: read_witness(output) });
    }
    Err(SolverError::Unreadable(line.to_string()))
}

/// Collects the register values the engine printed for a failing state.
///
/// An empty witness is returned when the engine named no values, so the
/// caller sees that the answer arrived without one.
fn read_witness(output: &str) -> Witness {
    let mut registers = Vec::new();
    for line in output.lines() {
        if let Some(pair) = register_value(line) {
            registers.push(pair);
        }
    }
    Witness { registers }
}

/// Reads one `0:R4=#x0000000000000001;` state line.
fn register_value(line: &str) -> Option<(String, u64)> {
    let trimmed = line.trim().trim_end_matches(';');
    let (left, right) = trimmed.split_once('=')?;
    let name = left.split_once(':')?.1;
    let digits = right.trim().strip_prefix("#x")?;
    let value = u64::from_str_radix(digits, 16).ok()?;
    Some((name.to_string(), value))
}
