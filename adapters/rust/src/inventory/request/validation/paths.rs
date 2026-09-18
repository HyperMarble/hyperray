// Validate absolute filesystem inputs before the owner starts a build.

use crate::inventory::BuildError;
use std::path::Path;

pub(super) fn file(path: &Path) -> Result<(), BuildError> {
    if !path.is_absolute() || !path.is_file() {
        return Err(BuildError::InvalidRequest(format!(
            "expected an absolute file: {}",
            path.display()
        )));
    }
    Ok(())
}

pub(super) fn directory(path: &Path) -> Result<(), BuildError> {
    if !path.is_absolute() || !path.is_dir() {
        return Err(BuildError::InvalidRequest(format!(
            "expected an absolute directory: {}",
            path.display()
        )));
    }
    Ok(())
}
