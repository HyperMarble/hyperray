// Purpose: finds a Cargo project and the lock file that pins its versions.
// Never:   builds a folder whose settings the project itself did not choose.
pub mod config_files;

use crate::blocked::Blocked;
use std::path::{Path, PathBuf};

/// A Cargo project whose dependency versions are pinned.
#[derive(Debug, PartialEq)]
pub struct Project {
    pub root: PathBuf,
    pub lock_file: PathBuf,
}

/// Accepts `root` only when it holds both Cargo.toml and Cargo.lock.
///
/// Without Cargo.toml nothing says how to build, and without Cargo.lock
/// two builds of the same commit can use different dependency versions.
pub fn find(root: &Path) -> Result<Project, Blocked> {
    if !root.join("Cargo.toml").is_file() {
        return Err(Blocked::NoManifest(root.to_path_buf()));
    }
    let lock_file = root.join("Cargo.lock");
    if !lock_file.is_file() {
        return Err(Blocked::NoLockFile(root.to_path_buf()));
    }
    Ok(Project {
        root: root.to_path_buf(),
        lock_file,
    })
}
