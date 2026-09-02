// Charon's output read back: for each function it saw, the file, the
// line span, and whether the body came out or was refused and why. No
// name is built here; the compiler already wrote every one.

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
    pub body: Body,
}

pub fn seen_in(ullbc_json: &str) -> Result<Vec<Seen>, serde_json::Error> {
    let output: Output = serde_json::from_str(ullbc_json)?;
    let files: HashMap<u32, String> = output
        .translated
        .files
        .into_iter()
        .filter_map(|file| Some((file.id, file.name.get("Local")?.clone())))
        .collect();
    Ok(output
        .translated
        .fun_decls
        .into_iter()
        .flatten()
        .filter_map(|decl| seen(&files, decl))
        .collect())
}

fn seen(files: &HashMap<u32, String>, decl: Decl) -> Option<Seen> {
    let span = decl.item_meta.span.data;
    Some(Seen {
        path: files.get(&span.file_id)?.clone(),
        start_line: span.beg.line,
        end_line: span.end.line,
        body: body_of(&decl.body),
    })
}

// A refused body is `{"Error": {"msg": …}}`; an unrequested one is the
// bare string `"Opaque"`; anything else carries the extracted body.
fn body_of(body: &serde_json::Value) -> Body {
    let refusal = body
        .get("Error")
        .and_then(|e| e.get("msg"))
        .and_then(|m| m.as_str());
    match refusal {
        Some(reason) => Body::Refused(reason.to_string()),
        None if body.is_string() => Body::NotRequested,
        None => Body::Extracted,
    }
}
