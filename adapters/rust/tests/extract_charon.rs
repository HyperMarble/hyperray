use hyperray_rust::extract::{refusals_in, Refusal};
use std::path::PathBuf;

fn fixture_log() -> String {
    let path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../../fixtures/alloy-ws-batch/charon-pubsub.log");
    std::fs::read_to_string(path).unwrap_or_default()
}

fn count(refusals: &[Refusal], reason: &str) -> usize {
    refusals.iter().filter(|r| r.reason == reason).count()
}

// Counts measured with grep on the same log on 2026-09-02.
#[test]
fn charon_log_yields_every_refusal_with_its_location() {
    let refusals = refusals_in(&fixture_log());
    assert_eq!(count(&refusals, "Coroutines are not supported"), 8);
    assert_eq!(
        count(&refusals, "Coroutine types are not supported yet"),
        32
    );
    assert_eq!(
        count(
            &refusals,
            "Hax panicked when translating `alloy_pubsub::frontend::{impl#1}`."
        ),
        2
    );
    let first = refusals
        .iter()
        .find(|r| r.reason == "Coroutines are not supported");
    assert_eq!(
        first,
        Some(&Refusal {
            reason: "Coroutines are not supported".to_string(),
            path: "crates/pubsub/src/frontend.rs".to_string(),
            line: 41,
        })
    );
}

#[test]
fn a_reason_without_a_location_is_not_a_refusal() {
    let log = "warning: unused import\nsomething else\nerror: bad\n  --> src/x.rs:7:3\n";
    let refusals = refusals_in(log);
    assert_eq!(refusals.len(), 1);
    assert_eq!(refusals[0].path, "src/x.rs");
    assert_eq!(refusals[0].line, 7);
}
