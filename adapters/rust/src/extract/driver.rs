// Compile the explicitly selected Cargo build with the matching rustc driver.
// Extraction must not enable features absent from the request.
use std::process::Command;

pub use super::outcome::Run;
use super::{dumps, fresh, outcome, toolchain, CompileRequest};

pub fn run(request: &CompileRequest) -> Run {
    match compile(request) {
        Ok(result) => result,
        Err(error) => outcome::failed(request, error),
    }
}

fn compile(request: &CompileRequest) -> Result<Run, String> {
    request.validate()?;
    let directory = &request.output_directory;
    std::fs::create_dir_all(directory).map_err(|error| error.to_string())?;
    let previous = dumps::in_directory(directory)?;
    let wrapper = fresh::driver_link(&request.driver, directory)?;
    let compiler = toolchain::compiler_for(&request.driver)?;
    let mut command = Command::new(&request.cargo);
    command
        .current_dir(&request.crate_directory)
        .args(["check", "--all-targets"])
        .env("CARGO_TARGET_DIR", directory.join("cargo"))
        .env("RUSTC", compiler)
        .env("RUSTC_WORKSPACE_WRAPPER", wrapper)
        .env("RUSTC_BOOTSTRAP", "1")
        .env("HYPERRAY_RUSTC_WRAPPER", "1")
        .env("HYPERRAY_MIR_DIR", directory)
        .env_remove("CARGO_PRIMARY_PACKAGE");
    if !request.features.default_features {
        command.arg("--no-default-features");
    }
    if !request.features.features.is_empty() {
        command
            .arg("--features")
            .arg(request.features.features.join(","));
    }
    outcome::report(request, &previous, command.output())
}
