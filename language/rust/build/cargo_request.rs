// Purpose: passes complete compilation requests to Cargo's own parser.
// Never: maintains an adapter list of accepted compiler or Cargo flags.
use std::ffi::OsString;

/// Native arguments for each Cargo command used to collect the build record.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct Arguments {
    pub build: Vec<OsString>,
    pub metadata: Vec<OsString>,
}
