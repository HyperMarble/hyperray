mod common;

use hyperray_rust::extract::refusals_in;

// Every `.log` under HYPERRAY_FIXTURES is a Charon run captured verbatim.
fn charon_logs() -> Vec<(String, String)> {
    let Some(root) = common::dir("HYPERRAY_FIXTURES") else {
        return Vec::new();
    };
    let Ok(entries) = std::fs::read_dir(&root) else {
        return Vec::new();
    };
    let mut found = Vec::new();
    for fixture in entries.filter_map(Result::ok) {
        let Ok(files) = std::fs::read_dir(fixture.path()) else {
            continue;
        };
        for file in files.filter_map(Result::ok) {
            let path = file.path();
            if path.extension().is_some_and(|e| e == "log") {
                if let Ok(text) = std::fs::read_to_string(&path) {
                    found.push((path.display().to_string(), text));
                }
            }
        }
    }
    found
}

// The rule: a refusal is Charon's `warning:`/`error:` line followed by a
// `-->` location. Every refusal read back has a reason, a file, and a
// line; the count equals the number of `-->` lines that follow a reason.
#[test]
fn every_refusal_in_every_charon_log_has_a_reason_and_a_location() {
    for (name, log) in charon_logs() {
        let refusals = refusals_in(&log);
        let located = log
            .lines()
            .zip(log.lines().skip(1))
            .filter(|(reason, next)| {
                (reason.starts_with("warning: ") || reason.starts_with("error: "))
                    && next.trim_start().starts_with("--> ")
            })
            .count();
        assert_eq!(refusals.len(), located, "{name}");
        for refusal in &refusals {
            assert!(!refusal.reason.is_empty(), "{name}");
            assert!(refusal.path.ends_with(".rs"), "{name}: {}", refusal.path);
            assert!(refusal.line >= 1, "{name}");
        }
    }
}

#[test]
fn a_reason_without_a_location_is_not_a_refusal() {
    let log = "warning: unused import\nsomething else\nerror: bad\n  --> src/x.rs:7:3\n";
    let refusals = refusals_in(log);
    assert_eq!(refusals.len(), 1);
    assert_eq!(refusals[0].path, "src/x.rs");
    assert_eq!(refusals[0].line, 7);
}
