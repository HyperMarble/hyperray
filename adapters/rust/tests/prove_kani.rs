mod common;

use hyperray_rust::prove::{results_in, run, Scope};

// The log was captured verbatim from `cargo kani` 0.67.0 on a two-harness
// crate: one proves old = new, one drives the old line to overflow.
#[test]
fn kani_log_yields_one_result_per_harness() {
    let Some(root) = common::dir("HYPERRAY_FIXTURES") else {
        return;
    };
    let Ok(log) = std::fs::read_to_string(root.join("kani-logs/two-harness.log")) else {
        return;
    };
    let results = results_in(&log);
    assert_eq!(results.len(), 2);
    for result in &results {
        assert!(result.checks > 0, "{}", result.harness);
        assert!(!result.time_s.is_empty(), "{}", result.harness);
        assert_eq!(
            result.passed,
            result.failed_checks.is_empty(),
            "{}",
            result.harness
        );
    }
    let failed = results.iter().find(|r| !r.passed);
    assert_eq!(
        failed.map(|r| r.failed_checks.clone()),
        Some(vec!["attempt to subtract with overflow".to_string()])
    );
}

#[test]
fn a_log_without_a_harness_line_yields_nothing() {
    assert!(results_in("VERIFICATION:- SUCCESSFUL\n").is_empty());
}

// The rule: every harness in the crate gets exactly one verdict, and a
// verdict is FAILED if and only if it names a failed check.
#[test]
fn kani_run_gives_every_harness_a_verdict() {
    let Some(sources) = common::dir("HYPERRAY_FIXTURE_SRC") else {
        return;
    };
    let crate_dir = sources.join("kani-shape");
    if !crate_dir.is_dir() {
        return;
    }
    let scope = Scope {
        crate_dir: &crate_dir,
        harness: None,
        default_unwind: 3,
        async_lib: false,
    };
    let done = run(&scope);
    assert!(done.kani.starts_with("cargo-kani"), "{}", done.kani);
    assert!(!done.results.is_empty(), "{}", done.log);
    for result in &done.results {
        assert_eq!(
            result.passed,
            result.failed_checks.is_empty(),
            "{}",
            result.harness
        );
    }
}
