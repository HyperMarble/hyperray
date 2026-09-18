// Normal MIR terminators retain the facts needed to reconstruct control
// flow. Unwind edges must not enter this boundary.

use super::{Operand, Place};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Branch {
    pub value: String,
    pub target: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum Terminator {
    Goto {
        target: usize,
    },
    Switch {
        discriminant: Operand,
        branches: Vec<Branch>,
        otherwise: usize,
    },
    Call {
        function: Option<String>,
        arguments: Vec<Operand>,
        destination: Place,
        target: Option<usize>,
    },
    Assert {
        condition: Operand,
        expected: bool,
        target: usize,
    },
    Drop {
        target: usize,
    },
    InlineAsm {
        target: Option<usize>,
    },
    End,
}
