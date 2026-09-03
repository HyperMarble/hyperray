// The size of an ADT input: fixed if every field is fixed, sized if any
// field is, unbuildable if any field is. `Opaque` (Charon did not
// translate the fields — `Vec` is one) is sized. `depth` counts ADT
// levels left to open; at 0 the walk stops and says sized.

use super::decl::{TypeDecl, TypeDeclKind};
use super::size::{of, Size};
use super::ty::{AdtRef, Ty};

pub fn adt_size(adt: &AdtRef, decls: &[Option<TypeDecl>], depth: u8) -> Size {
    let Some(Some(decl)) = decls.get(adt.id as usize) else {
        return Size::Unbuildable(format!("type {} not in type_decls", adt.id));
    };
    if depth == 0 {
        return Size::Sized;
    }
    let args = &adt.generics.types;
    match &decl.kind {
        TypeDeclKind::Opaque => Size::Sized,
        TypeDeclKind::Alias(ty) => of(ty, decls, args, depth - 1),
        TypeDeclKind::Struct(fields) | TypeDeclKind::Union(fields) => {
            fields_size(fields.iter().map(|f| &f.ty), decls, args, depth - 1)
        }
        TypeDeclKind::Enum(variants) => {
            let tys = variants.iter().flat_map(|v| v.fields.iter().map(|f| &f.ty));
            fields_size(tys, decls, args, depth - 1)
        }
        TypeDeclKind::Error(msg) => Size::Unbuildable(format!("Error: {msg}")),
    }
}

fn fields_size<'a>(
    tys: impl Iterator<Item = &'a Ty>,
    decls: &[Option<TypeDecl>],
    args: &[Ty],
    depth: u8,
) -> Size {
    let mut result = Size::Fixed;
    for ty in tys {
        match of(ty, decls, args, depth) {
            Size::Fixed => {}
            Size::Sized => result = Size::Sized,
            unbuildable => return unbuildable,
        }
    }
    result
}
