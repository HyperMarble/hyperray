// Purpose: Select one compiler function and require its machine locations.
// Never: Infer an ABI location or select an ambiguous compiler record.
// In: One fn section and the complete compiler function inventory.
// Out: The exact selected function fact.
// Fails: The name is absent, ambiguous, malformed, or lacks a location.

use crate::{FunctionFact, Section, ValidationFacts, ValidationFailure};

use super::term;

pub(crate) fn select(
    index: usize,
    section: &Section,
    facts: &ValidationFacts,
) -> Result<FunctionFact, ValidationFailure> {
    let value = term::only_value(&section.values, index)?;
    let name = term::symbol(value, index, "compiler function name")?;
    if facts.functions.is_empty() {
        return Err(ValidationFailure::engine(
            None,
            "compiler function inventory is absent",
        ));
    }
    let matches = facts
        .functions
        .iter()
        .filter(|function| function.name == name)
        .collect::<Vec<_>>();
    let function = match matches.as_slice() {
        [function] => (*function).clone(),
        [] => return Err(rejected(index, "function is absent from compiler metadata")),
        _ => {
            return Err(ValidationFailure::engine(
                None,
                "compiler function fact is ambiguous",
            ))
        }
    };
    require_locations(&function)?;
    Ok(function)
}

fn require_locations(function: &FunctionFact) -> Result<(), ValidationFailure> {
    let missing_argument = function
        .arguments
        .iter()
        .any(|value| value.location.as_deref().is_none_or(str::is_empty));
    let missing_result = function
        .result
        .as_ref()
        .is_some_and(|value| value.location.as_deref().is_none_or(str::is_empty));
    if missing_argument || missing_result {
        return Err(ValidationFailure::engine(
            None,
            "compiler machine location is absent",
        ));
    }
    Ok(())
}

fn rejected(index: usize, message: &str) -> ValidationFailure {
    ValidationFailure::rejected(Some(index), message)
}
