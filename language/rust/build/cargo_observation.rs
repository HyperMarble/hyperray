// Purpose: retains the exact request and both streams for a Cargo invocation.
// Never: reports a rejected request as a successful build or drops its stdout.
use crate::blocked::Blocked;
use std::ffi::OsString;
use std::path::Path;
use std::process::Command;

pub use crate::process_evidence::{
    Observation,
    RequestEvidence, //
};

pub fn capture(
    executable: &Path,
    arguments: &[OsString],
    directory: &Path,
) -> Result<Observation, Blocked> {
    let output = Command::new(executable)
        .args(arguments)
        .current_dir(directory)
        .output()
        .map_err(|error| Blocked::ToolMissing {
            tool: executable.display().to_string(),
            cause: error.to_string(),
        })?;
    Ok(Observation::from_output(
        executable, arguments, directory, output,
    ))
}
