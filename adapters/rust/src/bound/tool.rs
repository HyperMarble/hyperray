// Tool execution accepts only a configured executable and returns exact output.
// A missing tool, failed status, or empty version is an explicit Stage 3 error.

use super::Error;
use std::path::Path;
use std::process::{Command, Output};

pub fn run(command: &mut Command, name: &str) -> Result<Output, Error> {
    let output = command.output().map_err(|error| Error::Tool {
        tool: name.to_string(),
        reason: error.to_string(),
    })?;
    if output.status.success() {
        return Ok(output);
    }
    Err(Error::Tool {
        tool: name.to_string(),
        reason: failure(&output),
    })
}

pub fn version(program: &Path, arguments: &[&str], name: &str) -> Result<String, Error> {
    let mut command = Command::new(program);
    command.args(arguments);
    let output = run(&mut command, name)?;
    let version = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if !version.is_empty() {
        return Ok(version);
    }
    Err(Error::Tool {
        tool: name.to_string(),
        reason: "version output was empty".to_string(),
    })
}

fn failure(output: &Output) -> String {
    let stdout = String::from_utf8_lossy(&output.stdout);
    let stderr = String::from_utf8_lossy(&output.stderr);
    format!("exit status {}\n{stdout}{stderr}", output.status)
}
