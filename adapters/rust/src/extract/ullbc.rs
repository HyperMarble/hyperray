// The parts of Charon's ULLBC JSON this stage reads. Fields not listed
// are ignored on purpose; nothing here interprets them.

use super::span::ItemMeta;
use serde::de::IgnoredAny;
use serde::Deserialize;
use std::collections::HashMap;

#[derive(Deserialize)]
pub struct Output {
    pub translated: Translated,
}

#[derive(Deserialize)]
pub struct Translated {
    pub files: Vec<File>,
    pub fun_decls: Vec<Option<Decl>>,
    pub global_decls: Vec<Option<GlobalDecl>>,
}

#[derive(Deserialize)]
pub struct File {
    pub id: u32,
    pub name: HashMap<String, String>,
}

#[derive(Deserialize)]
pub struct Decl {
    pub item_meta: ItemMeta,
    pub body: BodyTag,
}

// A global (`const`/`static`) has a value, never a body. Only the
// metadata is read; the value expression is left to the prover.
#[derive(Deserialize)]
pub struct GlobalDecl {
    pub item_meta: ItemMeta,
}

// Every variant of Charon's `Body` (charon/src/ast/bodies.rs). Only the
// tag is kept; body contents are discarded while reading, so a whole
// crate file does not become a whole-crate tree in memory.
#[derive(Deserialize)]
pub enum BodyTag {
    Unstructured(IgnoredAny),
    Structured(IgnoredAny),
    TargetDispatch(IgnoredAny),
    Extern(IgnoredAny),
    Intrinsic(IgnoredAny),
    Opaque,
    Missing,
    Error(ErrorBody),
}

#[derive(Deserialize)]
pub struct ErrorBody {
    pub msg: String,
}
