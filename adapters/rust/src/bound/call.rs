// Call tracing retains the compiler function name and resolved arguments.
// More than one call definition must remain ambiguous.

use super::{operand, Expression};
use crate::mir::{Body, Input, Terminator};

pub fn resolve(
    body: &Body,
    inputs: &[Input],
    local: usize,
    active: &mut Vec<usize>,
) -> Option<Expression> {
    let (
        block,
        Terminator::Call {
            function,
            arguments,
            ..
        },
    ) = unique(body, local)?
    else {
        return None;
    };
    let before = body.blocks[block].statements.len();
    Some(Expression::Call {
        function: function.clone(),
        arguments: arguments
            .iter()
            .map(|value| operand::resolve(body, inputs, block, before, value, active))
            .collect(),
    })
}

fn unique(body: &Body, local: usize) -> Option<(usize, &Terminator)> {
    let found: Vec<(usize, &Terminator)> = body
        .blocks
        .iter()
        .enumerate()
        .filter_map(|(block, data)| match &data.terminator {
            Terminator::Call { destination, .. }
                if destination.local == local && destination.projection.is_empty() =>
            {
                Some((block, &data.terminator))
            }
            Terminator::Goto { .. }
            | Terminator::Switch { .. }
            | Terminator::Call { .. }
            | Terminator::Assert { .. }
            | Terminator::Drop { .. }
            | Terminator::InlineAsm { .. }
            | Terminator::End => None,
        })
        .collect();
    match found.as_slice() {
        [call] => Some(*call),
        _ => None,
    }
}
