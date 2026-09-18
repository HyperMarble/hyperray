// The Stage 3 row for one manifest function. Inputs and compiler cycles stay
// visible without turning a missing optimization hint into a verdict.

use super::Loop;
use crate::mir::MirType;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Row {
    pub path: String,
    pub name: String,
    pub start_line: u32,
    pub end_line: u32,
    pub inputs: Vec<Input>,
    pub loops: Vec<Loop>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Input {
    pub index: usize,
    pub local: usize,
    pub ty: MirType,
    pub domain: Domain,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum Domain {
    Fixed,
    Dynamic,
    Unknown { reason: String },
}
