// Scalar constants converted with compiler type information. Signed values
// keep their sign instead of becoming unsigned bit patterns.

use hyperray_rust::mir::{Scalar, ScalarKind};
use rustc_public::ty::{ConstantKind, MirConst, RigidTy, TyKind};

pub fn of(value: &MirConst) -> Option<Scalar> {
    let ConstantKind::Allocated(memory) = value.kind() else {
        return pointer_constant(value);
    };
    match value.ty().kind() {
        TyKind::RigidTy(RigidTy::Bool) => {
            held(memory.read_bool().ok()?.to_string(), ScalarKind::Boolean, 1)
        }
        TyKind::RigidTy(RigidTy::Char) => held(
            memory.read_uint().ok()?.to_string(),
            ScalarKind::Character,
            32,
        ),
        TyKind::RigidTy(RigidTy::Int(kind)) => held(
            memory.read_int().ok()?.to_string(),
            ScalarKind::Signed,
            integer_bits(kind.num_bytes())?,
        ),
        TyKind::RigidTy(RigidTy::Uint(kind)) => held(
            memory.read_uint().ok()?.to_string(),
            ScalarKind::Unsigned,
            integer_bits(kind.num_bytes())?,
        ),
        TyKind::RigidTy(RigidTy::Float(kind)) => held(
            memory.read_uint().ok()?.to_string(),
            ScalarKind::Float,
            float_bits(kind),
        ),
        _ => None,
    }
}

fn pointer_constant(value: &MirConst) -> Option<Scalar> {
    match value.ty().kind() {
        TyKind::RigidTy(RigidTy::Uint(rustc_public::ty::UintTy::Usize)) => held(
            value.eval_target_usize().ok()?.to_string(),
            ScalarKind::Unsigned,
            integer_bits(rustc_public::ty::UintTy::Usize.num_bytes())?,
        ),
        _ => None,
    }
}

fn held(value: String, class: ScalarKind, bits: u16) -> Option<Scalar> {
    Some(Scalar { value, class, bits })
}

fn integer_bits(bytes: usize) -> Option<u16> {
    let bits = bytes.checked_mul(8)?;
    u16::try_from(bits).ok()
}

fn float_bits(kind: rustc_public::ty::FloatTy) -> u16 {
    match kind {
        rustc_public::ty::FloatTy::F16 => 16,
        rustc_public::ty::FloatTy::F32 => 32,
        rustc_public::ty::FloatTy::F64 => 64,
        rustc_public::ty::FloatTy::F128 => 128,
    }
}
