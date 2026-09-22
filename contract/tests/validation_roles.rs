// Purpose: Test compiler argument, output, and state-role validation.
// Never: Accept a missing role, duplicate input, or post-state input condition.
// In: Contracts that vary one role rule from a valid compiler fact set.
// Out: Exact rejection for each conflict and acceptance for every input value.
// Fails: Contract roles can disagree with compiler metadata.

mod support;

use support::{facts, validation_result, FailureKind};

fn failure_kind(source: &str) -> Option<FailureKind> {
    validation_result(source, &facts()).err()
}

#[test]
fn rejects_missing_duplicate_and_unknown_arguments() {
    let sources = [
        "(hray (fn advance) (in when true) (out ret) (mem none) (os none) (req true))",
        "(hray (fn advance) (in arg1 all) (in arg1 all) (out ret) (mem none) (os none) (req true))",
        "(hray (fn advance) (in arg2 all) (out ret) (mem none) (os none) (req true))",
    ];
    assert!(sources
        .iter()
        .all(|source| { failure_kind(source) == Some(FailureKind::Rejected) }));
}

#[test]
fn rejects_post_state_roles_in_input_rules() {
    let source = "(hray (fn advance) (in arg1 all) (in when (= ret arg1)) \
        (out ret) (mem none) (os none) (req true))";
    assert_eq!(failure_kind(source), Some(FailureKind::Rejected));
}

#[test]
fn rejects_output_that_conflicts_with_compiler_metadata() {
    let source = "(hray (fn advance) (in arg1 all) (out none) \
        (mem none) (os none) (req true))";
    assert_eq!(failure_kind(source), Some(FailureKind::Rejected));
}
