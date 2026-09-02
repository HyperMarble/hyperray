// One `cargo kani` run over a crate. Flags are the ones the tool's own
// `--help` lists; `-Z async-lib` is added only when the caller says a
// harness drives an `async fn`. The log is handed to `report` unchanged.

use super::report::{results_in, HarnessResult};
use serde::Serialize;
use std::path::Path;
use std::process::Command;

#[derive(Debug, Serialize, PartialEq, Eq)]
pub struct Run {
    pub kani: String,
    pub exit_code: Option<i32>,
    pub log: String,
    pub results: Vec<HarnessResult>,
}

pub struct Scope<'a> {
    pub crate_dir: &'a Path,
    pub harness: Option<&'a str>,
    pub default_unwind: u32,
    pub async_lib: bool,
}

pub fn run(scope: &Scope) -> Run {
    let mut command = Command::new("cargo");
    command
        .current_dir(scope.crate_dir)
        .args(["kani", "--output-format", "regular"])
        .arg("--default-unwind")
        .arg(scope.default_unwind.to_string());
    if let Some(name) = scope.harness {
        command.args(["--harness", name]);
    }
    if scope.async_lib {
        command.args(["-Z", "async-lib"]);
    }
    let result = command.output();
    let log = match &result {
        Ok(done) => {
            let mut text = String::from_utf8_lossy(&done.stdout).into_owned();
            text.push_str(&String::from_utf8_lossy(&done.stderr));
            text
        }
        Err(error) => error.to_string(),
    };
    Run {
        kani: version(),
        exit_code: result.ok().and_then(|done| done.status.code()),
        results: results_in(&log),
        log,
    }
}

pub fn version() -> String {
    let output = Command::new("cargo").args(["kani", "--version"]).output();
    match output {
        Ok(done) => String::from_utf8_lossy(&done.stdout).trim().to_string(),
        Err(error) => error.to_string(),
    }
}
