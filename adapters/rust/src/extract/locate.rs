// EXTRACT pass 2: every whole function a hunk touches, read from the
// repository through `Reader` so every opened file is on record. It reads
// only files the patch names; nothing here walks the tree.

use super::function::{overlapping, Function};
use super::reader::{Opened, Reader};
use super::Change;
use serde::Serialize;
use std::path::Path;

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct Located {
    pub path: String,
    pub name: String,
    pub start_line: u32,
    pub end_line: u32,
    pub text: String,
}

#[derive(Debug, Serialize, PartialEq, Eq)]
pub struct Manifest {
    pub functions: Vec<Located>,
    pub hunks_without_function: Vec<(String, u32)>,
    pub opened: Vec<Opened>,
}

pub fn manifest(root: &Path, changes: &[Change]) -> std::io::Result<Manifest> {
    let mut reader = Reader::new(root);
    let mut manifest = Manifest {
        functions: Vec::new(),
        hunks_without_function: Vec::new(),
        opened: Vec::new(),
    };
    for change in changes {
        let text = reader.read(&change.path, "patched file")?;
        for hunk in &change.hunks {
            let (first, last) = hunk.added_range;
            locate(&mut manifest, &change.path, &text, first, last);
        }
    }
    manifest
        .functions
        .dedup_by(|a, b| a.path == b.path && a.start_line == b.start_line);
    manifest.opened = reader.opened().to_vec();
    Ok(manifest)
}

fn locate(manifest: &mut Manifest, path: &str, text: &str, first: u32, last: u32) {
    let found = overlapping(text, first, last);
    if found.is_empty() {
        manifest
            .hunks_without_function
            .push((path.to_string(), first));
    }
    for function in found {
        manifest.functions.push(located(path, function));
    }
}

fn located(path: &str, function: Function) -> Located {
    Located {
        path: path.to_string(),
        name: function.name,
        start_line: function.start_line,
        end_line: function.end_line,
        text: function.text,
    }
}
