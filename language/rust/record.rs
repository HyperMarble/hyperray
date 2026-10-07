// Purpose: the build record the adapter hands to the loader, as JSON.
// Never:   holds a value that was not read from the build or the machine.
use crate::build_facts::Compiled;
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

/// The project settings that decided how the code was built.
#[derive(Debug, PartialEq, Serialize)]
pub struct Settings {
    pub profile: String,
    pub lock_file: FileDigest,
}

/// Everything needed to know exactly which code was built, and how.
#[derive(Debug, PartialEq, Serialize)]
pub struct BuildRecord {
    pub language: String,
    pub artifacts: Vec<Artifact>,
    pub toolchain: Toolchain,
    pub settings: Settings,
    pub os_build: String,
}

/// The adapter's answer: a build, or the reason there is none.
#[derive(Debug, PartialEq, Serialize)]
#[serde(tag = "status", rename_all = "lowercase")]
pub enum Outcome {
    Built(Box<BuildRecord>),
    Blocked { reason: String },
}
