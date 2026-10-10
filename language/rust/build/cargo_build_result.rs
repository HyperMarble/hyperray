// Purpose: collects Cargo's JSON output and retains the exact build request.
// Never: reports a refused Cargo command as a built program.
use crate::blocked::Blocked;
use crate::cargo_build::BuildOutput;
use crate::cargo_messages::{
    files_of,
    native_code_of, //
};
use crate::cargo_observation::Observation;
use serde_json::Value;

pub(super) fn finish(process: Observation, members: &[Value]) -> Result<BuildOutput, Blocked> {
    let messages = stdout(&process)?;
    let mut output = BuildOutput {
        request: Some(process.request),
        files: Vec::new(),
        native_code: Vec::new(),
    };
    for line in messages.lines().filter(|line| line.starts_with('{')) {
        let message = parse(line, "cargo build output")?;
        if message["reason"] == "compiler-artifact" && members.contains(&message["package_id"]) {
            output.files.extend(files_of(&message)?);
        }
        if message["reason"] == "build-script-executed" {
            output.native_code.extend(native_code_of(&message));
        }
    }
    if output.files.is_empty() {
        return Err(Blocked::NoArtifact);
    }
    Ok(output)
}

pub(super) fn stdout(process: &Observation) -> Result<String, Blocked> {
    if !process.success {
        let output = text(&process.stderr, "cargo stderr")?;
        return Err(Blocked::ToolFailed {
            tool: "cargo".to_string(),
            output,
        });
    }
    text(&process.stdout, "cargo stdout")
}

fn text(bytes: &[u8], what: &str) -> Result<String, Blocked> {
    String::from_utf8(bytes.to_vec()).map_err(|error| Blocked::Unreadable {
        what: what.to_string(),
        cause: error.to_string(),
    })
}

pub(super) fn parse(text: &str, what: &str) -> Result<Value, Blocked> {
    serde_json::from_str(text).map_err(|error| Blocked::Unreadable {
        what: what.to_string(),
        cause: error.to_string(),
    })
}
