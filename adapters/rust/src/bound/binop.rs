// Charon's binary operators. Shape from
// charon/src/ast/bodies/expressions.rs:158 (BinOp), :128 (OverflowMode).
// Every variant is named; six are comparisons, the rest are not read.

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum BinOp {
    BitXor,
    BitAnd,
    BitOr,
    Eq,
    Lt,
    Le,
    Ne,
    Ge,
    Gt,
    Add(OverflowMode),
    Sub(OverflowMode),
    Mul(OverflowMode),
    Div(OverflowMode),
    Rem(OverflowMode),
    AddChecked,
    SubChecked,
    MulChecked,
    Shl(OverflowMode),
    Shr(OverflowMode),
    Offset,
    Cmp,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OverflowMode {
    Panic,
    UB,
    Wrap,
}

impl BinOp {
    pub fn is_comparison(self) -> bool {
        matches!(
            self,
            BinOp::Eq | BinOp::Lt | BinOp::Le | BinOp::Ne | BinOp::Ge | BinOp::Gt
        )
    }
}
