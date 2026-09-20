// Purpose: decides whether a proof run reached the end of the code.
// Never:   reports complete coverage for a run that hit its limit.
// In:      the engine's output
// Out:     Complete, or Incomplete with the reason the engine gave
// Fails:   not applicable, an unrecognised run is incomplete
use crate::verdict::Coverage;

/// The engine prints this when it stops at the instruction limit.
const LIMIT_MESSAGE: &str = "more than specified limit";

/// Reports whether the run reached the end of the code.
///
/// The engine writes `Never 0 0` when it gives up at the limit and
/// `Never 0 1` when it found a counterexample. The verdict line alone does
/// not separate the two, so the reason is read from the rest of the output.
pub fn read_coverage(output: &str) -> Coverage {
    if let Some(reason) = limit_reason(output) {
        return Coverage::Incomplete { reason };
    }
    if output.contains("Error") {
        return Coverage::Incomplete { reason: "the engine reported an error".to_string() };
    }
    Coverage::Complete
}

fn limit_reason(output: &str) -> Option<String> {
    output
        .lines()
        .find(|line| line.contains(LIMIT_MESSAGE))
        .map(|line| line.trim().to_string())
}
