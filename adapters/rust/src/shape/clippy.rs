// One `cargo clippy` run with the shape lints on. The thresholds go in a
// clippy.toml the run writes, found through CLIPPY_CONF_DIR (Clippy book,
// configuration). The output is handed to `finding` unchanged.

use super::finding::{findings_in, Finding, LINTS};
use serde::Serialize;
use std::path::Path;
use std::process::Command;

pub const MAX_LINES: u32 = 40;
pub const MAX_NESTING: u32 = 3;

#[derive(Debug, Serialize)]
pub struct Run {
    pub exit_code: Option<i32>,
    pub findings: Vec<Finding>,
}

pub fn run(crate_dir: &Path, conf_dir: &Path) -> std::io::Result<Run> {
    std::fs::write(conf_dir.join("clippy.toml"), thresholds())?;
    let mut command = Command::new("cargo");
    command
        .current_dir(crate_dir)
        .env("CLIPPY_CONF_DIR", conf_dir)
        .args(["clippy", "--message-format=json", "--all-features", "--"]);
    for lint in LINTS {
        command.args(["-W", lint]);
    }
    let done = command.output()?;
    Ok(Run {
        exit_code: done.status.code(),
        findings: findings_in(&String::from_utf8_lossy(&done.stdout)),
    })
}

// Keys from the Clippy book, lint_configuration.html.
fn thresholds() -> String {
    format!(
        "too-many-lines-threshold = {MAX_LINES}\n\
         excessive-nesting-threshold = {MAX_NESTING}\n"
    )
}
