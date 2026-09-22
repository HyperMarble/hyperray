// Purpose: Match in sections to every compiler-defined argument exactly once.
// Never: Treat a selected value as universal or allow post-state input roles.
// In: Ordered in sections and the selected function arguments.
// Out: Input guard expressions for later type validation.
// Fails: none, arguments, all, or when sections conflict with compiler facts.

use crate::{Expression, FunctionFact, Section, ValidationFailure};

use super::{input_argument, term};

pub(crate) fn check<'a>(
    sections: &'a [Section],
    function: &FunctionFact,
) -> Result<Vec<(usize, &'a Expression)>, ValidationFailure> {
    if function.arguments.is_empty() {
        return no_arguments(sections);
    }
    let mut seen = vec![false; function.arguments.len()];
    let mut guards = Vec::new();
    let mut guard_started = false;
    for (offset, section) in sections.iter().enumerate() {
        let index = offset + 2;
        let first = first_symbol(section, index)?;
        if first == "when" {
            guard_started = true;
            guards.push(guard(section, index)?);
            continue;
        }
        if guard_started {
            return Err(rejected(index, "argument appears after an input rule"));
        }
        input_argument::mark(section, index, &first, &mut seen)?;
    }
    if seen.iter().any(|present| !present) {
        return Err(rejected(2, "compiler argument is missing"));
    }
    Ok(guards)
}

fn no_arguments(sections: &[Section]) -> Result<Vec<(usize, &Expression)>, ValidationFailure> {
    if sections.len() != 1 || sections[0].values.len() != 1 {
        return Err(rejected(2, "input-free function requires `(in none)`"));
    }
    let name = term::symbol(&sections[0].values[0], 2, "`none`")?;
    if name != "none" {
        return Err(rejected(2, "input-free function requires `(in none)`"));
    }
    Ok(Vec::new())
}

fn first_symbol(section: &Section, index: usize) -> Result<String, ValidationFailure> {
    let first = section
        .values
        .first()
        .ok_or_else(|| rejected(index, "in section is empty"))?;
    term::symbol(first, index, "argument, `when`, or `none`")
}

fn guard(section: &Section, index: usize) -> Result<(usize, &Expression), ValidationFailure> {
    match section.values.as_slice() {
        [_, expression] => Ok((index, expression)),
        _ => Err(rejected(index, "`in when` requires one expression")),
    }
}

fn rejected(index: usize, message: &str) -> ValidationFailure {
    ValidationFailure::rejected(Some(index), message)
}
