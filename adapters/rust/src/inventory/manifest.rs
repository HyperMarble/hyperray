// BuildManifest binds compiler inventory, object output, and linked image.
// Input closure stays open when dep-info cannot establish runtime completeness.

use serde::{Deserialize, Serialize};
use std::path::PathBuf;

pub const MANIFEST_VERSION: u32 = 1;

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct ArtifactIdentity {
    pub path: PathBuf,
    pub sha256: String,
    pub size: u64,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct ToolIdentity {
    pub path: PathBuf,
    pub sha256: String,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct SysrootFileIdentity {
    pub path: PathBuf,
    pub sha256: String,
    pub size: u64,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct SysrootIdentity {
    pub path: PathBuf,
    pub target: String,
    pub toolchain: ToolIdentity,
    pub files: Vec<SysrootFileIdentity>,
    pub aggregate_sha256: String,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct BuildManifest {
    pub version: u32,
    pub compilation_id: String,
    pub target: String,
    pub driver: ToolIdentity,
    pub linker: ToolIdentity,
    pub source: ArtifactIdentity,
    pub sysroot: Option<SysrootIdentity>,
    pub boundary_artifact: Option<ArtifactIdentity>,
    pub extern_artifacts: Vec<ArtifactIdentity>,
    pub object: ArtifactIdentity,
    pub dep_info: ArtifactIdentity,
    pub inventory: ArtifactIdentity,
    pub elf: ArtifactIdentity,
    pub compiler_arguments: Vec<String>,
    pub linker_arguments: Vec<String>,
    pub unresolved_obligations: Vec<String>,
}
