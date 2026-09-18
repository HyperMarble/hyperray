// SHAPE runs the target crate's Clippy policy. It never supplies style rules.

mod clippy;
mod diagnostic;
mod finding;

pub use clippy::{run, Run};
pub use finding::{findings_in, Finding};
