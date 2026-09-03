// The slice of Charon's ULLBC this stage reads: each function's
// signature and body blocks, and every type declaration's kind. Fields
// not listed are ignored while reading. Body variants from
// charon/src/ast/bodies.rs, as in extract/ullbc.rs.

use super::block::Unstructured;
use super::decl::TypeDecl;
use super::ty::Signature;
use crate::extract::span::ItemMeta;
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
    pub type_decls: Vec<Option<TypeDecl>>,
}

#[derive(Deserialize)]
pub struct File {
    pub id: u32,
    pub name: HashMap<String, String>,
}

#[derive(Deserialize)]
pub struct Decl {
    pub item_meta: ItemMeta,
    pub signature: Signature,
    pub body: Body,
}

#[derive(Deserialize)]
pub enum Body {
    Unstructured(Unstructured),
    Structured(IgnoredAny),
    TargetDispatch(IgnoredAny),
    Extern(IgnoredAny),
    Intrinsic(IgnoredAny),
    Opaque,
    Missing,
    Error(IgnoredAny),
}

pub fn read(ullbc: std::fs::File) -> Result<Output, serde_json::Error> {
    serde_json::from_reader(std::io::BufReader::new(ullbc))
}

pub fn local_files(output: &Output) -> HashMap<u32, String> {
    output
        .translated
        .files
        .iter()
        .filter_map(|file| Some((file.id, file.name.get("Local")?.clone())))
        .collect()
}
