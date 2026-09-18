// MIR blocks keep assignments and their source locations. Unsupported
// statements must remain visible in their rvalue.

use super::{Place, Rvalue, Terminator};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Block {
    pub statements: Vec<Statement>,
    pub terminator: Terminator,
    pub terminator_line: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Statement {
    pub target: Place,
    pub value: Rvalue,
    pub line: u32,
}
