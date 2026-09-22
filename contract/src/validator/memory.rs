// Purpose: Validate mem section shapes and expose address and size expressions.
// Never: Guess memory access or treat declared access as observed execution.
// In: Ordered mem sections and their first document index.
// Out: Memory expressions for type validation and proof construction.
// Fails: none conflicts, access words, byte markers, or arity are invalid.

use crate::{Expression, Section, ValidationFailure};

use super::term;

pub(crate) struct MemoryUse<'a> {
    pub index: usize,
    pub address: &'a Expression,
    pub size: &'a Expression,
}

pub(crate) fn check(
    start: usize,
    sections: &[Section],
) -> Result<Vec<MemoryUse<'_>>, ValidationFailure> {
    if is_none(start, sections)? {
        return Ok(Vec::new());
    }
    sections
        .iter()
        .enumerate()
        .map(|(offset, section)| access(start + offset, section))
        .collect()
}

fn is_none(start: usize, sections: &[Section]) -> Result<bool, ValidationFailure> {
    let [section] = sections else {
        return Ok(false);
    };
    let [value] = section.values.as_slice() else {
        return Ok(false);
    };
    Ok(term::symbol(value, start, "memory access or `none`")? == "none")
}

fn access(index: usize, section: &Section) -> Result<MemoryUse<'_>, ValidationFailure> {
    let [address, access, size, bytes] = section.values.as_slice() else {
        return Err(rejected(
            index,
            "mem access requires address, access, size, and bytes",
        ));
    };
    let access = term::symbol(access, index, "`read` or `write`")?;
    if access != "read" && access != "write" {
        return Err(rejected(index, "mem access must be `read` or `write`"));
    }
    if term::symbol(bytes, index, "`bytes`")? != "bytes" {
        return Err(rejected(index, "mem size requires the `bytes` marker"));
    }
    Ok(MemoryUse {
        index,
        address,
        size,
    })
}

fn rejected(index: usize, message: &str) -> ValidationFailure {
    ValidationFailure::rejected(Some(index), message)
}
