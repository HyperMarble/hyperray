// Each manifest function joined to what the compiler said about it, by
// file and line. A function the compiler never mentioned is `Missing`; a
// file it never saw is `FileNotSeen`. The item path is the compiler's own.

use super::locate::Located;
use super::seen::{Body, Seen};
use serde::Serialize;

#[derive(Debug, Serialize, PartialEq, Eq)]
pub enum Status {
    Extracted,
    Missing,
    FileNotSeen,
}

#[derive(Debug, Serialize, PartialEq, Eq)]
pub struct Joined {
    pub path: String,
    pub name: String,
    pub start_line: u32,
    pub item_path: Option<String>,
    pub status: Status,
}

pub fn join(functions: &[Located], seen: &[Seen]) -> Vec<Joined> {
    functions
        .iter()
        .map(|function| {
            let hit = hit_for(function, seen);
            Joined {
                path: function.path.clone(),
                name: function.name.clone(),
                start_line: function.start_line,
                item_path: hit.and_then(|s| s.item_path.clone()),
                status: status_of(function, seen, hit),
            }
        })
        .collect()
}

// The compiler's span covers the whole body and the manifest's starts at
// the `fn` line, so containment is the match, not equality. Paths are both
// relative to the tree root and must match exactly.
fn hit_for<'a>(function: &Located, seen: &'a [Seen]) -> Option<&'a Seen> {
    seen.iter().find(|s| {
        s.path == function.path
            && s.start_line <= function.start_line
            && s.end_line >= function.end_line
    })
}

fn status_of(function: &Located, seen: &[Seen], hit: Option<&Seen>) -> Status {
    match hit.map(|s| &s.body) {
        Some(Body::Extracted) => Status::Extracted,
        Some(Body::NoBody) => Status::Missing,
        None if seen.iter().any(|s| s.path == function.path) => Status::Missing,
        None => Status::FileNotSeen,
    }
}
