// Phase A and B: every function Charon saw, sorted into one pile, with
// the constants its body compares against. The pile comes from the input
// sizes (size.rs) and a body cycle (edge.rs); the limits from limit.rs.
// Nothing here reads Rust text.

use super::decl::TypeDecl;
use super::edge::has_back_edge;
use super::limit::{limits_in, Limit};
use super::pile::{pile, Pile};
use super::size::{self, Size};
use super::ullbc::{local_files, Body, Decl, GlobalDecl, Output};
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
    pub limits: Vec<Limit>,
}

struct Tree<'a> {
    files: HashMap<u32, String>,
    types: &'a [Option<TypeDecl>],
    globals: &'a [Option<GlobalDecl>],
}

pub fn sort_all(output: &Output) -> Vec<Sorted> {
    let tree = Tree {
        files: local_files(output),
        types: &output.translated.type_decls,
        globals: &output.translated.global_decls,
    };
    let functions = output.translated.fun_decls.iter().flatten();
    functions.filter_map(|d| sorted(&tree, d)).collect()
}

// Two ADT levels are opened (`Option<Format>` → `Format` → its fields);
// deeper than that is reported sized.
fn sorted(tree: &Tree, decl: &Decl) -> Option<Sorted> {
    let span = &decl.item_meta.span.data;
    let inputs: Vec<Size> = decl
        .signature
        .inputs
        .iter()
        .map(|ty| size::of(ty, tree.types, &[], 2))
        .collect();
    let body = match &decl.body {
        Body::Unstructured(b) => Some(b),
        _ => None,
    };
    let charon_loop = body.is_some_and(has_back_edge);
    let limits = body.map_or_else(Vec::new, |b| limits_in(b, tree.globals, &tree.files));
    Some(Sorted {
        path: tree.files.get(&span.file_id)?.clone(),
        start_line: span.beg.line,
        end_line: span.end.line,
        charon_loop,
        pile: pile(charon_loop, &inputs),
        inputs,
        limits,
    })
}
