// These types read the Kani metadata fields that define proof coverage.
// Unknown Kani fields may be added, but required fields must remain present.

use serde::Deserialize;
use serde_json::Value;
use std::collections::BTreeMap;
use std::path::PathBuf;

#[derive(Debug, Deserialize)]
pub struct CrateMetadata {
    #[serde(skip)]
    pub source: PathBuf,
    pub crate_name: String,
    pub proof_harnesses: Vec<HarnessMetadata>,
    pub test_harnesses: Vec<HarnessMetadata>,
    pub contracted_functions: Vec<ContractMetadata>,
    pub unsupported_features: Vec<Value>,
    #[serde(default, rename = "autoharness_md")]
    pub autoharness: Option<AutomaticMetadata>,
}

#[derive(Debug, Deserialize)]
pub struct HarnessMetadata {
    pub pretty_name: String,
    pub crate_name: String,
    pub original_file: String,
    pub original_start_line: usize,
    pub original_end_line: usize,
    pub goto_file: Option<PathBuf>,
    pub attributes: Value,
    pub contract: Option<Value>,
    pub has_loop_contracts: bool,
    pub is_automatically_generated: bool,
}

#[derive(Debug, Deserialize)]
pub struct AutomaticMetadata {
    pub chosen: Vec<String>,
    pub skipped: BTreeMap<String, Value>,
}

#[derive(Debug, Deserialize)]
pub struct ContractMetadata {
    pub function: String,
    pub file: String,
    pub harnesses: Vec<String>,
}
