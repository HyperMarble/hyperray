// Purpose: Convert fact-provided SMT sort text with Yaspar.
// Never: Invent a type or accept malformed compiler and model metadata.
// In: One public Yaspar context and one upstream sort string.
// Out: One well-formed Yaspar sort.
// Fails: Sort syntax or meaning is unsupported by the selected logic.

use crate::ValidationFailure;
use yaspar_ir::ast::{Context, Sort, Typecheck};
use yaspar_ir::untyped::UntypedAst;

pub(crate) fn parse(context: &mut Context, source: &str) -> Result<Sort, ValidationFailure> {
    let untyped = UntypedAst.parse_sort_str(source).map_err(|error| {
        ValidationFailure::engine(None, format!("invalid upstream sort `{source}`: {error}"))
    })?;
    untyped.type_check(context).map_err(|error| {
        ValidationFailure::engine(
            None,
            format!("unsupported upstream sort `{source}`: {error}"),
        )
    })
}
