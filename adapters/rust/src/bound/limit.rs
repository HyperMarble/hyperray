// Phase B: the constant a body compares against. Each
// `Assign(_, BinaryOp(cmp, a, b))` with a `Const Global` on one side
// names a global by id; its evaluated value (`--consts values`) is the
// limit and its span the citation. No name is matched anywhere.

use super::binop::BinOp;
use super::block::Unstructured;
use super::constant::{ConstantExprKind, Literal};
use super::stmt::{Operand, Rvalue, StatementKind};
use super::ullbc::GlobalDecl;
use serde::Serialize;
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct Limit {
    pub value: String,
    pub compared_with: BinOp,
    pub path: String,
    pub line: u32,
    pub source: Option<String>,
}

pub fn limits_in(
    body: &Unstructured,
    globals: &[Option<GlobalDecl>],
    files: &HashMap<u32, String>,
) -> Vec<Limit> {
    let statements = body.body.iter().flat_map(|b| b.statements.iter());
    statements
        .filter_map(|s| compared_globals(&s.kind))
        .flat_map(|(op, ids)| ids.into_iter().map(move |id| (op, id)))
        .filter_map(|(op, id)| limit(id, op, globals, files))
        .collect()
}

fn compared_globals(kind: &StatementKind) -> Option<(BinOp, Vec<u32>)> {
    let StatementKind::Assign(_, Rvalue::BinaryOp(op, lhs, rhs)) = kind else {
        return None;
    };
    if !op.is_comparison() {
        return None;
    }
    let ids = [global_id(lhs), global_id(rhs)].into_iter().flatten();
    Some((*op, ids.collect()))
}

fn global_id(operand: &Operand) -> Option<u32> {
    let Operand::Const(constant) = operand else {
        return None;
    };
    match &constant.inner.0 {
        ConstantExprKind::Global(g) => Some(g.id),
        _ => None,
    }
}

// A global whose value is not an evaluated scalar (a static, an array, a
// call Charon could not fold) gives no limit; the caller reports that.
fn limit(
    id: u32,
    op: BinOp,
    globals: &[Option<GlobalDecl>],
    files: &HashMap<u32, String>,
) -> Option<Limit> {
    let decl = globals.get(id as usize)?.as_ref()?;
    let ConstantExprKind::Literal(Literal::Scalar(scalar)) = &decl.value.inner.0 else {
        return None;
    };
    let span = &decl.item_meta.span.data;
    Some(Limit {
        value: scalar.text().to_string(),
        compared_with: op,
        path: files.get(&span.file_id)?.clone(),
        line: span.beg.line,
        source: decl.item_meta.source_text.clone(),
    })
}
