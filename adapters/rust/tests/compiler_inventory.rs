// Public build-owner tests reject stale outputs and failed tool runs.
// Real builds use the same public request as the integration command.

use hyperray_rust::inventory::{build, BuildError, BuildRequest};
use std::fs;
use std::path::{Path, PathBuf};

fn request(output_directory: PathBuf) -> BuildRequest {
    BuildRequest {
        driver: PathBuf::from("/usr/bin/false"),
        linker: PathBuf::from("/usr/bin/false"),
        source: PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("../../fixtures/rust/compiler-inventory/src/lib.rs"),
        output_directory,
        target: "aarch64-apple-darwin".to_string(),
        compiler_flags: vec!["--crate-type=lib".to_string()],
        linker_flags: Vec::new(),
        boundary_artifact: None,
        extern_artifacts: Vec::new(),
        sysroot: None,
    }
}

fn temporary_directory(name: &str) -> PathBuf {
    std::env::temp_dir().join(format!(
        "hyperray-compiler-inventory-{name}-{}",
        std::process::id()
    ))
}

fn remove_directory(path: &Path) -> Result<(), Box<dyn std::error::Error>> {
    match fs::remove_dir_all(path) {
        Ok(()) => Ok(()),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(()),
        Err(error) => Err(Box::new(error)),
    }
}

#[test]
fn existing_output_directory_is_rejected() -> Result<(), Box<dyn std::error::Error>> {
    let output = temporary_directory("existing");
    remove_directory(&output)?;
    fs::create_dir(&output)?;
    let result = build(&request(output.clone()));
    remove_directory(&output)?;
    assert!(matches!(result, Err(BuildError::OutputDirectoryExists(_))));
    Ok(())
}

#[test]
fn caller_cannot_override_owned_compiler_output_flags() -> Result<(), Box<dyn std::error::Error>> {
    let output = temporary_directory("owned-flags");
    remove_directory(&output)?;
    let mut build_request = request(output);
    build_request.compiler_flags = vec!["--target=other-target".to_string()];
    let result = build(&build_request);
    assert!(matches!(result, Err(BuildError::InvalidRequest(_))));
    Ok(())
}
#[test]
fn failed_compiler_preserves_diagnostics_without_manifest() -> Result<(), Box<dyn std::error::Error>>
{
    let output = temporary_directory("failure");
    remove_directory(&output)?;
    let result = build(&request(output.clone()));
    let failure = match result {
        Err(failure) => failure,
        Ok(_) => return Err("false unexpectedly produced a successful build".into()),
    };
    assert!(matches!(failure, BuildError::CompilerFailed { .. }));
    assert!(!output.join("manifest.json").exists());
    assert!(output.join("failure.log").exists());
    remove_directory(&output)?;
    Ok(())
}
