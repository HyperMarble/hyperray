// Purpose: Test every fixed `.hray` expression form through the public validator.
// Never: Replace complete form coverage with selected examples.
// In: One valid representative for each of the fourteen expression forms.
// Out: Acceptance for the complete fixed expression-form inventory.
// Fails: A documented form cannot reach a validated contract.

mod support;

use contract::{parse, validate, LiteralFact, OperationFact, ValidationFacts};
use support::facts;

fn validation_facts() -> ValidationFacts {
    let mut facts = facts();
    facts.literals.push(LiteralFact {
        name: "model_true".into(),
        sort: "Bool".into(),
    });
    facts.operations.push(OperationFact {
        name: "is_zero".into(),
        arguments: vec!["(_ BitVec 64)".into()],
        result: "Bool".into(),
    });
    facts
}

fn accepts(requirement: &str, facts: &ValidationFacts) -> bool {
    let source = format!(
        "(hray (fn advance) (in arg1 all) (out ret) (mem none) \
         (os none) (req {requirement}))"
    );
    let Ok(contract) = parse(&source) else {
        return false;
    };
    validate(contract, facts).is_ok()
}

#[test]
fn accepts_each_fixed_expression_form() {
    let forms = [
        "true",
        "model_true",
        "(forall ((value1 Bool)) value1)",
        "(is_zero arg1)",
        "(exists ((value1 Bool)) value1)",
        "(forall ((value1 Bool)) value1)",
        "(= ret ret)",
        "(distinct ret arg1)",
        "(and true true)",
        "(or true false)",
        "(xor true false)",
        "(=> true true)",
        "(not false)",
        "(ite true true false)",
    ];
    let facts = validation_facts();
    assert!(forms.iter().all(|form| accepts(form, &facts)));
}
