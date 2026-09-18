// Run both owned processes and require their actual output artifacts.

use super::paths::Paths;
use crate::inventory::command::{require_output, run, ProcessKind};
use crate::inventory::{BuildError, BuildRequest};
use std::path::PathBuf;
use std::process::Command;

pub(super) fn compile(
    request: &BuildRequest,
    paths: &Paths,
    arguments: &[String],
    compilation_id: &str,
) -> Result<Vec<PathBuf>, BuildError> {
    let mut command = Command::new(&request.driver);
    command.args(arguments);
    command.current_dir(&paths.compiler_directory);
    command.env("HYPERRAY_INVENTORY_DIR", &paths.inventory_directory);
    command.env("HYPERRAY_COMPILATION_ID", compilation_id);
    if let Some(boundary) = &request.boundary_artifact {
        command.env("HYPERRAY_BOUNDARY_ARTIFACT", boundary);
    }
    run(
        &mut command,
        &request.output_directory.join("failure.log"),
        ProcessKind::Compiler,
    )?;
    require_output(&paths.dep_info)?;
    require_output(&paths.inventory)?;
    super::objects::find(paths)
}

pub(super) fn link(
    request: &BuildRequest,
    paths: &Paths,
    arguments: &[String],
) -> Result<(), BuildError> {
    let mut command = Command::new(&request.linker);
    command.args(arguments);
    run(
        &mut command,
        &request.output_directory.join("linker.log"),
        ProcessKind::Linker,
    )?;
    require_output(&paths.elf)
}
