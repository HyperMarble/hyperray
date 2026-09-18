// MIR values retain the operands that establish data flow. An unsupported
// value stays explicit as Other.

use super::{Operation, Place, Scalar};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "kind", content = "value", rename_all = "snake_case")]
pub enum Operand {
    Place(Place),
    Scalar(Scalar),
    Function(String),
    Other,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum Rvalue {
    Use {
        operand: Operand,
    },
    Binary {
        operation: Operation,
        left: Operand,
        right: Operand,
        checked: bool,
    },
    Length {
        place: Place,
    },
    Metadata {
        operand: Operand,
    },
    Cast {
        operand: Operand,
    },
    Unary {
        operation: String,
        operand: Operand,
    },
    Discriminant {
        place: Place,
    },
    Aggregate {
        operands: Vec<Operand>,
    },
    Address {
        place: Place,
    },
    Other,
}
