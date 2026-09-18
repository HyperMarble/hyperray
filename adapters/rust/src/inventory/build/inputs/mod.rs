// Observe every input before compilation and reject changes before finalization.

mod identity;
mod sysroot;

use super::super::{ArtifactIdentity, BuildError, BuildRequest, SysrootIdentity, ToolIdentity};
use std::path::Path;

pub(super) struct ObservedInputs {
    pub(super) source: ArtifactIdentity,
    pub(super) driver: ToolIdentity,
    pub(super) linker: ToolIdentity,
    pub(super) sysroot: Option<SysrootIdentity>,
    pub(super) boundary: Option<ArtifactIdentity>,
    pub(super) externs: Vec<ArtifactIdentity>,
}

pub(super) fn observe(request: &BuildRequest) -> Result<ObservedInputs, BuildError> {
    let driver = tool(&request.driver)?;
    Ok(ObservedInputs {
        source: artifact(&request.source)?,
        driver: driver.clone(),
        linker: tool(&request.linker)?,
        sysroot: request
            .sysroot
            .as_deref()
            .map(|path| sysroot::observe(path, &request.target, driver.clone()))
            .transpose()?,
        boundary: request
            .boundary_artifact
            .as_deref()
            .map(artifact)
            .transpose()?,
        externs: request
            .extern_artifacts
            .iter()
            .map(|value| artifact(&value.path))
            .collect::<Result<Vec<_>, _>>()?,
    })
}

pub(super) fn ensure_unchanged(
    request: &BuildRequest,
    before: &ObservedInputs,
) -> Result<(), BuildError> {
    let after = observe(request)?;
    if after.source != before.source {
        return Err(BuildError::InputChanged(request.source.clone()));
    }
    if after.driver != before.driver || after.linker != before.linker {
        return Err(BuildError::InputChanged(request.driver.clone()));
    }
    if after.sysroot != before.sysroot {
        return Err(BuildError::InputChanged(request.source.clone()));
    }
    if after.boundary != before.boundary || after.externs != before.externs {
        return Err(BuildError::InputChanged(request.source.clone()));
    }
    Ok(())
}

pub(super) fn compilation_id(
    request: &BuildRequest,
    inputs: &ObservedInputs,
) -> Result<String, BuildError> {
    identity::compilation_id(request, inputs)
}

pub(super) fn artifact(path: &Path) -> Result<ArtifactIdentity, BuildError> {
    identity::artifact(path)
}

fn tool(path: &Path) -> Result<ToolIdentity, BuildError> {
    identity::tool(path)
}
