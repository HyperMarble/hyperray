// Purpose: Test memory shapes and installed OS behavior contracts.
// Never: Infer an access, target, operation, or missing behavior contract.
// In: Contracts with memory and OS declarations plus explicit upstream facts.
// Out: Valid contracts, rejections, or exact engine errors.
// Fails: Effects can escape the declared contract or missing facts are guessed.
mod support;
use contract::{parse, validate, OsOperationFact, ValidationFailure};
use support::facts;

fn failure(source: &str, facts: &contract::ValidationFacts) -> Option<ValidationFailure> {
    parse(source)
        .ok()
        .and_then(|value| validate(value, facts).err())
}

#[test]
fn rejects_memory_none_conflicts_and_wrong_types() {
    let sources = [
        "(hray (fn advance) (in arg1 all) (out ret) (mem none) \
         (mem arg1 read arg1 bytes) (os none) (req true))",
        "(hray (fn advance) (in arg1 all) (out ret) \
         (mem true read arg1 bytes) (os none) (req true))",
    ];
    assert!(sources.iter().all(|source| {
        matches!(
            failure(source, &facts()),
            Some(ValidationFailure::Rejected(_))
        )
    }));
}

#[test]
fn accepts_installed_os_contract_and_reports_missing_behavior_as_engine_error() {
    let source = "(hray (fn advance) (in arg1 all) (out ret) (mem none) \
        (os wasi_snapshot_preview1 fd_read) (req true))";
    let mut facts = facts();
    let installed = OsOperationFact {
        interface: "wasi_snapshot_preview1".into(),
        operation: "fd_read".into(),
        target: facts.target.clone(),
        model: Some("wasi.fd_read.v1".into()),
    };
    facts.os_operations.push(installed.clone());
    let selected = parse(source)
        .ok()
        .and_then(|value| validate(value, &facts).ok())
        .map(|value| value.os_operations);
    facts.os_operations[0].model = None;
    assert_eq!(selected, Some(vec![installed]));
    assert!(matches!(
        failure(source, &facts),
        Some(ValidationFailure::Engine(_))
    ));
}

#[test]
fn rejects_unknown_or_wrong_target_os_operations() {
    let source = "(hray (fn advance) (in arg1 all) (out ret) (mem none) \
        (os wasi_snapshot_preview1 fd_read) (req true))";
    let mut wrong_target = facts();
    wrong_target.os_operations.push(OsOperationFact {
        interface: "wasi_snapshot_preview1".into(),
        operation: "fd_read".into(),
        target: "other-target".into(),
        model: Some("wasi.fd_read.v1".into()),
    });
    assert!(matches!(
        failure(source, &facts()),
        Some(ValidationFailure::Rejected(_))
    ));
    assert!(matches!(
        failure(source, &wrong_target),
        Some(ValidationFailure::Rejected(_))
    ));
}
