// Cargo inputs select library packages and explicit feature sets.
// The preparer must not guess manifests or silently add default features.
use super::paths::absolute_file;
use crate::FeatureSelection;
use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct CargoOptions {
    pub executable: PathBuf,
    pub subject: FeatureSelection,
    pub requirement: FeatureSelection,
}

impl CargoOptions {
    pub fn validate(&self, subject: &Path, requirement: &Path) -> Result<(), String> {
        absolute_file(&self.executable)?;
        if subject.file_name() != Some("Cargo.toml".as_ref())
            || requirement.file_name() != Some("Cargo.toml".as_ref())
        {
            return Err("Cargo sources must name package Cargo.toml files".into());
        }
        self.subject.validate()?;
        self.requirement.validate()
    }
}
