// Capture tests require named Clippy findings with primary Rust spans.

use crate::common;
use hyperray_rust::shape::{findings_in, Finding};
use std::path::Path;

#[test]
fn every_clippy_capture_yields_named_findings_with_a_primary_span() {
    let Some(root) = common::dir("HYPERRAY_FIXTURES") else {
        return;
    };
    let failures: Vec<String> = captures_in(&root)
        .iter()
        .flat_map(|log| findings_in(log))
        .filter_map(named)
        .collect();
    assert!(failures.is_empty(), "{}", failures.join("\n"));
}

fn captures_in(root: &Path) -> Vec<String> {
    let Ok(entries) = std::fs::read_dir(root) else {
        return Vec::new();
    };
    entries
        .filter_map(|entry| entry.ok().map(|found| found.path()))
        .filter(|path| path.extension().is_some_and(|kind| kind == "jsonl"))
        .filter_map(|path| std::fs::read_to_string(path).ok())
        .collect()
}

fn named(finding: Finding) -> Option<String> {
    let lint = finding.lint.starts_with("clippy::");
    let file = finding.path.ends_with(".rs");
    let lines = finding.line_start >= 1 && finding.line_start <= finding.line_end;
    let good = lint && file && lines && !finding.message.is_empty();
    match good {
        true => None,
        false => Some(format!("{} at {}", finding.lint, finding.path)),
    }
}

#[test]
fn a_non_json_line_is_skipped() {
    assert!(findings_in("Compiling foo\nwarning: bar\n").is_empty());
}

#[test]
fn a_target_selected_clippy_lint_is_kept() {
    let line = r#"{"reason":"compiler-message","message":{"message":"return is not needed","code":{"code":"clippy::needless_return"},"spans":[{"file_name":"src/lib.rs","line_start":2,"line_end":2,"is_primary":true}]}}"#;
    let findings = findings_in(line);
    assert_eq!(findings.len(), 1);
    assert_eq!(findings[0].lint, "clippy::needless_return");
}
