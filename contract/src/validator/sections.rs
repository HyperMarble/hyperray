// Purpose: Divide parsed sections by the fixed `.hray` order.
// Never: Interpret section contents or silently accept an absent section group.
// In: Parsed direct children of the `hray` root.
// Out: Indexed references for fn, in, out, mem, os, and req.
// Fails: A section is absent, misplaced, duplicated, or unknown.

use crate::{Contract, Section, ValidationFailure};

pub(crate) struct Groups<'a> {
    pub function: (usize, &'a Section),
    pub inputs: &'a [Section],
    pub output: (usize, &'a Section),
    pub memory: &'a [Section],
    pub os: &'a [Section],
    pub requirements: &'a [Section],
}

pub(crate) fn group(contract: &Contract) -> Result<Groups<'_>, ValidationFailure> {
    let sections = &contract.sections;
    let function = one(sections, 0, "fn")?;
    let input_end = repeated(sections, 1, "in")?;
    let output = one(sections, input_end, "out")?;
    let memory_start = input_end + 1;
    let memory_end = repeated(sections, memory_start, "mem")?;
    let os_end = repeated(sections, memory_end, "os")?;
    let req_end = repeated(sections, os_end, "req")?;
    if req_end != sections.len() {
        return Err(failure(req_end, "unknown or misplaced section"));
    }
    Ok(Groups {
        function,
        inputs: &sections[1..input_end],
        output,
        memory: &sections[memory_start..memory_end],
        os: &sections[memory_end..os_end],
        requirements: &sections[os_end..req_end],
    })
}

fn one<'a>(
    sections: &'a [Section],
    index: usize,
    name: &str,
) -> Result<(usize, &'a Section), ValidationFailure> {
    match sections.get(index) {
        Some(section) if section.name == name => Ok((index + 1, section)),
        _ => Err(failure(index, &format!("expected `{name}` section"))),
    }
}

fn repeated(sections: &[Section], start: usize, name: &str) -> Result<usize, ValidationFailure> {
    let count = sections[start..]
        .iter()
        .take_while(|section| section.name == name)
        .count();
    if count == 0 {
        return Err(failure(start, &format!("expected `{name}` section")));
    }
    Ok(start + count)
}

fn failure(index: usize, message: &str) -> ValidationFailure {
    ValidationFailure::rejected(Some(index + 1), message)
}
