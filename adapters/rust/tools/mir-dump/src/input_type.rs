// Structural rustc types become schema types without using memory layout.
// A bit width that does not fit the schema must return an error.

use hyperray_rust::mir::{MirType, ScalarKind};
use rustc_public::ty::{FloatTy, RigidTy, Ty, TyKind};

pub fn of(ty: Ty) -> Result<MirType, String> {
    match ty.kind() {
        TyKind::RigidTy(value) => rigid(value),
        TyKind::Param(_) => Ok(MirType::Param),
        TyKind::Alias(_, _) => Ok(MirType::Alias),
        TyKind::Bound(_, _) => Ok(MirType::Bound),
    }
}

fn rigid(ty: RigidTy) -> Result<MirType, String> {
    match ty {
        RigidTy::Bool => Ok(scalar(ScalarKind::Boolean, 1)),
        RigidTy::Char => Ok(scalar(ScalarKind::Character, 32)),
        RigidTy::Int(kind) => integer(ScalarKind::Signed, kind.num_bytes()),
        RigidTy::Uint(kind) => integer(ScalarKind::Unsigned, kind.num_bytes()),
        RigidTy::Float(kind) => Ok(scalar(ScalarKind::Float, float_bits(kind))),
        RigidTy::Array(element, length) => Ok(MirType::Array {
            length: length.eval_target_usize().ok(),
            element: Box::new(of(element)?),
        }),
        RigidTy::Slice(element) => Ok(MirType::Slice {
            element: Box::new(of(element)?),
        }),
        RigidTy::Str => Ok(MirType::Str),
        RigidTy::Ref(_, target, mutability) => Ok(MirType::Reference {
            mutable: matches!(mutability, rustc_public::mir::Mutability::Mut),
            target: Box::new(of(target)?),
        }),
        RigidTy::Tuple(fields) => Ok(MirType::Tuple {
            fields: fields.iter().copied().map(of).collect::<Result<_, _>>()?,
        }),
        RigidTy::Never => Ok(MirType::Never),
        RigidTy::Adt(_, _) => Ok(MirType::Adt),
        RigidTy::RawPtr(_, _) => Ok(MirType::RawPointer),
        RigidTy::FnDef(_, _) | RigidTy::FnPtr(_) | RigidTy::Closure(_, _) => Ok(MirType::Callable),
        RigidTy::Coroutine(_, _) | RigidTy::CoroutineClosure(_, _) => Ok(MirType::Coroutine),
        RigidTy::Foreign(_) | RigidTy::Dynamic(_, _) | RigidTy::CoroutineWitness(_, _) => {
            Ok(MirType::Opaque)
        }
        RigidTy::Pat(inner, _) => of(inner),
    }
}

fn integer(class: ScalarKind, bytes: usize) -> Result<MirType, String> {
    let bits = bytes
        .checked_mul(8)
        .ok_or_else(|| format!("integer width {bytes} bytes overflowed"))?;
    let bits = u16::try_from(bits).map_err(|_| format!("integer width {bits} does not fit u16"))?;
    Ok(scalar(class, bits))
}

fn scalar(class: ScalarKind, bits: u16) -> MirType {
    MirType::Scalar { class, bits }
}

fn float_bits(kind: FloatTy) -> u16 {
    match kind {
        FloatTy::F16 => 16,
        FloatTy::F32 => 32,
        FloatTy::F64 => 64,
        FloatTy::F128 => 128,
    }
}
