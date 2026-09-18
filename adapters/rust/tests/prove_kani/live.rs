// Every configured crate with a proof harness must return Kani verdicts.
// Extra Kani options come from the fixture environment.

use crate::common;
use hyperray_rust::prove::{run, Scope};

#[test]
fn kani_run_gives_every_harness_a_verdict() {
    let Some(sources) = common::dir("HYPERRAY_FIXTURE_SRC") else {
        return;
    };
    let Ok(entries) = std::fs::read_dir(&sources) else {
        return;
    };
    let kani_arguments = kani_arguments();
    for entry in entries.filter_map(Result::ok) {
        check_crate(&entry.path(), &kani_arguments);
    }
}

fn check_crate(crate_dir: &std::path::Path, kani_arguments: &[String]) {
    if !crate_dir.join("Cargo.toml").is_file() || !has_harness(crate_dir) {
        return;
    }
    let scope = Scope {
        crate_dir,
        harness: None,
        default_unwind: 3,
        async_lib: false,
        kani_arguments,
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

fn kani_arguments() -> Vec<String> {
    match std::env::var("HYPERRAY_KANI_ARGS") {
        Ok(value) => value.split_whitespace().map(str::to_string).collect(),
        Err(_) => Vec::new(),
    }
}

fn has_harness(crate_dir: &std::path::Path) -> bool {
    let Ok(entries) = std::fs::read_dir(crate_dir.join("src")) else {
        return false;
    };
    entries
        .filter_map(Result::ok)
        .filter_map(|entry| std::fs::read_to_string(entry.path()).ok())
        .any(|text| text.contains("#[kani::proof]"))
}
