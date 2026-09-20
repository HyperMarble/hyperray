// Purpose: reads a proof answer and what the model says an instruction does.
// Never:   returns an answer for a run that did not cover the code.
// In:      nothing, this file only names the parts
// Out:     the verdict, coverage, and instruction facts
// Fails:   not applicable
pub mod answer;
pub mod coverage;
pub mod facts;
pub mod footprint;
pub mod verdict;

pub use answer::read_answer;
pub use coverage::read_coverage;
pub use facts::{reads_caller_memory, returns, Facts, FactsError};
pub use verdict::{Coverage, SolverError, Verdict, Witness};
