// Purpose: Require one expression in every req section.
// Never: Evaluate, weaken, or combine requirement meaning during collection.
// In: Ordered req sections and their first document index.
// Out: Indexed expressions for type validation and later proof construction.
// Fails: A requirement section has zero or multiple values.

use crate::{Expression, Section, ValidationFailure};

use super::term;

pub(crate) fn collect(
    start: usize,
    sections: &[Section],
) -> Result<Vec<(usize, &Expression)>, ValidationFailure> {
    sections
        .iter()
        .enumerate()
        .map(|(offset, section)| {
            let index = start + offset;
            term::only_value(&section.values, index).map(|expression| (index, expression))
        })
        .collect()
}
