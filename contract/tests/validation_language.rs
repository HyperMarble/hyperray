// Purpose: Test the documented `.hray` expression boundary.
// Never: Treat every SMT-LIB term as a legal contract expression.
// In: Valid SMT terms that are inside or outside the `.hray` subset.
// Out: Acceptance only for documented literals, roles, applications, and quantifiers.
// Fails: An unsupported SMT term or invalid numbered name reaches proof construction.

mod support;

use contract::{parse, validate, ValidationFailure};
use support::{facts, validation_result, FailureKind};

fn result(requirement: &str) -> Result<(), FailureKind> {
    let source = format!(
        "(hray (fn advance) (in arg1 all) (out ret) (mem none) (os none) \
         (req {requirement}))"
    );
    validation_result(&source, &facts())
}

#[test]
fn rejects_smt_terms_outside_the_language() {
    let terms = [
        "(let ((value1 true)) value1)",
        "(match true ((value1 value1)))",
        "(! true :named claim)",
        "(lambda ((value1 Bool)) value1)",
        "(= 1 1)",
        "(= 1.0 1.0)",
        "(= \"text\" \"text\")",
        "(_ bv1 1)",
        "(as model_true Bool)",
    ];
    assert!(terms
        .iter()
        .all(|term| result(term) == Err(FailureKind::Rejected)));
}

#[test]
fn rejects_invalid_quantified_variable_names() {
    let terms = [
        "value1",
        "(forall ((x Bool)) x)",
        "(forall ((value0 Bool)) value0)",
        "(forall ((value01 Bool)) value01)",
        "(forall ((value1 Bool) (value1 Bool)) value1)",
    ];
    assert!(terms
        .iter()
        .all(|term| result(term) == Err(FailureKind::Rejected)));
}

#[test]
fn accepts_documented_quantifiers_and_literals() {
    let rule = "(and (forall ((value1 Bool)) (= value1 value1)) \
        (= #b0001 #b0001) (= #x01 #x01))";
    assert_eq!(result(rule), Ok(()));
}

#[test]
fn rejects_a_leading_zero_argument_number() {
    let source = "(hray (fn advance) (in arg01 all) (out ret) \
        (mem none) (os none) (req true))";
    let failure = parse(source)
        .ok()
        .and_then(|contract| validate(contract, &facts()).err());
    assert!(matches!(failure, Some(ValidationFailure::Rejected(_))));
}
