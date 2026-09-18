// Own the compiler and linker lifecycle and finalize only observed artifacts.
// Failure logs remain available without a successful manifest.

mod arguments;
mod inputs;
mod objects;
mod paths;
mod process;
mod result;

use super::{BuildError, BuildRequest};
use result::manifest;
use std::fs;

pub use result::BuildArtifacts;

pub fn build(request: &BuildRequest) -> Result<BuildArtifacts, BuildError> {
    request.validate()?;
    let observed = inputs::observe(request)?;
    let compilation_id = inputs::compilation_id(request, &observed)?;
    fs::create_dir(&request.output_directory).map_err(|error| BuildError::Io(error.to_string()))?;
    let paths = paths::Paths::new(&request.output_directory);
    fs::create_dir(&paths.compiler_directory).map_err(|error| BuildError::Io(error.to_string()))?;
    let compiler_arguments = arguments::compiler(request, &paths);
    let objects = process::compile(request, &paths, &compiler_arguments, &compilation_id)?;
    objects::combine(request, &paths, &objects)?;
    let linker_arguments = arguments::linker(request, &paths);
    process::link(request, &paths, &linker_arguments)?;
    inputs::ensure_unchanged(request, &observed)?;
    let manifest = manifest(
        request,
        &paths,
        &observed,
        compilation_id,
        compiler_arguments,
        linker_arguments,
    )?;
    result::write(request, manifest)
}
