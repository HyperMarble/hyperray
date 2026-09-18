// What the compiler saw: every function and global it built, each with its
// file and line. No name is built here; the compiler wrote them.
//
// `Body` has two states, not three. A row the compiler did not answer for
// is not written at all, so a later stage cannot mistake our gap for the
// code's (design.md §2).

use super::global::Global;
use crate::mir::{self, Item, Kind, ReadError};
use serde::Serialize;

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub enum Body {
    Extracted,
    NoBody,
}

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct Seen {
    pub path: String,
    pub start_line: u32,
    pub end_line: u32,
    pub item_path: Option<String>,
    pub parent_path: Option<String>,
    pub body: Body,
}

pub struct Read {
    pub functions: Vec<Seen>,
    pub globals: Vec<Global>,
}

pub fn seen_in(dump: std::fs::File) -> Result<Read, ReadError> {
    let items = mir::read(dump)?.items;
    let (functions, globals): (Vec<Item>, Vec<Item>) = items.into_iter().partition(is_function);
    Ok(Read {
        functions: functions.into_iter().map(seen).collect(),
        globals: globals.into_iter().map(global).collect(),
    })
}

fn is_function(item: &Item) -> bool {
    item.kind == Kind::Function || item.kind == Kind::Constructor
}

fn seen(item: Item) -> Seen {
    Seen {
        path: item.file,
        start_line: item.start_line,
        end_line: item.end_line,
        item_path: Some(item.name),
        parent_path: item.parent,
        body: match item.body.is_some() {
            true => Body::Extracted,
            false => Body::NoBody,
        },
    }
}

fn global(item: Item) -> Global {
    Global {
        path: item.file,
        start_line: item.start_line,
        value: item.value,
    }
}
