// Purpose: Generate every recursive expression form allowed by the `.hray` grammar.
// Never: Limit coverage to named examples or one expression depth.
// In: Generated literals, roles, applications, bindings, quantifiers, and annotations.
// Out: Acceptance for each syntactically valid generated contract.
// Fails: A grammar constructor or recursive combination cannot be parsed.

use contract::parse;
use proptest::prelude::*;

fn expression() -> impl Strategy<Value = String> {
    let leaf = prop_oneof![
        Just("arg1".into()),
        Just("ret".into()),
        Just("memory_before".into()),
        Just("true".into()),
        Just("false".into()),
        Just("#b0101".into()),
        Just("#x00ff".into()),
        Just("registered_literal".into()),
    ];
    leaf.prop_recursive(8, 512, 8, |inner| {
        prop_oneof![
            inner.clone().prop_map(|value| format!("(not {value})")),
            prop::collection::vec(inner.clone(), 1..8)
                .prop_map(|values| format!("(registered_op {})", values.join(" "))),
            (inner.clone(), inner.clone()).prop_map(|(left, right)| format!("(= {left} {right})")),
            inner
                .clone()
                .prop_map(|value| format!("((_ extract 7 0) {value})")),
            inner
                .clone()
                .prop_map(|value| format!("((as const Bool) {value})")),
            (inner.clone(), inner.clone())
                .prop_map(|(value, body)| format!("(let ((value1 {value})) {body})")),
            (prop::collection::vec(1usize..16, 1..5), inner.clone()).prop_map(|(names, body)| {
                let bindings = names
                    .iter()
                    .map(|name| format!("(value{name} (_ BitVec 64))"))
                    .collect::<Vec<_>>()
                    .join(" ");
                format!("(forall ({bindings}) {body})")
            },),
            inner
                .clone()
                .prop_map(|body| { format!("(exists ((value1 (_ BitVec 64))) {body})") }),
            inner.prop_map(|value| format!("(! {value} :named claim)")),
        ]
    })
}

proptest! {
    #[test]
    fn accepts_every_generated_expression_shape(requirement in expression()) {
        let source = format!(
            "(hray (fn f) (in none) (out none) (mem none) (os none) (req {requirement}))"
        );
        prop_assert!(parse(&source).is_ok(), "rejected: {requirement}");
    }
}
