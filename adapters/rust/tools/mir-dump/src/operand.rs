// MIR operands converted to places, scalar values, or compiler function
// names. Unsupported operands remain explicit.

use crate::{place, scalar};
use hyperray_rust::mir::Operand;
use rustc_public::crate_def::CrateDef;
use rustc_public::mir;
use rustc_public::ty::{RigidTy, TyKind};

pub fn of(value: &mir::Operand) -> Operand {
    match value {
        mir::Operand::Copy(place) | mir::Operand::Move(place) => Operand::Place(place::of(place)),
        mir::Operand::Constant(held) => constant(held),
        mir::Operand::RuntimeChecks(_) => Operand::Other,
    }
}

pub fn function_name(value: &mir::Operand) -> Option<String> {
    let mir::Operand::Constant(held) = value else {
        return None;
    };
    match held.const_.ty().kind() {
        TyKind::RigidTy(RigidTy::FnDef(definition, _)) => Some(definition.name().to_string()),
        _ => None,
    }
}

fn constant(held: &mir::ConstOperand) -> Operand {
    if let Some(name) = function_name(&mir::Operand::Constant(held.clone())) {
        return Operand::Function(name);
    }
    scalar::of(&held.const_).map_or(Operand::Other, Operand::Scalar)
}
