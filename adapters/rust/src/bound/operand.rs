// Operand tracing keeps scalar values and follows places through assignments.
// Unsupported compiler operands must stay unknown.

use super::{place_trace, Expression};
use crate::mir::{Body, Input, Operand};

pub fn resolve(
    body: &Body,
    inputs: &[Input],
    block: usize,
    before: usize,
    value: &Operand,
    active: &mut Vec<usize>,
) -> Expression {
    match value {
        Operand::Place(place) => place_trace::resolve(body, inputs, block, before, place, active),
        Operand::Scalar(value) => Expression::Scalar {
            value: value.clone(),
        },
        Operand::Function(name) => Expression::Unknown {
            reason: format!("function value {name} is not a loop limit"),
        },
        Operand::Other => Expression::Unknown {
            reason: "compiler operand is not represented".to_string(),
        },
    }
}
