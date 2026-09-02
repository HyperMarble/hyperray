// One Charon run over a crate, started from the modules the patch
// touches. It never edits the tree and never builds an item name.

use super::refusal::{refusals_in, Refusal};
use serde::Serialize;
use std::path::Path;
use std::process::Command;

#[derive(Debug, Serialize, PartialEq, Eq)]
pub struct Run {
    pub charon: String,
    pub exit_code: Option<i32>,
    pub output: String,
    pub refusals: Vec<Refusal>,
}

pub struct Scope<'a> {
    pub crate_dir: &'a Path,
    pub modules: &'a [String],
    pub cargo_args: &'a [String],
}

pub fn run(charon: &Path, scope: &Scope, output: &Path) -> Run {
    let mut command = Command::new(charon);
    command
        .current_dir(scope.crate_dir)
        .args(["cargo", "--ullbc", "--preset", "fast"])
        .args(["--sysroot", "default", "--no-dedup-serialized-ast"])
        .arg("--dest-file")
        .arg(output);
    for module in scope.modules {
        command.arg("--start-from-if-exists").arg(module);
    }
    command.arg("--").args(scope.cargo_args);
    let result = command.output();
    let log = match &result {
        Ok(done) => String::from_utf8_lossy(&done.stderr).into_owned(),
        Err(error) => error.to_string(),
    };
    Run {
        charon: charon.display().to_string(),
        exit_code: result.ok().and_then(|done| done.status.code()),
        output: output.display().to_string(),
        refusals: refusals_in(&log),
    }
}

// `src/a/b.rs` is the module `crate::a::b`; `src/a/mod.rs` is `crate::a`
// and `src/lib.rs` is `crate`. This is rustc's file-to-module rule.
pub fn module_of(file_in_crate: &str) -> String {
    let inner = file_in_crate
        .strip_prefix("src/")
        .unwrap_or(file_in_crate)
        .strip_suffix(".rs")
        .unwrap_or(file_in_crate);
    let mut segments: Vec<&str> = inner.split('/').collect();
    if matches!(segments.last(), Some(&"mod") | Some(&"lib")) {
        segments.pop();
    }
    match segments.is_empty() {
        true => "crate".to_string(),
        false => format!("crate::{}", segments.join("::")),
    }
}
