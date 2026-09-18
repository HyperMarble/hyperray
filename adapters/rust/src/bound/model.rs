// The public Stage 3 result binds compiler rows to generated proof models.
// It records the build inputs needed to audit the exact model boundary.

use super::{AutomaticSelection, ContractedFunction, HarnessModel, Row};
use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum HarnessMode {
    Declared,
    Automatic,
}

pub struct Scope<'a> {
    pub crate_dir: &'a Path,
    pub output_dir: &'a Path,
    pub cargo: &'a Path,
    pub cbmc: &'a Path,
    pub harness_mode: HarnessMode,
    pub kani_arguments: &'a [String],
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ToolVersions {
    pub kani: String,
    pub cbmc: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct Model {
    pub rows: Vec<Row>,
    pub harnesses: Vec<HarnessModel>,
    pub contracts: Vec<ContractedFunction>,
    pub automatic_selections: Vec<AutomaticSelection>,
    pub tools: ToolVersions,
    pub artifact_dir: PathBuf,
    pub harness_mode: HarnessMode,
    pub kani_arguments: Vec<String>,
}
