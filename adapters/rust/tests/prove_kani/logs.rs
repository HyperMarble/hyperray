// Captured Kani logs must yield one consistent result for each harness line.
// Text without a harness line must not create a verdict.

use crate::common;
use hyperray_rust::prove::results_in;

#[test]
fn every_kani_log_yields_one_result_per_harness() {
    let Some(root) = common::dir("HYPERRAY_FIXTURES") else {
        return;
    };
    let Ok(entries) = std::fs::read_dir(root.join("kani-logs")) else {
        return;
    };
    for entry in entries.filter_map(Result::ok) {
        check_log(&entry);
    }
}

fn check_log(entry: &std::fs::DirEntry) {
    let Ok(log) = std::fs::read_to_string(entry.path()) else {
        return;
    };
    let harness_lines = log
        .lines()
        .filter(|line| line.starts_with("Checking harness "))
        .count();
    let results = results_in(&log);
    assert_eq!(results.len(), harness_lines, "{}", entry.path().display());
    for result in &results {
        assert!(!result.time_s.is_empty(), "{}", result.harness);
        assert_eq!(
            result.passed,
            result.failed_checks.is_empty(),
            "{}",
            result.harness
        );
    }
}

#[test]
fn a_log_without_a_harness_line_yields_nothing() {
    assert!(results_in("VERIFICATION:- SUCCESSFUL\n").is_empty());
}
