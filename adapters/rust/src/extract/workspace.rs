// The crate a patched file belongs to: the nearest ancestor directory
// with a Cargo.toml. That is cargo's own rule for finding a package.

use std::path::{Path, PathBuf};

pub fn crate_dir(root: &Path, patched_file: &str) -> Option<PathBuf> {
    let mut dir = root.join(patched_file);
    while dir.pop() {
        if dir.join("Cargo.toml").is_file() {
            return Some(dir);
        }
        if dir == root {
            return None;
        }
    }
    None
}
