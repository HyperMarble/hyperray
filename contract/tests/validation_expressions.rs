// Purpose: Test requirement types and fact-provided literals and operations.
// Never: Accept an unknown symbol or copy model signatures into validator code.
// In: Requirement expressions and explicit model registry facts.
// Out: Acceptance for registered meaning and rejection for invalid meaning.
// Fails: Type or registry errors reach the solver.

mod support;

use contract::{LiteralFact, OperationFact};
use support::{facts, validation_result, FailureKind};

#[test]
fn rejects_non_boolean_and_unknown_requirements() {
    let sources = [
        "(hray (fn advance) (in arg1 all) (out ret) (mem none) (os none) (req arg1))",
        "(hray (fn advance) (in arg1 all) (out ret) (mem none) (os none) (req unknown))",
    ];
    assert!(sources
        .iter()
        .all(|source| { validation_result(source, &facts()) == Err(FailureKind::Rejected) }));
}

#[test]
fn accepts_fact_registered_literals_and_operations() {
    let source = "(hray (fn advance) (in arg1 all) (out ret) (mem none) (os none) \
        (req (is_expected expected_value)))";
    let mut facts = facts();
    facts.literals.push(LiteralFact {
        name: "expected_value".into(),
        sort: "(_ BitVec 64)".into(),
    });
    facts.operations.push(OperationFact {
        name: "is_expected".into(),
        arguments: vec!["(_ BitVec 64)".into()],
        result: "Bool".into(),
    });
    assert_eq!(validation_result(source, &facts), Ok(()));
}

#[test]
fn reports_invalid_upstream_sort_facts_as_engine_error() {
    let mut facts = facts();
    facts.functions[0].arguments[0].sort = "MissingSort".into();
    let source = "(hray (fn advance) (in arg1 all) (out ret) (mem none) \
        (os none) (req true))"
        .to_string();
    assert_eq!(validation_result(&source, &facts), Err(FailureKind::Engine));
}
