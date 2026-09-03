// One input's size class, read from Charon's TyKind. `Sized` means the
// prover needs a limit; `Fixed` means the type's width is the limit;
// `Unbuildable` is pile 4, named by the TyKind that caused it. ADTs are
// walked in adt.rs.

use super::adt::adt_size;
use super::decl::TypeDecl;
use super::ty::{DeBruijnVar, Ty, TyKind};
use serde::Serialize;

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub enum Size {
    Fixed,
    Sized,
    Unbuildable(String),
}

// `args` are the type arguments of the ADT whose field is being read;
// empty at the top level, where a TypeVar is the function's own generic.
pub fn of(ty: &Ty, decls: &[Option<TypeDecl>], args: &[Ty], depth: u8) -> Size {
    match &ty.kind {
        TyKind::Literal(_) | TyKind::Never => Size::Fixed,
        TyKind::Ref(inner) => of(&inner.1, decls, args, depth),
        TyKind::RawPtr(inner) => of(&inner.0, decls, args, depth),
        TyKind::Slice(_) | TyKind::Array(_) | TyKind::Pattern(_) => Size::Sized,
        TyKind::Adt(adt) => adt_size(adt, decls, depth),
        TyKind::TypeVar(var) => type_var(var, decls, args, depth),
        TyKind::DynTrait(_) => Size::Unbuildable("DynTrait".into()),
        TyKind::FnPtr(_) => Size::Unbuildable("FnPtr".into()),
        TyKind::FnDef(_) => Size::Unbuildable("FnDef".into()),
        TyKind::TraitType(_) => Size::Unbuildable("TraitType".into()),
        TyKind::PtrMetadata(_) => Size::Unbuildable("PtrMetadata".into()),
        TyKind::Error(msg) => Size::Unbuildable(format!("Error: {msg}")),
    }
}

// A field typed `Bound(0, n)` is the ADT's n-th parameter; the caller's
// n-th type argument stands in for it. Any other variable is the
// function's own generic, which the prover cannot build.
fn type_var(var: &DeBruijnVar, decls: &[Option<TypeDecl>], args: &[Ty], depth: u8) -> Size {
    match var {
        DeBruijnVar::Bound(0, index) => match args.get(*index as usize) {
            Some(actual) => of(actual, decls, &[], depth),
            None => Size::Unbuildable("TypeVar".into()),
        },
        DeBruijnVar::Bound(..) | DeBruijnVar::Free(_) => Size::Unbuildable("TypeVar".into()),
    }
}
