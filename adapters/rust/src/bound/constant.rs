// Charon's constant expressions as this stage reads them. Shapes from
// charon/src/ast/bodies/values.rs:47 (ConstantExprKind), :152
// (Literal), :185 (ScalarValue). Every variant is named; only `Literal`
// and `Global` carry data this stage reads.

use serde::de::IgnoredAny;
use serde::Deserialize;

// `{"Untagged": [kind, ty]}` — a `ConstantExpr` is `{kind, ty}` and
// Charon's serializer wraps it as an untagged pair.
#[derive(Deserialize)]
pub struct ConstantExpr {
    #[serde(rename = "Untagged")]
    pub inner: (ConstantExprKind, IgnoredAny),
}

#[derive(Deserialize)]
pub enum ConstantExprKind {
    Literal(Literal),
    Adt(IgnoredAny, IgnoredAny),
    Array(IgnoredAny),
    Global(GlobalRef),
    TraitConst(IgnoredAny, IgnoredAny),
    VTableRef(IgnoredAny),
    Discriminant(IgnoredAny, IgnoredAny),
    Ref(IgnoredAny, IgnoredAny),
    Ptr(IgnoredAny, IgnoredAny, IgnoredAny),
    Var(IgnoredAny),
    Call(IgnoredAny, IgnoredAny),
    FnDef(IgnoredAny),
    FnPtr(IgnoredAny),
    SizeOf(IgnoredAny),
    AlignOf(IgnoredAny),
    TypeId(IgnoredAny),
    PtrNoProvenance(IgnoredAny),
    RawMemory(IgnoredAny),
    Opaque(String),
}

#[derive(Deserialize)]
pub struct GlobalRef {
    pub id: u32,
}

#[derive(Deserialize)]
pub enum Literal {
    Scalar(Scalar),
    Float(IgnoredAny),
    Bool(bool),
    Char(char),
    ByteStr(IgnoredAny),
    Str(String),
}

// `{"Unsigned": [<UIntTy>, "<digits>"]}`; Charon writes the number as a
// string (`scalar_value_ser_de`, values.rs:186).
#[derive(Deserialize)]
pub enum Scalar {
    Unsigned(String, String),
    Signed(String, String),
}

impl Scalar {
    pub fn text(&self) -> &str {
        match self {
            Scalar::Unsigned(_, n) | Scalar::Signed(_, n) => n,
        }
    }
}
