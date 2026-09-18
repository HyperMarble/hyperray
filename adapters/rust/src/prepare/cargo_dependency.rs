// Cargo owns path-dependency declarations and feature activation.
// Subject and requirement features must remain explicit and separate.
use super::{step, CargoOptions, FeatureSelection, Request};
use std::path::Path;
use std::process::Command;

pub fn configure(request: &Request, options: &CargoOptions) -> Result<(), String> {
    dependency(
        request,
        options,
        "subject",
        &request.subject.source,
        &options.subject,
    )?;
    dependency(
        request,
        options,
        "requirement",
        &request.requirement.source,
        &options.requirement,
    )
}

fn dependency(
    request: &Request,
    options: &CargoOptions,
    name: &str,
    source: &Path,
    features: &FeatureSelection,
) -> Result<(), String> {
    let directory = source
        .parent()
        .ok_or("Cargo package has no parent directory")?;
    let mut command = Command::new(&options.executable);
    command
        .args(["add", "--offline", "--path"])
        .arg(directory)
        .args(["--rename", name]);
    if !features.default_features {
        command.arg("--no-default-features");
    }
    if !features.features.is_empty() {
        command.arg("--features").arg(features.features.join(","));
    }
    command.env("RUSTC", &request.tools.rustc);
    step::execute(
        &mut command,
        &request.directory,
        &format!("cargo-add-{name}"),
    )
}
