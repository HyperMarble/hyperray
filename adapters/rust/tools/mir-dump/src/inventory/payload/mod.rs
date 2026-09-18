// Build structural body records while keeping unsupported nested values opaque.

mod kind;

use rustc_public::mir::{BasicBlock, Body, Statement, Terminator};
use serde_json::{json, Value};

pub(super) fn body(value: &Body) -> Value {
    json!({
        "blocks": value.blocks.iter().enumerate().map(block).collect::<Vec<_>>(),
        "locals": opaque("locals", value.locals()),
        "arg_count": value.arg_locals().len(),
        "spread_arg": opaque("spread_arg", &value.spread_arg()),
        "span": opaque("span", &value.span),
        "var_debug_info": opaque("var_debug_info", &value.var_debug_info),
        "source_scopes": opaque("source_scopes_unexposed_by_public_api", value),
    })
}

pub(super) fn statement(value: &Statement) -> Value {
    json!({
        "statement_kind": kind::statement(&value.kind),
        "source_info": opaque("source_info", &value.source_info),
    })
}

pub(super) fn terminator(value: &Terminator) -> Value {
    json!({
        "terminator_kind": kind::terminator(&value.kind),
        "source_info": opaque("source_info", &value.source_info),
    })
}

fn block(indexed: (usize, &BasicBlock)) -> Value {
    let (index, value) = indexed;
    json!({
        "index": index,
        "statements": value.statements.iter().map(statement).collect::<Vec<_>>(),
        "terminator": terminator(&value.terminator),
    })
}

pub(super) fn opaque<T: std::fmt::Debug + ?Sized>(tag: &str, value: &T) -> Value {
    json!({"tag": tag, "opaque_debug": format!("{value:?}")})
}
