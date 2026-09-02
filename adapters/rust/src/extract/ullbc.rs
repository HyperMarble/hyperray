// The parts of Charon's ULLBC JSON this stage reads. Fields not listed
// are ignored on purpose; nothing here interprets them.

use serde::Deserialize;
use std::collections::HashMap;

#[derive(Deserialize)]
pub struct Output {
    pub translated: Translated,
}

#[derive(Deserialize)]
pub struct Translated {
    pub files: Vec<File>,
    pub fun_decls: Vec<Option<Decl>>,
}

#[derive(Deserialize)]
pub struct File {
    pub id: u32,
    pub name: HashMap<String, String>,
}

#[derive(Deserialize)]
pub struct Decl {
    pub item_meta: ItemMeta,
    pub body: serde_json::Value,
}

#[derive(Deserialize)]
pub struct ItemMeta {
    pub span: Span,
}

#[derive(Deserialize)]
pub struct Span {
    pub data: SpanData,
}

#[derive(Deserialize)]
pub struct SpanData {
    pub file_id: u32,
    pub beg: Position,
    pub end: Position,
}

#[derive(Deserialize)]
pub struct Position {
    pub line: u32,
}
