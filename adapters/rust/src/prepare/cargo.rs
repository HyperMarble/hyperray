// Cargo builds the generated connection with its real dependency graph.
// No manual dependency linker or library filename guesses belong here.
use super::{cargo_artifact, cargo_dependency, step, CargoOptions, Request};
use std::fs::{self, File};
use std::io::BufReader;
use std::process::Command;

pub fn libraries(request: &Request, options: &CargoOptions) -> Result<(), String> {
    let manifest = request.directory.join("Cargo.toml");
    fs::write(&manifest, include_str!("cargo_manifest.toml")).map_err(|error| error.to_string())?;
    cargo_dependency::configure(request, options)?;
    let mut command = Command::new(&options.executable);
    command
        .args([
            "build",
            "--lib",
            "--locked",
            "--offline",
            "--message-format=json",
            "--target-dir",
        ])
        .arg(request.directory.join("cargo-target"))
        .env("RUSTC", &request.tools.rustc)
        .env(
            "CARGO_PROFILE_DEV_OPT_LEVEL",
            request.optimization.to_string(),
        )
        .env("CARGO_PROFILE_DEV_PANIC", "abort");
    step::execute_json(&mut command, &request.directory, "cargo-build")?;
    let output = File::open(request.directory.join("cargo-build.jsonl"))
        .map_err(|error| error.to_string())?;
    let canonical = manifest.canonicalize().map_err(|error| error.to_string())?;
    let archive = cargo_artifact::select(BufReader::new(output), &canonical)?;
    fs::copy(archive, request.directory.join("libbinding.a")).map_err(|error| error.to_string())?;
    Ok(())
}
