// Purpose: retains native process requests, results and both byte streams.
// Never: replaces invalid bytes or discards an owner's refusal.
use crate::native_argument::NativeArgument;
use serde::{
    Deserialize,
    Serialize, //
};
use std::ffi::OsString;
use std::path::Path;
use std::process::Output;

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct RequestEvidence {
    pub executable: NativeArgument,
    pub directory: NativeArgument,
    pub arguments: Vec<NativeArgument>,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct Observation {
    pub request: RequestEvidence,
    pub success: bool,
    pub status_code: Option<i32>,
    pub status: String,
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
            status_code: output.status.code(),
            status: output.status.to_string(),
            stdout: output.stdout,
            stderr: output.stderr,
        }
    }
}
