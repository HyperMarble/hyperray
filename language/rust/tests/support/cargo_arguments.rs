// Purpose: supplies native arguments for the Cargo commands exercised by tests.
// Never: translates settings or chooses flags for the caller.
use language_rust::cargo_request::Arguments;
use std::ffi::OsString;

pub fn for_commands(build: &[&str], metadata: &[&str], doctests: &[&str]) -> Arguments {
    Arguments {
        build: build.iter().map(OsString::from).collect(),
        metadata: metadata.iter().map(OsString::from).collect(),
        doctests: doctests.iter().map(OsString::from).collect(),
    }
}
