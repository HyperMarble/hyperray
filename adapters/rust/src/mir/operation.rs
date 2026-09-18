// Every binary MIR operator in the pinned compiler. A compiler update that
// adds an operator must cause the driver match to fail at build time.

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(rename_all = "snake_case")]
pub enum Operation {
    Add,
    AddUnchecked,
    Subtract,
    SubtractUnchecked,
    Multiply,
    MultiplyUnchecked,
    Divide,
    Remainder,
    BitXor,
    BitAnd,
    BitOr,
    ShiftLeft,
    ShiftLeftUnchecked,
    ShiftRight,
    ShiftRightUnchecked,
    Equal,
    Less,
    LessEqual,
    NotEqual,
    GreaterEqual,
    Greater,
    Compare,
    Offset,
}

impl Operation {
    pub fn is_comparison(self) -> bool {
        matches!(
            self,
            Self::Equal
                | Self::Less
                | Self::LessEqual
                | Self::NotEqual
                | Self::GreaterEqual
                | Self::Greater
        )
    }
}
