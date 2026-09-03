// Phase A: every function Charon saw, sorted into one pile. The pile is
// decided by the input sizes (size.rs) and by whether the body has a
// cycle (edge.rs). Nothing here reads Rust text.

use super::decl::TypeDecl;
use super::edge::has_back_edge;
use super::pile::{pile, Pile};
use super::size::{self, Size};
use super::ullbc::{local_files, Body, Decl, Output};
use serde::Serialize;
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct Sorted {
    pub path: String,
    pub start_line: u32,
    pub end_line: u32,
    pub charon_loop: bool,
    pub inputs: Vec<Size>,
    pub pile: Pile,
}

pub fn sort_all(output: &Output) -> Vec<Sorted> {
    let files = local_files(output);
    let decls = &output.translated.type_decls;
    let functions = output.translated.fun_decls.iter().flatten();
    functions.filter_map(|d| sorted(&files, decls, d)).collect()
}

// Two ADT levels are opened (`Option<Format>` → `Format` → its fields);
// deeper than that is reported sized.
fn sorted(files: &HashMap<u32, String>, decls: &[Option<TypeDecl>], decl: &Decl) -> Option<Sorted> {
    let span = &decl.item_meta.span.data;
    let inputs: Vec<Size> = decl
        .signature
        .inputs
        .iter()
        .map(|ty| size::of(ty, decls, &[], 2))
        .collect();
    let charon_loop = matches!(&decl.body, Body::Unstructured(b) if has_back_edge(b));
    Some(Sorted {
        path: files.get(&span.file_id)?.clone(),
        start_line: span.beg.line,
        end_line: span.end.line,
        charon_loop,
        pile: pile(charon_loop, &inputs),
        inputs,
    })
}
