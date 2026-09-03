// The JSON `mir-dump` leaves behind: one row per item the crate compiled.
// Field names are the driver's; nothing here interprets them.

use serde::Deserialize;

#[derive(Deserialize)]
pub struct Row {
    pub name: String,
    pub kind: Kind,
    pub file: String,
    pub start_line: u32,
    pub end_line: u32,
    pub has_body: bool,
    pub value: Option<u128>,
}

// Every `ItemKind` the compiler reports (rustc_public lib.rs:178), so an
// unseen kind fails to parse instead of passing as a function.
#[derive(Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum Kind {
    Fn,
    Static,
    Const,
    Ctor,
}
