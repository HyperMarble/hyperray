// Place tracing preserves projections and unwraps the value field of checked
// arithmetic. It must not treat the overflow field as the arithmetic value.

use super::{definition, local, rvalue, Expression};
use crate::mir::{Body, Input, Place, Projection, Rvalue};

pub fn resolve(
    body: &Body,
    inputs: &[Input],
    block: usize,
    before: usize,
    place: &Place,
    active: &mut Vec<usize>,
) -> Expression {
    if place.projection == [Projection::Field { index: 0 }] {
        if let Some(result) = checked_value(body, inputs, block, before, place.local, active) {
            return result;
        }
    }
    let base = local::resolve(body, inputs, block, before, place.local, active);
    match place.projection.is_empty() {
        true => base,
        false => Expression::Projection {
            value: Box::new(base),
            projection: place.projection.clone(),
        },
    }
}

fn checked_value(
    body: &Body,
    inputs: &[Input],
    block: usize,
    before: usize,
    local: usize,
    active: &mut Vec<usize>,
) -> Option<Expression> {
    let found = definition::reaching(body, block, before, local)?;
    let Rvalue::Binary { checked: true, .. } = &found.statement.value else {
        return None;
    };
    Some(rvalue::resolve(
        body,
        inputs,
        found.block,
        found.index,
        &found.statement.value,
        active,
    ))
}
