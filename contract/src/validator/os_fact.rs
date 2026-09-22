// Purpose: Select one installed OS operation for the compiled target.
// Never: Select by list order or accept absent and ambiguous upstream facts.
// In: A section index, interface, operation, target, and complete OS facts.
// Out: The one exact target-specific operation fact.
// Fails: Interface, operation, target, or uniqueness checks fail.

use crate::{OsOperationFact, ValidationFailure};

pub(crate) fn select<'a>(
    index: usize,
    interface: &str,
    operation: &str,
    target: &str,
    facts: &'a [OsOperationFact],
) -> Result<&'a OsOperationFact, ValidationFailure> {
    if !facts.iter().any(|fact| fact.interface == interface) {
        return Err(rejected(index, "OS interface is not installed"));
    }
    let operations = facts
        .iter()
        .filter(|fact| fact.interface == interface && fact.operation == operation);
    if operations.clone().next().is_none() {
        return Err(rejected(index, "operation is not part of the OS interface"));
    }
    let mut matches = operations.filter(|fact| fact.target == target);
    let Some(selected) = matches.next() else {
        return Err(rejected(
            index,
            "OS operation conflicts with the compiled target",
        ));
    };
    if matches.next().is_some() {
        return Err(ValidationFailure::engine(
            Some(index),
            "target OS operation fact is ambiguous",
        ));
    }
    Ok(selected)
}

fn rejected(index: usize, message: &str) -> ValidationFailure {
    ValidationFailure::rejected(Some(index), message)
}
