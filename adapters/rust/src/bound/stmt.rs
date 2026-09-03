// The slice of a statement this stage reads: an `Assign` whose rvalue is
// a comparison with a constant on one side. Shapes from
// charon/src/ast/bodies/unstructured.rs:61 (StatementKind) and
// bodies/expressions.rs:24 (Rvalue), :107 (Operand). Every variant is
// named so an unseen one fails to parse instead of passing as "no
// constant here".

use super::binop::BinOp;
use super::constant::ConstantExpr;
use serde::de::IgnoredAny;
use serde::Deserialize;

#[derive(Deserialize)]
pub struct Statement {
    pub kind: StatementKind,
}

#[derive(Deserialize)]
pub enum StatementKind {
    Assign(IgnoredAny, Rvalue),
    SetDiscriminant(IgnoredAny, IgnoredAny),
    StorageLive(IgnoredAny),
    StorageDead(IgnoredAny),
    PlaceMention(IgnoredAny),
    Borrowck(IgnoredAny),
    Assert {
        assert: IgnoredAny,
        on_failure: IgnoredAny,
    },
    Nop,
}

#[derive(Deserialize)]
pub enum Rvalue {
    Use(IgnoredAny, IgnoredAny),
    Ref {
        place: IgnoredAny,
        kind: IgnoredAny,
        ptr_metadata: IgnoredAny,
    },
    RawPtr {
        place: IgnoredAny,
        kind: IgnoredAny,
        ptr_metadata: IgnoredAny,
    },
    BinaryOp(BinOp, Operand, Operand),
    UnaryOp(IgnoredAny, IgnoredAny),
    NullaryOp(IgnoredAny, IgnoredAny),
    Discriminant(IgnoredAny),
    Aggregate(IgnoredAny, IgnoredAny),
    Len(IgnoredAny, IgnoredAny, IgnoredAny),
    Repeat(IgnoredAny, IgnoredAny, IgnoredAny),
}

#[derive(Deserialize)]
pub enum Operand {
    Copy(IgnoredAny),
    Move(IgnoredAny),
    Const(ConstantExpr),
}
