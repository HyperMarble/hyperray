// Operand diagnostics resolve compiler definitions.
// Ambiguous definitions must remain explicit local expressions.

use super::operand as operand_trace;
use super::Expression;
use crate::mir::{Body, Input, Operand};

pub fn operand(body: &Body, inputs: &[Input], block: usize, value: &Operand) -> Expression {
    let mut active = Vec::new();
    operand_trace::resolve(
        body,
        inputs,
        block,
        body.blocks[block].statements.len(),
        value,
        &mut active,
    )
}
