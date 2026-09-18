// Compiler items and bodies in MIR schema 4. Every field is public so an
// external adapter caller can construct and examine the boundary.

use super::{Block, CompilerInstance, MirType};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Dump {
    pub schema_version: u32,
    pub crate_name: String,
    pub items: Vec<Item>,
    pub instances: Vec<CompilerInstance>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Item {
    pub name: String,
    pub parent: Option<String>,
    pub kind: Kind,
    pub file: String,
    pub start_line: u32,
    pub end_line: u32,
    pub value: Option<String>,
    pub inputs: Vec<Input>,
    pub body: Option<Body>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum Kind {
    Function,
    Static,
    Constant,
    Constructor,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Input {
    pub index: usize,
    pub local: usize,
    pub ty: MirType,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Body {
    pub local_count: usize,
    pub blocks: Vec<Block>,
}
