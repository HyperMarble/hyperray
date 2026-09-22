// Purpose: Check one or more contract expressions against one required SMT sort.
// Never: Accept an expression outside `.hray` or hide a missing reported sort.
// In: A type context, indexed expressions, and the required result sort.
// Out: Confirmation that every expression has the required legal meaning.
// Fails: Language, type, registry, or result-sort validation fails.

use crate::{Expression, ValidationFailure};
use yaspar_ir::ast::{Context, FetchSort, Sort, Typecheck};

use super::expression_language;

pub(crate) fn check_many(
    context: &mut Context,
    expressions: &[(usize, &Expression)],
    expected: &Sort,
    message: &str,
) -> Result<(), ValidationFailure> {
    for (index, expression) in expressions {
        check(context, *index, expression, expected, message)?;
    }
    Ok(())
}

pub(crate) fn check(
    context: &mut Context,
    index: usize,
    expression: &Expression,
    expected: &Sort,
    message: &str,
) -> Result<(), ValidationFailure> {
    expression_language::check(index, expression)?;
    let typed = expression.type_check(context).map_err(|error| {
        ValidationFailure::rejected(Some(index), format!("invalid expression: {error}"))
    })?;
    let actual = typed.maybe_sort(context).ok_or_else(|| {
        ValidationFailure::engine(Some(index), "typed expression has no reported sort")
    })?;
    if &actual != expected {
        return Err(ValidationFailure::rejected(Some(index), message));
    }
    Ok(())
}
