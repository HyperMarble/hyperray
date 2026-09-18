// Record the actual tool versions used by this preparation request.
// Version recording is not source or dependency provenance certification.
use super::{Request, ToolVersion};
use std::path::Path;
use std::process::Command;

pub fn collect(request: &Request) -> Result<Vec<ToolVersion>, String> {
    let tools = &request.tools;
    let mut versions = Vec::new();
    for (name, path, arguments) in [
        ("rustc", &tools.rustc, vec!["--version", "--verbose"]),
        ("spin", &tools.spin, vec!["-V"]),
        ("clang", &tools.clang, vec!["--version"]),
    ] {
        versions.push(version(name, path, &arguments, &request.directory)?);
    }
    if let Some(cargo) = &request.cargo {
        versions.push(version(
            "cargo",
            &cargo.executable,
            &["--version"],
            &request.directory,
        )?);
    }
    Ok(versions)
}

fn version(
    name: &str,
    path: &Path,
    arguments: &[&str],
    directory: &Path,
) -> Result<ToolVersion, String> {
    let output = Command::new(path)
        .args(arguments)
        .current_dir(directory)
        .output()
        .map_err(|error| format!("{name} version: {error}"))?;
    if !output.status.success() {
        return Err(format!(
            "{name} version failed: {}",
            String::from_utf8_lossy(&output.stderr)
        ));
    }
    let version = String::from_utf8(output.stdout).map_err(|error| error.to_string())?;
    if version.trim().is_empty() {
        return Err(format!("{name} version output is empty"));
    }
    Ok(ToolVersion {
        tool: name.into(),
        executable: path.into(),
        version,
    })
}
