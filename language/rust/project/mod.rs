// Purpose: finds a Cargo project and the lock file that pins its versions.
// Never:   builds a folder whose settings the project itself did not choose.
pub mod config_files;
mod config_include_io;

use crate::blocked::Blocked;
use std::ffi::OsString;
use std::path::{
    Path,
    PathBuf, //
};

/// A Cargo project whose dependency versions are pinned.
#[derive(Debug, PartialEq)]
pub struct Project {
    pub root: PathBuf,
    pub lock_file: PathBuf,
}

/// Accepts `root` only when it holds Cargo.toml and its workspace holds
/// Cargo.lock. A crate on its own is its own workspace.
///
/// Without Cargo.toml nothing says how to build, and without Cargo.lock
/// two builds of the same commit can use different dependency versions.
pub fn find(root: &Path) -> Result<Project, Blocked> {
    if !root.join("Cargo.toml").is_file() {
        return Err(Blocked::NoManifest(root.to_path_buf()));
    }
    let workspace = workspace_of(root)?;
    let lock_file = workspace.join("Cargo.lock");
    if !lock_file.is_file() {
        return Err(Blocked::NoLockFile(workspace));
    }
    Ok(Project {
        root: root.to_path_buf(),
        lock_file,
    })
}

/// The top folder of the workspace `root` belongs to, as Cargo names it.
/// Cargo keeps one Cargo.lock there for every crate in the workspace. When
/// Cargo refuses, its own words are the reason.
fn workspace_of(root: &Path) -> Result<PathBuf, Blocked> {
    let words = ["locate-project", "--workspace", "--message-format", "plain"];
    let arguments = words.map(OsString::from);
    let process = crate::cargo_observation::capture(Path::new("cargo"), &arguments, root)?;
    let manifest = crate::build::cargo_build_result::stdout(&process)?;
    let folder = Path::new(manifest.trim()).parent();
    folder
        .map(Path::to_path_buf)
        .ok_or_else(|| Blocked::Unreadable {
            what: "cargo locate-project".to_string(),
            cause: "the workspace manifest has no folder".to_string(),
        })
}
