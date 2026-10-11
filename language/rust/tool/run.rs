// Purpose: runs one tool in a folder and returns what it printed.
// Never:   treats a tool that failed, or could not start, as a success.
use crate::blocked::Blocked;
use std::path::Path;
use std::process::Command;

/// Runs `tool` with `args` inside `folder`, returning its standard output.
pub fn printed(tool: &str, args: &[&str], folder: &Path) -> Result<String, Blocked> {
    let output = Command::new(tool)
        .args(args)
        .current_dir(folder)
        .output()
        .map_err(|error| Blocked::ToolMissing {
            tool: tool.to_string(),
            cause: error.to_string(),
        })?;
    if !output.status.success() {
        return Err(Blocked::ToolFailed {
            tool: tool.to_string(),
            output: String::from_utf8_lossy(&output.stderr).trim().to_string(),
        });
    }
    Ok(String::from_utf8_lossy(&output.stdout).to_string())
}
