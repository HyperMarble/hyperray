// Run Clippy with the target crate's policy. Never inject Hyperray's policy.

use super::finding::{findings_in, Finding};
use serde::Serialize;
use std::path::Path;
use std::process::Command;

#[derive(Debug, Serialize)]
pub struct Run {
    pub exit_code: Option<i32>,
    pub findings: Vec<Finding>,
}

pub fn run(crate_dir: &Path) -> std::io::Result<Run> {
    let mut command = Command::new("cargo");
    command
        .current_dir(crate_dir)
        .args(["clippy", "--message-format=json", "--all-features"]);
    let done = command.output()?;
    Ok(Run {
        exit_code: done.status.code(),
        findings: findings_in(&String::from_utf8_lossy(&done.stdout)),
    })
}
