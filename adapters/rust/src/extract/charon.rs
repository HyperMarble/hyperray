// One Charon run over a crate with every feature on, so no `#[cfg]` body
// is hidden. It never edits the tree and never names an item; Charon
// starts from `crate` and reports every span it saw.

use super::refusal::{refusals_in, Refusal};
use serde::Serialize;
use std::path::Path;
use std::process::Command;

#[derive(Debug, Serialize, PartialEq, Eq)]
pub struct Run {
    pub charon: String,
    pub exit_code: Option<i32>,
    pub output: String,
    pub log: String,
    pub refusals: Vec<Refusal>,
}

pub fn run(charon: &Path, crate_dir: &Path, output: &Path) -> Run {
    let result = Command::new(charon)
        .current_dir(crate_dir)
        .args(["cargo", "--ullbc", "--preset", "fast"])
        .args(["--sysroot", "default", "--no-dedup-serialized-ast"])
        .args(["--consts", "values"])
        .arg("--dest-file")
        .arg(output)
        .args(["--", "--all-features"])
        .output();
    let log = match &result {
        Ok(done) => String::from_utf8_lossy(&done.stderr).into_owned(),
        Err(error) => error.to_string(),
    };
    Run {
        charon: charon.display().to_string(),
        exit_code: result.ok().and_then(|done| done.status.code()),
        output: output.display().to_string(),
        refusals: refusals_in(&log),
        log,
    }
}
