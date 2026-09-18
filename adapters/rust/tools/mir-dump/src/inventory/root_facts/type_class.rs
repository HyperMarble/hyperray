// Source type classes preserve compiler identity independently from layout ABI.
// They never reduce ADTs, references, or pointers to scalar machine values.

use rustc_public::ty::{RigidTy, Ty, TyKind};

pub(super) fn name(ty: Ty) -> String {
    match ty.kind() {
        TyKind::RigidTy(rigid) => rigid_name(&rigid),
        TyKind::Alias(kind, _) => format!("alias:{kind:?}").to_ascii_lowercase(),
        TyKind::Param(_) => "param".to_string(),
        TyKind::Bound(_, _) => "bound".to_string(),
    }
}

fn rigid_name(ty: &RigidTy) -> String {
    match ty {
        RigidTy::Bool => "bool".to_string(),
        RigidTy::Char => "char".to_string(),
        RigidTy::Int(value) => format!("int:{value:?}").to_ascii_lowercase(),
        RigidTy::Uint(value) => format!("uint:{value:?}").to_ascii_lowercase(),
        RigidTy::Float(value) => format!("float:{value:?}").to_ascii_lowercase(),
        RigidTy::Adt(definition, _) => format!("adt:{:?}", definition.kind()).to_ascii_lowercase(),
        RigidTy::Foreign(_) => "foreign".to_string(),
        RigidTy::Str => "str".to_string(),
        RigidTy::Array(_, _) => "array".to_string(),
        RigidTy::Pat(_, _) => "pattern".to_string(),
        RigidTy::Slice(_) => "slice".to_string(),
        RigidTy::RawPtr(_, _) => "raw_pointer".to_string(),
        RigidTy::Ref(_, _, _) => "reference".to_string(),
        RigidTy::FnDef(_, _) | RigidTy::FnPtr(_) => "function".to_string(),
        RigidTy::Closure(_, _) => "closure".to_string(),
        RigidTy::Coroutine(_, _) | RigidTy::CoroutineClosure(_, _) => "coroutine".to_string(),
        RigidTy::Dynamic(_, _) => "dynamic".to_string(),
        RigidTy::Never => "never".to_string(),
        RigidTy::Tuple(_) => "tuple".to_string(),
        RigidTy::CoroutineWitness(_, _) => "coroutine_witness".to_string(),
    }
}
