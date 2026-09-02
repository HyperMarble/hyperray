mod common;

use hyperray_rust::prove::auto_in;

// The log was captured verbatim from `cargo kani autoharness -Z autoharness`
// on a real crate: Kani chose some functions, skipped the rest with a reason.
#[test]
fn autoharness_log_yields_a_verdict_per_chosen_and_a_reason_per_skip() {
    let Some(root) = common::dir("HYPERRAY_FIXTURES") else {
        return;
    };
    let path = root.join("kani-logs/autoharness-noodles-util.log");
    let Ok(log) = std::fs::read_to_string(path) else {
        return;
    };
    let auto = auto_in(&log);
    assert!(!auto.verdicts.is_empty());
    assert!(!auto.skipped.is_empty());
    for verdict in &auto.verdicts {
        assert!(!verdict.harness.is_empty());
        assert_eq!(
            verdict.passed,
            verdict.failed_checks.is_empty(),
            "{}",
            verdict.harness
        );
    }
    for skip in &auto.skipped {
        assert!(!skip.function.is_empty());
        assert!(!skip.reason.is_empty(), "{}", skip.function);
    }
    let chosen: std::collections::BTreeSet<_> = auto.verdicts.iter().map(|v| &v.harness).collect();
    let skipped: std::collections::BTreeSet<_> = auto.skipped.iter().map(|s| &s.function).collect();
    assert!(chosen.is_disjoint(&skipped));
}

#[test]
fn a_log_without_tables_yields_nothing() {
    let auto = auto_in("Compiling foo\n");
    assert!(auto.verdicts.is_empty());
    assert!(auto.skipped.is_empty());
}
