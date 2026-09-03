// Charon's type expressions as this stage reads them. Shape from
// charon/src/ast/type_level/types.rs:45 (TyKind), type_level.rs:34
// (GenericArgs), type_level/vars.rs:87 (DeBruijnVar). Every variant is
// named so an unseen one fails to parse.

use serde::de::IgnoredAny;
use serde::Deserialize;

#[derive(Deserialize)]
pub struct Signature {
    pub inputs: Vec<Ty>,
}

// Charon writes a `Ty` as `{"Untagged": <TyKind>}`.
#[derive(Deserialize)]
pub struct Ty {
    #[serde(rename = "Untagged")]
    pub kind: TyKind,
}

#[derive(Deserialize)]
pub enum TyKind {
    Adt(AdtRef),
    TypeVar(DeBruijnVar),
    Literal(IgnoredAny),
    Never,
    Ref(Box<(IgnoredAny, Ty, IgnoredAny)>),
    RawPtr(Box<(Ty, IgnoredAny)>),
    TraitType(IgnoredAny),
    DynTrait(IgnoredAny),
    FnPtr(IgnoredAny),
    FnDef(IgnoredAny),
    PtrMetadata(IgnoredAny),
    Array(IgnoredAny),
    Slice(IgnoredAny),
    Pattern(IgnoredAny),
    Error(String),
}

#[derive(Deserialize)]
pub struct AdtRef {
    pub id: u32,
    pub generics: GenericArgs,
}

#[derive(Deserialize)]
pub struct GenericArgs {
    pub types: Vec<Ty>,
}

// `Bound(depth, index)`: depth 0 is the innermost binder — for an ADT
// field, the ADT's own type parameters.
#[derive(Deserialize)]
pub enum DeBruijnVar {
    Bound(u32, u32),
    Free(u32),
}
