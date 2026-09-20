// Runs a proof and reports whether the answer covers the code.
// An answer that did not cover the code is an error, never a verdict.
pub mod answer;
pub mod coverage;
pub mod verdict;

pub use answer::read_answer;
pub use coverage::read_coverage;
pub use verdict::{Coverage, SolverError, Verdict, Witness};
