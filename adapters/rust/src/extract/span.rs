// Charon's item metadata: where an item sits in which file, its name,
// and its source text when Charon kept it. Shape from
// charon/src/ast/meta.rs and ast/meta/names.rs.

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

// PathElem has five variants (names.rs:34). Only `Ident` carries a plain
// string; the others are kept as their tag so a name with one of them
// is known to be not printable, never guessed.
#[derive(Deserialize)]
pub struct PathElem(pub HashMap<String, serde_json::Value>);

// `noodles_util::alignment::io::reader::builder::is_cram_file_definition_version`
// when every element is an Ident; None when any element is not.
pub fn plain_path(name: &[PathElem]) -> Option<String> {
    let mut parts = Vec::with_capacity(name.len());
    for elem in name {
        let ident = elem.0.get("Ident")?;
        parts.push(ident.get(0)?.as_str()?.to_string());
    }
    Some(parts.join("::"))
}
