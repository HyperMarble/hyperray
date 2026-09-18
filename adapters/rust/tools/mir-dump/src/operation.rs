// Binary operators converted exhaustively from the pinned compiler to the
// stable schema.

use hyperray_rust::mir::Operation;
use rustc_public::mir::BinOp;

pub fn of(value: &BinOp) -> Operation {
    match value {
        BinOp::Add => Operation::Add,
        BinOp::AddUnchecked => Operation::AddUnchecked,
        BinOp::Sub => Operation::Subtract,
        BinOp::SubUnchecked => Operation::SubtractUnchecked,
        BinOp::Mul => Operation::Multiply,
        BinOp::MulUnchecked => Operation::MultiplyUnchecked,
        BinOp::Div => Operation::Divide,
        BinOp::Rem => Operation::Remainder,
        BinOp::BitXor => Operation::BitXor,
        BinOp::BitAnd => Operation::BitAnd,
        BinOp::BitOr => Operation::BitOr,
        BinOp::Shl => Operation::ShiftLeft,
        BinOp::ShlUnchecked => Operation::ShiftLeftUnchecked,
        BinOp::Shr => Operation::ShiftRight,
        BinOp::ShrUnchecked => Operation::ShiftRightUnchecked,
        BinOp::Eq => Operation::Equal,
        BinOp::Lt => Operation::Less,
        BinOp::Le => Operation::LessEqual,
        BinOp::Ne => Operation::NotEqual,
        BinOp::Ge => Operation::GreaterEqual,
        BinOp::Gt => Operation::Greater,
        BinOp::Cmp => Operation::Compare,
        BinOp::Offset => Operation::Offset,
    }
}
