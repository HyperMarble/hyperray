// SHAPE: the compiler measures every function against the rules, and any
// reshaped function is accepted only with a proof that it equals the
// original. Nothing here reads source text to judge it.

mod clippy;
mod diagnostic;
mod finding;

pub use clippy::{run, Run, MAX_LINES, MAX_NESTING};
pub use finding::{findings_in, Finding, LINTS};
