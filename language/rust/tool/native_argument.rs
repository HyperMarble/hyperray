// Purpose: records an OS argument without losing its native encoding.
// Never: replaces non-Unicode bytes with display substitutions.
use serde::{
    Deserialize,
    Serialize, //
};
use std::ffi::{
    OsStr,
    OsString, //
};
#[cfg(unix)]
use std::os::unix::ffi::{
    OsStrExt,
    OsStringExt, //
};
#[cfg(windows)]
use std::os::windows::ffi::{
    OsStrExt,
    OsStringExt, //
};

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
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

    pub fn to_os_string(&self) -> Option<OsString> {
        #[cfg(unix)]
        return Some(OsString::from_vec(self.encoded.clone()));
        #[cfg(windows)]
        return from_windows_bytes(&self.encoded);
        #[cfg(not(any(unix, windows)))]
        self.text.as_ref().map(OsString::from)
    }
}

pub fn encoded(value: &OsStr) -> Vec<u8> {
    #[cfg(unix)]
    return value.as_bytes().to_vec();
    #[cfg(windows)]
    return value.encode_wide().flat_map(u16::to_le_bytes).collect();
    #[cfg(not(any(unix, windows)))]
    value.as_encoded_bytes().to_vec()
}

#[cfg(windows)]
fn from_windows_bytes(bytes: &[u8]) -> Option<OsString> {
    if !bytes.len().is_multiple_of(2) {
        return None;
    }
    let wide: Vec<u16> = bytes
        .chunks_exact(2)
        .map(|pair| u16::from_le_bytes([pair[0], pair[1]]))
        .collect();
    Some(OsString::from_wide(&wide))
}
