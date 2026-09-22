// Purpose: Type-check declared memory address and size expressions.
// Never: Read address, size, or memory sorts when no memory access is declared.
// In: Pre-state context, declared memory expressions, and memory sort facts.
// Out: Confirmation that each address and size has its required sort.
// Fails: A required sort is absent or an expression has the wrong sort.

use crate::{ValidationFacts, ValidationFailure};
use yaspar_ir::ast::Context;

use super::memory::MemoryUse;
use super::{expression_term, sort};

pub(crate) fn check(
    context: &mut Context,
    memory: &[MemoryUse<'_>],
    facts: &ValidationFacts,
) -> Result<(), ValidationFailure> {
    if memory.is_empty() {
        return Ok(());
    }
    let address_sort = sort::parse(context, &facts.address_sort)?;
    let size_sort = sort::parse(context, &facts.size_sort)?;
    for access in memory {
        expression_term::check(
            context,
            access.index,
            access.address,
            &address_sort,
            "wrong address type",
        )?;
        expression_term::check(
            context,
            access.index,
            access.size,
            &size_sort,
            "wrong size type",
        )?;
    }
    Ok(())
}
