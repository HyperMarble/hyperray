// Purpose: Coordinate complete `.hray` validation against upstream facts.
// Never: Parse source, infer missing metadata, or duplicate stage decisions.
// In: One parsed contract and compiler, model, and OS facts.
// Out: A validated contract with its exact selected function.
// Fails: Any structure, metadata, registry, type, or OS check fails.

mod expression;
mod expression_language;
mod expression_memory;
mod expression_term;
mod facts;
mod function;
mod input;
mod input_argument;
mod memory;
mod model_registry;
mod numbered_name;
mod os;
mod os_fact;
mod outcome;
mod output;
mod registry;
mod requirement;
mod sections;
mod sort;
mod term;

pub use facts::*;
pub use outcome::*;

use crate::Contract;

pub fn validate(
    contract: Contract,
    facts: &ValidationFacts,
) -> Result<ValidatedContract, ValidationFailure> {
    let groups = sections::group(&contract)?;
    let function = function::select(groups.function.0, groups.function.1, facts)?;
    let guards = input::check(groups.inputs, &function)?;
    output::check(groups.output.0, groups.output.1, &function)?;
    let memory_start = groups.output.0 + 1;
    let memory = memory::check(memory_start, groups.memory)?;
    let os_start = memory_start + groups.memory.len();
    let os_operations = os::check(os_start, groups.os, facts)?;
    let requirement_start = os_start + groups.os.len();
    let requirements = requirement::collect(requirement_start, groups.requirements)?;
    expression::check(&guards, &memory, &requirements, &function, facts)?;
    Ok(ValidatedContract {
        contract,
        function,
        os_operations,
    })
}
