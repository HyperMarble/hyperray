// Raw compiler inventory keeps every concrete instance and body position.
// It never replaces non-Assign statements with a synthetic semantic operation.

use serde::{Deserialize, Serialize};
use serde_json::Value;

use super::RootFacts;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct RawInventory {
    pub version: u32,
    pub compilation_id: String,
    pub instances: Vec<RawInstance>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct RawInstance {
    pub id: String,
    pub symbol: String,
    pub name: String,
    pub kind: String,
    pub generic_arguments: Vec<String>,
    pub abi: Option<Value>,
    pub abi_error: Option<String>,
    pub body_status: String,
    pub body: Option<RawBody>,
    pub root_obligations: Vec<String>,
    pub root_facts: Option<RootFacts>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct RawBody {
    pub phase: String,
    pub body_digest: String,
    pub positions: Vec<RawPosition>,
    pub payload: Value,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct RawPosition {
    pub block: usize,
    pub statement: Option<usize>,
    pub operation_id: String,
    pub payload: Value,
}
