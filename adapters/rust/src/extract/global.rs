// A global Charon saw: a const or static, with its file, line, and the
// source text Charon kept. This is where a bound's constant comes from.

use super::span::ItemMeta;
use super::ullbc::GlobalDecl;
use serde::Serialize;
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct Global {
    pub path: String,
    pub start_line: u32,
    pub source_text: Option<String>,
}

pub fn global(files: &HashMap<u32, String>, decl: GlobalDecl) -> Option<Global> {
    let ItemMeta {
        span, source_text, ..
    } = decl.item_meta;
    Some(Global {
        path: files.get(&span.data.file_id)?.clone(),
        start_line: span.data.beg.line,
        source_text,
    })
}
