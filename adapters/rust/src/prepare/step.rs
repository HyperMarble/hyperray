// Compiler stages retain their full output in a dedicated log.
// An unsuccessful stage must never produce a prepared result.
use std::fs::OpenOptions;
use std::path::Path;
use std::process::{Command, Stdio};

pub fn execute(command: &mut Command, directory: &Path, stage: &str) -> Result<(), String> {
    let path = directory.join(format!("{stage}.log"));
    let log = OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(&path)
        .map_err(|error| format!("create {}: {error}", path.display()))?;
    let diagnostic = log.try_clone().map_err(|error| error.to_string())?;
    let status = command
        .current_dir(directory)
        .stdin(Stdio::null())
        .stdout(log)
        .stderr(diagnostic)
        .status()
        .map_err(|error| format!("start {stage}: {error}"))?;
    if !status.success() {
        return Err(format!(
            "{stage} returned {status}; diagnostics: {}",
            path.display()
        ));
    }
    Ok(())
}

pub fn execute_json(command: &mut Command, directory: &Path, stage: &str) -> Result<(), String> {
    let path = directory.join(format!("{stage}.log"));
    let diagnostic = OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(&path)
        .map_err(|error| format!("create {}: {error}", path.display()))?;
    let output = OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(directory.join(format!("{stage}.jsonl")))
        .map_err(|error| error.to_string())?;
    let status = command
        .current_dir(directory)
        .stdin(Stdio::null())
        .stdout(output)
        .stderr(diagnostic)
        .status()
        .map_err(|error| format!("start {stage}: {error}"))?;
    if !status.success() {
        return Err(format!(
            "{stage} returned {status}; diagnostics: {}",
            path.display()
        ));
    }
    Ok(())
}
