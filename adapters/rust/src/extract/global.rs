// A global the compiler saw: a const or static with its file, line, and
// value when the compiler could evaluate one. It never carries source text:
// a number needs no parsing and cannot be misread.
//
// A `const` has no value here on purpose. rustc inlines a const at its use
// site, where MIR carries it already evaluated (`MirConst::eval_target_usize`,
// rustc_public ty/tys.rs:185); stage 3 reads it from the comparison itself.

use serde::Serialize;

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct Global {
    pub path: String,
    pub start_line: u32,
    pub value: Option<u128>,
}
