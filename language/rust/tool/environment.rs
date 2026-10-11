// Purpose: records every native environment value that may shape a build.
// Never:   writes arbitrary or credential-like values into the build record.
use crate::build_facts::EnvVar;
use sha2::{
    Digest,
    Sha256, //
};
use std::ffi::OsString;

/// Captures the current environment without panicking on non-Unicode values.
pub fn capture_environment() -> Vec<EnvVar> {
    native_environment(std::env::vars_os())
}

/// Hashes every value and never copies plaintext into the record.
pub fn native_environment(vars: impl Iterator<Item = (OsString, OsString)>) -> Vec<EnvVar> {
    let mut found: Vec<EnvVar> = vars
        .map(|(name, value)| EnvVar {
            value: None,
            value_sha256: hash_value(&crate::native_argument::encoded(&value)),
            name: name.to_str().map(str::to_string),
            name_encoded: crate::native_argument::encoded(&name),
        })
        .collect();
    found.sort_by(|left, right| left.name_encoded.cmp(&right.name_encoded));
    found
}

fn hash_value(value: &[u8]) -> String {
    format!("{:x}", Sha256::digest(value))
}
