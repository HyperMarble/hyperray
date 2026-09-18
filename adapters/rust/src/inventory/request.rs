// BuildRequest contains caller-selected compiler and linker inputs.
// It never contains claimed exit codes, output digests, or a reusable output path.

use serde::{Deserialize, Serialize};
use std::path::PathBuf;

use super::BuildError;

mod validation;

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ExternArtifact {
    pub name: String,
    pub path: PathBuf,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct BuildRequest {
    pub driver: PathBuf,
    pub linker: PathBuf,
    pub source: PathBuf,
    pub output_directory: PathBuf,
    pub target: String,
    pub compiler_flags: Vec<String>,
    pub linker_flags: Vec<String>,
    pub boundary_artifact: Option<PathBuf>,
    pub extern_artifacts: Vec<ExternArtifact>,
    pub sysroot: Option<PathBuf>,
}

impl BuildRequest {
    pub(crate) fn validate(&self) -> Result<(), BuildError> {
        validation::check(self)
    }
}
