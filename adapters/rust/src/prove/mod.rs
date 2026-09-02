// PROVE: run the prover over a crate and read its verdicts back. Used by
// SHAPE to prove old = new, and by stage 4 for every function.

mod auto;
mod kani;
mod report;
mod skip;

pub use auto::{auto_in, Auto};
pub use kani::{run, version, Run, Scope};
pub use report::{results_in, HarnessResult};
pub use skip::Skipped;
