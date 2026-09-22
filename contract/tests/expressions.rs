// Purpose: Test that the parser preserves complete SMT expression trees.
// Never: Enforce the later `.hray` validator's expression subset.
// In: Valid and malformed SMT terms inside sections.
// Out: Acceptance for SMT syntax and rejection for malformed terms.
// Fails: The parser reparses or truncates nested SMT expressions.

use contract::parse;
use proptest::prelude::*;

fn contract(requirement: &str) -> String {
    format!("(hray (fn f) (in none) (out none) (mem none) (os none) (req {requirement}))")
}

#[test]
fn accepts_all_smt_term_shapes_for_validation() {
    let cases = [
        "(= ((_ extract 7 0) arg1) #x00)",
        "(forall ((value1 (_ BitVec 64))) (= value1 value1))",
        "(= memory_after ((as const (Array (_ BitVec 8) (_ BitVec 8))) #x00))",
        "(let ((value1 true)) value1)",
        "(! true :named claim)",
        "1",
        "\"text\"",
    ];
    assert!(cases.iter().all(|term| parse(&contract(term)).is_ok()));
}

#[test]
fn rejects_malformed_smt_terms() {
    let cases = ["#b", "#x", "(f", "(forall ())"];
    let accepted: Vec<&str> = cases
        .iter()
        .copied()
        .filter(|term| parse(&contract(term)).is_ok())
        .collect();
    assert!(accepted.is_empty(), "accepted forms: {accepted:?}");
}

fn expression() -> impl Strategy<Value = String> {
    let leaf = prop_oneof![
        Just("arg1".to_string()),
        Just("#x00".to_string()),
        Just("true".to_string()),
    ];
    leaf.prop_recursive(8, 256, 4, |inner| {
        (inner.clone(), inner).prop_map(|(left, right)| format!("(= {left} {right})"))
    })
}

proptest! {
    #[test]
    fn preserves_recursive_smt_terms(requirement in expression()) {
        prop_assert!(parse(&contract(&requirement)).is_ok());
    }
}
