// Preserve every public MIR variant tag and opaque nested compiler metadata.

use super::opaque;
use rustc_public::mir::{StatementKind, TerminatorKind};
use serde_json::Value;

pub(super) fn statement(value: &StatementKind) -> Value {
    match value {
        StatementKind::Assign(place, rvalue) => opaque("Assign", &(place, rvalue)),
        StatementKind::FakeRead(cause, place) => opaque("FakeRead", &(cause, place)),
        StatementKind::SetDiscriminant {
            place,
            variant_index,
        } => opaque("SetDiscriminant", &(place, variant_index)),
        StatementKind::StorageLive(local) => opaque("StorageLive", local),
        StatementKind::StorageDead(local) => opaque("StorageDead", local),
        StatementKind::PlaceMention(place) => opaque("PlaceMention", place),
        StatementKind::AscribeUserType {
            place,
            projections,
            variance,
        } => opaque("AscribeUserType", &(place, projections, variance)),
        StatementKind::Coverage(value) => opaque("Coverage", value),
        StatementKind::Intrinsic(value) => opaque("Intrinsic", value),
        StatementKind::ConstEvalCounter => opaque("ConstEvalCounter", value),
        StatementKind::Nop => opaque("Nop", value),
    }
}

pub(super) fn terminator(value: &TerminatorKind) -> Value {
    match value {
        TerminatorKind::Goto { target } => opaque("Goto", target),
        TerminatorKind::SwitchInt { discr, targets } => opaque("SwitchInt", &(discr, targets)),
        TerminatorKind::Resume => opaque("Resume", value),
        TerminatorKind::Abort => opaque("Abort", value),
        TerminatorKind::Return => opaque("Return", value),
        TerminatorKind::Unreachable => opaque("Unreachable", value),
        TerminatorKind::Drop {
            place,
            target,
            unwind,
        } => opaque("Drop", &(place, target, unwind)),
        TerminatorKind::Call {
            func,
            args,
            destination,
            target,
            unwind,
        } => opaque("Call", &(func, args, destination, target, unwind)),
        TerminatorKind::Assert {
            cond,
            expected,
            msg,
            target,
            unwind,
        } => opaque("Assert", &(cond, expected, msg, target, unwind)),
        TerminatorKind::InlineAsm {
            template,
            operands,
            options,
            line_spans,
            destination,
            unwind,
        } => opaque(
            "InlineAsm",
            &(template, operands, options, line_spans, destination, unwind),
        ),
    }
}
