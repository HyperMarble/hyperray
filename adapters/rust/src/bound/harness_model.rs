// A harness model names the final linked GOTO artifact and its full loop set.
// Kani attributes remain visible because stubs change what a proof covers.

use super::GotoLoop;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::path::PathBuf;

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum HarnessClass {
    Proof,
    Test,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct HarnessModel {
    pub crate_name: String,
    pub harness: String,
    pub class: HarnessClass,
    pub original_file: String,
    pub original_start_line: usize,
    pub original_end_line: usize,
    pub automatically_generated: bool,
    pub attributes: Value,
    pub contract: Option<Value>,
    pub has_loop_contracts: bool,
    pub goto_file: PathBuf,
    pub loops: Vec<GotoLoop>,
}
