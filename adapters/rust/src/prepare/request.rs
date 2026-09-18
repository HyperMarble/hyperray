// Public preparation inputs keep the program and its requirement separate.
// Unknown fields and unsafe source interpolation must remain errors.
use super::paths::{absolute_file, identifier};
use super::{CargoOptions, Limits};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct FunctionSource {
    pub source: PathBuf,
    pub function: String,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct Tools {
    pub rustc: PathBuf,
    pub spin: PathBuf,
    pub clang: PathBuf,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct Request {
    pub subject: FunctionSource,
    pub requirement: FunctionSource,
    pub tools: Tools,
    pub directory: PathBuf,
    pub optimization: u8,
    pub limits: Limits,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cargo: Option<CargoOptions>,
}

impl Request {
    pub fn validate(&self) -> Result<(), String> {
        self.limits.validate()?;
        self.subject.validate()?;
        self.requirement.validate()?;
        if let Some(cargo) = &self.cargo {
            cargo.validate(&self.subject.source, &self.requirement.source)?;
        }
        if !self.directory.is_absolute() || self.optimization > 3 {
            return Err("absolute output directory and optimization 0..3 are required".into());
        }
        for tool in [&self.tools.rustc, &self.tools.spin, &self.tools.clang] {
            absolute_file(tool)?;
        }
        Ok(())
    }
}

impl FunctionSource {
    fn validate(&self) -> Result<(), String> {
        absolute_file(&self.source)?;
        if !self.function.split("::").all(identifier) {
            return Err(format!("invalid function path: {}", self.function));
        }
        Ok(())
    }
}
