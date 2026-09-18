// Compiler instances identify every item that contributes to code generation.
// They never merge two monomorphized functions by their source name.

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct CompilerInstance {
    pub id: String,
    pub symbol: String,
    pub name: String,
    pub kind: CompilerInstanceKind,
    pub generic_arguments: Vec<String>,
    pub type_signature: String,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum CompilerInstanceKind {
    Function,
    Static,
    GlobalAssembly,
    Intrinsic,
    LlvmIntrinsic,
    Virtual,
    Shim,
}
