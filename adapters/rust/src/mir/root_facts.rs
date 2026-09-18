// Owned compiler facts describe ABI inputs without claiming valid machine roots.
// The types retain source identity, target identity, and unresolved lowering details.

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct RootFacts {
    pub version: u32,
    pub target: String,
    pub endian: String,
    pub pointer_bits: usize,
    pub convention: String,
    pub fixed_count: u32,
    pub c_variadic: bool,
    pub requires_caller_location: bool,
    pub args: Vec<ArgumentFact>,
    pub ret: ArgumentFact,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ArgumentFact {
    pub source_type_class: String,
    pub size_bits: usize,
    pub align_bytes: usize,
    pub pass_mode: String,
    pub layout_abi: String,
    pub scalar_components: Vec<ScalarFact>,
    pub unresolved: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ScalarFact {
    pub kind: String,
    pub primitive: String,
    pub width_bits: usize,
    pub signed: Option<bool>,
    pub address_space: Option<u32>,
    pub valid_range: Option<WrappingRange>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct WrappingRange {
    pub start: String,
    pub end: String,
}
