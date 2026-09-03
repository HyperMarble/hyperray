// Charon's type declarations as this stage reads them. Shape from
// charon/src/ast/items/type_decl.rs:56 (TypeDeclKind), :75 (Variant),
// :91 (Field). Every variant is named so an unseen one fails to parse.

use super::ty::Ty;
use serde::Deserialize;

#[derive(Deserialize)]
pub struct TypeDecl {
    pub kind: TypeDeclKind,
}

#[derive(Deserialize)]
pub enum TypeDeclKind {
    Struct(Vec<Field>),
    Enum(Vec<Variant>),
    Union(Vec<Field>),
    Opaque,
    Alias(Ty),
    Error(String),
}

#[derive(Deserialize)]
pub struct Variant {
    pub fields: Vec<Field>,
}

#[derive(Deserialize)]
pub struct Field {
    pub ty: Ty,
}
