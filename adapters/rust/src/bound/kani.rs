// Kani produces final proof artifacts without running verification in Stage 3.
// The caller supplies extra Kani options; this layer invents no proof limit.

use super::model::{HarnessMode, Scope};
use super::{tool, Error};
use std::path::Path;
use std::process::Command;

pub fn generate(scope: &Scope<'_>, target: &Path) -> Result<(), Error> {
    let mode_arguments: &[&str] = match scope.harness_mode {
        HarnessMode::Declared => &[],
        HarnessMode::Automatic => &["autoharness", "-Z", "autoharness"],
    };
    let mut command = Command::new(scope.cargo);
    command
        .current_dir(scope.crate_dir)
        .arg("kani")
        .args(mode_arguments)
        .args([
            "--only-codegen",
            "--force-build",
            "--workspace",
            "--all-features",
            "--target-dir",
        ])
        .arg(target)
        .args(scope.kani_arguments);
    tool::run(&mut command, "Kani")?;
    Ok(())
}

pub fn version(cargo: &Path) -> Result<String, Error> {
    tool::version(cargo, &["kani", "--version"], "Kani")
}
