// Charon's output read back: every function and global it saw, each
// with its file and line. No name is built here; the compiler wrote them.

use super::global::{global, Global};
use super::span::plain_path;
use super::ullbc::{Decl, Output};
use serde::Serialize;
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub enum Body {
    Extracted,
    Refused(String),
    NotRequested,
}

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct Seen {
    pub path: String,
    pub start_line: u32,
    pub end_line: u32,
    pub item_path: Option<String>,
    pub body: Body,
}

pub struct Read {
    pub functions: Vec<Seen>,
    pub globals: Vec<Global>,
}

pub fn seen_in(ullbc: std::fs::File) -> Result<Read, serde_json::Error> {
    let reader = std::io::BufReader::new(ullbc);
    let output: Output = serde_json::from_reader(reader)?;
    let files = local_files(&output);
    let translated = output.translated;
    let functions = translated.fun_decls.into_iter().flatten();
    let globals = translated.global_decls.into_iter().flatten();
    Ok(Read {
        functions: functions.filter_map(|d| function(&files, d)).collect(),
        globals: globals.filter_map(|d| global(&files, d)).collect(),
    })
}

fn local_files(output: &Output) -> HashMap<u32, String> {
    output
        .translated
        .files
        .iter()
        .filter_map(|file| Some((file.id, file.name.get("Local")?.clone())))
        .collect()
}

fn function(files: &HashMap<u32, String>, decl: Decl) -> Option<Seen> {
    let span = decl.item_meta.span.data;
    Some(Seen {
        path: files.get(&span.file_id)?.clone(),
        start_line: span.beg.line,
        end_line: span.end.line,
        item_path: plain_path(&decl.item_meta.name),
        body: super::body::of(&decl.body),
    })
}
