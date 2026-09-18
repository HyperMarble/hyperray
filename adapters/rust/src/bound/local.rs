// Local tracing follows one unambiguous definition or call result. A cycle
// or competing definitions remains a local expression.

use super::{call, definition, rvalue, Expression};
use crate::mir::{Body, Input};

pub fn resolve(
    body: &Body,
    inputs: &[Input],
    block: usize,
    before: usize,
    local: usize,
    active: &mut Vec<usize>,
) -> Expression {
    if let Some(input) = inputs.iter().find(|input| input.local == local) {
        return Expression::Input {
            index: input.index,
            local,
        };
    }
    if active.contains(&local) {
        return Expression::Local { local };
    }
    active.push(local);
    let resolved = definition::reaching(body, block, before, local)
        .map(|found| {
            rvalue::resolve(
                body,
                inputs,
                found.block,
                found.index,
                &found.statement.value,
                active,
            )
        })
        .or_else(|| call::resolve(body, inputs, local, active));
    let result = match resolved {
        Some(value) => value,
        None => Expression::Local { local },
    };
    active.pop();
    result
}
