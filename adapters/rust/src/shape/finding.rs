// Shape findings read back from cargo's JSON lines. Only lines starting
// `{` are JSON (Cargo book, external-tools); only shape lints are kept.

use super::diagnostic::{Line, Message};
use serde::Serialize;

// Lint names from the Clippy book, lints.html. Each one names a rule in
// CODE_RULES: length, nesting, and the calls forbidden in library code.
pub const LINTS: [&str; 6] = [
    "clippy::too_many_lines",
    "clippy::excessive_nesting",
    "clippy::unwrap_used",
    "clippy::expect_used",
    "clippy::panic",
    "clippy::unreachable",
];

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct Finding {
    pub lint: String,
    pub path: String,
    pub line_start: u32,
    pub line_end: u32,
    pub message: String,
}

pub fn findings_in(stdout: &str) -> Vec<Finding> {
    stdout
        .lines()
        .filter(|line| line.starts_with('{'))
        .filter_map(|line| serde_json::from_str::<Line>(line).ok())
        .filter(|line| line.reason == "compiler-message")
        .filter_map(|line| line.message)
        .filter_map(finding)
        .collect()
}

fn finding(message: Message) -> Option<Finding> {
    let lint = message.code?.code;
    if !LINTS.contains(&lint.as_str()) {
        return None;
    }
    let span = message.spans.into_iter().find(|s| s.is_primary)?;
    Some(Finding {
        lint,
        path: span.file_name,
        line_start: span.line_start,
        line_end: span.line_end,
        message: message.message,
    })
}
