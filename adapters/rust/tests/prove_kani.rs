mod common;

use hyperray_rust::prove::{results_in, run, Scope};

// Every `kani-logs/*.log` under HYPERRAY_FIXTURES is a `cargo kani` run
// captured verbatim. The rule: a verdict is FAILED if and only if it
// names a failed check, and every verdict carries a time.
#[test]
fn every_kani_log_yields_one_result_per_harness() {
    let Some(root) = common::dir("HYPERRAY_FIXTURES") else {
        return;
    };
    let Ok(entries) = std::fs::read_dir(root.join("kani-logs")) else {
        return;
    };
    for entry in entries.filter_map(Result::ok) {
        let Ok(log) = std::fs::read_to_string(entry.path()) else {
            continue;
        };
        let harness_lines = log
            .lines()
            .filter(|l| l.starts_with("Checking harness "))
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
}

#[test]
fn a_log_without_a_harness_line_yields_nothing() {
    assert!(results_in("VERIFICATION:- SUCCESSFUL\n").is_empty());
}

// The rule: every crate under HYPERRAY_FIXTURE_SRC that has a Kani harness
// gets exactly one verdict per harness, FAILED iff it names a failed check.
#[test]
fn kani_run_gives_every_harness_a_verdict() {
    let Some(sources) = common::dir("HYPERRAY_FIXTURE_SRC") else {
        return;
    };
    let Ok(entries) = std::fs::read_dir(&sources) else {
        return;
    };
    for entry in entries.filter_map(Result::ok) {
        let crate_dir = entry.path();
        if !crate_dir.join("Cargo.toml").is_file() || !has_harness(&crate_dir) {
            continue;
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
}

// A crate has a harness when some file under src/ carries `#[kani::proof]`.
fn has_harness(crate_dir: &std::path::Path) -> bool {
    let Ok(entries) = std::fs::read_dir(crate_dir.join("src")) else {
        return false;
    };
    entries
        .filter_map(Result::ok)
        .filter_map(|e| std::fs::read_to_string(e.path()).ok())
        .any(|text| text.contains("#[kani::proof]"))
}
