// Assignment values converted to the data-flow forms that Stage 3 reads.
// All other compiler values stay explicit.

use crate::{operand, operation, place};
use hyperray_rust::mir::Rvalue;
use rustc_public::mir::{self, UnOp};

pub fn of(value: &mir::Rvalue) -> Rvalue {
    match value {
        mir::Rvalue::Use(value, _) => Rvalue::Use {
            operand: operand::of(value),
        },
        mir::Rvalue::BinaryOp(op, left, right) => binary(op, left, right, false),
        mir::Rvalue::CheckedBinaryOp(op, left, right) => binary(op, left, right, true),
        mir::Rvalue::Len(value) => Rvalue::Length {
            place: place::of(value),
        },
        mir::Rvalue::Cast(_, value, _) => Rvalue::Cast {
            operand: operand::of(value),
        },
        mir::Rvalue::UnaryOp(UnOp::PtrMetadata, value) => Rvalue::Metadata {
            operand: operand::of(value),
        },
        mir::Rvalue::UnaryOp(op, value) => Rvalue::Unary {
            operation: format!("{op:?}"),
            operand: operand::of(value),
        },
        mir::Rvalue::Discriminant(value) => Rvalue::Discriminant {
            place: place::of(value),
        },
        mir::Rvalue::Aggregate(_, values) => Rvalue::Aggregate {
            operands: values.iter().map(operand::of).collect(),
        },
        mir::Rvalue::AddressOf(_, value)
        | mir::Rvalue::Ref(_, _, value)
        | mir::Rvalue::Reborrow(_, _, value) => Rvalue::Address {
            place: place::of(value),
        },
        mir::Rvalue::CopyForDeref(value) => Rvalue::Use {
            operand: hyperray_rust::mir::Operand::Place(place::of(value)),
        },
        mir::Rvalue::Repeat(_, _) | mir::Rvalue::ThreadLocalRef(_) => Rvalue::Other,
    }
}

fn binary(op: &mir::BinOp, left: &mir::Operand, right: &mir::Operand, checked: bool) -> Rvalue {
    Rvalue::Binary {
        operation: operation::of(op),
        left: operand::of(left),
        right: operand::of(right),
        checked,
    }
}
