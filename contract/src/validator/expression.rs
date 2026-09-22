// Purpose: Type-check contract expressions in their pre-state and post-state contexts.
// Never: Mix post-state roles into input rules or require unused memory facts.
// In: Input guards, memory expressions, requirements, and validation facts.
// Out: Confirmation that every expression is legal and has its required sort.
// Fails: Context construction or any expression check fails.

use crate::{Expression, FunctionFact, ValidationFacts, ValidationFailure};
use yaspar_ir::ast::ObjectAllocatorExt;

use super::memory::MemoryUse;
use super::{expression_memory, expression_term, registry};

pub(crate) fn check(
    guards: &[(usize, &Expression)],
    memory: &[MemoryUse<'_>],
    requirements: &[(usize, &Expression)],
    function: &FunctionFact,
    facts: &ValidationFacts,
) -> Result<(), ValidationFailure> {
    let has_memory = !memory.is_empty();
    let mut before = registry::build(facts, function, has_memory, false)?;
    let bool_sort = before.bool_sort();
    expression_term::check_many(
        &mut before,
        guards,
        &bool_sort,
        "input rule must have type Bool",
    )?;
    expression_memory::check(&mut before, memory, facts)?;
    let mut after = registry::build(facts, function, has_memory, true)?;
    let bool_sort = after.bool_sort();
    expression_term::check_many(
        &mut after,
        requirements,
        &bool_sort,
        "requirement must have type Bool",
    )
}
