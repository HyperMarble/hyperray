// Compiler inputs preserve the caller's build configuration.
// No missing tool or feature policy can receive an implicit substitute.
use crate::FeatureSelection;
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct CompileRequest {
    pub driver: PathBuf,
    pub cargo: PathBuf,
    pub crate_directory: PathBuf,
    pub output_directory: PathBuf,
    pub features: FeatureSelection,
}

impl CompileRequest {
    pub fn validate(&self) -> Result<(), String> {
        for tool in [&self.driver, &self.cargo] {
            absolute_file(tool)?;
        }
        if !self.crate_directory.is_absolute() || !self.output_directory.is_absolute() {
            return Err("compiler crate and output directories must be absolute".into());
        }
        absolute_file(&self.crate_directory.join("Cargo.toml"))?;
        self.features.validate()
    }
}

fn absolute_file(path: &std::path::Path) -> Result<(), String> {
    if !path.is_absolute() || !path.is_file() {
        return Err(format!("expected an absolute file: {}", path.display()));
    }
    Ok(())
}
