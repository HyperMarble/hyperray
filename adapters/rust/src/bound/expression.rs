// Compiler-derived expressions describe exit conditions for diagnostics.
// Unknown data stays visible and cannot establish a bound.

use crate::mir::{Operation, Projection, Scalar};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum Expression {
    Scalar {
        value: Scalar,
    },
    Input {
        index: usize,
        local: usize,
    },
    Local {
        local: usize,
    },
    Length {
        value: Box<Expression>,
    },
    Binary {
        operation: Operation,
        left: Box<Expression>,
        right: Box<Expression>,
        checked: bool,
    },
    Call {
        function: Option<String>,
        arguments: Vec<Expression>,
    },
    Projection {
        value: Box<Expression>,
        projection: Vec<Projection>,
    },
    Unknown {
        reason: String,
    },
}
