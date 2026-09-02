// The router's input: each manifest function joined to what Charon said
// about it, by file and line. A function Charon never mentioned is
// `Missing`, and that is reported, never dropped.

use super::locate::Located;
use super::seen::{Body, Seen};
use serde::Serialize;

#[derive(Debug, Serialize, PartialEq, Eq)]
pub enum Status {
    Extracted,
    Refused(String),
    Missing,
}

#[derive(Debug, Serialize, PartialEq, Eq)]
pub struct Joined {
    pub path: String,
    pub name: String,
    pub start_line: u32,
    pub status: Status,
}

pub fn join(functions: &[Located], seen: &[Seen]) -> Vec<Joined> {
    functions
        .iter()
        .map(|function| Joined {
            path: function.path.clone(),
            name: function.name.clone(),
            start_line: function.start_line,
            status: status_of(function, seen),
        })
        .collect()
}

// Charon's span starts at the first attribute or the `fn` line and the
// manifest's at the `fn` line, so containment is the match, not equality.
fn status_of(function: &Located, seen: &[Seen]) -> Status {
    let hit = seen.iter().find(|s| {
        s.path.ends_with(&function.path)
            && s.start_line <= function.start_line
            && s.end_line >= function.end_line
    });
    match hit.map(|s| &s.body) {
        Some(Body::Extracted) => Status::Extracted,
        Some(Body::Refused(reason)) => Status::Refused(reason.clone()),
        Some(Body::NotRequested) | None => Status::Missing,
    }
}
