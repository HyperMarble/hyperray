// Purpose: retains native process requests, results and both byte streams.
// Never: replaces invalid bytes or discards an owner's refusal.
use crate::native_argument::NativeArgument;
use serde::Serialize;
use std::ffi::OsString;
use std::path::Path;
use std::process::Output;

#[derive(Clone, Debug, PartialEq, Serialize)]
pub struct RequestEvidence {
    pub executable: NativeArgument,
    pub directory: NativeArgument,
    pub arguments: Vec<NativeArgument>,
}

#[derive(Clone, Debug, PartialEq)]
pub struct Observation {
    pub request: RequestEvidence,
    pub success: bool,
    pub stdout: Vec<u8>,
    pub stderr: Vec<u8>,
}

impl Observation {
    pub fn from_output(
        executable: &Path,
        arguments: &[OsString],
        directory: &Path,
        output: Output,
    ) -> Self {
        let arguments = arguments
            .iter()
            .map(|value| NativeArgument::of(value))
            .collect();
        Self {
            request: RequestEvidence {
                executable: NativeArgument::of(executable.as_os_str()),
                directory: NativeArgument::of(directory.as_os_str()),
                arguments,
            },
            success: output.status.success(),
            stdout: output.stdout,
            stderr: output.stderr,
        }
    }
}
