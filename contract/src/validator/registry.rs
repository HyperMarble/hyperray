// Purpose: Build Yaspar type contexts from compiler and model facts.
// Never: Copy operation signatures or create missing role types.
// In: Validation facts, selected function, memory use, and state phase.
// Out: A context containing only legal roles, literals, and operations.
// Fails: Logic, sort, symbol, or registry metadata is invalid.

use crate::{FunctionFact, ValidationFacts, ValidationFailure, ValueFact};
use yaspar_ir::ast::{Context, Sig};

use super::sort;

pub(crate) fn build(
    facts: &ValidationFacts,
    function: &FunctionFact,
    memory: bool,
    post_state: bool,
) -> Result<Context, ValidationFailure> {
    let mut context = Context::new();
    context
        .set_ctx_logic(&facts.logic)
        .map_err(|error| engine_error("logic", error))?;
    for (index, value) in function.arguments.iter().enumerate() {
        add_value(&mut context, &format!("arg{}", index + 1), value)?;
    }
    if post_state {
        if let Some(result) = &function.result {
            add_value(&mut context, "ret", result)?;
        }
    }
    if memory {
        add_sort_symbol(&mut context, "memory_before", &facts.memory_sort)?;
        if post_state {
            add_sort_symbol(&mut context, "memory_after", &facts.memory_sort)?;
        }
    }
    super::model_registry::add(&mut context, facts)?;
    Ok(context)
}

fn add_value(
    context: &mut Context,
    name: &str,
    value: &ValueFact,
) -> Result<(), ValidationFailure> {
    add_sort_symbol(context, name, &value.sort)
}

pub(crate) fn add_sort_symbol(
    context: &mut Context,
    name: &str,
    source: &str,
) -> Result<(), ValidationFailure> {
    let value_sort = sort::parse(context, source)?;
    context
        .add_symbol(name, Sig::sort(value_sort))
        .map_err(|error| engine_error(name, error))
}

pub(crate) fn engine_error(name: &str, error: impl std::fmt::Display) -> ValidationFailure {
    ValidationFailure::engine(None, format!("invalid upstream `{name}` fact: {error}"))
}
