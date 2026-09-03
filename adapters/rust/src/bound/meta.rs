// Charon's item metadata, still read by stage 3 until it moves to the
// compiler's own MIR. Shape from charon/src/ast/meta.rs.

use serde::Deserialize;
use std::collections::HashMap;

#[derive(Deserialize)]
pub struct ItemMeta {
    pub span: Span,
    pub name: Vec<PathElem>,
    pub source_text: Option<String>,
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

// PathElem has five variants (names.rs:34); only the tag is read, so a
// name holding a non-Ident element is known to be not printable.
#[derive(Deserialize)]
pub struct PathElem(pub HashMap<String, serde_json::Value>);
