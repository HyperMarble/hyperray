// Clippy findings read from Cargo JSON lines. Target policy selects the lints.

use super::diagnostic::{Line, Message};
use serde::Serialize;

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
    if !lint.starts_with("clippy::") {
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
