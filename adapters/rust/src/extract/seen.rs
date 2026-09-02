// Charon's output read back: for each function it saw, the file, the
// line span, and whether the body came out or was refused and why. No
// name is built here; the compiler already wrote every one.

use super::ullbc::{BodyTag, Decl, Output};
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

pub fn seen_in(ullbc: std::fs::File) -> Result<Vec<Seen>, serde_json::Error> {
    let reader = std::io::BufReader::new(ullbc);
    let output: Output = serde_json::from_reader(reader)?;
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

// Charon's own tags, read back. `Missing` is Charon's word for a std
// body it never had; `Opaque` is one it was told to skip.
fn body_of(body: &BodyTag) -> Body {
    match body {
        BodyTag::Error(error) => Body::Refused(error.msg.clone()),
        BodyTag::Opaque | BodyTag::Missing => Body::NotRequested,
        BodyTag::Unstructured(_)
        | BodyTag::Structured(_)
        | BodyTag::TargetDispatch(_)
        | BodyTag::Extern(_)
        | BodyTag::Intrinsic(_) => Body::Extracted,
    }
}
