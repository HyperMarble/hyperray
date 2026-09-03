// What the compiler saw: every function and global it built, each with its
// file and line. No name is built here; the compiler wrote them.
//
// `Body` has two states, not three. A row the compiler did not answer for
// is not written at all, so a later stage cannot mistake our gap for the
// code's (design.md §2).

use super::global::Global;
use super::mir::{Kind, Row};
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
    pub body: Body,
}

pub struct Read {
    pub functions: Vec<Seen>,
    pub globals: Vec<Global>,
}

pub fn seen_in(dump: std::fs::File) -> Result<Read, serde_json::Error> {
    let reader = std::io::BufReader::new(dump);
    let rows: Vec<Row> = serde_json::from_reader(reader)?;
    let (functions, globals): (Vec<Row>, Vec<Row>) = rows.into_iter().partition(is_function);
    Ok(Read {
        functions: functions.into_iter().map(seen).collect(),
        globals: globals.into_iter().map(global).collect(),
    })
}

fn is_function(row: &Row) -> bool {
    row.kind == Kind::Fn || row.kind == Kind::Ctor
}

fn seen(row: Row) -> Seen {
    Seen {
        path: row.file,
        start_line: row.start_line,
        end_line: row.end_line,
        item_path: Some(row.name),
        body: match row.has_body {
            true => Body::Extracted,
            false => Body::NoBody,
        },
    }
}

fn global(row: Row) -> Global {
    Global {
        path: row.file,
        start_line: row.start_line,
        value: row.value,
    }
}
