// Purpose: Match one in declaration to one compiler argument.
// Never: Accept a missing, duplicate, zero, or unknown argument number.
// In: One in section, its index, and the argument-presence table.
// Out: One newly marked compiler argument.
// Fails: The declaration is not `argN all` exactly once.

use crate::{Section, ValidationFailure};

use super::{numbered_name, term};

pub(crate) fn mark(
    section: &Section,
    index: usize,
    name: &str,
    seen: &mut [bool],
) -> Result<(), ValidationFailure> {
    if section.values.len() != 2 {
        return Err(rejected(index, "argument input requires `argN all`"));
    }
    let all = term::symbol(&section.values[1], index, "`all`")?;
    if all != "all" {
        return Err(rejected(index, "argument input requires `all`"));
    }
    if !numbered_name::has_positive_suffix(name, "arg") {
        return Err(rejected(index, "argument number does not exist"));
    }
    let number = name
        .strip_prefix("arg")
        .and_then(|value| value.parse::<usize>().ok());
    let slot = number
        .and_then(|value| value.checked_sub(1))
        .and_then(|value| seen.get_mut(value));
    let Some(slot) = slot else {
        return Err(rejected(index, "argument number does not exist"));
    };
    if std::mem::replace(slot, true) {
        return Err(rejected(index, "argument occurs more than once"));
    }
    Ok(())
}

fn rejected(index: usize, message: &str) -> ValidationFailure {
    ValidationFailure::rejected(Some(index), message)
}
