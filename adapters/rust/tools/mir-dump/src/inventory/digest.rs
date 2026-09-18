// Hash owned raw payload bytes without leaving the child stdin open.

use serde_json::Value;
use std::io::Write;
use std::process::{Command, Stdio};

pub(super) fn digest(value: &Value) -> Result<String, String> {
    let bytes = serde_json::to_vec(value).map_err(|error| error.to_string())?;
    let mut child = Command::new("/usr/bin/shasum")
        .args(["-a", "256"])
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .spawn()
        .map_err(|error| error.to_string())?;
    let mut input = child
        .stdin
        .take()
        .ok_or_else(|| "cannot open digest input".to_string())?;
    input.write_all(&bytes).map_err(|error| error.to_string())?;
    drop(input);
    let output = child
        .wait_with_output()
        .map_err(|error| error.to_string())?;
    if !output.status.success() {
        return Err("digest command failed".to_string());
    }
    String::from_utf8_lossy(&output.stdout)
        .split_whitespace()
        .next()
        .map(str::to_string)
        .ok_or_else(|| "digest command returned no value".to_string())
}
