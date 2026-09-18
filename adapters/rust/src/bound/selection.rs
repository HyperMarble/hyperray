// Autoharness selection records both generated harnesses and explicit skips.
// A skipped function must never disappear from the Stage 3 evidence.

use super::metadata::CrateMetadata;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::BTreeMap;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct AutomaticSelection {
    pub crate_name: String,
    pub chosen: Vec<String>,
    pub skipped: BTreeMap<String, Value>,
}

pub(crate) fn all(metadata: &[CrateMetadata]) -> Vec<AutomaticSelection> {
    let mut selections: Vec<AutomaticSelection> = metadata.iter().filter_map(selection).collect();
    selections.sort_by(|left, right| left.crate_name.cmp(&right.crate_name));
    selections
}

fn selection(metadata: &CrateMetadata) -> Option<AutomaticSelection> {
    let source = metadata.autoharness.as_ref()?;
    let mut chosen = source.chosen.clone();
    chosen.sort();
    Some(AutomaticSelection {
        crate_name: metadata.crate_name.clone(),
        chosen,
        skipped: source.skipped.clone(),
    })
}
