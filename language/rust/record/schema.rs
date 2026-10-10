// Purpose: the build record the adapter hands to the loader, as JSON.
// Never:   holds a value that was not read from the build or the machine.
use crate::build_facts::{
    CToolchain,
    Compiled,
    EnvVar,
    NativeCode, //
};
use serde::Serialize;

/// A file named by the hash of its bytes.
#[derive(Debug, PartialEq, Serialize)]
pub struct FileDigest {
    pub path: String,
    pub sha256: String,
}

/// One file the build produced, what kind of target made it, and how.
#[derive(Debug, PartialEq, Serialize)]
pub struct Artifact {
    pub kind: String,
    pub file: FileDigest,
    pub features: Vec<String>,
    pub compiled: Compiled,
    /// The file's debug information (names and source lines, no code), when
    /// the build made it: the one file inside Apple's `.dSYM` bundle.
    pub debug_info: Option<FileDigest>,
}

/// The compiler that produced the artifacts.
#[derive(Debug, PartialEq, Serialize)]
pub struct Toolchain {
    pub version: String,
    pub host: String,
    pub compiler: FileDigest,
}

/// The project lock file and configuration files that shaped the build.
#[derive(Debug, PartialEq, Serialize)]
pub struct Settings {
    pub lock_file: FileDigest,
    /// Cargo's settings files for this build, deepest first (the project's
    /// own `.cargo/config.toml`, each parent folder's, the home one).
    pub config_files: Vec<FileDigest>,
}

/// Everything needed to know exactly which code was built, and how.
#[derive(Debug, PartialEq, Serialize)]
pub struct BuildRecord {
    pub language: String,
    pub cargo_request: Option<crate::cargo_observation::RequestEvidence>,
    pub artifacts: Vec<Artifact>,
    pub toolchain: Toolchain,
    pub settings: Settings,
    pub environment: Vec<EnvVar>,
    pub native_code: Vec<NativeCode>,
    pub c_toolchain: CToolchain,
    pub os_build: String,
}

/// The adapter's answer: a build, or the reason there is none.
#[derive(Debug, PartialEq, Serialize)]
#[serde(tag = "status", rename_all = "lowercase")]
pub enum Outcome {
    Built(Box<BuildRecord>),
    Blocked { reason: String },
}
