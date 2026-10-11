// Purpose: records an OS argument without losing its native encoding.
// Never: replaces non-Unicode bytes with display substitutions.
use serde::Serialize;
use std::ffi::OsStr;

#[derive(Clone, Debug, PartialEq, Serialize)]
pub struct NativeArgument {
    pub text: Option<String>,
    pub encoded: Vec<u8>,
}

impl NativeArgument {
    pub fn of(value: &OsStr) -> Self {
        let text = value.to_str().map(str::to_string);
        let encoded = encoded(value);
        Self { text, encoded }
    }
}

/// The value's own bytes, as the operating system holds them.
pub fn encoded(value: &OsStr) -> Vec<u8> {
    #[cfg(unix)]
    return std::os::unix::ffi::OsStrExt::as_bytes(value).to_vec();
    #[cfg(not(unix))]
    value.as_encoded_bytes().to_vec()
}
