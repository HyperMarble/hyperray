// SHAPE: the compiler measures every changed function against the rules and
// reports each break with its file and line. Nothing here reads source text
// to judge it, and nothing is ever split or rewritten (design.md §4).

mod clippy;
mod diagnostic;
mod finding;

pub use clippy::{run, Run, MAX_LINES, MAX_NESTING};
pub use finding::{findings_in, Finding, LINTS};
