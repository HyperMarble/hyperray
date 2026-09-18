// Compute compilation identity from observed input bytes and build selections.

use super::ObservedInputs;
use crate::inventory::{identity, ArtifactIdentity, BuildError, BuildRequest, ToolIdentity};
use serde_json::json;
use std::path::Path;

pub(super) fn compilation_id(
    request: &BuildRequest,
    inputs: &ObservedInputs,
) -> Result<String, BuildError> {
    let value = json!({
        "target": request.target,
        "compiler_flags": request.compiler_flags,
        "linker_flags": request.linker_flags,
        "source": inputs.source,
        "driver": inputs.driver,
        "linker": inputs.linker,
        "boundary": inputs.boundary,
        "externs": inputs.externs,
        "sysroot": inputs.sysroot,
    });
    let bytes =
        serde_json::to_vec(&value).map_err(|error| BuildError::Manifest(error.to_string()))?;
    identity::bytes(&bytes)
}

pub(super) fn artifact(path: &Path) -> Result<ArtifactIdentity, BuildError> {
    let (sha256, size) = identity::file(path)?;
    Ok(ArtifactIdentity {
        path: path.to_path_buf(),
        sha256,
        size,
    })
}

pub(super) fn tool(path: &Path) -> Result<ToolIdentity, BuildError> {
    let (sha256, _) = identity::file(path)?;
    Ok(ToolIdentity {
        path: path.to_path_buf(),
        sha256,
    })
}
