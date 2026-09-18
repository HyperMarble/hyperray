// Validate caller inputs and reserve the build owner flags.

use super::BuildRequest;
use crate::inventory::BuildError;
mod paths;

pub(super) fn check(request: &BuildRequest) -> Result<(), BuildError> {
    if request.target.is_empty() {
        return Err(BuildError::InvalidRequest("target is empty".to_string()));
    }
    if !request.output_directory.is_absolute() {
        return Err(BuildError::InvalidRequest(
            "output directory must be absolute".to_string(),
        ));
    }
    if request.output_directory.exists() {
        return Err(BuildError::OutputDirectoryExists(
            request.output_directory.clone(),
        ));
    }
    for path in [&request.driver, &request.linker, &request.source] {
        paths::file(path)?;
    }
    if let Some(path) = &request.boundary_artifact {
        paths::file(path)?;
    }
    for artifact in &request.extern_artifacts {
        if artifact.name.is_empty() {
            return Err(BuildError::InvalidRequest(
                "extern crate name is empty".to_string(),
            ));
        }
        paths::file(&artifact.path)?;
    }
    if let Some(path) = &request.sysroot {
        paths::directory(path)?;
    }
    reject_owned_flags(
        &request.compiler_flags,
        &["--target", "--emit", "--sysroot", "-o"],
    )?;
    reject_owned_flags(&request.linker_flags, &["-o"])
}

fn reject_owned_flags(flags: &[String], owned: &[&str]) -> Result<(), BuildError> {
    for flag in flags {
        if owned.iter().any(|name| owned_flag_matches(flag, name)) {
            return Err(BuildError::InvalidRequest(format!(
                "caller cannot override owned flag: {flag}"
            )));
        }
    }
    Ok(())
}

fn owned_flag_matches(flag: &str, name: &str) -> bool {
    if name == "-o" {
        return flag == name || flag.starts_with("-o");
    }
    flag == name || flag.starts_with(&format!("{name}="))
}
