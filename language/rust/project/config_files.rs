// Purpose: records the Cargo settings files that shaped this build: the
//          project's own `.cargo/config.toml`, those of every folder above
//          it, and the user's home one, in the order Cargo reads them.
// Never:   reads the settings itself; Cargo already applied them. It only
//          names each file by its content so two builds can be told apart.
use crate::blocked::Blocked;
use crate::digest::digest_of;
use crate::record::FileDigest;
use std::path::{Path, PathBuf};

/// Cargo reads `.cargo/config` when it exists, else `.cargo/config.toml`.
const NAMES: [&str; 2] = ["config", "config.toml"];

/// Every settings file Cargo reads for a build run in `root`, deepest first:
/// `root/.cargo/config.toml`, each parent folder's, then the home one
/// (`$CARGO_HOME/config.toml`, by default `$HOME/.cargo/config.toml`).
pub fn config_files(root: &Path) -> Result<Vec<FileDigest>, Blocked> {
    let root = root.canonicalize().map_err(|error| Blocked::Unreadable {
        what: root.display().to_string(),
        cause: error.to_string(),
    })?;
    let folders = root.ancestors().map(|folder| folder.join(".cargo"));
    let mut found = Vec::new();
    for file in folders
        .chain(cargo_home())
        .filter_map(|folder| present(&folder))
    {
        if !found
            .iter()
            .any(|known: &FileDigest| known.path == file.display().to_string())
        {
            found.push(digest_of(&file)?);
        }
    }
    Ok(found)
}

fn present(folder: &Path) -> Option<PathBuf> {
    NAMES
        .iter()
        .map(|name| folder.join(name))
        .find(|file| file.is_file())
}

fn cargo_home() -> Option<PathBuf> {
    let home = std::env::var_os("HOME").map(|home| PathBuf::from(home).join(".cargo"));
    std::env::var_os("CARGO_HOME").map(PathBuf::from).or(home)
}
