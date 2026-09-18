// Fixture location finds a named source tree or returns an explicit absence.

use std::path::{Path, PathBuf};

pub fn tree_for(sources: &Path, name: &str) -> Option<PathBuf> {
    let tree = sources.join(name.split('-').next()?);
    match tree.is_dir() {
        true => Some(tree),
        false => None,
    }
}
