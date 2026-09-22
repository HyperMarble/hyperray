// Purpose: Test the validator through its public compiler-neutral facts.
// Never: Depend on one source language, ABI, register name, or selected fixture path.
// In: Parsed contracts plus compiler, model, and OS facts.
// Out: A validated contract, rejection, or exact engine error.
// Fails: Validation guesses a missing fact or accepts a conflicting contract.

mod support;

use contract::{parse, validate, ValidationFailure};
use support::facts;

fn source(function: &str) -> String {
    format!(
        "(hray (fn {function}) (in arg1 all) (out ret) (mem none) (os none) \
         (req (= ret (bvadd arg1 #x0000000000000001))))"
    )
}

#[test]
fn validates_from_language_neutral_facts() {
    let parsed = parse(&source("advance"));
    let result = parsed.ok().and_then(|value| validate(value, &facts()).ok());
    assert!(result.is_some());
}

#[test]
fn rejects_a_function_absent_from_the_compiler_inventory() {
    let parsed = parse(&source("missing"));
    let result = parsed
        .ok()
        .and_then(|value| validate(value, &facts()).err());
    assert!(matches!(result, Some(ValidationFailure::Rejected(_))));
}

#[test]
fn reports_engine_error_when_the_compiler_omits_a_machine_location() {
    let mut facts = facts();
    facts.functions[0].arguments[0].location = None;
    let parsed = parse(&source("advance"));
    let result = parsed.ok().and_then(|value| validate(value, &facts).err());
    assert!(matches!(result, Some(ValidationFailure::Engine(_))));
}

#[test]
fn reports_engine_error_for_ambiguous_compiler_function_facts() {
    let mut facts = facts();
    facts.functions.push(facts.functions[0].clone());
    let parsed = parse(&source("advance"));
    let result = parsed.ok().and_then(|value| validate(value, &facts).err());
    assert!(matches!(result, Some(ValidationFailure::Engine(_))));
}
