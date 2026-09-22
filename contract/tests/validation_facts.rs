// Purpose: Test validator decisions that depend on upstream fact inventories.
// Never: Let unrelated facts block a contract or select the first matching target.
// In: Valid contracts plus complete compiler, memory, and OS facts.
// Out: Acceptance for relevant facts and engine errors for ambiguous facts.
// Fails: Fact order or an unused fact changes contract validity.

mod support;

use contract::OsOperationFact;
use support::{facts, validation_result, FailureKind};

fn source(os: &str) -> String {
    format!("(hray (fn advance) (in arg1 all) (out ret) (mem none) {os} (req true))")
}

fn operation(target: &str) -> OsOperationFact {
    OsOperationFact {
        interface: "wasi_snapshot_preview1".into(),
        operation: "fd_read".into(),
        target: target.into(),
        model: Some("wasi.fd_read.v1".into()),
    }
}

#[test]
fn ignores_unused_memory_sort_facts() {
    let mut facts = facts();
    facts.address_sort = "MissingAddressSort".into();
    facts.size_sort = "MissingSizeSort".into();
    facts.memory_sort = "MissingMemorySort".into();
    assert_eq!(validation_result(&source("(os none)"), &facts), Ok(()));
}

#[test]
fn selects_the_operation_for_the_compiled_target() {
    let mut facts = facts();
    let target = facts.target.clone();
    facts.os_operations = vec![operation("other-target"), operation(&target)];
    let os = "(os wasi_snapshot_preview1 fd_read)";
    assert_eq!(validation_result(&source(os), &facts), Ok(()));
}

#[test]
fn reports_ambiguous_target_operation_facts_as_engine_error() {
    let mut facts = facts();
    let target = facts.target.clone();
    facts.os_operations = vec![operation(&target), operation(&target)];
    let os = "(os wasi_snapshot_preview1 fd_read)";
    assert_eq!(
        validation_result(&source(os), &facts),
        Err(FailureKind::Engine)
    );
}
