// Command helpers capture compiler and linker diagnostics in the fresh output.
// They never convert a non-zero process status into successful artifact evidence.

use super::BuildError;
use std::path::Path;
use std::process::{Command, Output};

pub fn run(command: &mut Command, log: &Path, kind: ProcessKind) -> Result<Output, BuildError> {
    let output = command
        .output()
        .map_err(|error| BuildError::Io(format!("run {kind:?}: {error}")))?;
    if output.status.success() {
        return Ok(output);
    }
    let diagnostics = [output.stdout.as_slice(), output.stderr.as_slice()].concat();
    std::fs::write(log, diagnostics).map_err(|error| BuildError::Io(error.to_string()))?;
    let status = output.status.code();
    match kind {
        ProcessKind::Compiler => Err(BuildError::CompilerFailed {
            status,
            log: log.to_path_buf(),
        }),
        ProcessKind::Linker => Err(BuildError::LinkerFailed {
            status,
            log: log.to_path_buf(),
        }),
    }
}

pub fn require_output(path: &Path) -> Result<(), BuildError> {
    if path.is_file() {
        return Ok(());
    }
    Err(BuildError::MissingArtifact(path.to_path_buf()))
}

#[derive(Clone, Copy, Debug)]
pub enum ProcessKind {
    Compiler,
    Linker,
}
