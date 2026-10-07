// Purpose: the facts that say how the code was built, beyond the compiler:
//          each file's build settings, the environment, C code built into it,
//          and the C compiler and SDK used for that code.
// Never:   holds a value that was not read from Cargo, Apple's tools, or the
//          environment the build ran in.
use crate::record::FileDigest;
use serde::Serialize;

/// How Cargo compiled one file, as it reports it in its build messages.
#[derive(Debug, PartialEq, Serialize)]
pub struct Compiled {
    pub opt_level: String,
    pub debuginfo: String,
    pub debug_assertions: bool,
    pub overflow_checks: bool,
    pub test: bool,
}

/// One environment variable that can change the build. `value` is `None`
/// when the name looks like a secret, so only the fact that it was set is kept.
#[derive(Debug, PartialEq, Serialize)]
pub struct EnvVar {
    pub name: String,
    pub value: Option<String>,
}

/// Native (non-Rust) code a package's build script asked to link in, such as
/// a C library it compiled, exactly as Cargo reports it.
#[derive(Debug, PartialEq, Serialize)]
pub struct NativeCode {
    pub package: String,
    pub linked_libs: Vec<String>,
}

/// The C compiler that build scripts use by default, and on macOS the SDK.
#[derive(Debug, PartialEq, Serialize)]
pub struct CToolchain {
    pub compiler: FileDigest,
    pub version: String,
    pub sdk_path: Option<String>,
    pub sdk_version: Option<String>,
}
