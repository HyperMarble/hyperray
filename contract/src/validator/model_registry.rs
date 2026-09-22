// Purpose: Add model literals and operations to a Yaspar type context.
// Never: Copy a model operation list or infer a missing signature.
// In: Registry facts and a context using the selected logic.
// Out: Symbols with exact upstream signatures.
// Fails: A sort or symbol fact is invalid.

use crate::{OperationFact, ValidationFacts, ValidationFailure};
use yaspar_ir::ast::{Context, Sig};

use super::{registry, sort};

pub(crate) fn add(context: &mut Context, facts: &ValidationFacts) -> Result<(), ValidationFailure> {
    for literal in &facts.literals {
        registry::add_sort_symbol(context, &literal.name, &literal.sort)?;
    }
    for operation in &facts.operations {
        add_operation(context, operation)?;
    }
    Ok(())
}

fn add_operation(
    context: &mut Context,
    operation: &OperationFact,
) -> Result<(), ValidationFailure> {
    let mut arguments = Vec::new();
    for source in &operation.arguments {
        arguments.push(sort::parse(context, source)?);
    }
    let result = sort::parse(context, &operation.result)?;
    context
        .extend_symbol(&operation.name, Sig::func(arguments, result))
        .map_err(|error| registry::engine_error(&operation.name, error))
}
