// Purpose: Match the out section to the compiler-defined result.
// Never: Infer a result or machine location from contract text.
// In: One out section and the selected function result fact.
// Out: Confirmation that none or ret matches compiler metadata.
// Fails: The section shape or result presence conflicts with compiler facts.

use crate::{FunctionFact, Section, ValidationFailure};

use super::term;

pub(crate) fn check(
    index: usize,
    section: &Section,
    function: &FunctionFact,
) -> Result<(), ValidationFailure> {
    let value = term::only_value(&section.values, index)?;
    let name = term::symbol(value, index, "`none` or `ret`")?;
    let matches = matches!(
        (name.as_str(), &function.result),
        ("none", None) | ("ret", Some(_))
    );
    if !matches {
        return Err(ValidationFailure::rejected(
            Some(index),
            "out section conflicts with compiler result metadata",
        ));
    }
    Ok(())
}
