// Charon's item metadata: where an item sits in which file, and its
// source text when Charon kept it. Shape from charon/src/ast/meta.rs.

use serde::Deserialize;

#[derive(Deserialize)]
pub struct ItemMeta {
    pub span: Span,
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
