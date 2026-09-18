// Measure the selected sysroot as a sorted set of regular-file identities.

mod aggregate;

use crate::inventory::identity;
use crate::inventory::{BuildError, SysrootFileIdentity, SysrootIdentity, ToolIdentity};
use std::fs;
use std::path::Path;

pub(super) fn observe(
    root: &Path,
    target: &str,
    toolchain: ToolIdentity,
) -> Result<SysrootIdentity, BuildError> {
    let mut files = Vec::new();
    collect(root, root, &mut files)?;
    files.sort_by(|left, right| left.path.cmp(&right.path));
    let aggregate_sha256 = aggregate::digest(root, target, &toolchain, &files);
    Ok(SysrootIdentity {
        path: root.to_path_buf(),
        target: target.to_string(),
        toolchain,
        files,
        aggregate_sha256,
    })
}

fn collect(
    root: &Path,
    directory: &Path,
    files: &mut Vec<SysrootFileIdentity>,
) -> Result<(), BuildError> {
    let entries = fs::read_dir(directory).map_err(|error| io_error(directory, error))?;
    for entry in entries {
        let entry = entry.map_err(|error| io_error(directory, error))?;
        let path = entry.path();
        let kind = entry.file_type().map_err(|error| io_error(&path, error))?;
        if kind.is_dir() {
            collect(root, &path, files)?;
        } else if kind.is_file() {
            files.push(file(root, &path)?);
        }
    }
    Ok(())
}

fn file(root: &Path, path: &Path) -> Result<SysrootFileIdentity, BuildError> {
    let relative = path
        .strip_prefix(root)
        .map_err(|error| BuildError::Manifest(error.to_string()))?;
    let (sha256, size) = identity::file(path)?;
    Ok(SysrootFileIdentity {
        path: relative.to_path_buf(),
        sha256,
        size,
    })
}

fn io_error(path: &Path, error: std::io::Error) -> BuildError {
    BuildError::HashFailed(path.to_path_buf(), error.to_string())
}
