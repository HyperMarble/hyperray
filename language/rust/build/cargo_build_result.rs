// Purpose: collects Cargo's JSON output and retains the exact build request.
// Never: reports a refused Cargo command as a built program.
use crate::blocked::Blocked;
use crate::cargo_build::BuildOutput;
use crate::cargo_handover::is_handed_over;
use crate::cargo_messages::{
    files_of,
    is_local,
    native_code_of, //
};
use crate::cargo_observation::Observation;
use serde_json::Value;

pub(super) fn finish(process: Observation) -> Result<BuildOutput, Blocked> {
    if !process.success {
        return Err(super::cargo_refusal::refused(&process));
    }
    let messages = stdout(&process)?;
    let mut output = BuildOutput {
        request: Some(process.request),
        files: Vec::new(),
        native_code: Vec::new(),
        libraries: Vec::new(),
        not_built: Vec::new(),
    };
    for line in messages.lines().filter(|line| line.starts_with('{')) {
        let message = parse(line, "cargo build output")?;
        if message["reason"] == "compiler-artifact" {
            add_compiled(&message, &mut output)?;
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

/// Adds what one compiled-file message reports: its files when they are
/// handed over, and its library when the crate is the project's own.
fn add_compiled(message: &Value, output: &mut BuildOutput) -> Result<(), Blocked> {
    if is_handed_over(message) {
        output.files.extend(files_of(message)?);
    }
    if is_local(message) {
        let library = super::cargo_library::library_of(message);
        output.libraries.extend(library);
    }
    Ok(())
}

pub(crate) fn stdout(process: &Observation) -> Result<String, Blocked> {
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
