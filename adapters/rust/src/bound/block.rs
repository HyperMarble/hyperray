// Charon's ULLBC block terminators. Variants from
// charon/src/ast/bodies/unstructured.rs:108; every one is named so a
// new variant fails to parse instead of hiding a jump.

use super::stmt::Statement;
use serde::de::IgnoredAny;
use serde::Deserialize;

#[derive(Deserialize)]
pub struct Unstructured {
    pub body: Vec<Block>,
}

#[derive(Deserialize)]
pub struct Block {
    pub statements: Vec<Statement>,
    pub terminator: Terminator,
}

#[derive(Deserialize)]
pub struct Terminator {
    pub kind: TerminatorKind,
}

#[derive(Deserialize)]
pub enum TerminatorKind {
    Goto {
        target: u32,
    },
    // `branches` are block ids. `data.branches` and `data.fallback` are
    // BranchIds — indexes into this list — never block ids
    // (charon/src/ast/bodies.rs:149-152). Every block a switch can reach
    // is already in `branches`, so `data` is not read for control flow.
    Switch {
        branches: Vec<u32>,
        data: IgnoredAny,
    },
    Call {
        target: u32,
        on_unwind: u32,
        call: IgnoredAny,
    },
    Drop {
        target: u32,
        on_unwind: u32,
        kind: IgnoredAny,
        place: IgnoredAny,
        fn_ptr: IgnoredAny,
    },
    Assert {
        target: u32,
        on_unwind: u32,
        assert: IgnoredAny,
    },
    InlineAsm {
        targets: Vec<u32>,
        on_unwind: u32,
        asm: IgnoredAny,
    },
    Abort(IgnoredAny),
    Return,
    UnwindResume,
}
