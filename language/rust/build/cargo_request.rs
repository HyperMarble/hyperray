// Purpose: passes complete compilation requests to Cargo's own parser.
// Never: maintains an adapter list of accepted compiler or Cargo flags.
use std::ffi::OsString;

/// Native arguments for each Cargo command used to collect the build record.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct Arguments {
    pub build: Vec<OsString>,
    pub metadata: Vec<OsString>,
}

/// `--locked` for a Cargo command, unless the owner's arguments already
/// hold it: Cargo refuses the same flag given twice.
pub(crate) fn locked(given: &[OsString]) -> Option<OsString> {
    let absent = !given.iter().any(|argument| argument == "--locked");
    absent.then(|| OsString::from("--locked"))
}
