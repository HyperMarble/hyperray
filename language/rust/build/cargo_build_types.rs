// Purpose: declares Cargo's reported artifacts and the executed build request.
// Never: invents a file that Cargo did not report.
use crate::build_facts::{
    Compiled,
    NativeCode,
    NotBuilt, //
};
use crate::cargo_observation::RequestEvidence;
use std::path::PathBuf;

/// One file the build made, with every kind Cargo reports for the target
/// that made it, and how.
#[derive(Debug, PartialEq)]
pub struct Built {
    pub kind: Vec<String>,
    pub path: PathBuf,
    /// The features Cargo reports this file was built with.
    pub features: Vec<String>,
    pub compiled: Compiled,
    /// Apple's `.dSYM` debug-info bundle for this file, when Cargo made one.
    pub debug_info: Option<PathBuf>,
}

/// Everything one build reported: the project's own files, any dynamic
/// library a program loads at run time, and the native code any package's
/// build script linked in (dependencies' too).
#[derive(Debug, PartialEq)]
pub struct BuildOutput {
    pub request: Option<RequestEvidence>,
    pub files: Vec<Built>,
    pub native_code: Vec<NativeCode>,
    /// The project's own libraries the build compiled.
    pub libraries: Vec<super::cargo_library::Library>,
    pub not_built: Vec<NotBuilt>,
}
