// Purpose: Match os sections to installed target-specific operation contracts.
// Never: Copy an interface operation list or assume missing behavior.
// In: Ordered os sections, selected target, and installed OS operation facts.
// Out: Confirmation that each named operation has a behavior contract.
// Fails: none conflicts or an interface, operation, target, or contract is absent.

use crate::OsOperationFact;
use crate::{Section, ValidationFacts, ValidationFailure};

use super::{os_fact, term};

pub(crate) fn check(
    start: usize,
    sections: &[Section],
    facts: &ValidationFacts,
) -> Result<Vec<OsOperationFact>, ValidationFailure> {
    if is_none(start, sections)? {
        return Ok(Vec::new());
    }
    let mut selected = Vec::new();
    for (offset, section) in sections.iter().enumerate() {
        selected.push(check_operation(start + offset, section, facts)?);
    }
    Ok(selected)
}

fn is_none(start: usize, sections: &[Section]) -> Result<bool, ValidationFailure> {
    let [section] = sections else {
        return Ok(false);
    };
    let [value] = section.values.as_slice() else {
        return Ok(false);
    };
    Ok(term::symbol(value, start, "OS operation or `none`")? == "none")
}

fn check_operation(
    index: usize,
    section: &Section,
    facts: &ValidationFacts,
) -> Result<OsOperationFact, ValidationFailure> {
    let [interface, operation] = section.values.as_slice() else {
        return Err(rejected(
            index,
            "os section requires interface and operation",
        ));
    };
    let interface = term::symbol(interface, index, "interface name")?;
    let operation = term::symbol(operation, index, "operation name")?;
    let fact = os_fact::select(
        index,
        &interface,
        &operation,
        &facts.target,
        &facts.os_operations,
    )?;
    if fact.model.is_none() {
        return Err(ValidationFailure::engine(
            Some(index),
            "OS behavior model is absent",
        ));
    }
    Ok(fact.clone())
}

fn rejected(index: usize, message: &str) -> ValidationFailure {
    ValidationFailure::rejected(Some(index), message)
}
