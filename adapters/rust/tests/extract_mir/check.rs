// MIR checks require named functions, placed globals, and a real source tree.

use crate::extract_mir_read::Checked;
use hyperray_rust::extract::{Global, Joined, Status};
use std::path::{Path, PathBuf};

fn answered(name: &str, function: &Joined) {
    let seen = function.status != Status::FileNotSeen;
    assert!(seen, "{name}: a patched file never reached the compiler");
    let path = match &function.item_path {
        Some(path) => path.as_str(),
        None => "",
    };
    let named = path.is_empty() || path.ends_with(&function.name);
    assert!(named, "{path} does not end in {}", function.name);
}

pub fn checked(name: &str, read: &Checked, ran: &mut u32) {
    assert!(!read.joined.is_empty(), "{name}: no rows");
    read.joined.iter().for_each(|row| answered(name, row));
    read.globals.iter().for_each(|global| placed(name, global));
    *ran += 1;
}

fn placed(name: &str, global: &Global) {
    assert!(!global.path.is_empty(), "{name}: a global with no file");
    assert!(global.start_line >= 1, "{name}: {}", global.path);
}

pub fn tree_for(sources: &Path, name: &str) -> Option<PathBuf> {
    let tree = sources.join(name.split('-').next()?);
    match tree.is_dir() {
        true => Some(tree),
        false => None,
    }
}
