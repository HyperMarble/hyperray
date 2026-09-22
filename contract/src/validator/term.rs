// Purpose: Read fixed `.hray` words and names from Yaspar term nodes.
// Never: Reparse rendered SMT text or accept indexed and qualified identifiers.
// In: One parsed expression and its owning section index.
// Out: One simple symbol or an exact contract rejection.
// Fails: The expression is not a simple symbol.

use crate::{Expression, ValidationFailure};
use yaspar_ir::ast::alg::Term as ExpressionKind;

pub(crate) fn symbol(
    expression: &Expression,
    section_index: usize,
    expected: &str,
) -> Result<String, ValidationFailure> {
    let ExpressionKind::Global(identifier, _) = &***expression else {
        return Err(ValidationFailure::rejected(
            Some(section_index),
            format!("expected {expected}"),
        ));
    };
    if !identifier.0.indices.is_empty() || identifier.1.is_some() {
        return Err(ValidationFailure::rejected(
            Some(section_index),
            format!("expected simple {expected}"),
        ));
    }
    Ok(identifier.0.symbol.as_str().to_owned())
}

pub(crate) fn only_value(
    values: &[Expression],
    section_index: usize,
) -> Result<&Expression, ValidationFailure> {
    match values {
        [value] => Ok(value),
        _ => Err(ValidationFailure::rejected(
            Some(section_index),
            "section requires exactly one value",
        )),
    }
}
