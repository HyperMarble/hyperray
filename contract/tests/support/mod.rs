// Purpose: Build common language-neutral facts for validator tests.
// Never: Hide a compiler, model, or OS assumption from each test input.
// In: Optional test-specific changes to public fact values.
// Out: Complete facts and contract source for one ARM64-sized value shape.
// Fails: This module does not perform parsing or validation.

use contract::{parse, validate, FunctionFact, ValidationFacts, ValidationFailure, ValueFact};

// Each integration test compiles this shared module as a separate crate.
#[allow(dead_code)]
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum FailureKind {
    Rejected,
    Engine,
}

pub fn value() -> ValueFact {
    ValueFact {
        sort: "(_ BitVec 64)".into(),
        location: Some("compiler-location".into()),
    }
}

#[allow(dead_code)]
pub fn validation_result(source: &str, facts: &ValidationFacts) -> Result<(), FailureKind> {
    let contract = parse(source).map_err(|_| FailureKind::Rejected)?;
    validate(contract, facts)
        .map(|_| ())
        .map_err(|failure| match failure {
            ValidationFailure::Rejected(_) => FailureKind::Rejected,
            ValidationFailure::Engine(_) => FailureKind::Engine,
        })
}

pub fn facts() -> ValidationFacts {
    ValidationFacts {
        target: "arm64-apple-macos".into(),
        functions: vec![FunctionFact {
            name: "advance".into(),
            arguments: vec![value()],
            result: Some(value()),
        }],
        logic: "ALL".into(),
        address_sort: "(_ BitVec 64)".into(),
        size_sort: "(_ BitVec 64)".into(),
        memory_sort: "(Array (_ BitVec 64) (_ BitVec 8))".into(),
        operations: Vec::new(),
        literals: Vec::new(),
        os_operations: Vec::new(),
    }
}
