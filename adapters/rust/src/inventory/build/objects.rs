// Find compiler object outputs and combine multiple CGUs into one actual object.

use super::paths::Paths;
use crate::inventory::command::{require_output, run, ProcessKind};
use crate::inventory::{BuildError, BuildRequest};
use std::fs;
use std::path::PathBuf;
use std::process::Command;

pub(super) fn find(paths: &Paths) -> Result<Vec<PathBuf>, BuildError> {
    if paths.object.is_file() {
        return Ok(vec![paths.object.clone()]);
    }
    let entries = fs::read_dir(&paths.compiler_directory)
        .map_err(|error| BuildError::Io(error.to_string()))?;
    let mut objects = Vec::new();
    for entry in entries {
        let path = entry
            .map_err(|error| BuildError::Io(error.to_string()))?
            .path();
        if path.extension().is_some_and(|extension| extension == "o") {
            objects.push(path);
        }
    }
    objects.sort();
    if objects.is_empty() {
        return Err(BuildError::MissingArtifact(paths.object.clone()));
    }
    Ok(objects)
}

pub(super) fn combine(
    request: &BuildRequest,
    paths: &Paths,
    objects: &[PathBuf],
) -> Result<(), BuildError> {
    if objects.len() == 1 && objects[0] == paths.object {
        return Ok(());
    }
    let mut command = Command::new(&request.linker);
    command.arg("-r").args(objects).arg("-o").arg(&paths.object);
    run(
        &mut command,
        &request.output_directory.join("object-linker.log"),
        ProcessKind::Linker,
    )?;
    require_output(&paths.object)
}
