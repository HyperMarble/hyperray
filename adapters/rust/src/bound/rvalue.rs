// Rvalue tracing preserves arithmetic, lengths, and address projections.
// Unsupported forms must return an explicit unknown expression.

use super::{operand, place_trace, Expression};
use crate::mir::{Body, Input, Rvalue};

pub fn resolve(
    body: &Body,
    inputs: &[Input],
    block: usize,
    before: usize,
    value: &Rvalue,
    active: &mut Vec<usize>,
) -> Expression {
    match value {
        Rvalue::Use { operand: value } | Rvalue::Cast { operand: value } => {
            operand::resolve(body, inputs, block, before, value, active)
        }
        Rvalue::Binary {
            operation,
            left,
            right,
            checked,
        } => Expression::Binary {
            operation: *operation,
            left: Box::new(operand::resolve(body, inputs, block, before, left, active)),
            right: Box::new(operand::resolve(body, inputs, block, before, right, active)),
            checked: *checked,
        },
        Rvalue::Length { place } => Expression::Length {
            value: Box::new(place_trace::resolve(
                body, inputs, block, before, place, active,
            )),
        },
        Rvalue::Metadata { operand: value } => Expression::Length {
            value: Box::new(operand::resolve(body, inputs, block, before, value, active)),
        },
        Rvalue::Address { place } => {
            place_trace::resolve(body, inputs, block, before, place, active)
        }
        Rvalue::Unary { operation, .. } => {
            unknown(format!("unary operation {operation} is not traced"))
        }
        Rvalue::Discriminant { .. } => unknown("enum discriminant is not a linear limit"),
        Rvalue::Aggregate { .. } => unknown("aggregate value is not a linear limit"),
        Rvalue::Other => unknown("compiler value is not represented"),
    }
}

fn unknown(reason: impl Into<String>) -> Expression {
    Expression::Unknown {
        reason: reason.into(),
    }
}
