// One `mir-dump` run over a crate with every feature on, so no `#[cfg]`
// body is hidden. It never edits the tree and never names an item; the
// compiler reports every item it built.

use serde::Serialize;
use std::path::Path;
use std::process::{Command, Output};

#[derive(Debug, Serialize, PartialEq, Eq)]
pub struct Run {
    pub driver: String,
    pub exit_code: Option<i32>,
    pub output_dir: String,
    pub dumps: usize,
    pub log: String,
}

pub fn run(driver: &Path, crate_dir: &Path, output_dir: &Path) -> Run {
    let mut command = Command::new("cargo");
    command
        .current_dir(crate_dir)
        .args(["build", "--all-features"])
        .env("RUSTC", driver)
        .env("RUSTC_BOOTSTRAP", "1")
        .env("HYPERRAY_MIR_DIR", output_dir);
    report(driver, output_dir, command.output())
}

// A cached crate makes cargo exit 0 without running the compiler, so the
// exit code alone cannot say whether this stage produced anything. The
// count of files makes an empty run visible instead of silent.
fn report(driver: &Path, output_dir: &Path, result: std::io::Result<Output>) -> Run {
    let log = match &result {
        Ok(done) => String::from_utf8_lossy(&done.stderr).into_owned(),
        Err(error) => error.to_string(),
    };
    Run {
        driver: driver.display().to_string(),
        exit_code: result.ok().and_then(|done| done.status.code()),
        output_dir: output_dir.display().to_string(),
        dumps: dumps_in(output_dir),
        log,
    }
}

fn dumps_in(output_dir: &Path) -> usize {
    let Ok(entries) = std::fs::read_dir(output_dir) else {
        return 0;
    };
    entries
        .filter_map(|entry| entry.ok())
        .filter(|found| found.file_name().to_string_lossy().ends_with(".mir.json"))
        .count()
}
